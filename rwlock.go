// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

/*
#include <semaphore.h>       // For sem_*

#include "ngx_ip_blocker_shm.h"
*/
import "C"

import "sync/atomic"

// A rwLock is a reader/writer mutual exclusion lock.
// The lock can be held by an arbitrary number of readers
// or a single writer.
// rwLocks can be created as part of other structures.
// Create must be called before use.
type rwLock C.ngx_ip_blocker_rwlock_st

func (rw *rwLock) Create() {
	w := (*mutex)(&rw.w)
	w.Create()

	if _, err := C.sem_init(&rw.writer_sem, 1, 0); err != nil {
		panic(err)
	}

	if _, err := C.sem_init(&rw.reader_sem, 1, 0); err != nil {
		panic(err)
	}

	rw.reader_count = 0
	rw.reader_wait = 0
}

// Lock locks rw for writing.
// If the lock is already locked for reading or writing,
// Lock blocks until the lock is available.
// To ensure that the lock eventually becomes available,
// a blocked Lock call excludes new readers from acquiring
// the lock.
func (rw *rwLock) Lock() {
	// First, resolve competition with other writers.
	w := (*mutex)(&rw.w)
	w.Lock()

	// Announce to readers there is a pending writer.
	r := atomic.AddInt32((*int32)(&rw.reader_count), -C.NGX_IP_BLOCKER_MAX_READERS) + C.NGX_IP_BLOCKER_MAX_READERS

	// Wait for active readers.
	if r != 0 && atomic.AddInt32((*int32)(&rw.reader_wait), r) != 0 {
		if _, err := C.sem_wait(&rw.writer_sem); err != nil {
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
	r := atomic.AddInt32((*int32)(&rw.reader_count), C.NGX_IP_BLOCKER_MAX_READERS)
	if r >= C.NGX_IP_BLOCKER_MAX_READERS {
		panic("sync: Unlock of unlocked rwLock")
	}

	// Unblock blocked readers, if any.
	for i := 0; i < int(r); i++ {
		if _, err := C.sem_post(&rw.reader_sem); err != nil {
			panic(err)
		}
	}

	// Allow other writers to proceed.
	w := (*mutex)(&rw.w)
	w.Unlock()
}
