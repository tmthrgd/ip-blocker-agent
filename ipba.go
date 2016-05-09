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
	"bufio"
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"sync"
	"sync/atomic"
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

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++

		if ip[j] > 0 {
			break
		}
	}
}

type rwLock C.ngx_ip_blocker_rwlock_st

func (rw *rwLock) Create() {
	if err := sem_init(&rw.writer_sem, true, 0); err != nil {
		panic(err)
	}

	if err := sem_init(&rw.reader_sem, true, 0); err != nil {
		panic(err)
	}

	rw.reader_count = 0
	rw.reader_wait = 0
}

// Lock locks rw for writing.
// If the lock is already locked for reading or writing,
// Lock blocks until the lock is available.
// To ensure that the lock eventually becomes available,
// a blocked Lock call excludes new readers from acquiring
// the lock.
func (rw *rwLock) Lock() {
	// First, resolve competition with other writers.
	//rw.w.Lock()

	// Announce to readers there is a pending writer.
	r := atomic.AddInt32((*int32)(&rw.reader_count), -C.NGX_IP_BLOCKER_MAX_READERS) + C.NGX_IP_BLOCKER_MAX_READERS

	// Wait for active readers.
	if r != 0 && atomic.AddInt32((*int32)(&rw.reader_wait), r) != 0 {
		if err := sem_wait(&rw.writer_sem); err != nil {
			panic(err)
		}
	}
}

// Unlock unlocks rw for writing.  It is a run-time error if rw is
// not locked for writing on entry to Unlock.
//
// As with Mutexes, a locked rwLock is not associated with a particular
// goroutine.  One goroutine may RLock (Lock) an rwLock and then
// arrange for another goroutine to RUnlock (Unlock) it.
func (rw *rwLock) Unlock() {
	// Announce to readers there is no active writer.
	r := atomic.AddInt32((*int32)(&rw.reader_count), C.NGX_IP_BLOCKER_MAX_READERS)
	if r >= C.NGX_IP_BLOCKER_MAX_READERS {
		panic("sync: Unlock of unlocked rwLock")
	}

	// Unblock blocked readers, if any.
	for i := 0; i < int(r); i++ {
		if err := sem_post(&rw.reader_sem); err != nil {
			panic(err)
		}
	}

	// Allow other writers to proceed.
	//rw.w.Unlock()
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

	ip4s := &ipSearcher{net.IPv4len, nil}
	ip6s := &ipSearcher{net.IPv6len, nil}

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
				ip4s.UnsortedInsertMany(ips)
			} else {
				ip6s.UnsortedInsertMany(ips)
			}
			mut.Unlock()
		}(block)
	}

	wg.Wait()

	wg.Add(1)
	go func() {
		ip4s.Sort()
		wg.Done()
	}()

	ip6s.Sort()
	wg.Wait()

	ip4BasePos := ngx_align(headerSize, ngx_cacheline_size)
	ip6BasePos := ngx_align(ip4BasePos+uintptr(len(ip4s.IPs)), ngx_cacheline_size)
	size := ngx_align(ip6BasePos+uintptr(len(ip6s.IPs)), ngx_cacheline_size)

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
	lock := (*rwLock)(&header.lock)

	lock.Create()

	lock.Lock()

	header.ip4.base = C.off_t(ip4BasePos)
	header.ip4.len = C.size_t(len(ip4s.IPs))

	header.ip6.base = C.off_t(ip6BasePos)
	header.ip6.len = C.size_t(len(ip6s.IPs))

	ip4Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos))
	copy(ip4Base[:len(ip4s.IPs):ip6BasePos-ip4BasePos], ip4s.IPs)

	ip6Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos))
	copy(ip6Base[:len(ip6s.IPs):size-ip6BasePos], ip6s.IPs)

	lock.Unlock()

	fmt.Printf("mapped %d bytes to %x\n", size, addr)
	fmt.Printf("\tIP4 of %d bytes (%d entries) mapped to %x\n", header.ip4.len, ip4s.Len(), uintptr(addr)+uintptr(header.ip4.base))
	fmt.Printf("\tIP6 of %d bytes (%d entries) mapped to %x\n", header.ip6.len, ip6s.Len(), uintptr(addr)+uintptr(header.ip6.base))

	stdin := bufio.NewScanner(os.Stdin)

	for stdin.Scan() {
		line := stdin.Text()
		if len(line) == 0 {
			fmt.Printf("invalid input: %s\n", line)
			continue
		}

		switch line[0] {
		case '+':
			fallthrough
		case '-':
			if len(line) <= 1 {
				fmt.Printf("invalid input: %s\n", line)
				continue
			}
		case '!':
			if len(line) != 1 {
				fmt.Printf("invalid input: %s\n", line)
				continue
			}
		default:
			fmt.Printf("invalid operation: %c\n", line[0])
			continue
		}

		if line[0] == '!' {
			ip4s.Clear()
			ip6s.Clear()
		} else {
			ip := net.ParseIP(line[1:])
			if ip == nil {
				fmt.Printf("invalid ip address: %s\n", line[1:])
				continue
			}

			ip4 := ip.To4()

			switch line[0] {
			case '+':
				if ip4 != nil {
					ip4s.Insert(ip4)
				} else {
					ip6s.Insert(ip)
				}
			case '-':
				if ip4 != nil {
					ip4s.Remove(ip4)
				} else {
					ip6s.Remove(ip)
				}
			}
		}

		if _, err := C.munmap(addr, C.size_t(size)); err != nil {
			panic(err)
		}

		ip4BasePos2 := ngx_align(headerSize, ngx_cacheline_size)
		ip6BasePos2 := ngx_align(ip4BasePos2+uintptr(len(ip4s.IPs)), ngx_cacheline_size)
		size2 := ngx_align(ip6BasePos2+uintptr(len(ip6s.IPs)), ngx_cacheline_size)

		if size2 > size {
			size = size2
		}

		ip4BasePos = ngx_align(size, ngx_cacheline_size)
		ip6BasePos = ngx_align(ip4BasePos+uintptr(len(ip4s.IPs)), ngx_cacheline_size)
		size = ngx_align(ip6BasePos+uintptr(len(ip6s.IPs)), ngx_cacheline_size)

		if err = unix.Ftruncate(int(fd), int64(size)); err != nil {
			panic(err)
		}

		addr, err = C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, fd, 0)
		if err != nil {
			panic(err)
		}

		header = (*C.ngx_ip_blocker_shm_st)(addr)
		lock = (*rwLock)(&header.lock)

		ip4Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos))
		copy(ip4Base[:len(ip4s.IPs):ip6BasePos-ip4BasePos], ip4s.IPs)

		ip6Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos))
		copy(ip6Base[:len(ip6s.IPs):size-ip6BasePos], ip6s.IPs)

		lock.Lock()

		header.ip4.base = C.off_t(ip4BasePos)
		header.ip4.len = C.size_t(len(ip4s.IPs))

		header.ip6.base = C.off_t(ip6BasePos)
		header.ip6.len = C.size_t(len(ip6s.IPs))

		lock.Unlock()

		ip4Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos2))
		copy(ip4Base[:len(ip4s.IPs):ip6BasePos2-ip4BasePos2], ip4s.IPs)

		ip6Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos2))
		copy(ip6Base[:len(ip6s.IPs):size2-ip6BasePos2], ip6s.IPs)

		lock.Lock()

		header.ip4.base = C.off_t(ip4BasePos2)
		header.ip6.base = C.off_t(ip6BasePos2)

		lock.Unlock()

		if _, err := C.munmap(addr, C.size_t(size)); err != nil {
			panic(err)
		}

		size = size2

		if err = unix.Ftruncate(int(fd), int64(size)); err != nil {
			panic(err)
		}

		addr, err = C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, fd, 0)
		if err != nil {
			panic(err)
		}

		header = (*C.ngx_ip_blocker_shm_st)(addr)
		lock = (*rwLock)(&header.lock)

		fmt.Printf("mapped %d bytes to %x\n", size, addr)
		fmt.Printf("\tIP4 of %d bytes (%d entries) mapped to %x\n", header.ip4.len, ip4s.Len(), uintptr(addr)+uintptr(header.ip4.base))
		fmt.Printf("\tIP6 of %d bytes (%d entries) mapped to %x\n", header.ip6.len, ip6s.Len(), uintptr(addr)+uintptr(header.ip6.base))
	}

	if err = stdin.Err(); err != nil {
		panic(err)
	}
}
