// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"encoding/binary"
	"math/rand"
	"sort"
	"testing"
)

func TestBinarySearcherSort(t *testing.T) {
	var strings = [...]string{"Hel", "abc", "foo", "bar", "foo", "f00", "%*&", "^*&", "***"}

	a := newBinarySearcher(len(strings[0]), nil)
	size := a.Size()

	a.Data = make([]byte, len(strings)*size)
	for i, x := range strings {
		copy(a.Data[i*size:(i+1)*size], x)
	}

	a.Sort()

	if !sort.IsSorted(a) {
		t.Error("not sorted after sort")
	}
}

func TestBinarySearcherSortRand(t *testing.T) {
	a := newBinarySearcher(16, nil)
	size := a.Size()

	a.Data = make([]byte, 1000*size)
	for i := 0; i < 1000; i++ {
		rand.Read(a.Data[i*size : (i+1)*size])
	}

	a.Sort()

	if !sort.IsSorted(a) {
		t.Error("not sorted after sort")
	}
}

func testSizePanic(fn func()) (didPanic bool) {
	defer func() {
		err := recover()
		didPanic = err == "invalid size"

		if !didPanic && err != nil {
			panic(err)
		}
	}()

	fn()
	return
}

func TestBinarySearcherSize(t *testing.T) {
	var test [20]byte

	a := newBinarySearcher(16, nil)

	if !testSizePanic(func() {
		a.Index(test[:])
	}) {
		t.Error("did not panic in Index")
	}

	if !testSizePanic(func() {
		a.Contains(test[:])
	}) {
		t.Error("did not panic in Contains")
	}

	if !testSizePanic(func() {
		a.Insert(test[:])
	}) {
		t.Error("did not panic in Insert")
	}

	if !testSizePanic(func() {
		a.Remove(test[:])
	}) {
		t.Error("did not panic in Remove")
	}

	if !testSizePanic(func() {
		a.InsertRange(test[:], 1)
	}) {
		t.Error("did not panic in InsertRange(..., 1)")
	}

	if !testSizePanic(func() {
		a.RemoveRange(test[:], 1)
	}) {
		t.Error("did not panic in RemoveRange(..., 1)")
	}

	if !testSizePanic(func() {
		a.InsertRange(test[:], 0)
	}) {
		t.Error("did not panic in InsertRange(..., 0)")
	}

	if !testSizePanic(func() {
		a.RemoveRange(test[:], 0)
	}) {
		t.Error("did not panic in RemoveRange(..., 0)")
	}
}

func TestBinarySearcherInsert(t *testing.T) {
	a := newBinarySearcher(4, nil)

	var x [4]byte
	for i := 0; i < 1000; i++ {
		incrBytes(x[:])
		incrBytes(x[:])
		a.Insert(x[:])
	}

	if a.Len() != 1000 {
		t.Errorf("invalid length, expected 1000, got %d", a.Len())
	}

	for i := 0; i < a.Len()-1; i++ {
		diff := subBytes(a.Data[(i+1)*4:(i+2)*4], a.Data[i*4:(i+1)*4])
		if diff != 2 {
			t.Errorf("invalid sort, difference should be 2, got %d", diff)
			t.Errorf("\ta: %x, b: %x", a.Data[i*4:(i+1)*4], a.Data[(i+1)*4:(i+2)*4])
		}
	}

	var y [4]byte
	incrBytes(y[:])

	for i := 0; i < 2000; i++ {
		incrBytes(y[:])
		a.Insert(y[:])
	}

	if a.Len() != 2000 {
		t.Errorf("invalid length, expected 2000, got %d", a.Len())
	}

	for i := 0; i < a.Len()-1; i++ {
		diff := subBytes(a.Data[(i+1)*4:(i+2)*4], a.Data[i*4:(i+1)*4])
		if diff != 1 {
			t.Errorf("invalid sort, difference should be 1, got %d", diff)
			t.Errorf("\ta: %x, b: %x", a.Data[i*4:(i+1)*4], a.Data[(i+1)*4:(i+2)*4])
		}
	}
}

func TestBinarySearcherInsertRange(t *testing.T) {
	a := newBinarySearcher(4, nil)

	var x [4]byte
	for i := 0; i < 79; i++ {
		incrBytes(x[:])
	}

	a.InsertRange(x[:], 1747)

	if a.Len() != 1747 {
		t.Errorf("invalid length, expected 1747, got %d", a.Len())
	}

	for i := 0; i < 1747; i++ {
		if !a.Contains(x[:]) {
			t.Errorf("does not contain %x", x)
		}

		if pos := a.Index(x[:]); pos != i {
			t.Errorf("InsertRange inserted at wrong position, expected %d, got %d", i, pos)
		}

		incrBytes(x[:])
	}
}

func TestBinarySearcherRemoveRange(t *testing.T) {
	a := newBinarySearcher(4, nil)

	var x [4]byte
	for i := 0; i < 157; i++ {
		incrBytes(x[:])
	}

	a.InsertRange(x[:], 893)

	if a.Len() != 893 {
		t.Errorf("invalid length, expected 893, got %d", a.Len())
	}

	for i := 0; i < 17; i++ {
		incrBytes(x[:])
	}

	a.RemoveRange(x[:], 731)

	if a.Len() != 162 {
		t.Errorf("invalid length, expected 162, got %d", a.Len())
	}

	var y [4]byte
	for i := 0; i < 157; i++ {
		incrBytes(y[:])
	}

	for i := 0; i < 17; i++ {
		if !a.Contains(y[:]) {
			t.Errorf("does not contain %x", y)
		}

		if pos := a.Index(y[:]); pos != i {
			t.Errorf("wrong position, expected %d, got %d", i, pos)
		}

		incrBytes(y[:])
	}

	for i := 0; i < 731; i++ {
		if a.Contains(y[:]) {
			t.Errorf("contains removed %x", y)
		}

		incrBytes(y[:])
	}

	for i := 0; i < 145; i++ {
		if !a.Contains(y[:]) {
			t.Errorf("does not contain %x", y)
		}

		if pos := a.Index(y[:]); pos != 17+i {
			t.Errorf("wrong position, expected %d, got %d", 17+i, pos)
		}

		incrBytes(y[:])
	}
}

func TestBinarySearcherRemove(t *testing.T) {
	a := newBinarySearcher(4, nil)

	var x [4]byte
	for i := 0; i < 21; i++ {
		incrBytes(x[:])
	}

	a.InsertRange(x[:], 1342)

	if a.Len() != 1342 {
		t.Errorf("invalid length, expected 1342, got %d", a.Len())
	}

	for i := 0; i < 1342; i += 2 {
		incrBytes(x[:])
		a.Remove(x[:])
		incrBytes(x[:])
	}

	var y [4]byte
	for i := 0; i < 21; i++ {
		incrBytes(y[:])
	}

	for i, j := 0, 0; i < 1342; i++ {
		if i%2 == 0 {
			if !a.Contains(y[:]) {
				t.Errorf("does not contain %x", y)
			}

			if pos := a.Index(y[:]); pos != j {
				t.Errorf("wrong position, expected %d, got %d", j, pos)
			}

			j++
		} else {
			if a.Contains(y[:]) {
				t.Errorf("contains %x", y)
			}
		}

		incrBytes(y[:])
	}
}

func subBytes32(x, y []byte) int {
	const maxInt = int(^uint(0) >> 1)

	if len(x) != len(y) {
		panic("different lengths")
	}

	l := len(x)
	var size int

	switch {
	case l >= 8:
		size = 4
	case l > 4:
		panic("invalid length")
	case l == 4:
		size = 4
	case l == 3:
		panic("invalid length")
	case l == 2:
		size = 2
	case l == 1:
		size = 1
	default:
		return 0
	}

	l -= size

	for i := 0; i < l; i++ {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return 0
			}

			return maxInt
		}
	}

	var a, b uint32

	switch size {
	case 4:
		a = binary.BigEndian.Uint32(x[l:])
		b = binary.BigEndian.Uint32(y[l:])
	case 2:
		a = uint32(binary.BigEndian.Uint16(x[l:]))
		b = uint32(binary.BigEndian.Uint16(y[l:]))
	case 1:
		a = uint32(x[l])
		b = uint32(y[l])
	}

	if a <= b {
		return 0
	}

	c := a - b

	if uint(c) >= uint(maxInt) {
		return maxInt
	}

	return int(c)
}

func subBytes64(x, y []byte) int {
	const maxInt = int(^uint(0) >> 1)

	if len(x) != len(y) {
		panic("different lengths")
	}

	l := len(x)

	switch {
	case l >= 8:
	case l > 4:
		panic("invalid length")
	default:
		return subBytes32(x, y)
	}

	l -= 8

	for i := 0; i < l; i++ {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return 0
			}

			return maxInt
		}
	}

	a := binary.BigEndian.Uint64(x[l:])
	b := binary.BigEndian.Uint64(y[l:])

	if a <= b {
		return 0
	}

	c := a - b

	if uint(c) >= uint(maxInt) {
		return maxInt
	}

	return int(c)
}

var subBytes func(x, y []byte) int

func init() {
	if ^uint(0) == uint(^uint32(0)) {
		subBytes = subBytes32
	} else {
		subBytes = subBytes64
	}
}
