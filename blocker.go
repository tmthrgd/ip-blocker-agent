// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import "unsafe"

type ipBlock struct {
	base  uint64
	len   uint64
	locks uint64
}

type shmHeader struct {
	version uint32

	ip4, ip6, ip6Route uint32 // The smallest atomic is 32-bits.

	blocks [4]ipBlock
}

func castToHeader(data []byte) *shmHeader {
	return (*shmHeader)(unsafe.Pointer(&data[0]))
}

const (
	headerSize = unsafe.Sizeof(shmHeader{})

	version = 0x00000003
)
