// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
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
