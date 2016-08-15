// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build amd64,!gccgo,!appengine

package incr

func IncrementBytes(base, data []byte) {
	if len(data) == 0 {
		return
	}

	switch len(base) {
	case 4:
		if len(data)&0x03 != 0 {
			panic("invalid data length")
		}

		incrementBytes4Asm(&base[0], &data[0], uint64(len(data)))
	case 8:
		if len(data)&0x07 != 0 {
			panic("invalid data length")
		}

		incrementBytes8Asm(&base[0], &data[0], uint64(len(data)))
	case 16:
		if len(data)&0x0f != 0 {
			panic("invalid data length")
		}

		incrementBytes16Asm(&base[0], &data[0], uint64(len(data)))
	default:
		incrementBytesFallback(base, data)
	}
}

//go:generate go run asm_gen.go

// This function is implemented in incr4_amd64.s
//go:noescape
func incrementBytes4Asm(base *byte, data *byte, len uint64)

// This function is implemented in incr8_amd64.s
//go:noescape
func incrementBytes8Asm(base *byte, data *byte, len uint64)

// This function is implemented in incr16_amd64.s
//go:noescape
func incrementBytes16Asm(base *byte, data *byte, len uint64)
