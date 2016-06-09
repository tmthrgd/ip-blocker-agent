// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"bytes"
	"sort"
	"sync"
)

var defaultCompare = bytes.Compare

type binarySearcher struct {
	Data []byte

	size int

	compare func(a, b []byte) int

	buffer []byte
}

func newBinarySearcher(size int, compare func(a, b []byte) int) *binarySearcher {
	if compare == nil {
		compare = defaultCompare
	}

	return &binarySearcher{
		size: size,

		compare: compare,
	}
}

func (s *binarySearcher) Len() int {
	return len(s.Data) / s.size
}

func (s *binarySearcher) Less(i, j int) bool {
	return s.compare(s.Data[i*s.size:(i+1)*s.size], s.Data[j*s.size:(j+1)*s.size]) < 0
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
		return s.compare(s.Data[i*s.size:(i+1)*s.size], check) >= 0
	})
}

func (s *binarySearcher) search(check []byte) (pos int, has bool) {
	pos = s.Index(check)
	has = pos*s.size < len(s.Data) && s.compare(check, s.Data[pos*s.size:(pos+1)*s.size]) == 0
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

var bufPool = &sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func (s *binarySearcher) InsertRange(base []byte, num int) {
	s.insertRange(append([]byte(nil), base...), num)
}

func (s *binarySearcher) insertRange(base []byte, num int) {
	if len(base) != s.size {
		panic("invalid size")
	}

	x := base

	buf := bufPool.Get().(*bytes.Buffer)

	for num > 0 {
		pos, has := s.search(x)
		if has {
			for {
				incrBytes(x)

				num--
				pos++

				if num <= 0 || pos*s.size >= len(s.Data) || s.compare(x, s.Data[pos*s.size:(pos+1)*s.size]) != 0 {
					break
				}
			}

			if num <= 0 {
				break
			}
		}

		var toInsert int

		if pos*s.size == len(s.Data) {
			toInsert = num
		} else {
			toInsert = subBytes(s.Data[pos*s.size:(pos+1)*s.size], x)
			if toInsert > num {
				toInsert = num
			}
		}

		if toInsert > 1<<16 {
			toInsert = 1 << 16
		}

		num -= toInsert

		buf.Reset()
		buf.Grow(toInsert * s.size)
		buf.Write(x)

		for i := 1; i < toInsert; i++ {
			incrBytes(x)
			buf.Write(x)
		}

		b := buf.Bytes()

		s.Data = append(s.Data, b...)
		copy(s.Data[(pos+toInsert)*s.size:], s.Data[pos*s.size:])
		copy(s.Data[pos*s.size:(pos+toInsert)*s.size], b)

		if num != 0 {
			incrBytes(x)
		}
	}

	bufPool.Put(buf)
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
