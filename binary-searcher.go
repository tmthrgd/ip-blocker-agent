// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"bytes"
	"sort"

	"github.com/tmthrgd/ip-blocker-agent/internal/incr"
)

type binarySearcher struct {
	Data []byte

	size int

	buffer []byte
}

func (s *binarySearcher) Len() int {
	return len(s.Data) / s.size
}

func (s *binarySearcher) Less(i, j int) bool {
	return bytes.Compare(s.Data[i*s.size:(i+1)*s.size], s.Data[j*s.size:(j+1)*s.size]) < 0
}

func (s *binarySearcher) Swap(i, j int) {
	if s.buffer == nil {
		s.buffer = make([]byte, s.size)
	}

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
		return bytes.Compare(s.Data[i*s.size:(i+1)*s.size], check) >= 0
	})
}

func (s *binarySearcher) search(check []byte) (pos int, has bool) {
	pos = s.Index(check)
	has = pos*s.size < len(s.Data) && bytes.Equal(check, s.Data[pos*s.size:(pos+1)*s.size])
	return
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

func (s *binarySearcher) Remove(b []byte) bool {
	pos, has := s.search(b)
	if has {
		s.Data = append(s.Data[:pos*s.size], s.Data[(pos+1)*s.size:]...)
		return true
	}

	return false
}

func (s *binarySearcher) InsertRange(base []byte, num int) {
	startPos := s.Index(base)
	var endPos int

	if startPos*s.size == len(s.Data) {
		endPos = s.Len()
	} else {
		if s.buffer == nil {
			s.buffer = make([]byte, s.size)
		}

		end := s.buffer
		copy(end, base)

		if addIntToBytes(end, num) {
			panic("overflow")
		}

		endPos = s.Index(end)
		if endPos-startPos == num {
			return
		}
	}

	if need := (s.Len() - (endPos - startPos) + num) * s.size; cap(s.Data) < need {
		data := make([]byte, need, need+(need>>3) /*= need * 1.125*/)
		copy(data, s.Data[:startPos*s.size])
		copy(data[(startPos+num)*s.size:], s.Data[endPos*s.size:])
		s.Data = data
	} else {
		s.Data = s.Data[:need]
		copy(s.Data[(startPos+num)*s.size:], s.Data[endPos*s.size:])
	}

	incr.IncrementBytes(base, s.Data[startPos*s.size:(startPos+num)*s.size])
}

func (s *binarySearcher) RemoveRange(base []byte, num int) {
	startPos := s.Index(base)
	if startPos*s.size == len(s.Data) {
		return
	}

	if s.buffer == nil {
		s.buffer = make([]byte, s.size)
	}

	end := s.buffer
	copy(end, base)

	var endPos int

	if addIntToBytes(end, num) {
		endPos = s.Len()
	} else {
		endPos = s.Index(end)
	}

	if startPos != endPos {
		s.Data = append(s.Data[:startPos*s.size], s.Data[endPos*s.size:]...)
	}
}

func (s *binarySearcher) Clear() {
	if cap(s.Data) <= int(pageSize) {
		s.Data = s.Data[:0]
	} else {
		s.Data = nil
	}
}
