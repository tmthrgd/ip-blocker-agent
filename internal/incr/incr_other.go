// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build !amd64 gccgo appengine

package incr

import "encoding/binary"

func incrBytesIn4Encoding(base, data []byte) {
	baseUint := binary.BigEndian.Uint32(base)

	for i := 0; i < len(data); i, baseUint = i+4, baseUint+1 {
		binary.BigEndian.PutUint32(data[i:], baseUint)
	}
}

func incrBytesIn8Encoding(base, data []byte) {
	baseUint := binary.BigEndian.Uint64(base)

	for i := 0; i < len(data); i, baseUint = i+8, baseUint+1 {
		binary.BigEndian.PutUint64(data[i:], baseUint)
	}
}

func incrBytesIn16Encoding(base, data []byte) {
	baseHigh := binary.BigEndian.Uint64(base)
	baseLow := binary.BigEndian.Uint64(base[8:])

	for i := 0; i < len(data); i += 16 {
		binary.BigEndian.PutUint64(data[i:], baseHigh)
		binary.BigEndian.PutUint64(data[i+8:], baseLow)

		if baseLow == ^uint64(0) {
			baseLow = 0
			baseHigh++
		} else {
			baseLow++
		}
	}
}

func IncrementBytes(base, data []byte) {
	switch len(base) {
	case 4:
		if len(data)&0x03 != 0 {
			panic("invalid data length")
		}

		incrBytesIn4Encoding(base, data)
	case 8:
		if len(data)&0x07 != 0 {
			panic("invalid data length")
		}

		incrBytesIn8Encoding(base, data)
	case 16:
		if len(data)&0x0f != 0 {
			panic("invalid data length")
		}

		incrBytesIn16Encoding(base, data)
	default:
		incrementBytesFallback(base, data)
	}
}
