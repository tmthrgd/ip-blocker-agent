// Created by cgo -godefs - DO NOT EDIT
// cgo -godefs blocker.go

package blocker

type mutex struct {
	Sem [32]byte
}

type rwLock struct {
	W           mutex
	WriterSem   [32]byte
	ReaderSem   [32]byte
	ReaderCount int32
	ReaderWait  int32
}

type ipBlock struct {
	Base uint64
	Len  uint64
}

type shmHeader struct {
	Version  uint32
	Revision uint32
	Lock     rwLock
	IP4      ipBlock
	IP6      ipBlock
	IP6Route ipBlock
}

func (h *shmHeader) rwLocker() *rwLock {
	return &h.Lock
}

func (h *shmHeader) setBlocks(ip4, ip4len, ip6, ip6len, ip6r, ip6rlen int) {
	h.IP4.Base = uint64(ip4)
	h.IP4.Len = uint64(ip4len)

	h.IP6.Base = uint64(ip6)
	h.IP6.Len = uint64(ip6len)

	h.IP6Route.Base = uint64(ip6r)
	h.IP6Route.Len = uint64(ip6rlen)
}

const (
	headerSize = 0xa0

	rwLockMaxReaders = 0x40000000

	version = uint32((^uint(0)>>32)&0x80000000) | 0x00000001
)
