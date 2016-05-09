package main

import (
	"bytes"
	"net"
	"sort"
)

type ipSearcher struct {
	Size int
	IPs  []byte
}

func (p *ipSearcher) Len() int {
	return len(p.IPs) / p.Size
}

func (p *ipSearcher) Less(i, j int) bool {
	return bytes.Compare(p.IPs[i*p.Size:(i+1)*p.Size], p.IPs[j*p.Size:(j+1)*p.Size]) < 0
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

func (p *ipSearcher) ipToByte(ip net.IP) []byte {
	if p.Size == net.IPv4len {
		ip = ip.To4()
	} else {
		ip = ip.To16()
	}

	if ip == nil {
		return nil
	}

	return []byte(ip)
}

func (p *ipSearcher) rawSearch(check []byte) int {
	return sort.Search(p.Len(), func(i int) bool {
		return bytes.Compare(p.IPs[i*p.Size:(i+1)*p.Size], check) >= 0
	})
}

func (p *ipSearcher) Search(ip net.IP) int {
	check := p.ipToByte(ip)
	if check == nil {
		return len(p.IPs)
	}

	return p.rawSearch(check)
}

func (p *ipSearcher) Contains(ip net.IP) bool {
	check := p.ipToByte(ip)
	if check == nil {
		return false
	}

	pos := p.rawSearch(check)
	return pos*p.Size < len(p.IPs) && bytes.Equal(check, p.IPs[pos*p.Size:(pos+1)*p.Size])
}

func (p *ipSearcher) UnsortedInsert(ip net.IP) bool {
	b := p.ipToByte(ip)
	if b == nil {
		return false
	}

	p.IPs = append(p.IPs, b...)
	return true
}

func (p *ipSearcher) UnsortedInsertMany(ips []byte) {
	p.IPs = append(p.IPs, ips...)
}

func (p *ipSearcher) Insert(ip net.IP) bool {
	b := p.ipToByte(ip)
	if b == nil {
		return false
	}

	pos := p.rawSearch(b)
	if pos*p.Size < len(p.IPs) && bytes.Equal(b, p.IPs[pos*p.Size:(pos+1)*p.Size]) {
		return false
	}

	p.IPs = append(p.IPs, b...)
	copy(p.IPs[(pos+1)*p.Size:], p.IPs[pos*p.Size:])
	copy(p.IPs[pos*p.Size:], b)
	return true
}

func (p *ipSearcher) Remove(ip net.IP) bool {
	b := p.ipToByte(ip)
	if b == nil {
		return false
	}

	pos := p.rawSearch(b)
	if pos*p.Size >= len(p.IPs) || !bytes.Equal(b, p.IPs[pos*p.Size:(pos+1)*p.Size]) {
		return false
	}

	p.IPs = append(p.IPs[:pos*p.Size], p.IPs[(pos+1)*p.Size:]...)
	return true
}

func (p *ipSearcher) Clear() {
	p.IPs = nil
}
