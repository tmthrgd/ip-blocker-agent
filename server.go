// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

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
	"os"
	"sync"
	"unsafe"
)

var (
	// ErrAlreadyBatching will be returned on attempts to call
	// (*Server).Batch() when the *Server is already
	// batching.
	ErrAlreadyBatching = errors.New("already batching")

	// ErrNotBatching will be returned on attempts to call
	// (*Server).Commit() when (*Server).Batch() has
	// not previously been called.
	ErrNotBatching = errors.New("not batching")
)

var (
	cachelineSize uintptr
	pageSize      uintptr
)

/* ngx_align, taken from ngx_config.h */
func align(d, a uintptr) uintptr {
	return (d + (a - 1)) &^ (a - 1)
}

func calculateOffsets(base uintptr, ip4Len, ip6Len, ip6rLen int) (ip4BasePos, ip6BasePos, ip6rBasePos, end, size uintptr) {
	ip4BasePos = align(base, cachelineSize)
	ip6BasePos = align(ip4BasePos+uintptr(ip4Len), cachelineSize)
	ip6rBasePos = align(ip6BasePos+uintptr(ip6Len), cachelineSize)
	end = align(ip6rBasePos+uintptr(ip6rLen), cachelineSize)
	size = align(end, pageSize)
	return
}

func init() {
	pageSize = uintptr(os.Getpagesize())

	if csize, err := C.sysconf(C._SC_LEVEL1_DCACHE_LINESIZE); err == nil {
		cachelineSize = uintptr(csize)
	} else {
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

// Server is an IP blocker shared memory server.
type Server struct {
	file *os.File

	ip4s  *binarySearcher
	ip6s  *binarySearcher
	ip6rs *binarySearcher

	addr unsafe.Pointer

	end  uintptr
	size uintptr

	mu sync.Mutex

	closed   bool
	batching bool
}

// New creates a new IP blocker shared memory server
// with specified name and permissions.
//
// This will fail if a shared memory region has already
// been created with the same name and not unlinked.
func New(name string, perms int) (*Server, error) {
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	fd, err := C.shm_open(nameC, C.O_CREAT|C.O_EXCL|C.O_TRUNC|C.O_RDWR, C.mode_t(perms))
	if err != nil {
		return nil, err
	}

	file := os.NewFile(uintptr(fd), name)

	ip4BasePos, ip6BasePos, ip6rBasePos, end, size := calculateOffsets(headerSize, 0, 0, 0)

	if err = file.Truncate(int64(size)); err != nil {
		return nil, err
	}

	addr, err := C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, C.int(file.Fd()), 0)
	if err != nil {
		return nil, err
	}

	header := (*C.ngx_ip_blocker_shm_st)(addr)
	lock := (*rwLock)(&header.lock)

	lock.Create()

	lock.Lock()

	header.ip4.base = C.ssize_t(ip4BasePos)
	header.ip6.base = C.ssize_t(ip6BasePos)
	header.ip6route.base = C.ssize_t(ip6rBasePos)

	header.revision = 1

	lock.Unlock()

	return &Server{
		file: file,

		ip4s:  newBinarySearcher(net.IPv4len, nil),
		ip6s:  newBinarySearcher(net.IPv6len, nil),
		ip6rs: newBinarySearcher(net.IPv6len/2, nil),

		addr: addr,

		end:  end,
		size: size,
	}, nil
}

func (s *Server) commit() error {
	s.batching = false

	if _, err := C.munmap(s.addr, C.size_t(s.size)); err != nil {
		return err
	}

	ip4BasePos2, ip6BasePos2, ip6rBasePos2, end2, size2 := calculateOffsets(headerSize, len(s.ip4s.Data), len(s.ip6s.Data), len(s.ip6rs.Data))

	end := s.end
	if end2 > end {
		end = end2
	}

	ip4BasePos, ip6BasePos, ip6rBasePos, end, size := calculateOffsets(end, len(s.ip4s.Data), len(s.ip6s.Data), len(s.ip6rs.Data))

	if err := s.file.Truncate(int64(size)); err != nil {
		return err
	}

	addr, err := C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, C.int(s.file.Fd()), 0)
	if err != nil {
		return err
	}

	header := (*C.ngx_ip_blocker_shm_st)(addr)
	lock := (*rwLock)(&header.lock)

	ip4Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos))
	copy(ip4Base[:len(s.ip4s.Data):ip6BasePos-ip4BasePos], s.ip4s.Data)

	ip6Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos))
	copy(ip6Base[:len(s.ip6s.Data):ip6rBasePos-ip6BasePos], s.ip6s.Data)

	ip6rBase := (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6rBasePos))
	copy(ip6rBase[:len(s.ip6rs.Data):size-ip6rBasePos], s.ip6rs.Data)

	lock.Lock()

	header.ip4.base = C.ssize_t(ip4BasePos)
	header.ip4.len = C.size_t(len(s.ip4s.Data))

	header.ip6.base = C.ssize_t(ip6BasePos)
	header.ip6.len = C.size_t(len(s.ip6s.Data))

	header.ip6route.base = C.ssize_t(ip6rBasePos)
	header.ip6route.len = C.size_t(len(s.ip6rs.Data))

	header.revision++

	lock.Unlock()

	ip4Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip4BasePos2))
	copy(ip4Base[:len(s.ip4s.Data):ip6BasePos2-ip4BasePos2], s.ip4s.Data)

	ip6Base = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6BasePos2))
	copy(ip6Base[:len(s.ip6s.Data):ip6rBasePos2-ip6BasePos2], s.ip6s.Data)

	ip6rBase = (*[1 << 30]byte)(unsafe.Pointer(uintptr(addr) + ip6rBasePos2))
	copy(ip6rBase[:len(s.ip6rs.Data):size2-ip6rBasePos2], s.ip6rs.Data)

	lock.Lock()

	header.ip4.base = C.ssize_t(ip4BasePos2)
	header.ip6.base = C.ssize_t(ip6BasePos2)
	header.ip6route.base = C.ssize_t(ip6rBasePos2)

	header.revision++

	if err = s.file.Truncate(int64(size2)); err != nil {
		lock.Unlock()
		return err
	}

	lock.Unlock()

	if _, err = C.munmap(addr, C.size_t(size)); err != nil {
		return err
	}

	addr, err = C.mmap(nil, C.size_t(size2), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, C.int(s.file.Fd()), 0)
	if err != nil {
		return err
	}

	s.addr = addr
	s.end = end2
	s.size = size2
	return nil
}

// Commit ends a batching operation and commits all
// the changes to shared memory.
//
// Will fail if Closed() has already been called or
// if Batch() has not yet been called.
func (s *Server) Commit() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}

	if !s.batching {
		return ErrNotBatching
	}

	return s.commit()
}

func (s *Server) doInsertRemove(ip net.IP, insert bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}

	if ip4 := ip.To4(); ip4 != nil {
		if insert {
			s.ip4s.Insert(ip4)
		} else {
			s.ip4s.Remove(ip4)
		}
	} else if ip6 := ip.To16(); ip6 != nil {
		if insert {
			s.ip6s.Insert(ip6)
		} else {
			s.ip6s.Remove(ip6)
		}
	} else {
		return &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}

	if s.batching {
		return nil
	}

	return s.commit()
}

// Insert inserts a single IP address into the
// blocklist.
//
// If presently batching, Insert() will not commit the
// changes to shared memory.
//
// Will fail if Closed() has already been called.
func (s *Server) Insert(ip net.IP) error {
	return s.doInsertRemove(ip, true)
}

// Remove removes a single IP address from the
// blocklist.
//
// If the IP address is covered by a range added with
// InsertRange() that was larger than /64 then calling
// Remove() will fail to remove the IP address.
// Instead, RemoveRange() must be used to remove the
// entire range.
//
// If presently batching, Insert() will not commit the
// changes to shared memory.
//
// Will fail if Closed() has already been called.
func (s *Server) Remove(ip net.IP) error {
	return s.doInsertRemove(ip, false)
}

func (s *Server) doInsertRemoveRange(ip net.IP, ipnet *net.IPNet, insert bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}

	masked := ip.Mask(ipnet.Mask)
	if masked == nil {
		return &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}

	var ips *binarySearcher

	if ip4 := masked.To4(); ip4 != nil {
		ip = ip4
		ips = s.ip4s
	} else if ip6 := masked.To16(); ip6 != nil {
		ip = ip6

		if ones, _ := ipnet.Mask.Size(); ones <= s.ip6rs.Size()*8 {
			ips = s.ip6rs
		} else {
			ips = s.ip6s
		}
	} else {
		return &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}

	size := ips.Size()

	if insert {
		for ; ipnet.Contains(ip); incIP(ip[:size]) {
			ips.Insert(ip[:size])
		}
	} else {
		for ; ipnet.Contains(ip); incIP(ip[:size]) {
			ips.Remove(ip[:size])
		}
	}

	if s.batching {
		return nil
	}

	return s.commit()
}

// InsertRange inserts all IP addresses in a CIDR
// block into the blocklist.
//
// If the net.IP is a valid IPv6 address and the
// CIDR block is larger than /64, the range is
// inserted into a separate route list. IP addresses
// inserted in this way cannot be removed with
// Remove().
//
// If presently batching, InsertRange() will not
// commit the changes to shared memory.
//
// Will fail if Closed() has already been called.
func (s *Server) InsertRange(ip net.IP, ipnet *net.IPNet) error {
	return s.doInsertRemoveRange(ip, ipnet, true)
}

// RemoveRange removes all IP addresses in a CIDR
// block from the the blocklist.
//
// If the net.IP is a valid IPv6 address and the
// CIDR block is larger than /64, the range is
// removed from a separate route list. IP addresses
// removed in this way cannot have been inserted with
// Insert().
//
// If presently batching, RemoveRange() will not
// commit the changes to shared memory.
//
// Will fail if Closed() has already been called.
func (s *Server) RemoveRange(ip net.IP, ipnet *net.IPNet) error {
	return s.doInsertRemoveRange(ip, ipnet, false)
}

// Clear removes all IP addresses and ranges from the
// blocklist.
//
// If presently batching, Clear() will not commit the
// changes to shared memory.
//
// Will fail if Closed() has already been called.
func (s *Server) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}

	s.ip4s.Clear()
	s.ip6s.Clear()
	s.ip6rs.Clear()

	if s.batching {
		return nil
	}

	return s.commit()
}

// Batch beings batching all changes and withholds
// committing them to shared memory until Commit()
// is manually called.
//
// Will fail if Closed() has already been called or
// if the blocker is already batching.
func (s *Server) Batch() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}

	if s.batching {
		return ErrAlreadyBatching
	}

	s.batching = true
	return nil
}

func (s *Server) close() error {
	s.closed = true

	if _, err := C.munmap(s.addr, C.size_t(s.size)); err != nil {
		return err
	}

	return s.file.Close()
}

// Close closes the servers shared memory and releases
// the file descriptor.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}

	return s.close()
}

// Unlink removes the server and closes the backing
// shared memory.
//
// It is the equivalent of calling Close() followed by
// Unlink(string) with the same name as New.
//
// Taken from shm_unlink(3):
// 	The  operation  of shm_unlink() is analogous to unlink(2): it removes a
// 	shared memory object name, and, once all processes  have  unmapped  the
// 	object, de-allocates and destroys the contents of the associated memory
// 	region.  After a successful shm_unlink(),  attempts  to  shm_open()  an
// 	object  with  the same name will fail (unless O_CREAT was specified, in
// 	which case a new, distinct object is created).
func (s *Server) Unlink() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return Unlink(s.file.Name())
	}

	if err := s.close(); err != nil {
		return err
	}

	return Unlink(s.file.Name())
}

// IsBatching returns a boolean indicating whether the
// server is currently batching operations.
func (s *Server) IsBatching() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return !s.closed && s.batching
}

// Name returns the name of the shared memory.
func (s *Server) Name() string {
	return s.file.Name()
}

// String returns a human readable representation of
// the blocklist state.
func (s *Server) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return "<closed>"
	}

	header := (*C.ngx_ip_blocker_shm_st)(s.addr)

	return fmt.Sprintf("mapped %d bytes to %x\n"+
		"\tIP4 of %d bytes (%d entries) mapped to %x\n"+
		"\tIP6 of %d bytes (%d entries) mapped to %x\n"+
		"\tIP6 routes of %d bytes (%d entries) mapped to %x",
		s.size, s.addr,
		header.ip4.len, s.ip4s.Len(), uintptr(s.addr)+uintptr(header.ip4.base),
		header.ip6.len, s.ip6s.Len(), uintptr(s.addr)+uintptr(header.ip6.base),
		header.ip6route.len, s.ip6rs.Len(), uintptr(s.addr)+uintptr(header.ip6route.base))
}