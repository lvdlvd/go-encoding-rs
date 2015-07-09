// Copyright 2012 Google Inc. All Rights Reserved.
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

package rs

import (
	"bytes"
	"testing"
)

func TestErasureCoder(t *testing.T) {
	var in = [][]byte{
		[]byte{1, 2, 3, 4, 5},
		[]byte{41, 42, 43, 44, 45},
		[]byte{11, 22, 33, 44, 55},
	}

	// an encoder that encodes 3 blocks into the 3 originals plus 2 code blocks.
	// Normally you wouldn't ask for the originals, and just specify []byte{3,4}
	c := NewErasureCoder([]byte{0, 1, 2}, []byte{0, 1, 2, 3, 4})

	if c.Degree() != 3 {
		t.Error("ErasureCoder has wrong degree ", c.Degree(), " != 3")
	}

	if c.NumOutputs() != 5 {
		t.Error("ErasureCoder has wrong degree ", c.NumOutputs(), " != 5")
	}

	out := c.Code(in)

	// Check that 0,1,2 are identical to input.
	for i := 0; i < 3; i++ {
		if !bytes.Equal(in[i], out[i]) {
			t.Error(in[i], " != ", out[i])
		}
	}

	// update input[1]
	in_updated := []byte{51, 62, 73, 84, 95}
	delta := make([]byte, len(in_updated))
	for i := range in[1] {
		delta[i] = in[1][i] ^ in_updated[i]
	}
	in[1] = in_updated

	c.Update(1, delta, out)

	// Check that 0,1,2 are still identical to input.
	for i := 0; i < 3; i++ {
		if !bytes.Equal(in[i], out[i]) {
			t.Error(in[i], " != ", out[i])
		}
	}

	// Reconstruct 1 and 2 from 0 and the two code blocks
	c2 := NewErasureCoder([]byte{0, 3, 4}, []byte{1, 2})
	var in2 = [][]byte{out[0], out[3], out[4]}
	out2 := c2.Code(in2)

	if !bytes.Equal(out2[0], in[1]) {
		t.Error(out2[0], " != ", in[1])
	}

	if !bytes.Equal(out2[1], in[2]) {
		t.Error(out2[1], " != ", in[2])
	}

}

// For testing the panic test, change !ok to ok below
func recoverExpected(t *testing.T) {
	e := recover()
	_, ok := e.(error)
	if !ok {
		t.Error(e)
	}
}

func TestCodePanicOnBadInput(t *testing.T) {
	defer recoverExpected(t)
	c := NewErasureCoder([]byte{0, 1, 2}, []byte{0, 1, 2, 3, 4})
	c.Code([][]byte{[]byte{1}}) // should panic
	t.Error("Failed to panic")
}

func TestCodePanicOnRaggedInput(t *testing.T) {
	defer recoverExpected(t)
	c := NewErasureCoder([]byte{0, 1, 2}, []byte{0, 1, 2, 3, 4})
	c.Code([][]byte{[]byte{1, 2}, []byte{1}, []byte{1}}) // should panic
	t.Error("Failed to panic")
}

func TestUpdatePanicOnBadIndex(t *testing.T) {
	defer recoverExpected(t)
	c := NewErasureCoder([]byte{0, 1, 2}, []byte{0, 1, 2, 3, 4})
	out := [][]byte{[]byte{0}, []byte{0}, []byte{0}, []byte{0}, []byte{0}}
	c.Update(3, []byte{1}, out) // should panic
	t.Error("Failed to panic")
}

func TestUpdatePanicOnRaggedInput(t *testing.T) {
	defer recoverExpected(t)
	c := NewErasureCoder([]byte{0, 1, 2}, []byte{0, 1, 2, 3, 4})
	out := [][]byte{[]byte{0}, []byte{0}, []byte{0, 2}, []byte{0}, []byte{0}}
	c.Update(0, []byte{1}, out) // should panic
	t.Error("Failed to panic")
}

func TestUpdatePanicOnBadInput(t *testing.T) {
	defer recoverExpected(t)
	c := NewErasureCoder([]byte{0, 1, 2}, []byte{0, 1, 2, 3, 4})
	out := [][]byte{[]byte{0}, []byte{0}, []byte{0}, []byte{0}}
	c.Update(0, []byte{1}, out) // should panic
	t.Error("Failed to panic")
}
