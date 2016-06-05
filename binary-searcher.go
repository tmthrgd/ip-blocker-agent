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

func (s *binarySearcher) InsertRange(base []byte, num int) []byte {
	if len(base) != s.size {
		panic("invalid size")
	}

	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)

	x := append([]byte(nil), base...)

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

	return x
}

func (s *binarySearcher) RemoveRange(base []byte, num int) []byte {
	if len(base) != s.size {
		panic("invalid size")
	}

	x := append([]byte(nil), base...)

	for num > 0 {
		pos, has := s.search(x)
		if !has {
			if pos*s.size == len(s.Data) {
				break
			}

			by := subBytes(s.Data[pos*s.size:(pos+1)*s.size], x)
			num -= by

			for ; by > 0; by-- {
				incrBytes(x)
			}

			continue
		}

		startPos := pos

		for num > 0 && pos*s.size < len(s.Data) && s.compare(x, s.Data[pos*s.size:(pos+1)*s.size]) == 0 {
			incrBytes(x)

			num--
			pos++
		}

		s.Data = append(s.Data[:startPos*s.size], s.Data[pos*s.size:]...)
	}

	return x
}

func (s *binarySearcher) Clear() {
	if cap(s.Data) <= int(pageSize) {
		s.Data = s.Data[:0]
	} else {
		s.Data = nil
	}
}
