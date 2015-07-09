// Copyright 2011 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package rs implements Reed-Solomon Encoding and Decoding of erasure codes
// using the Galois Group GF(2^8) with characteristic polynomial x^8 + x^4 + x^3 + x^2 + 1.
package rs

import "fmt"

// ------------------------------------------------------------------------------
// the Galois Group GG(2^8) with characteristic polynomial x^8 + x^4 + x^3 + x^2 + 1
const (
	cp_84320 = 1<<8 | 1<<4 | 1<<3 | 1<<2 | 1<<0
)

// multiply the hard way, only used for testing.
func galois_multiply(aa, bb uint8) uint8 {
	var (
		a uint16 = uint16(aa)
		b uint16 = uint16(bb)
		c uint16 = cp_84320 << 7
		p uint16 = 0
	)

	// multiplication in Z_2
	for ; a != 0; a >>= 1 {
		if a&1 != 0 {
			p ^= b
		}
		b <<= 1
	}

	// Reducing the product modulo c
	for i := 1 << 15; i >= 1<<8; i >>= 1 {
		if p&uint16(i) != 0 {
			p ^= c
		}
		c >>= 1
	}

	return uint8(p)
}

var (
	exp [255]uint8
	log [256]uint8
	inv [256]uint8
)

func init() {
	var a uint8 = 1
	for i := range exp {
		exp[i] = a
		log[a] = uint8(i)
		a = galois_multiply(a, 2)
	}

	inv[0] = 0
	inv[1] = 1
	for i := 2; i < 256; i++ {
		var idx int = 255 - int(log[i])
		inv[i] = exp[idx]
	}
}

func mult(a, b uint8) uint8 {
	if a == 0 || b == 0 {
		return 0
	}

	var idx int = int(log[a]) + int(log[b])
	// Go's % on signed types preserves sign, do it by hand.
	if idx >= 255 {
		idx -= 255
	}
	return exp[idx]
}

// ------------------------------------------------------------------------------

type ErasureCoder struct {
	interp [][]uint8 // the Lagrange interpolation factors
}

// Construct an empty X x Y matrix out of slices.
func makeMatrix(x, y int) (out [][]uint8) {
	out = make([][]uint8, x)
	for i := range out {
		out[i] = make([]uint8, y)
	}
	return
}

// Compute the Langrange interpolation factor \prod k!=i (x - x_i) / (x_k - x_i).
func lagrange(in_x []uint8, i int, xj uint8) (r uint8) {
	r = 1
	for k, xk := range in_x {
		if k == i {
			continue
		}
		f := mult(xj^xk, inv[in_x[i]^xk])
		r = mult(r, f)
	}
	return
}

// NewErasureCoder creates a de/encoder that can compute P(out_x[]) from P(in_x[])
// The polynomial P is of degree len(in_x), and P(in_x[i]) = d[i]
// for inputs d[].
func NewErasureCoder(in_x, out_x []uint8) (p *ErasureCoder) {
	p = new(ErasureCoder)
	p.interp = makeMatrix(len(in_x), len(out_x))
	for i := range in_x {
		for j := range out_x {
			p.interp[i][j] = lagrange(in_x, i, out_x[j])
		}
	}
	return
}

// Return the degree of the computed polynomial, which is equal to the number of inputs.
func (p *ErasureCoder) Degree() int {
	return len(p.interp)
}

// Return the number of outputs the ErasureCoder will compute.
func (p *ErasureCoder) NumOutputs() int {
	return len(p.interp[0])
}

// De/Encode in[] to out[] by recovering the polynomial and evaluating
// at the out_x abscissae.  The out[] will have as many elements as
// the out_x array passed to NewErasureCoder.  All rows of the in[]
// matrix must be of equal size, and the rows of out[] will be of this
// size too. The in[] matrix must have the same number of columns as
// the degree of the Erasurecoder.  (Since these preconditions can be
// checked by the user, i've chosen to panic() rather than return an
// error variable if they are not satisfied.)
func (p *ErasureCoder) Code(in [][]uint8) (out [][]uint8) {
	if len(in) != p.Degree() {
		panic(fmt.Errorf("Wrong number of inputs: %d for Erasure coder of degree: %d", len(in), p.Degree()))
	}

	for i := 0; i < len(in); i++ {
		if len(in[i]) != len(in[0]) {
			panic(fmt.Errorf("Ragged input matrix: [0]%d != [%d]%d  ", len(in[0]), i, len(in[i])))
		}
	}

	out = makeMatrix(len(p.interp[0]), len(in[0]))
	for i := 0; i < len(in); i++ {
		for j := 0; j < len(in[i]); j++ {
			for k := 0; k < len(p.interp[i]); k++ {
				out[k][j] ^= mult(in[i][j], p.interp[i][k])
			}
		}
	}
	return
}

// Update out[][] for an update of the abscissa in_x with values
// in_delta[].  in_delta should be the xor of the original value with
// the update.  the lenght of in_delta and the lenghts of the elements
// of out should all be the same, idx is the index into in_x[] passed
// to NewErasurecoder, and out should have as many elements as
// out_x[].  Typically out[][] was returned by an earlier call to
// Code().  Alternatively out[][] can be a zero matrix of the right
// dimension, and it can be xor-ed by the caller with an earlier
// output of Code().
func (p *ErasureCoder) Update(idx uint8, in_delta []uint8, out [][]uint8) {
	if idx >= uint8(len(p.interp)) {
		panic(fmt.Errorf("Abscissa index out of range %d for polynomial of degree %d", idx, len(p.interp)))
	}

	if len(out) != len(p.interp[0]) {
		panic(fmt.Errorf("Wrong number of in/outputs: %d != %d", len(out), len(p.interp[0])))
	}

	for i := 0; i < len(out); i++ {
		if len(in_delta) != len(out[i]) {
			panic(fmt.Errorf("Ragged or uneven input matrices: in %d != out[%d]%d  ", len(in_delta), i, len(out[i])))
		}
	}
	for j := 0; j < len(in_delta); j++ {
		for k := 0; k < len(p.interp[idx]); k++ {
			out[k][j] ^= mult(in_delta[j], p.interp[idx][k])
		}
	}
	return
}
