// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"errors"
	"net"
	"os"
	"sync"
	"sync/atomic"

	searcher "github.com/tmthrgd/binary-searcher"
	"github.com/tmthrgd/go-shm"
	"golang.org/x/sys/unix"
)

// Client is an IP blocker shared memory client.
type Client struct {
	file *os.File

	hdrData []byte

	dmu  sync.RWMutex
	dwg  sync.WaitGroup
	data []byte

	mu sync.RWMutex

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

	if stat.Size() < int64(headerSize) {
		file.Close()
		return nil, ErrInvalidSharedMemory
	}

	data, err := unix.Mmap(int(file.Fd()), 0, int(headerSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, err
	}

	header := castToHeader(data)

	if atomic.LoadUint32(&header.version) != version {
		unix.Munmap(data)
		file.Close()
		return nil, ErrInvalidSharedMemory
	}

	return &Client{
		file: file,

		hdrData: data,
	}, nil
}

func takeLock(addr *uint64) bool {
	for v := atomic.LoadUint64(addr); v < ^uint64(0)-1; {
		if atomic.CompareAndSwapUint64(addr, v, v+1) {
			return true
		}

		v = atomic.LoadUint64(addr)
	}

	return false
}

func releaseLock(addr *uint64) {
	if atomic.AddUint64(addr, ^uint64(0)) == ^uint64(0) {
		panic("ip-blocker-agent: unlock of unlocked mutex")
	}
}

func (c *Client) mmap(offset, length uint64) ([]byte, *sync.WaitGroup, error) {
	size := int(offset + length)
	if size <= 0 || uint64(size) < offset {
		return nil, nil, errors.New("overflow")
	}

	c.dmu.RLock()
	if size <= len(c.data) {
		c.dwg.Add(1)
		c.dmu.RUnlock()
		return c.data[offset:size], &c.dwg, nil
	}

	c.dmu.RUnlock()

	c.dmu.Lock()
	defer c.dmu.Unlock()

	if size <= len(c.data) {
		c.dwg.Add(1)
		return c.data[offset:size], &c.dwg, nil
	}

	c.dwg.Wait()

	if c.data != nil {
		if err := unix.Munmap(c.data); err != nil {
			return nil, nil, err
		}
	}

	c.data = nil

	data, err := unix.Mmap(int(c.file.Fd()), 0, size, unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, nil, err
	}

	c.data = data

	c.dwg.Add(1)
	return data[offset:size], &c.dwg, nil
}

func (c *Client) doBlock(blockIdx *uint32, fn func(block *ipBlock) error) error {
	h := castToHeader(c.hdrData)

	var block *ipBlock
	for {
		idx := atomic.LoadUint32(blockIdx)
		if idx >= uint32(len(h.blocks)) {
			return errors.New("invalid block index")
		}

		block = &h.blocks[idx]

		if takeLock(&block.locks) {
			break
		}
	}
	defer releaseLock(&block.locks)

	return fn(block)
}

func (c *Client) do(blockIdx *uint32, fn func([]byte) error) error {
	return c.doBlock(blockIdx, func(block *ipBlock) error {
		if block.base == 0 {
			return nil
		}

		data, wg, err := c.mmap(block.base, block.len)
		if err != nil {
			return err
		}
		defer wg.Done()

		return fn(data)
	})
}

func (c *Client) contains(ip net.IP, blockIdx *uint32) (has bool, err error) {
	return has, c.do(blockIdx, func(data []byte) error {
		has = searcher.New(data, len(ip)).Contains(ip)
		return nil
	})
}

// Contains returns a boolean indicating whether the
// IP address is in the blocklist.
func (c *Client) Contains(ip net.IP) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false, ErrClosed
	}

	h := castToHeader(c.hdrData)

	if ip4 := ip.To4(); ip4 != nil {
		return c.contains(ip4, &h.ip4)
	}

	if ip6 := ip.To16(); ip6 != nil {
		if has, err := c.contains(ip6[:net.IPv6len/2], &h.ip6Route); has || err != nil {
			return has, err
		}

		return c.contains(ip6, &h.ip6)
	}

	return false, &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
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

	if c.hdrData != nil {
		if err := unix.Munmap(c.hdrData); err != nil {
			return err
		}

		c.hdrData = nil
	}

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

func (c *Client) count(blockIdx *uint32, len int) (count int, err error) {
	return count, c.doBlock(blockIdx, func(block *ipBlock) error {
		count = int(block.len / uint64(len))
		return nil
	})
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

	h := castToHeader(c.hdrData)

	if ip4, err = c.count(&h.ip4, net.IPv4len); err != nil {
		return 0, 0, 0, err
	}

	if ip6, err = c.count(&h.ip6, net.IPv6len); err != nil {
		return 0, 0, 0, err
	}

	if ip6routes, err = c.count(&h.ip6Route, net.IPv6len/2); err != nil {
		return 0, 0, 0, err
	}

	return
}
