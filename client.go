// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"net"
	"os"
	"sync"
	"sync/atomic"

	"github.com/tmthrgd/binary-searcher"
	"github.com/tmthrgd/go-shm"
	"golang.org/x/sys/unix"
)

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
	file, err := shm.Open(name, os.O_RDWR, 0)
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
		return nil, ErrInvalidSharedMemory
	}

	data, err := unix.Mmap(int(file.Fd()), 0, int(size), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, err
	}

	header := castToHeader(&data[0])

	if atomic.LoadUint32((*uint32)(&header.Version)) != version {
		file.Close()
		return nil, ErrInvalidSharedMemory
	}

	client := &Client{
		file: file,

		data: data,
	}

	lock := (*rwLock)(&header.Lock)
	lock.RLock()

	client.revision = uint32(header.Revision)

	stat, err = file.Stat()
	if err != nil {
		lock.RUnlock()

		unix.Munmap(data)
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

		header = castToHeader(&client.data[0])
		lock = (*rwLock)(&header.Lock)
	} else if !client.checkSharedMemory() {
		lock.RUnlock()

		unix.Munmap(data)
		file.Close()
		return nil, ErrInvalidSharedMemory
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
		defer func() {
			c.mu.Unlock()
			c.mu.RLock()

			if err != nil {
				return
			}

			header := castToHeader(&c.data[0])
			if c.revision != uint32(header.Revision) {
				err = c.remap(false)
			}
		}()

		if c.closed {
			return ErrClosed
		}

		header := castToHeader(&c.data[0])
		if c.revision == uint32(header.Revision) {
			return nil
		}
	}

	data := c.data
	c.data = nil

	stat, err := c.file.Stat()
	if err != nil {
		goto err
	}

	c.data, err = unix.Mmap(int(c.file.Fd()), 0, int(stat.Size()), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		goto err
	}

	if !c.checkSharedMemory() {
		err = ErrInvalidSharedMemory
		goto err
	}

	c.revision = uint32(castToHeader(&c.data[0]).Revision)

	err = unix.Munmap(data)
	return

err:
	if len(c.data) == 0 || len(c.data) >= int(headerSize) {
		header := castToHeader(&data[0])
		lock := (*rwLock)(&header.Lock)
		lock.RUnlock()
	} else {
		err = LockReleaseFailedError{err}
	}

	unix.Munmap(data)
	return
}

func (c *Client) checkSharedMemory() bool {
	if c.closed {
		panic(ErrClosed)
	}

	header := castToHeader(&c.data[0])

	const maxInt = int(^uint(0) >> 1)
	return len(c.data) >= int(headerSize) &&
		uintptr(headerSize)+uintptr(header.IP4.Len+header.IP6.Len+header.IP6Route.Len) <= uintptr(maxInt) &&
		len(c.data) >= int(headerSize)+int(header.IP4.Len+header.IP6.Len+header.IP6Route.Len) &&
		(header.IP4.Len == 0 || uintptr(header.IP4.Base) >= headerSize) &&
		(header.IP6.Len == 0 || uintptr(header.IP6.Base) >= headerSize) &&
		(header.IP6Route.Len == 0 || uintptr(header.IP6Route.Base) >= headerSize) &&
		uintptr(header.IP4.Base)+uintptr(header.IP4.Len) <= uintptr(maxInt) &&
		uintptr(header.IP6.Base)+uintptr(header.IP6.Len) <= uintptr(maxInt) &&
		uintptr(header.IP6Route.Base)+uintptr(header.IP6Route.Len) <= uintptr(maxInt) &&
		int(uintptr(header.IP4.Base)+uintptr(header.IP4.Len)) <= len(c.data) &&
		int(uintptr(header.IP6.Base)+uintptr(header.IP6.Len)) <= len(c.data) &&
		int(uintptr(header.IP6Route.Base)+uintptr(header.IP6Route.Len)) <= len(c.data) &&
		header.IP4.Len%4 == 0 &&
		header.IP6.Len%16 == 0 &&
		header.IP6Route.Len%8 == 0
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
		return false, ErrInvalidSharedMemory
	}

	header := castToHeader(&c.data[0])
	lock := (*rwLock)(&header.Lock)

	lock.RLock()

	if c.revision != uint32(header.Revision) {
		/* RUnlock is called inside of remap iff an error is returned */
		if err := c.remap(false); err != nil {
			return false, err
		}

		header = castToHeader(&c.data[0])
		lock = (*rwLock)(&header.Lock)
	}

	defer lock.RUnlock()

	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4

		if header.IP4.Len == 0 {
			return false, nil
		}

		end := int(header.IP4.Base) + int(header.IP4.Len)
		searcher := searcher.New(c.data[header.IP4.Base:end:end], net.IPv4len)
		return searcher.Contains(ip), nil
	} else if ip6 := ip.To16(); ip6 != nil {
		ip = ip6

		if header.IP6Route.Len != 0 {
			end := int(header.IP6Route.Base) + int(header.IP6Route.Len)
			searcher := searcher.New(c.data[header.IP6Route.Base:end:end], net.IPv6len/2)

			if searcher.Contains(ip[:net.IPv6len/2]) {
				return true, nil
			}
		}

		if header.IP6.Len == 0 {
			return false, nil
		}

		end := int(header.IP6.Base) + int(header.IP6.Len)
		searcher := searcher.New(c.data[header.IP6.Base:end:end], net.IPv6len)
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
		if err := unix.Munmap(c.data); err != nil {
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
		err = ErrInvalidSharedMemory
		return
	}

	header := castToHeader(&c.data[0])

	lock := (*rwLock)(&header.Lock)
	lock.RLock()

	ip4 = int(header.IP4.Len / net.IPv4len)
	ip6 = int(header.IP6.Len / net.IPv6len)
	ip6routes = int(header.IP6Route.Len / (net.IPv6len / 2))

	lock.RUnlock()
	return
}
