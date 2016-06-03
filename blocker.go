// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// An efficient shared memory IP blocking system for nginx.
//
// See https://github.com/tmthrgd/nginx-ip-blocker
package blocker

/*
#cgo LDFLAGS: -lrt

#include <stdlib.h>          // For free
#include <fcntl.h>           // For O_* constants
#include <sys/stat.h>        // For mode constants
#include <sys/mman.h>        // For shm_*
#include <unistd.h>          // For sysconf and _SC_* constants

#include "ngx_ip_blocker_shm.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	// ErrUnkownName is returned from Unlink when the
	// shared memory does not exist.
	ErrUnkownName = unix.ENOENT

	// ErrClosed will be returned on attempts to call
	// methods after (*IPBlocker).Close() has been called.
	ErrClosed = errors.New("shared memory closed")

	// ErrAlreadyBatching will be returned on attempts to call
	// (*IPBlocker).Batch() when the *IPBlocker is already
	// batching.
	ErrAlreadyBatching = errors.New("already batching")

	// ErrNotBatching will be returned on attempts to call
	// (*IPBlocker).Commit() when (*IPBlocker).Batch() has
	// not previously been called.
	ErrNotBatching = errors.New("not batching")
)

const headerSize = unsafe.Sizeof(C.ngx_ip_blocker_shm_st{})

var (
	cachelineSize uintptr
	pageSize      uintptr
)

/* taken from ngx_config.h */
func ngx_align(d, a uintptr) uintptr {
	return (d + (a - 1)) &^ (a - 1)
}

func calculateOffsets(base uintptr, ip4Len, ip6Len, ip6rLen int) (ip4BasePos, ip6BasePos, ip6rBasePos, end, size uintptr) {
	ip4BasePos = ngx_align(base, cachelineSize)
	ip6BasePos = ngx_align(ip4BasePos+uintptr(ip4Len), cachelineSize)
	ip6rBasePos = ngx_align(ip6BasePos+uintptr(ip6Len), cachelineSize)
	end = ngx_align(ip6rBasePos+uintptr(ip6rLen), cachelineSize)
	size = ngx_align(end, pageSize)
	return
}

func init() {
	pageSize = uintptr(unix.Getpagesize())

	if csize, err := C.sysconf(C._SC_LEVEL1_DCACHE_LINESIZE); err == nil {
		cachelineSize = uintptr(csize)
	} else {
		fmt.Printf("sysconf(_SC_LEVEL1_DCACHE_LINESIZE) = %s\n", err)

		cachelineSize = 64
	}
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++

		if ip[j] > 0 {
			break
		}
	}
}

// Unlink removes the previously created blocker.
//
// Taken from shm_unlink(3):
// 	The  operation  of shm_unlink() is analogous to unlink(2): it removes a
// 	shared memory object name, and, once all processes  have  unmapped  the
// 	object, de-allocates and destroys the contents of the associated memory
// 	region.  After a successful shm_unlink(),  attempts  to  shm_open()  an
// 	object  with  the same name will fail (unless O_CREAT was specified, in
// 	which case a new, distinct object is created).
func Unlink(name string) error {
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	_, err := C.shm_unlink(nameC)
	return err
}

// An IP blocker shared memory instance.
type IPBlocker struct {
	name string

	fd int

	ip4s  *ipSearcher
	ip6s  *ipSearcher
	ip6rs *ipSearcher

	addr unsafe.Pointer

	end  uintptr
	size uintptr

	mu sync.Mutex

	closed   bool
	batching bool
}

// Create a new IP blocker shared memory instance with
// specified name and permissions.
//
// This will fail if a shared memory region has already
// been created with the same name and not unlinked.
func New(name string, perms int) (*IPBlocker, error) {
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	fd, err := C.shm_open(nameC, C.O_CREAT|C.O_EXCL|C.O_TRUNC|C.O_RDWR, C.mode_t(perms))
	if err != nil {
		return nil, err
	}

	ip4s := newIPSearcher(net.IPv4len, nil)
	ip6s := newIPSearcher(net.IPv6len, nil)
	ip6rs := newIPSearcher(net.IPv6len/2, nil)

	ip4BasePos, ip6BasePos, ip6rBasePos, end, size := calculateOffsets(headerSize, len(ip4s.IPs), len(ip6s.IPs), len(ip6rs.IPs))

	if err = unix.Ftruncate(int(fd), int64(size)); err != nil {
		return nil, err
	}

	addr, err := C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, fd, 0)
	if err != nil {
		return nil, err
	}

	header := (*C.ngx_ip_blocker_shm_st)(addr)
	lock := (*rwLock)(&header.lock)

	lock.Create()

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

	return &IPBlocker{
		name: name,

		fd: int(fd),

		ip4s:  ip4s,
		ip6s:  ip6s,
		ip6rs: ip6rs,

		addr: addr,

		end:  end,
		size: size,
	}, nil
}

func (b *IPBlocker) commit() error {
	b.batching = false

	if _, err := C.munmap(b.addr, C.size_t(b.size)); err != nil {
		return err
	}

	ip4BasePos2, ip6BasePos2, ip6rBasePos2, end2, size2 := calculateOffsets(headerSize, len(b.ip4s.IPs), len(b.ip6s.IPs), len(b.ip6rs.IPs))

	end := b.end
	if end2 > end {
		end = end2
	}

	ip4BasePos, ip6BasePos, ip6rBasePos, end, size := calculateOffsets(end, len(b.ip4s.IPs), len(b.ip6s.IPs), len(b.ip6rs.IPs))

	if err := unix.Ftruncate(b.fd, int64(size)); err != nil {
		return err
	}

	addr, err := C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, C.int(b.fd), 0)
	if err != nil {
		return err
	}

	header := (*C.ngx_ip_blocker_shm_st)(addr)
	lock := (*rwLock)(&header.lock)

	ip4Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos))
	copy(ip4Base[:len(b.ip4s.IPs):ip6BasePos-ip4BasePos], b.ip4s.IPs)

	ip6Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos))
	copy(ip6Base[:len(b.ip6s.IPs):ip6rBasePos-ip6BasePos], b.ip6s.IPs)

	ip6rBase := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6rBasePos))
	copy(ip6rBase[:len(b.ip6rs.IPs):size-ip6rBasePos], b.ip6rs.IPs)

	lock.Lock()

	header.ip4.base = C.ssize_t(ip4BasePos)
	header.ip4.len = C.size_t(len(b.ip4s.IPs))

	header.ip6.base = C.ssize_t(ip6BasePos)
	header.ip6.len = C.size_t(len(b.ip6s.IPs))

	header.ip6route.base = C.ssize_t(ip6rBasePos)
	header.ip6route.len = C.size_t(len(b.ip6rs.IPs))

	header.revision++

	lock.Unlock()

	ip4Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos2))
	copy(ip4Base[:len(b.ip4s.IPs):ip6BasePos2-ip4BasePos2], b.ip4s.IPs)

	ip6Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos2))
	copy(ip6Base[:len(b.ip6s.IPs):ip6rBasePos2-ip6BasePos2], b.ip6s.IPs)

	ip6rBase = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6rBasePos2))
	copy(ip6rBase[:len(b.ip6rs.IPs):size2-ip6rBasePos2], b.ip6rs.IPs)

	lock.Lock()

	header.ip4.base = C.ssize_t(ip4BasePos2)
	header.ip6.base = C.ssize_t(ip6BasePos2)
	header.ip6route.base = C.ssize_t(ip6rBasePos2)

	header.revision++

	if err = unix.Ftruncate(b.fd, int64(size2)); err != nil {
		lock.Unlock()
		return err
	}

	lock.Unlock()

	if _, err = C.munmap(addr, C.size_t(size)); err != nil {
		return err
	}

	addr, err = C.mmap(nil, C.size_t(size2), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, C.int(b.fd), 0)
	if err != nil {
		return err
	}

	b.addr = addr
	b.end = end2
	b.size = size2
	return nil
}

// Ends a batching operation and commits all the changes
// to shared memory.
//
// Will fail if Closed() has already been called or
// if Batch() has not yet been called.
func (b *IPBlocker) Commit() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrClosed
	}

	if !b.batching {
		return ErrNotBatching
	}

	return b.commit()
}

func (b *IPBlocker) doInsertRemove(ip net.IP, insert bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrClosed
	}

	if ip4 := ip.To4(); ip4 != nil {
		if insert {
			b.ip4s.Insert(ip4)
		} else {
			b.ip4s.Remove(ip4)
		}
	} else if ip6 := ip.To16(); ip6 != nil {
		if insert {
			b.ip6s.Insert(ip6)
		} else {
			b.ip6s.Remove(ip6)
		}
	} else {
		return &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}

	if b.batching {
		return nil
	}

	return b.commit()
}

// Inserts a single IP address into the blocklist.
//
// If presently batching, Insert() will not commit the
// changes to shared memory.
//
// Will fail if Closed() has already been called.
func (b *IPBlocker) Insert(ip net.IP) error {
	return b.doInsertRemove(ip, true)
}

// Removes a single IP address from the blocklist.
//
// If presently batching, Insert() will not commit the
// changes to shared memory.
//
// Will fail if Closed() has already been called.
func (b *IPBlocker) Remove(ip net.IP) error {
	return b.doInsertRemove(ip, false)
}

func (b *IPBlocker) doInsertRemoveRange(ip net.IP, ipnet *net.IPNet, insert bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrClosed
	}

	masked := ip.Mask(ipnet.Mask)
	if masked == nil {
		return &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}

	var ips *ipSearcher

	if ip4 := masked.To4(); ip4 != nil {
		ip = ip4
		ips = b.ip4s
	} else if ip6 := masked.To16(); ip6 != nil {
		ip = ip6

		if ones, _ := ipnet.Mask.Size(); ones <= b.ip6rs.Size*8 {
			ips = b.ip6rs
		} else {
			ips = b.ip6s
		}
	} else {
		return &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}

	if insert {
		for ; ipnet.Contains(ip); incIP(ip[:ips.Size]) {
			ips.Insert(ip[:ips.Size])
		}
	} else {
		for ; ipnet.Contains(ip); incIP(ip[:ips.Size]) {
			ips.Remove(ip[:ips.Size])
		}
	}

	if b.batching {
		return nil
	}

	return b.commit()
}

// Inserts an IP address range into the blocklist.
//
// If presently batching, InsertRange() will not commit
// the changes to shared memory.
//
// Will fail if Closed() has already been called.
func (b *IPBlocker) InsertRange(ip net.IP, ipnet *net.IPNet) error {
	return b.doInsertRemoveRange(ip, ipnet, true)
}

// Removes an IP address range into the blocklist.
//
// If presently batching, RemoveRange() will not commit
// the changes to shared memory.
//
// Will fail if Closed() has already been called.
func (b *IPBlocker) RemoveRange(ip net.IP, ipnet *net.IPNet) error {
	return b.doInsertRemoveRange(ip, ipnet, false)
}

// Removes all IP addresses and ranges from the blocklist.
//
// If presently batching, Clear() will not commit the
// changes to shared memory.
//
// Will fail if Closed() has already been called.
func (b *IPBlocker) Clear() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrClosed
	}

	b.ip4s.Clear()
	b.ip6s.Clear()
	b.ip6rs.Clear()

	if b.batching {
		return nil
	}

	return b.commit()
}

// Batches all changes and withholds committing them
// to the blocklist until Commit() is manually called.
//
// Will fail if Closed() has already been called or
// if the blocker is already batching.
func (b *IPBlocker) Batch() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrClosed
	}

	if b.batching {
		return ErrAlreadyBatching
	}

	b.batching = true
	return nil
}

func (b *IPBlocker) close() error {
	b.closed = true

	if _, err := C.munmap(b.addr, C.size_t(b.size)); err != nil {
		return err
	}

	return unix.Close(b.fd)
}

// Closes the blockers shared memory.
func (b *IPBlocker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrClosed
	}

	return b.close()
}

// Unlinks and closes the blockers shared memory.
//
// Taken from shm_unlink(3):
// 	The  operation  of shm_unlink() is analogous to unlink(2): it removes a
// 	shared memory object name, and, once all processes  have  unmapped  the
// 	object, de-allocates and destroys the contents of the associated memory
// 	region.  After a successful shm_unlink(),  attempts  to  shm_open()  an
// 	object  with  the same name will fail (unless O_CREAT was specified, in
// 	which case a new, distinct object is created).
func (b *IPBlocker) Unlink() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return Unlink(b.name)
	}

	if err := b.close(); err != nil {
		return err
	}

	return Unlink(b.name)
}

// Returns true if the blocker is currently
// batching.
func (b *IPBlocker) IsBatching() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return !b.closed && b.batching
}

// Returns a human readable representation of
// the blocklist state.
func (b *IPBlocker) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return "<closed>"
	}

	header := (*C.ngx_ip_blocker_shm_st)(b.addr)

	return fmt.Sprintf("mapped %d bytes to %x\n"+
		"\tIP4 of %d bytes (%d entries) mapped to %x\n"+
		"\tIP6 of %d bytes (%d entries) mapped to %x\n"+
		"\tIP6 routes of %d bytes (%d entries) mapped to %x",
		b.size, b.addr,
		header.ip4.len, b.ip4s.Len(), uintptr(b.addr)+uintptr(header.ip4.base),
		header.ip6.len, b.ip6s.Len(), uintptr(b.addr)+uintptr(header.ip6.base),
		header.ip6route.len, b.ip6rs.Len(), uintptr(b.addr)+uintptr(header.ip6route.base))
}
