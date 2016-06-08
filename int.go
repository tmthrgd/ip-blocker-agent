// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import "encoding/binary"

func incrBytes(b []byte) {
	for j := len(b) - 1; j >= 0; j-- {
		b[j]++

		if b[j] > 0 {
			break
		}
	}
}

func addIntToBytes(x []byte, y int) (overflow bool) {
	for i := len(x) - 1; i >= 0 && y > 0; i-- {
		c := int(x[i]) + (y & 0xff)
		y >>= 8

		if c >= 1<<8 {
			y++

			c -= 1 << 8
		}

		x[i] = byte(c)
	}

	return y != 0
}

func subBytes32(x, y []byte) int {
	const maxInt = int(^uint(0) >> 1)

	if len(x) != len(y) {
		panic("different lengths")
	}

	l := len(x)
	var size int

	switch {
	case l >= 8:
		size = 4
	case l > 4:
		panic("invalid length")
	case l == 4:
		size = 4
	case l == 3:
		panic("invalid length")
	case l == 2:
		size = 2
	case l == 1:
		size = 1
	default:
		return 0
	}

	l -= size

	for i := 0; i < l; i++ {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return 0
			}

			return maxInt
		}
	}

	var a, b uint32

	switch size {
	case 4:
		a = binary.BigEndian.Uint32(x[l:])
		b = binary.BigEndian.Uint32(y[l:])
	case 2:
		a = uint32(binary.BigEndian.Uint16(x[l:]))
		b = uint32(binary.BigEndian.Uint16(y[l:]))
	case 1:
		a = uint32(x[l])
		b = uint32(y[l])
	}

	if a <= b {
		return 0
	}

	c := a - b

	if uint(c) >= uint(maxInt) {
		return maxInt
	}

	return int(c)
}

func subBytes64(x, y []byte) int {
	const maxInt = int(^uint(0) >> 1)

	if len(x) != len(y) {
		panic("different lengths")
	}

	l := len(x)

	switch {
	case l >= 8:
	case l > 4:
		panic("invalid length")
	default:
		return subBytes32(x, y)
	}

	l -= 8

	for i := 0; i < l; i++ {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return 0
			}

			return maxInt
		}
	}

	a := binary.BigEndian.Uint64(x[l:])
	b := binary.BigEndian.Uint64(y[l:])

	if a <= b {
		return 0
	}

	c := a - b

	if uint(c) >= uint(maxInt) {
		return maxInt
	}

	return int(c)
}

var subBytes func(x, y []byte) int

func init() {
	if ^uint(0) == uint(^uint32(0)) {
		subBytes = subBytes32
	} else {
		subBytes = subBytes64
	}
}
