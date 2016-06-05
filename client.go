// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

//#include "ngx_ip_blocker_shm.h"
import "C"

import (
	"errors"
	"net"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

var errInvalidSharedMem = errors.New("invalid shared memory")

// Client is an IP blocker shared memory client.
type Client struct {
	file *os.File

	data []byte

	mu sync.RWMutex

	revision uint32

	closed bool
}

// Open returns a new IP blocker shared memory client
// specified by name.
func Open(name string) (*Client, error) {
	file, err := shmOpen(name, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

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

	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, err
	}

	header := (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&data[0]))
	lock := (*rwLock)(&header.lock)

	client := &Client{
		file: file,

		data: data,
	}

	lock.RLock()

	client.revision = uint32(header.revision)

	stat, err = file.Stat()
	if err != nil {
		lock.RUnlock()

		syscall.Munmap(data)
		file.Close()
		return nil, err
	}

	if stat.Size() != size {
		/* shm has changed since we mmaped it (unlikely but possible) */

		/* RUnlock is called inside of remap iff an err is returned */
		if err := client.remap(true); err != nil {
			file.Close()
			return nil, err
		}

		header = (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&client.data[0]))
		lock = (*rwLock)(&header.lock)
	} else if !client.checkSharedMemory() {
		lock.RUnlock()

		syscall.Munmap(data)
		file.Close()
		return nil, errInvalidSharedMem
	}

	lock.RUnlock()
	return client, nil
}

/* RLock must be held before calling remap */
func (c *Client) remap(force bool) (err error) {
	if c.closed {
		panic(ErrClosed)
	}

	if !force {
		c.mu.RUnlock()
		c.mu.Lock()
		defer c.mu.RLock()
		defer c.mu.Unlock()

		if c.closed {
			return ErrClosed
		}

		header := (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&c.data[0]))
		if c.revision == uint32(header.revision) {
			return nil
		}
	}

	data := c.data
	c.data = nil

	stat, err := c.file.Stat()
	if err != nil {
		goto err
	}

	c.data, err = syscall.Mmap(int(c.file.Fd()), 0, int(stat.Size()), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		goto err
	}

	if !c.checkSharedMemory() {
		err = errInvalidSharedMem
		goto err
	}

	c.revision = uint32((*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&c.data[0])).revision)

	err = syscall.Munmap(data)
	return

err:
	if len(c.data) == 0 || len(c.data) >= int(headerSize) {
		header := (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&data[0]))
		lock := (*rwLock)(&header.lock)
		lock.RUnlock()
	} else {
		os.Stderr.WriteString("failed to release read lock")
	}

	syscall.Munmap(data)
	return
}

func (c *Client) checkSharedMemory() bool {
	if c.closed {
		panic(ErrClosed)
	}

	header := (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&c.data[0]))

	const maxInt = int(^uint(0) >> 1)
	return len(c.data) >= int(headerSize) &&
		uintptr(headerSize)+uintptr(header.ip4.len+header.ip6.len+header.ip6route.len) <= uintptr(maxInt) &&
		len(c.data) >= int(headerSize)+int(header.ip4.len+header.ip6.len+header.ip6route.len) &&
		(header.ip4.len == 0 || uintptr(header.ip4.base) >= headerSize) &&
		(header.ip6.len == 0 || uintptr(header.ip6.base) >= headerSize) &&
		(header.ip6route.len == 0 || uintptr(header.ip6route.base) >= headerSize) &&
		uintptr(header.ip4.base)+uintptr(header.ip4.len) <= uintptr(maxInt) &&
		uintptr(header.ip6.base)+uintptr(header.ip6.len) <= uintptr(maxInt) &&
		uintptr(header.ip6route.base)+uintptr(header.ip6route.len) <= uintptr(maxInt) &&
		int(uintptr(header.ip4.base)+uintptr(header.ip4.len)) <= len(c.data) &&
		int(uintptr(header.ip6.base)+uintptr(header.ip6.len)) <= len(c.data) &&
		int(uintptr(header.ip6route.base)+uintptr(header.ip6route.len)) <= len(c.data) &&
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

	if len(c.data) < int(headerSize) {
		return false, errInvalidSharedMem
	}

	header := (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&c.data[0]))
	lock := (*rwLock)(&header.lock)

	lock.RLock()

	if c.revision != uint32(header.revision) {
		/* RUnlock is called inside of remap iff an error is returned */
		if err := c.remap(false); err != nil {
			return false, err
		}

		header = (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&c.data[0]))
		lock = (*rwLock)(&header.lock)
	}

	defer lock.RUnlock()

	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4

		if header.ip4.len == 0 {
			return false, nil
		}

		searcher := newBinarySearcher(net.IPv4len, nil)
		searcher.Data = c.data[header.ip4.base : int(header.ip4.base)+int(header.ip4.len)]

		return searcher.Contains(ip), nil
	} else if ip6 := ip.To16(); ip6 != nil {
		ip = ip6

		if header.ip6route.len != 0 {
			searcher := newBinarySearcher(net.IPv6len/2, nil)
			searcher.Data = c.data[header.ip6route.base : int(header.ip6route.base)+int(header.ip6route.len)]

			if searcher.Contains(ip[:net.IPv6len/2]) {
				return true, nil
			}
		}

		if header.ip6.len == 0 {
			return false, nil
		}

		searcher := newBinarySearcher(net.IPv6len, nil)
		searcher.Data = c.data[header.ip6.base : int(header.ip6.base)+int(header.ip6.len)]

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

	if c.data != nil {
		if err := syscall.Munmap(c.data); err != nil {
			return err
		}

		c.data = nil
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

	if len(c.data) < int(headerSize) {
		err = errInvalidSharedMem
		return
	}

	header := (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&c.data[0]))

	ip4 = int(header.ip4.len / net.IPv4len)
	ip6 = int(header.ip6.len / net.IPv6len)
	ip6routes = int(header.ip6route.len / (net.IPv6len / 2))
	return
}

func (c *Client) rwlockerForTest() *rwLock {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		panic(ErrClosed)
	}

	if len(c.data) < int(headerSize) {
		panic(errInvalidSharedMem)
	}

	header := (*C.ngx_ip_blocker_shm_st)(unsafe.Pointer(&c.data[0]))
	return (*rwLock)(&header.lock)
}
