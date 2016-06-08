// Created by cgo -godefs - DO NOT EDIT
// cgo -godefs blocker.go

package blocker

import "unsafe"

type mutex struct {
	Sem [16]byte
}

type rwLock struct {
	W           mutex
	WriterSem   [16]byte
	ReaderSem   [16]byte
	ReaderCount int32
	ReaderWait  int32
}

type ipBlock struct {
	Base uint32
	Len  uint32
}

type shmHeader struct {
	Version  uint32
	Revision uint32
	Lock     rwLock
	IP4      ipBlock
	IP6      ipBlock
	IP6Route ipBlock
}

func castToHeader(data *byte) *shmHeader {
	return (*shmHeader)(unsafe.Pointer(data))
}

func (h *shmHeader) setBlocks(ip4, ip4len, ip6, ip6len, ip6r, ip6rlen int) {
	h.IP4.Base = uint32(ip4)
	h.IP4.Len = uint32(ip4len)

	h.IP6.Base = uint32(ip6)
	h.IP6.Len = uint32(ip6len)

	h.IP6Route.Base = uint32(ip6r)
	h.IP6Route.Len = uint32(ip6rlen)
}

const (
	headerSize = 0x58

	rwLockMaxReaders = 0x40000000

	version = uint32((^uint(0)>>32)&0x80000000) | 0x00000001
)
