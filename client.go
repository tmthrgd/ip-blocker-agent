// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

/*
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
	"net"
	"os"
	"sync"
	"unsafe"
)

var errInvalidSharedMem = errors.New("invalid shared memory")

// Client is an IP blocker shared memory client.
type Client struct {
	file *os.File

	addr unsafe.Pointer
	size int64

	mu sync.RWMutex

	revision uint32

	closed bool
}

// Open returns a new IP blocker shared memory client
// specified by name.
func Open(name string) (*Client, error) {
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	fd, err := C.shm_open(nameC, C.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	file := os.NewFile(uintptr(fd), name)

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	size := stat.Size()
	if size < int64(headerSize) {
		file.Close()
		return nil, errInvalidSharedMem
	}

	addr, err := C.mmap(nil, C.size_t(size), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, C.int(file.Fd()), 0)
	if err != nil {
		file.Close()
		return nil, err
	}

	header := (*C.ngx_ip_blocker_shm_st)(addr)
	lock := (*rwLock)(&header.lock)

	client := &Client{
		file: file,

		addr: addr,
		size: size,
	}

	lock.RLock()

	client.revision = uint32(header.revision)

	stat, err = file.Stat()
	if err != nil {
		lock.RUnlock()

		C.munmap(addr, C.size_t(size))
		file.Close()
		return nil, err
	}

	if stat.Size() != size {
		/* shm has changed since we mmaped it (unlikely but possible) */

		/* RUnlock is called inside of remap iff an err is returned */
		if err := client.remap(); err != nil {
			file.Close()
			return nil, err
		}

		header = (*C.ngx_ip_blocker_shm_st)(client.addr)
		lock = (*rwLock)(&header.lock)
	} else if !client.checkSharedMemory() {
		lock.RUnlock()

		C.munmap(addr, C.size_t(size))
		file.Close()
		return nil, errInvalidSharedMem
	}

	lock.RUnlock()
	return client, nil
}

/* RLock must be held before calling remap */
func (c *Client) remap() (err error) {
	if c.closed {
		panic(ErrClosed)
	}

	addr, size := c.addr, c.size
	c.addr, c.size = nil, 0

	stat, err := c.file.Stat()
	if err != nil {
		goto err
	}

	c.addr, err = C.mmap(nil, C.size_t(stat.Size()), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, C.int(c.file.Fd()), 0)
	if err != nil {
		goto err
	}

	c.size = stat.Size()

	if !c.checkSharedMemory() {
		err = errInvalidSharedMem
		goto err
	}

	c.revision = uint32((*C.ngx_ip_blocker_shm_st)(c.addr).revision)

	_, err = C.munmap(addr, C.size_t(size))
	return

err:
	if c.size == 0 || c.size >= int64(headerSize) {
		header := (*C.ngx_ip_blocker_shm_st)(addr)
		lock := (*rwLock)(&header.lock)
		lock.RUnlock()
	} else {
		os.Stderr.WriteString("failed to release read lock")
	}

	C.munmap(addr, C.size_t(size))
	return
}

func (c *Client) checkSharedMemory() bool {
	if c.closed {
		panic(ErrClosed)
	}

	header := (*C.ngx_ip_blocker_shm_st)(c.addr)

	return c.size >= int64(headerSize) &&
		c.size >= int64(headerSize)+int64(header.ip4.len+header.ip6.len+header.ip6route.len) &&
		(header.ip4.len == 0 || uintptr(header.ip4.base) >= headerSize) &&
		(header.ip6.len == 0 || uintptr(header.ip6.base) >= headerSize) &&
		(header.ip6route.len == 0 || uintptr(header.ip6route.base) >= headerSize) &&
		int64(uintptr(header.ip4.base)+uintptr(header.ip4.len)) <= c.size &&
		int64(uintptr(header.ip6.base)+uintptr(header.ip6.len)) <= c.size &&
		int64(uintptr(header.ip6route.base)+uintptr(header.ip6route.len)) <= c.size &&
		header.ip4.len%4 == 0 &&
		header.ip6.len%16 == 0 &&
		header.ip6route.len%8 == 0
}

// Contains returns a boolean indicating whether the
// IP address is in the blocklist.
func (c *Client) Contains(ip net.IP) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false, ErrClosed
	}

	if c.addr == nil || c.size < int64(headerSize) {
		return false, errInvalidSharedMem
	}

	header := (*C.ngx_ip_blocker_shm_st)(c.addr)
	lock := (*rwLock)(&header.lock)

	lock.RLock()

	if c.revision != uint32(header.revision) {
		/* RUnlock is called inside of remap iff an error is returned */
		if err := c.remap(); err != nil {
			return false, err
		}

		header = (*C.ngx_ip_blocker_shm_st)(c.addr)
		lock = (*rwLock)(&header.lock)
	}

	defer lock.RUnlock()

	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4

		if header.ip4.len == 0 {
			return false, nil
		}

		searcher := newBinarySearcher(net.IPv4len, nil)
		ip4Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(c.addr) + uintptr(header.ip4.base)))
		searcher.Data = ip4Base[:header.ip4.len:header.ip4.len]

		return searcher.Contains(ip), nil
	} else if ip6 := ip.To16(); ip6 != nil {
		ip = ip6

		if header.ip6route.len != 0 {
			searcher := newBinarySearcher(net.IPv6len/2, nil)
			ip6rBase := (*[1 << 30]byte)(unsafe.Pointer(uintptr(c.addr) + uintptr(header.ip6route.base)))
			searcher.Data = ip6rBase[:header.ip6route.len:header.ip6route.len]

			if searcher.Contains(ip[:net.IPv6len/2]) {
				return true, nil
			}
		}

		if header.ip6.len == 0 {
			return false, nil
		}

		searcher := newBinarySearcher(net.IPv6len, nil)
		ip6Base := (*[1 << 30]byte)(unsafe.Pointer(uintptr(c.addr) + uintptr(header.ip6.base)))
		searcher.Data = ip6Base[:header.ip6.len:header.ip6.len]

		return searcher.Contains(ip), nil
	} else {
		return false, &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}
}

// Close closes the blockers shared memory and
// releases the file descriptor.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrClosed
	}

	c.closed = true

	if c.addr != nil {
		if _, err := C.munmap(c.addr, C.size_t(c.size)); err != nil {
			return err
		}

		c.addr = nil
		c.size = 0
	}

	return c.file.Close()
}

// Name returns the name of the shared memory.
func (c *Client) Name() string {
	return c.file.Name()
}

// Count returns the number of IPv4 addresses, IPv6
// address and IPv6 routes stored in the blocklist.
//
// Will fail if Closed() has been called.
func (c *Client) Count() (ip4, ip6, ip6routes int, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		err = ErrClosed
		return
	}

	if c.addr == nil || c.size < int64(headerSize) || !c.checkSharedMemory() {
		err = errInvalidSharedMem
		return
	}

	header := (*C.ngx_ip_blocker_shm_st)(c.addr)

	ip4 = int(header.ip4.len / net.IPv4len)
	ip6 = int(header.ip6.len / net.IPv6len)
	ip6routes = int(header.ip6route.len / (net.IPv6len / 2))
	return
}
