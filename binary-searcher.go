// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"bytes"
	"sort"
)

type binarySearcher struct {
	Data []byte

	size int

	compare func(a, b []byte) int

	buffer []byte
}

func newBinarySearcher(size int, compare func(a, b []byte) int) *binarySearcher {
	if compare == nil {
		compare = bytes.Compare
	}

	return &binarySearcher{
		size: size,

		compare: compare,

		buffer: make([]byte, size),
	}
}

func (s *binarySearcher) Len() int {
	return len(s.Data) / s.size
}

func (s *binarySearcher) Less(i, j int) bool {
	return s.compare(s.Data[i*s.size:(i+1)*s.size], s.Data[j*s.size:(j+1)*s.size]) < 0
}

func (s *binarySearcher) Swap(i, j int) {
	copy(s.buffer, s.Data[i*s.size:(i+1)*s.size])
	copy(s.Data[i*s.size:(i+1)*s.size], s.Data[j*s.size:(j+1)*s.size])
	copy(s.Data[j*s.size:(j+1)*s.size], s.buffer)
}

func (s *binarySearcher) Sort() {
	sort.Sort(s)
}

func (s *binarySearcher) Size() int {
	return s.size
}

func (s *binarySearcher) Index(check []byte) int {
	if len(check) != s.size {
		panic("invalid size")
	}

	return sort.Search(s.Len(), func(i int) bool {
		return s.compare(s.Data[i*s.size:(i+1)*s.size], check) >= 0
	})
}

func (s *binarySearcher) search(check []byte) (pos int, has bool) {
	pos = s.Index(check)
	has = pos*s.size < len(s.Data) && s.compare(check, s.Data[pos*s.size:(pos+1)*s.size]) == 0
	return
}

func (s *binarySearcher) Search(check []byte) []byte {
	pos, has := s.search(check)
	if has {
		return s.Data[pos*s.size : (pos+1)*s.size]
	}

	return nil
}

func (s *binarySearcher) Contains(check []byte) bool {
	_, has := s.search(check)
	return has
}

func (s *binarySearcher) Insert(b []byte) bool {
	pos, has := s.search(b)
	if has {
		return false
	}

	s.Data = append(s.Data, b...)
	copy(s.Data[(pos+1)*s.size:], s.Data[pos*s.size:])
	copy(s.Data[pos*s.size:(pos+1)*s.size], b)
	return true
}

func (s *binarySearcher) Replace(b []byte) bool {
	if pos, has := s.search(b); has {
		copy(s.Data[pos*s.size:(pos+1)*s.size], b)
		return true
	}

	return false
}

func (s *binarySearcher) InsertOrReplace(b []byte) {
	pos, has := s.search(b)
	if has {
		copy(s.Data[pos*s.size:(pos+1)*s.size], b)
		return
	}

	s.Data = append(s.Data, b...)
	copy(s.Data[(pos+1)*s.size:], s.Data[pos*s.size:])
	copy(s.Data[pos*s.size:(pos+1)*s.size], b)
}

func (s *binarySearcher) Remove(b []byte) bool {
	pos, has := s.search(b)
	if has {
		s.Data = append(s.Data[:pos*s.size], s.Data[(pos+1)*s.size:]...)
		return true
	}

	return false
}

func (s *binarySearcher) Clear() {
	if cap(s.Data) <= int(pageSize) {
		s.Data = s.Data[:0]
	} else {
		s.Data = nil
	}
}
