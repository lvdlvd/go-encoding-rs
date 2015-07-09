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

/*
 Simple commandline utility for Reed-Solomon encoding.

 This Program Reed-Solomon Encodes/Decodes the named infiles to the
 named outfiles by constructing a polynomial in GF(2^8) that
 interpolates through the bytes of infile[i] at the abscissa listed as
 the i'th value to the -i flag. The degree of the polynomial is equal
 to the number of input files which must be equal to the number of
 listed infiles.

 This polynomial is then evaluated at the abscissae listed in the -o
 parameter to produce each of the files named as ofiles.

 On output all files will be padded with zero bytes to the lenght of
 the longest input file.

 [TODO flags currently requires all flags come before all files.
  better do my own parsing. the examples below are off.]

 Example use:
     rsc -i 0,1,2 foo0.org foo1.org foo2.org -o 3,4,5 foo.rs3 foo.rs4 foo.rs5

 This produces foo.rs[3..5] from the originals foo[0..2].

 Now as long as you have any 3 of the total set of 6, you can
 reconstruct the other three. e.g.:

     rsc -i 0,3,5 foo0.org foo.rs3 foo.rs5  -o 1 foo1.org

 Note that the output may be longer than the original foo1.org,
 because of padding, so you may have to keep track of the original lengths
 if your fileformat does not cope with that gracefully.  You also have
 to keep track of the order of the polynomial used, eg, the number of
 inputs, and which abscissa each file belongs to. (TODO(lvd), read/write
 a toc on stdin/out to keep track of this)

 You can also use any 3 to construct a new one that can be used to
 decode instead of any other, e.g.:

     rsc -i 0,3,5 foo0.org foo.rs3 foo.rs5  -o 7 foo.rs7

     rsc -i 0,3,7 foo0.org foo.rs3 foo.rs7  -o 2 foo2.org

*/
package main

import (
	"github.com/lvdlvd/go-encoding-rs"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

//var kUsage = "Usage: %s -i 0,1... infile0 infile1... -o 3,4... ofile3 ofile4..."
const kUsage = "Usage: %s -i 0,1... -o 3,4...  infile0 infile1... ofile3 ofile4...\n"

func usage(msg ...interface{}) {
	if len(msg) > 0 {
		fmt.Fprintln(os.Stderr, msg...)
	}
	fmt.Fprintf(os.Stderr, kUsage, os.Args[0])
	os.Exit(1)
}

func crash(msg ...interface{}) {
	if len(msg) > 0 {
		fmt.Fprintln(os.Stderr, msg...)
	}
	os.Exit(1)
}

// -----------------------------------------------------------------------------
//   Flags of type []byte, parsed from comma separated string
// -----------------------------------------------------------------------------
type byteArrayFlag struct {
	values []byte
}

func (p *byteArrayFlag) String() string {
	vals := make([]string, len(p.values))
	for i, v := range p.values {
		vals[i] = strconv.Itoa(int(v))
	}
	return strings.Join(vals, ",")
}

func (p *byteArrayFlag) Set(s string) error {
	if len(s) == 0 {
		return nil
	}
	p.values = make([]byte, strings.Count(s, ",")+1)
	for i, v := range strings.SplitN(s, ",", -1) {
		b, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		p.values[i] = byte(b)
	}
	return nil
}
// -----------------------------------------------------------------------------

func main() {

	var idx_in, idx_out byteArrayFlag

	// TODO flags currently requires all flags come before all files.  better do my own parsing
	flag.Var(&idx_in, "i", "")
	flag.Var(&idx_out, "o", "")
	flag.Usage = func() { usage("Error parsing flags.") }
	flag.Parse()

	if len(idx_in.values) == 0 || len(idx_out.values) == 0 {
		usage("Please specify both input and output abscissae -i <byte>,... and -o <byte>,...")
	}

	if len(flag.Args()) != len(idx_in.values)+len(idx_out.values) {
		usage("Please specify as many input and output files as values to -i and -o.")
	}

	in_files := make([]*os.File, len(idx_in.values))

	for i, _ := range in_files {
		f, err := os.Open(flag.Arg(i))
		if err != nil {
			crash("could not open ", flag.Arg(i), " for reading:", err)
		}
		in_files[i] = f
	}

	out_files := make([]*os.File, len(idx_out.values))

	for i, _ := range out_files {
		const O_OUTPUT = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
		f, err := os.OpenFile(flag.Arg(i+len(in_files)), O_OUTPUT, 0644)
		if err != nil {
			crash("could not open ", flag.Arg(i+len(in_files)), " for writing:", err)
		}
		out_files[i] = f
	}

	coder := rs.NewErasureCoder(idx_in.values, idx_out.values)

	const kBlocksize = 1024 << 7 // 128k

	for {
		in := make([][]byte, len(idx_in.values))
		max_n := 0
		all_closed := true
		for i, f := range in_files {
			in[i] = make([]byte, kBlocksize)
			if f == nil {
				continue
			}
			n, err := f.Read(in[i])
			if err == nil {
				all_closed = false
			} else if err == io.EOF {
				f.Close()
				in_files[i] = nil
			} else {
				crash("Error reading from ", flag.Arg(i), ": ", err)
			}
			if max_n < n {
				max_n = n
			}
		}

		if max_n == 0 {
			break
		} else if max_n < kBlocksize {
			for i := range in {
				in[i] = in[i][0:max_n]
			}
		}

		out := coder.Code(in)

		for i, f := range out_files {
			if _, err := f.Write(out[i]); err != nil {
				crash("Error writing to ", flag.Arg(i+len(in_files)), ": ", err)
			}
		}

		if all_closed {
			break
		}
	}

	for i, f := range out_files {
		if err := f.Close(); err != nil {
			crash("Error closing ", flag.Arg(i+len(in_files)), ": ", err)
		}
	}
}
