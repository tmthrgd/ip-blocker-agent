// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmthrgd/binary-searcher"
	"github.com/tmthrgd/go-shm"
	"github.com/tmthrgd/ip-blocker-agent/internal/incr"
	"golang.org/x/sys/unix"
)

const cachelineSize = 64

var pageSize = os.Getpagesize()

/* ngx_align, taken from ngx_config.h */
func align(d, a int) int {
	return (d + (a - 1)) &^ (a - 1)
}

func alignPS(d int) int {
	return align(d, pageSize)
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
	return shm.Unlink(name)
}

// Server is an IP blocker shared memory server.
type Server struct {
	file *os.File

	ip4s  searcher.BinarySearcher
	ip6s  searcher.BinarySearcher
	ip6rs searcher.BinarySearcher

	hdrData []byte

	mu sync.Mutex

	closed   bool
	batching bool
}

// New creates a new IP blocker shared memory server
// with specified name and permissions.
//
// This will fail if a shared memory region has already
// been created with the same name and not unlinked.
func New(name string, perm os.FileMode) (*Server, error) {
	file, err := shm.Open(name, os.O_CREATE|os.O_EXCL|os.O_TRUNC|os.O_RDWR, perm)
	if err != nil {
		return nil, err
	}

	if err = file.Truncate(int64(headerSize)); err != nil {
		return nil, err
	}

	data, err := unix.Mmap(int(file.Fd()), 0, int(headerSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	header := castToHeader(data)

	atomic.StoreUint32((*uint32)(&header.version), version)

	return &Server{
		file: file,

		ip4s:  searcher.BinarySearcher{Size: net.IPv4len, IncrementBytes: incr.IncrementBytes},
		ip6s:  searcher.BinarySearcher{Size: net.IPv6len, IncrementBytes: incr.IncrementBytes},
		ip6rs: searcher.BinarySearcher{Size: net.IPv6len / 2, IncrementBytes: incr.IncrementBytes},

		hdrData: data,
	}, nil
}

func (s *Server) increaseSize(n int64) (int64, error) {
	fi, err := s.file.Stat()
	if err != nil {
		return 0, err
	}

	size := fi.Size()
	size = int64(alignPS(int(size)))
	if size+n < n {
		return 0, errors.New("size overflows")
	}

	if err := s.file.Truncate(size + n); err != nil {
		return 0, err
	}

	return size, nil
}

func doMmap(f *os.File, offset int64, len int, rw bool, fn func([]byte) error) error {
	prot := unix.PROT_READ
	if rw {
		prot |= unix.PROT_WRITE
	}

	data, err := unix.Mmap(int(f.Fd()), offset, len, prot, unix.MAP_SHARED)
	runtime.KeepAlive(f)
	if err != nil {
		return err
	}
	defer unix.Munmap(data)

	return fn(data)
}

func (s *Server) commitBlock(blockIdx *uint32, bs *searcher.BinarySearcher) error {
	h := castToHeader(s.hdrData)

	newIdx := ^uint32(0)
	for i := range h.blocks {
		if i := uint32(i); h.ip4 != i && h.ip6 != i && h.ip6Route != i {
			newIdx = i
			break
		}
	}

	if newIdx >= uint32(len(h.blocks)) {
		return errors.New("failed to find free block")
	}

	oldBlock := &h.blocks[*blockIdx]
	newBlock := &h.blocks[newIdx]

	if lock := atomic.LoadUint64(&newBlock.locks); lock != 0 && lock != ^uint64(0) {
		panic("block still in use")
	}

	if len(bs.Data) != 0 {
		offset, err := s.increaseSize(int64(len(bs.Data)))
		if err != nil {
			return err
		}

		if err := doMmap(s.file, offset, len(bs.Data), true, func(data []byte) error {
			copy(data, bs.Data)
			return nil
		}); err != nil {
			return err
		}

		*newBlock = ipBlock{
			base: uint64(offset),
			len:  uint64(len(bs.Data)),
		}
	} else {
		*newBlock = ipBlock{}
	}

	atomic.StoreUint32(blockIdx, newIdx)

	for atomic.CompareAndSwapUint64(&oldBlock.locks, 0, ^uint64(0)) {
		time.Sleep(50 * time.Microsecond)
	}

	base, len := oldBlock.base, oldBlock.len
	*oldBlock = ipBlock{}

	if base == 0 {
		return nil
	}

	err := unix.Fallocate(int(s.file.Fd()), unix.FALLOC_FL_PUNCH_HOLE|unix.FALLOC_FL_KEEP_SIZE, int64(base), int64(len))
	runtime.KeepAlive(s.file)
	return err
}

func (s *Server) commit() error {
	s.batching = false

	h := castToHeader(s.hdrData)

	if err := s.commitBlock(&h.ip4, &s.ip4s); err != nil {
		return err
	}

	if err := s.commitBlock(&h.ip6, &s.ip6s); err != nil {
		return err
	}

	return s.commitBlock(&h.ip6Route, &s.ip6rs)
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

	var ips *searcher.BinarySearcher

	if ip4 := masked.To4(); ip4 != nil {
		ip = ip4
		ips = &s.ip4s
	} else if ip6 := masked.To16(); ip6 != nil {
		ip = ip6

		if ones, _ := ipnet.Mask.Size(); ones <= s.ip6rs.Size*8 {
			ips = &s.ip6rs
		} else {
			ips = &s.ip6s
		}
	} else {
		return &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}

	base := ip[:ips.Size]
	ones, _ := ipnet.Mask.Size()
	ones = len(base)*8 - ones

	if (^uint(0) == uint(^uint32(0)) && ones > 30) || (^uint(0) != uint(^uint32(0)) && ones > 62) {
		return errRangeTooLarge
	}

	if insert {
		ips.InsertRange(base, 1<<uint(ones))
	} else {
		ips.RemoveRange(base, 1<<uint(ones))
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

const serializedHeader = "ip-blocker-agent-v1\x00\xb1\x0c\x11\x57"

// Save serializes the blocklist into w.
//
// The server can be recreated later with Load.
func (s *Server) Save(w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}

	if _, err := io.WriteString(w, serializedHeader); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, uint64(len(s.ip4s.Data))); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, uint64(len(s.ip6s.Data))); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, uint64(len(s.ip6rs.Data))); err != nil {
		return err
	}

	if _, err := w.Write(s.ip4s.Data); err != nil {
		return err
	}

	if _, err := w.Write(s.ip6s.Data); err != nil {
		return err
	}

	if _, err := w.Write(s.ip6rs.Data); err != nil {
		return err
	}

	return nil
}

// Load loads the serialised blocklist in r into s.
//
// If presently batching, Load() will not commit the
// changes to shared memory.
//
// It will fail if the current blocklist is not empty
// or r contains invalid data.
func (s *Server) Load(r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}

	var header [len(serializedHeader)]byte

	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}

	if string(header[:]) != serializedHeader {
		return InvalidDataError{errInvalidHeader}
	}

	var l4, l6, l6r uint64

	if err := binary.Read(r, binary.BigEndian, &l4); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &l6); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &l6r); err != nil {
		return err
	}

	if l4%4 != 0 || l6%16 != 0 || l6r%8 != 0 {
		return InvalidDataError{errInvalidHeader}
	}

	s.ip4s.Data = make([]byte, l4)
	s.ip6s.Data = make([]byte, l6)
	s.ip6rs.Data = make([]byte, l6r)

	if _, err := io.ReadFull(r, s.ip4s.Data); err != nil {
		return err
	}

	if _, err := io.ReadFull(r, s.ip6s.Data); err != nil {
		return err
	}

	if _, err := io.ReadFull(r, s.ip6rs.Data); err != nil {
		return err
	}

	if s.batching {
		return nil
	}

	return s.commit()
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

	s.ip4s.Clear()
	s.ip6s.Clear()
	s.ip6rs.Clear()

	if err := unix.Munmap(s.hdrData); err != nil {
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
		return shm.Unlink(s.file.Name())
	}

	if err := s.close(); err != nil {
		return err
	}

	return shm.Unlink(s.file.Name())
}

// IsBatching returns a boolean indicating whether the
// server is currently batching operations.
func (s *Server) IsBatching() bool {
	s.mu.Lock()
	isBatching := !s.closed && s.batching
	s.mu.Unlock()
	return isBatching
}

// Name returns the name of the shared memory.
func (s *Server) Name() string {
	return s.file.Name()
}

// Count returns the number of IPv4 addresses, IPv6
// address and IPv6 routes stored in the blocklist.
//
// It only considers those committed to shared memory.
// It will return 'stale' results if batching.
//
// Will fail if Closed() has been called.
func (s *Server) Count() (ip4, ip6, ip6routes int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		err = ErrClosed
		return
	}

	h := castToHeader(s.hdrData)

	ip4 = int(h.blocks[h.ip4].len / net.IPv4len)
	ip6 = int(h.blocks[h.ip6].len / net.IPv6len)
	ip6routes = int(h.blocks[h.ip6Route].len / (net.IPv6len / 2))

	return
}
