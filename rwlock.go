// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package blocker

import (
	"sync/atomic"

	"github.com/tmthrgd/go-sem"
)

// A rwLock is a reader/writer mutual exclusion lock.
// The lock can be held by an arbitrary number of readers
// or a single writer.
// rwLocks can be created as part of other structures.
// Create must be called before use.
//
// type rwLock C.ip_blocker_rwlock_st
// 	see blocker.go

func (rw *rwLock) Create() {
	w := (*mutex)(&rw.W)
	w.Create()

	writerSem := (*sem.Semaphore)(&rw.WriterSem)
	if err := writerSem.Init(0); err != nil {
		panic(err)
	}

	readerSem := (*sem.Semaphore)(&rw.ReaderSem)
	if err := readerSem.Init(0); err != nil {
		panic(err)
	}

	rw.ReaderCount = 0
	rw.ReaderWait = 0
}

// Lock locks rw for writing.
// If the lock is already locked for reading or writing,
// Lock blocks until the lock is available.
// To ensure that the lock eventually becomes available,
// a blocked Lock call excludes new readers from acquiring
// the lock.
func (rw *rwLock) Lock() {
	// First, resolve competition with other writers.
	w := (*mutex)(&rw.W)
	w.Lock()

	// Announce to readers there is a pending writer.
	r := atomic.AddInt32((*int32)(&rw.ReaderCount), -rwLockMaxReaders) + rwLockMaxReaders

	// Wait for active readers.
	if r != 0 && atomic.AddInt32((*int32)(&rw.ReaderWait), r) != 0 {
		writerSem := (*sem.Semaphore)(&rw.WriterSem)
		if err := writerSem.Wait(); err != nil {
			panic(err)
		}
	}
}

// Unlock unlocks rw for writing.  It is a run-time error if rw is
// not locked for writing on entry to Unlock.
//
// As with Mutexes, a locked rwLock is not associated with a particular
// goroutine.  One goroutine may RLock (Lock) an rwLock and then
// arrange for another goroutine to RUnlock (Unlock) it.
func (rw *rwLock) Unlock() {
	// Announce to readers there is no active writer.
	r := atomic.AddInt32((*int32)(&rw.ReaderCount), rwLockMaxReaders)
	if r >= rwLockMaxReaders {
		panic("sync: Unlock of unlocked rwLock")
	}

	// Unblock blocked readers, if any.
	readerSem := (*sem.Semaphore)(&rw.ReaderSem)
	for i := 0; i < int(r); i++ {
		if err := readerSem.Post(); err != nil {
			panic(err)
		}
	}

	// Allow other writers to proceed.
	w := (*mutex)(&rw.W)
	w.Unlock()
}

// RLock locks rw for reading.
func (rw *rwLock) RLock() {
	if atomic.AddInt32((*int32)(&rw.ReaderCount), 1) < 0 {
		// A writer is pending, wait for it.
		readerSem := (*sem.Semaphore)(&rw.ReaderSem)
		if err := readerSem.Wait(); err != nil {
			panic(err)
		}
	}
}

// RUnlock undoes a single RLock call;
// it does not affect other simultaneous readers.
// It is a run-time error if rw is not locked for reading
// on entry to RUnlock.
func (rw *rwLock) RUnlock() {
	if r := atomic.AddInt32((*int32)(&rw.ReaderCount), -1); r < 0 {
		if r+1 == 0 || r+1 == -rwLockMaxReaders {
			panic("sync: RUnlock of unlocked rwLock")
		}

		// A writer is pending.
		if atomic.AddInt32((*int32)(&rw.ReaderWait), -1) == 0 {
			// The last reader unblocks the writer.
			writerSem := (*sem.Semaphore)(&rw.WriterSem)
			if err := writerSem.Post(); err != nil {
				panic(err)
			}
		}
	}
}
