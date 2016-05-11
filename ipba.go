package main

/*
#cgo LDFLAGS: -lrt

#include <stdlib.h>          // For free
#include <fcntl.h>           // For O_* constants
#include <sys/stat.h>        // For mode constants
#include <sys/mman.h>        // For shm_*
#include <semaphore.h>       // For sem_*

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
	"strings"
	"sync/atomic"
	"unsafe"
)

const (
	ngx_cacheline_size = 64

	headerSize = unsafe.Sizeof(C.ngx_ip_blocker_shm_st{})
)

var pageSize uintptr

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

func calculateOffsets(base uintptr, ip4Len, ip6Len, ip6rLen int) (ip4BasePos, ip6BasePos, ip6rBasePos, end, size uintptr) {
	ip4BasePos = ngx_align(base, ngx_cacheline_size)
	ip6BasePos = ngx_align(ip4BasePos+uintptr(ip4Len), ngx_cacheline_size)
	ip6rBasePos = ngx_align(ip6BasePos+uintptr(ip6Len), ngx_cacheline_size)
	end = ngx_align(ip6rBasePos+uintptr(ip6rLen), ngx_cacheline_size)
	size = ngx_align(end, pageSize)
	return
}

type mutex C.ngx_ip_blocker_mutex_st

func (m *mutex) Create() {
	if _, err := C.sem_init(&m.sem, 1, 1); err != nil {
		panic(err)
	}
}

func (m *mutex) Lock() {
	if _, err := C.sem_wait(&m.sem); err != nil {
		panic(err)
	}
}

func (m *mutex) Unlock() {
	if _, err := C.sem_post(&m.sem); err != nil {
		panic(err)
	}
}

type rwLock C.ngx_ip_blocker_rwlock_st

func (rw *rwLock) Create() {
	w := (*mutex)(&rw.w)
	w.Create()

	if _, err := C.sem_init(&rw.writer_sem, 1, 0); err != nil {
		panic(err)
	}

	if _, err := C.sem_init(&rw.reader_sem, 1, 0); err != nil {
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
	w := (*mutex)(&rw.w)
	w.Lock()

	// Announce to readers there is a pending writer.
	r := atomic.AddInt32((*int32)(&rw.reader_count), -C.NGX_IP_BLOCKER_MAX_READERS) + C.NGX_IP_BLOCKER_MAX_READERS

	// Wait for active readers.
	if r != 0 && atomic.AddInt32((*int32)(&rw.reader_wait), r) != 0 {
		if _, err := C.sem_wait(&rw.writer_sem); err != nil {
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
		if _, err := C.sem_post(&rw.reader_sem); err != nil {
			panic(err)
		}
	}

	// Allow other writers to proceed.
	w := (*mutex)(&rw.w)
	w.Unlock()
}

func init() {
	pageSize = uintptr(os.Getpagesize())
}

func main() {
	var name string
	flag.StringVar(&name, "name", "/ngx-ip-blocker", "the shared memory name")

	var whitelist bool
	flag.BoolVar(&whitelist, "whitelist", false, "operate in whitelist mode")

	flag.Parse()

	if len(name) == 0 {
		fmt.Println("-name cannot be empty")
		os.Exit(1)
	}

	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	/* Taken from shm_unlink(3):
	 *
	 * The  operation  of shm_unlink() is analogous to unlink(2): it removes a
	 * shared memory object name, and, once all processes  have  unmapped  the
	 * object, de-allocates and destroys the contents of the associated memory
	 * region.  After a successful shm_unlink(),  attempts  to  shm_open()  an
	 * object  with  the same name will fail (unless O_CREAT was specified, in
	 * which case a new, distinct object is created).
	 */
	_, err := C.shm_unlink(nameC)
	if err != nil && err != unix.ENOENT {
		panic(err)
	}

	fd, err := C.shm_open(nameC, C.O_CREAT|C.O_EXCL|C.O_TRUNC|C.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}

	defer func() {
		/* Taken from shm_unlink(3):
		 *
		 * The  operation  of shm_unlink() is analogous to unlink(2): it removes a
		 * shared memory object name, and, once all processes  have  unmapped  the
		 * object, de-allocates and destroys the contents of the associated memory
		 * region.  After a successful shm_unlink(),  attempts  to  shm_open()  an
		 * object  with  the same name will fail (unless O_CREAT was specified, in
		 * which case a new, distinct object is created).
		 */
		_, err := C.shm_unlink(nameC)
		if err != nil {
			panic(err)
		}
	}()
	defer unix.Close(int(fd))

	ip4s := newIPSearcher(net.IPv4len, nil)
	ip6s := newIPSearcher(net.IPv6len, nil)
	ip6rs := newIPSearcher(64/8, nil)

	ip4BasePos, ip6BasePos, ip6rBasePos, end, size := calculateOffsets(headerSize, len(ip4s.IPs), len(ip6s.IPs), len(ip6rs.IPs))

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

	if whitelist {
		header.whitelist = 1
	} else {
		header.whitelist = 0
	}

	lock.Lock()

	header.ip4.base = C.ssize_t(ip4BasePos)
	header.ip4.len = C.size_t(len(ip4s.IPs))

	header.ip6.base = C.ssize_t(ip6BasePos)
	header.ip6.len = C.size_t(len(ip6s.IPs))

	header.ip6route.base = C.ssize_t(ip6rBasePos)
	header.ip6route.len = C.size_t(len(ip6rs.IPs))

	ip4Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos))
	copy(ip4Base[:len(ip4s.IPs):ip6BasePos-ip4BasePos], ip4s.IPs)

	ip6Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos))
	copy(ip6Base[:len(ip6s.IPs):ip6rBasePos-ip6BasePos], ip6s.IPs)

	ip6rBase := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6rBasePos))
	copy(ip6rBase[:len(ip6rs.IPs):size-ip6rBasePos], ip6rs.IPs)

	header.revision = 1

	lock.Unlock()

	fmt.Printf("mapped %d bytes to %x\n", size, addr)
	fmt.Printf("\tIP4 of %d bytes (%d entries) mapped to %x\n", header.ip4.len, ip4s.Len(), uintptr(addr)+uintptr(header.ip4.base))
	fmt.Printf("\tIP6 of %d bytes (%d entries) mapped to %x\n", header.ip6.len, ip6s.Len(), uintptr(addr)+uintptr(header.ip6.base))
	fmt.Printf("\tIP6 routes of %d bytes (%d entries) mapped to %x\n", header.ip6route.len, ip6rs.Len(), uintptr(addr)+uintptr(header.ip6route.base))

	stdin := bufio.NewScanner(os.Stdin)

	var batch bool

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
		case 'b':
			if len(line) != 1 && !strings.EqualFold(line, "batch") {
				fmt.Printf("invalid input: %s\n", line)
			} else if batch {
				fmt.Println("already batching operations")
			}

			batch = true
			continue
		case 'B':
			if len(line) != 1 && !strings.EqualFold(line, "batch") {
				fmt.Printf("invalid input: %s\n", line)
				continue
			} else if !batch {
				fmt.Println("not batching operations")
				continue
			}

			batch = false
		case 'q':
			fallthrough
		case 'Q':
			if len(line) == 1 || strings.EqualFold(line, "quit") {
				return
			}

			fmt.Printf("invalid input: %s\n", line)
			continue
		default:
			fmt.Printf("invalid operation: %c\n", line[0])
			continue
		}

		switch line[0] {
		case 'B':
		case '!':
			ip4s.Clear()
			ip6s.Clear()
			ip6rs.Clear()
		default:
			if strings.Contains(line[1:], "/") {
				ip, ipnet, err := net.ParseCIDR(line[1:])
				if err != nil {
					fmt.Printf("invalid cidr mask: %s (%v)\n", line[1:], err)
					continue
				}

				ip = ip.Mask(ipnet.Mask)
				var ips *ipSearcher

				if ip4 := ip.To4(); ip4 != nil {
					ip = ip4
					ips = ip4s
				} else {
					ip = ip.To16()

					if ones, _ := ipnet.Mask.Size(); ones <= ip6rs.Size*8 {
						ips = ip6rs
					} else {
						ips = ip6s
					}
				}

				switch line[0] {
				case '+':
					for ; ipnet.Contains(ip); incIP(ip[:ips.Size]) {
						ips.Insert(ip[:ips.Size])
					}
				case '-':
					for ; ipnet.Contains(ip); incIP(ip[:ips.Size]) {
						ips.Remove(ip[:ips.Size])
					}
				}
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
		}

		if batch {
			continue
		}

		if _, err := C.munmap(addr, C.size_t(size)); err != nil {
			panic(err)
		}

		ip4BasePos2, ip6BasePos2, ip6rBasePos2, end2, size2 := calculateOffsets(headerSize, len(ip4s.IPs), len(ip6s.IPs), len(ip6rs.IPs))

		if end2 > end {
			end = end2
		}

		ip4BasePos, ip6BasePos, ip6rBasePos, end, size = calculateOffsets(end, len(ip4s.IPs), len(ip6s.IPs), len(ip6rs.IPs))

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
		copy(ip6Base[:len(ip6s.IPs):ip6rBasePos-ip6BasePos], ip6s.IPs)

		ip6rBase = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6rBasePos))
		copy(ip6rBase[:len(ip6rs.IPs):size-ip6rBasePos], ip6rs.IPs)

		lock.Lock()

		header.ip4.base = C.ssize_t(ip4BasePos)
		header.ip4.len = C.size_t(len(ip4s.IPs))

		header.ip6.base = C.ssize_t(ip6BasePos)
		header.ip6.len = C.size_t(len(ip6s.IPs))

		header.ip6route.base = C.ssize_t(ip6rBasePos)
		header.ip6route.len = C.size_t(len(ip6rs.IPs))

		header.revision++

		lock.Unlock()

		ip4Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos2))
		copy(ip4Base[:len(ip4s.IPs):ip6BasePos2-ip4BasePos2], ip4s.IPs)

		ip6Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos2))
		copy(ip6Base[:len(ip6s.IPs):ip6rBasePos2-ip6BasePos2], ip6s.IPs)

		ip6rBase = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6rBasePos2))
		copy(ip6rBase[:len(ip6rs.IPs):size2-ip6rBasePos2], ip6rs.IPs)

		lock.Lock()

		header.ip4.base = C.ssize_t(ip4BasePos2)
		header.ip6.base = C.ssize_t(ip6BasePos2)
		header.ip6route.base = C.ssize_t(ip6rBasePos2)

		header.revision++

		if err = unix.Ftruncate(int(fd), int64(size2)); err != nil {
			panic(err)
		}

		lock.Unlock()

		if _, err := C.munmap(addr, C.size_t(size)); err != nil {
			panic(err)
		}

		end = end2
		size = size2

		addr, err = C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, fd, 0)
		if err != nil {
			panic(err)
		}

		header = (*C.ngx_ip_blocker_shm_st)(addr)
		lock = (*rwLock)(&header.lock)

		fmt.Printf("mapped %d bytes to %x\n", size, addr)
		fmt.Printf("\tIP4 of %d bytes (%d entries) mapped to %x\n", header.ip4.len, ip4s.Len(), uintptr(addr)+uintptr(header.ip4.base))
		fmt.Printf("\tIP6 of %d bytes (%d entries) mapped to %x\n", header.ip6.len, ip6s.Len(), uintptr(addr)+uintptr(header.ip6.base))
		fmt.Printf("\tIP6 routes of %d bytes (%d entries) mapped to %x\n", header.ip6route.len, ip6rs.Len(), uintptr(addr)+uintptr(header.ip6route.base))
	}

	if err = stdin.Err(); err != nil {
		panic(err)
	}
}
