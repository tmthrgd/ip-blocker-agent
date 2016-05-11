package main

import (
	"bytes"
	"net"
	"os"
	"sort"
)

type ipSearcher struct {
	Size int
	IPs  []byte

	Compare func(a, b []byte) int
}

func newIPSearcher(size int, compare func(a, b []byte) int) *ipSearcher {
	if compare == nil {
		compare = bytes.Compare
	}

	return &ipSearcher{
		Size: size,

		Compare: compare,
	}
}

func (p *ipSearcher) Len() int {
	return len(p.IPs) / p.Size
}

func (p *ipSearcher) Less(i, j int) bool {
	return p.Compare(p.IPs[i*p.Size:(i+1)*p.Size], p.IPs[j*p.Size:(j+1)*p.Size]) < 0
}

func (p *ipSearcher) Swap(i, j int) {
	var tmp [net.IPv6len]byte

	copy(tmp[:], p.IPs[i*p.Size:(i+1)*p.Size])
	copy(p.IPs[i*p.Size:(i+1)*p.Size], p.IPs[j*p.Size:(j+1)*p.Size])
	copy(p.IPs[j*p.Size:(j+1)*p.Size], tmp[:])
}

func (p *ipSearcher) Sort() {
	sort.Sort(p)
}

func (p *ipSearcher) Index(check []byte) int {
	return sort.Search(p.Len(), func(i int) bool {
		return p.Compare(p.IPs[i*p.Size:(i+1)*p.Size], check) >= 0
	})
}

func (p *ipSearcher) Search(check []byte) []byte {
	pos := p.Index(check)

	if pos*p.Size < len(p.IPs) && p.Compare(check, p.IPs[pos*p.Size:(pos+1)*p.Size]) == 0 {
		return p.IPs[pos*p.Size : (pos+1)*p.Size]
	}

	return nil
}

func (p *ipSearcher) Contains(check []byte) bool {
	return p.Search(check) != nil
}

func (p *ipSearcher) Insert(b []byte) bool {
	pos := p.Index(b)

	if pos*p.Size < len(p.IPs) && p.Compare(b, p.IPs[pos*p.Size:(pos+1)*p.Size]) == 0 {
		return false
	}

	p.IPs = append(p.IPs, b...)
	copy(p.IPs[(pos+1)*p.Size:], p.IPs[pos*p.Size:])
	copy(p.IPs[pos*p.Size:], b)
	return true
}

func (p *ipSearcher) Replace(b []byte) bool {
	pos := p.Index(b)

	if pos*p.Size < len(p.IPs) && p.Compare(b, p.IPs[pos*p.Size:(pos+1)*p.Size]) == 0 {
		copy(p.IPs[pos*p.Size:], b)
		return true
	}

	return false
}

func (p *ipSearcher) InsertOrReplace(b []byte) {
	pos := p.Index(b)

	if pos*p.Size < len(p.IPs) && p.Compare(b, p.IPs[pos*p.Size:(pos+1)*p.Size]) == 0 {
		copy(p.IPs[pos*p.Size:], b)
	} else {
		p.IPs = append(p.IPs, b...)
		copy(p.IPs[(pos+1)*p.Size:], p.IPs[pos*p.Size:])
		copy(p.IPs[pos*p.Size:], b)
	}
}

func (p *ipSearcher) Remove(b []byte) bool {
	pos := p.Index(b)

	if pos*p.Size >= len(p.IPs) || p.Compare(b, p.IPs[pos*p.Size:(pos+1)*p.Size]) != 0 {
		return false
	}

	p.IPs = append(p.IPs[:pos*p.Size], p.IPs[(pos+1)*p.Size:]...)
	return true
}

func (p *ipSearcher) Clear() {
	if cap(p.IPs) <= os.Getpagesize() {
		p.IPs = p.IPs[:0]
	} else {
		p.IPs = nil
	}
}
