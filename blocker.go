// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import "unsafe"

type blockHeader struct {
	len   uint64
	locks uint64
}

type ipBlock struct {
	base uint64
}

type shmHeader struct {
	version uint32

	ip4, ip6, ip6Route ipBlock
}

func castToHeader(data []byte) *shmHeader {
	return (*shmHeader)(unsafe.Pointer(&data[0]))
}

func caseToBlockHeader(data []byte) *blockHeader {
	return (*blockHeader)(unsafe.Pointer(&data[0]))
}

const (
	headerSize      = unsafe.Sizeof(shmHeader{})
	blockHeaderSize = unsafe.Sizeof(blockHeader{})

	version = 0x00000002
)
