package main

/*
#cgo LDFLAGS: -lrt

#include <stdlib.h>          // For free
#include <fcntl.h>           // For O_* constants
#include <sys/stat.h>        // For mode constants
#include <sys/mman.h>        // For shm_*

#include "ngx_ip_blocker_shm.h"
*/
import "C"

import (
	"bytes"
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"sort"
	"sync"
	"unsafe"
)

const (
	ngx_cacheline_size = 64

	headerSize = unsafe.Sizeof(C.ngx_ip_blocker_shm_st{})
)

/* taken from ngx_config.h */
func ngx_align(d, a uintptr) uintptr {
	return (d + (a - 1)) &^ (a - 1)
}

type ipSearcher struct {
	ips  []byte
	Size int
}

func (p *ipSearcher) Len() int {
	return len(p.ips) / p.Size
}

func (p *ipSearcher) Less(i, j int) bool {
	return bytes.Compare(p.ips[i*p.Size:(i+1)*p.Size], p.ips[j*p.Size:(j+1)*p.Size]) < 0
}

func (p *ipSearcher) Swap(i, j int) {
	var tmp [net.IPv6len]byte

	copy(tmp[:], p.ips[i*p.Size:(i+1)*p.Size])
	copy(p.ips[i*p.Size:(i+1)*p.Size], p.ips[j*p.Size:(j+1)*p.Size])
	copy(p.ips[j*p.Size:(j+1)*p.Size], tmp[:])
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++

		if ip[j] > 0 {
			break
		}
	}
}

func main() {
	var name string
	flag.StringVar(&name, "name", "/ngx-ip-blocker", "the shared memory name")

	var unlink bool
	flag.BoolVar(&unlink, "unlink", false, "unlink shared memory")

	flag.Parse()

	if len(name) == 0 {
		fmt.Println("-name cannot be empty")
		os.Exit(1)
	}

	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	if unlink {
		_, err := C.shm_unlink(nameC)
		if err != nil {
			panic(err)
		}

		return
	}

	fd, err := C.shm_open(nameC, C.O_CREAT|C.O_EXCL|C.O_TRUNC|C.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}

	defer unix.Close(int(fd))

	var wg sync.WaitGroup
	var mut sync.Mutex

	var blockedIP4s []byte
	var blockedIP6s []byte

	for _, block := range [...]string{
		/* boradcast; RFC 1700 */
		//"0.0.0.0/8", /* too big */

		/* link-local addresses; RFC 3927 */
		"169.254.0.0/16",

		/* TEST-NET, TEST-NET2, TEST-NET3; RFC 5737 */
		"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24",

		/* multicast; RFC 5771 */
		//"224.0.0.0/4", /* too big */

		/* "limited broadcast"; RFC 6890 */
		"255.255.255.255/32",

		/* Unspecified address */
		"::/128",

		/* Discard prefix; RFC 6666 */
		//"100::/64", /* 2^64; too big */

		/* documentation */
		//"2001:db8::/32", /* 2^96; too big */
	} {
		wg.Add(1)

		go func(block string) {
			defer wg.Done()

			ip, ipnet, err := net.ParseCIDR(block)
			if err != nil {
				panic(err)
			}

			ip = ip.Mask(ipnet.Mask)

			if ip4 := ip.To4(); ip4 != nil {
				ip = ip4
			} else {
				ip = ip.To16()
			}

			var ips []byte

			for ; ipnet.Contains(ip); incIP(ip) {
				ips = append(ips, ip...)
			}

			mut.Lock()
			if len(ip) == net.IPv4len {
				blockedIP4s = append(blockedIP4s, ips...)
			} else {
				blockedIP6s = append(blockedIP6s, ips...)
			}
			mut.Unlock()
		}(block)
	}

	wg.Wait()

	ip4s := &ipSearcher{blockedIP4s, net.IPv4len}
	ip6s := &ipSearcher{blockedIP6s, net.IPv6len}

	wg.Add(1)
	go func() {
		sort.Sort(ip4s)
		wg.Done()
	}()

	sort.Sort(ip6s)
	wg.Wait()

	ip4BasePos := ngx_align(headerSize, ngx_cacheline_size)
	ip6BasePos := ngx_align(ip4BasePos+uintptr(len(blockedIP4s)), ngx_cacheline_size)
	size := ngx_align(ip6BasePos+uintptr(len(blockedIP6s)), ngx_cacheline_size)

	if err = unix.Ftruncate(int(fd), int64(size)); err != nil {
		panic(err)
	}

	addr, err := C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, fd, 0)
	if err != nil {
		panic(err)
	}

	defer func() {
		if _, err := C.munmap(addr, C.size_t(size)); err != nil {
			panic(err)
		}
	}()

	header := (*C.ngx_ip_blocker_shm_st)(addr)

	header.ip4.base = C.size_t(ip4BasePos)
	header.ip4.len = C.size_t(len(blockedIP4s))

	header.ip6.base = C.size_t(ip6BasePos)
	header.ip6.len = C.size_t(len(blockedIP6s))

	fmt.Printf("mapped %d bytes to %x\n", size, addr)
	fmt.Printf("\tIP4 of %d bytes (%d entries) mapped to %x\n", header.ip4.len, ip4s.Len(), uintptr(addr)+ip4BasePos)
	fmt.Printf("\tIP6 of %d bytes (%d entries) mapped to %x\n", header.ip6.len, ip6s.Len(), uintptr(addr)+ip6BasePos)

	ip4Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos))
	copy(ip4Base[:header.ip4.len:ip6BasePos-ip4BasePos], blockedIP4s)

	ip6Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos))
	copy(ip6Base[:header.ip6.len:size-ip6BasePos], blockedIP6s)
}
