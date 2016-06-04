package blocker

import (
	"bytes"
	"sort"
)

type ipSearcher struct {
	Size int
	IPs  []byte

	Compare func(a, b []byte) int

	buffer []byte
}

func newIPSearcher(size int, compare func(a, b []byte) int) *ipSearcher {
	if compare == nil {
		compare = bytes.Compare
	}

	return &ipSearcher{
		Size: size,

		Compare: compare,

		buffer: make([]byte, size),
	}
}

func (p *ipSearcher) Len() int {
	return len(p.IPs) / p.Size
}

func (p *ipSearcher) Less(i, j int) bool {
	return p.Compare(p.IPs[i*p.Size:(i+1)*p.Size], p.IPs[j*p.Size:(j+1)*p.Size]) < 0
}

func (p *ipSearcher) Swap(i, j int) {
	copy(p.buffer, p.IPs[i*p.Size:(i+1)*p.Size])
	copy(p.IPs[i*p.Size:(i+1)*p.Size], p.IPs[j*p.Size:(j+1)*p.Size])
	copy(p.IPs[j*p.Size:(j+1)*p.Size], p.buffer)
}

func (p *ipSearcher) Sort() {
	sort.Sort(p)
}

func (p *ipSearcher) Index(check []byte) int {
	if len(check) != p.Size {
		panic("invalid size")
	}

	return sort.Search(p.Len(), func(i int) bool {
		return p.Compare(p.IPs[i*p.Size:(i+1)*p.Size], check) >= 0
	})
}

func (p *ipSearcher) search(check []byte) (pos int, has bool) {
	pos = p.Index(check)
	has = pos*p.Size < len(p.IPs) && p.Compare(check, p.IPs[pos*p.Size:(pos+1)*p.Size]) == 0
	return
}

func (p *ipSearcher) Search(check []byte) []byte {
	pos, has := p.search(check)
	if has {
		return p.IPs[pos*p.Size : (pos+1)*p.Size]
	}

	return nil
}

func (p *ipSearcher) Contains(check []byte) bool {
	_, has := p.search(check)
	return has
}

func (p *ipSearcher) Insert(b []byte) bool {
	pos, has := p.search(b)
	if has {
		return false
	}

	p.IPs = append(p.IPs, b...)
	copy(p.IPs[(pos+1)*p.Size:], p.IPs[pos*p.Size:])
	copy(p.IPs[pos*p.Size:(pos+1)*p.Size], b)
	return true
}

func (p *ipSearcher) Replace(b []byte) bool {
	if pos, has := p.search(b); has {
		copy(p.IPs[pos*p.Size:(pos+1)*p.Size], b)
		return true
	}

	return false
}

func (p *ipSearcher) InsertOrReplace(b []byte) {
	pos, has := p.search(b)
	if has {
		copy(p.IPs[pos*p.Size:(pos+1)*p.Size], b)
		return
	}

	p.IPs = append(p.IPs, b...)
	copy(p.IPs[(pos+1)*p.Size:], p.IPs[pos*p.Size:])
	copy(p.IPs[pos*p.Size:(pos+1)*p.Size], b)
}

func (p *ipSearcher) Remove(b []byte) bool {
	pos, has := p.search(b)
	if has {
		p.IPs = append(p.IPs[:pos*p.Size], p.IPs[(pos+1)*p.Size:]...)
		return true
	}

	return false
}

func (p *ipSearcher) Clear() {
	if cap(p.IPs) <= int(pageSize) {
		p.IPs = p.IPs[:0]
	} else {
		p.IPs = nil
	}
}
