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

	data, err := unix.Mmap(int(file.Fd()), 0, int(headerSize), unix.PROT_READ, unix.MAP_SHARED)
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
	for v := atomic.LoadUint64(addr); v != 0 && v != ^uint64(0); {
		if atomic.CompareAndSwapUint64(addr, v, v+1) {
			return true
		}

		v = atomic.LoadUint64(addr)
	}

	return false
}

func releaseLock(addr *uint64) {
	if atomic.AddUint64(addr, ^uint64(0)) == 0 {
		panic("ip-blocker-agent: unlock of unlocked mutex")
	}
}

var errLockFailure = errors.New("failed to acquite lock")

func (c *Client) do(block *ipBlock, fn func([]byte) error) error {
redo:
	offset := int64(atomic.LoadUint64(&block.base))
	if offset == 0 {
		return nil
	}

	if err := doMmap(c.file, offset, int(blockHeaderSize), true, func(data []byte) error {
		bh := caseToBlockHeader(data)
		if !takeLock(&bh.locks) {
			return errLockFailure
		}
		defer releaseLock(&bh.locks)

		return doMmap(c.file, offset, int(blockHeaderSize)+int(bh.len), false, func(data []byte) error {
			return fn(data[int(blockHeaderSize):])
		})
	}); err != nil {
		if err == errLockFailure {
			goto redo
		}

		return err
	}

	return nil
}

func (c *Client) contains(ip net.IP, block *ipBlock) (has bool, err error) {
	return has, c.do(block, func(data []byte) error {
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

	h := castToHeader(c.hdrData)

	if err := c.do(&h.ip4, func(data []byte) error {
		ip4 = len(data) / net.IPv4len
		return nil
	}); err != nil {
		return 0, 0, 0, err
	}

	if err := c.do(&h.ip6, func(data []byte) error {
		ip6 = len(data) / net.IPv6len
		return nil
	}); err != nil {
		return 0, 0, 0, err
	}

	if err := c.do(&h.ip6Route, func(data []byte) error {
		ip6routes = len(data) / (net.IPv6len / 2)
		return nil
	}); err != nil {
		return 0, 0, 0, err
	}

	return
}
