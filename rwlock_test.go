// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// GOMAXPROCS=10 go test

package blocker

import (
	"fmt"
	"runtime"
	//. "sync"
	"sync/atomic"
	"testing"
)

func parallelReader(rw *rwLock, clocked, cunlock, cdone chan bool) {
	rw.RLock()
	clocked <- true
	<-cunlock
	rw.RUnlock()
	cdone <- true
}

func doTestParallelReaders(numReaders, gomaxprocs int) {
	runtime.GOMAXPROCS(gomaxprocs)
	var rw rwLock
	rw.Create()
	clocked := make(chan bool)
	cunlock := make(chan bool)
	cdone := make(chan bool)
	for i := 0; i < numReaders; i++ {
		go parallelReader(&rw, clocked, cunlock, cdone)
	}
	// Wait for all parallel RLock()s to succeed.
	for i := 0; i < numReaders; i++ {
		<-clocked
	}
	for i := 0; i < numReaders; i++ {
		cunlock <- true
	}
	// Wait for the goroutines to finish.
	for i := 0; i < numReaders; i++ {
		<-cdone
	}
}

func TestParallelReaders(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(-1))
	doTestParallelReaders(1, 4)
	doTestParallelReaders(3, 4)
	doTestParallelReaders(4, 2)
}

func reader(rw *rwLock, numIterations int, activity *int32, cdone chan bool) {
	for i := 0; i < numIterations; i++ {
		rw.RLock()
		n := atomic.AddInt32(activity, 1)
		if n < 1 || n >= 10000 {
			panic(fmt.Sprintf("wlock(%d)\n", n))
		}
		for i := 0; i < 100; i++ {
		}
		atomic.AddInt32(activity, -1)
		rw.RUnlock()
	}
	cdone <- true
}

func writer(rw *rwLock, numIterations int, activity *int32, cdone chan bool) {
	for i := 0; i < numIterations; i++ {
		rw.Lock()
		n := atomic.AddInt32(activity, 10000)
		if n != 10000 {
			panic(fmt.Sprintf("wlock(%d)\n", n))
		}
		for i := 0; i < 100; i++ {
		}
		atomic.AddInt32(activity, -10000)
		rw.Unlock()
	}
	cdone <- true
}

func HammerRWLock(gomaxprocs, numReaders, numIterations int) {
	runtime.GOMAXPROCS(gomaxprocs)
	// Number of active readers + 10000 * number of active writers.
	var activity int32
	var rw rwLock
	rw.Create()
	cdone := make(chan bool)
	go writer(&rw, numIterations, &activity, cdone)
	var i int
	for i = 0; i < numReaders/2; i++ {
		go reader(&rw, numIterations, &activity, cdone)
	}
	go writer(&rw, numIterations, &activity, cdone)
	for ; i < numReaders; i++ {
		go reader(&rw, numIterations, &activity, cdone)
	}
	// Wait for the 2 writers and all readers to finish.
	for i := 0; i < 2+numReaders; i++ {
		<-cdone
	}
}

func TestRWLock(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(-1))
	n := 1000
	if testing.Short() {
		n = 5
	}
	HammerRWLock(1, 1, n)
	HammerRWLock(1, 3, n)
	HammerRWLock(1, 10, n)
	HammerRWLock(4, 1, n)
	HammerRWLock(4, 3, n)
	HammerRWLock(4, 10, n)
	HammerRWLock(10, 1, n)
	HammerRWLock(10, 3, n)
	HammerRWLock(10, 10, n)
	HammerRWLock(10, 5, n)
}

func TestUnlockPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("unlock of unlocked rwLock did not panic")
		}
	}()
	var rw rwLock
	rw.Create()
	rw.Unlock()
}

func TestUnlockPanic2(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("unlock of unlocked rwLock did not panic")
		}
	}()
	var rw rwLock
	rw.Create()
	rw.RLock()
	rw.Unlock()
}

func TestRUnlockPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("read unlock of unlocked rwLock did not panic")
		}
	}()
	var rw rwLock
	rw.Create()
	rw.RUnlock()
}

func TestRUnlockPanic2(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("read unlock of unlocked rwLock did not panic")
		}
	}()
	var rw rwLock
	rw.Create()
	rw.Lock()
	rw.RUnlock()
}

func BenchmarkRWLockUncontended(b *testing.B) {
	type PaddedRWLock struct {
		rwLock
		pad [32]uint32
	}
	b.RunParallel(func(pb *testing.PB) {
		var rw PaddedRWLock
		rw.Create()
		for pb.Next() {
			rw.RLock()
			rw.RLock()
			rw.RUnlock()
			rw.RUnlock()
			rw.Lock()
			rw.Unlock()
		}
	})
}

func benchmarkRWLock(b *testing.B, localWork, writeRatio int) {
	var rw rwLock
	rw.Create()
	b.RunParallel(func(pb *testing.PB) {
		foo := 0
		for pb.Next() {
			foo++
			if foo%writeRatio == 0 {
				rw.Lock()
				rw.Unlock()
			} else {
				rw.RLock()
				for i := 0; i != localWork; i++ {
					foo *= 2
					foo /= 2
				}
				rw.RUnlock()
			}
		}
		_ = foo
	})
}

func BenchmarkRWLockWrite100(b *testing.B) {
	benchmarkRWLock(b, 0, 100)
}

func BenchmarkRWLockWrite10(b *testing.B) {
	benchmarkRWLock(b, 0, 10)
}

func BenchmarkRWLockWorkWrite100(b *testing.B) {
	benchmarkRWLock(b, 100, 100)
}

func BenchmarkRWLockWorkWrite10(b *testing.B) {
	benchmarkRWLock(b, 100, 10)
}
