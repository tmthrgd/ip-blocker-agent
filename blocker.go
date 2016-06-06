// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build !linux !386,!amd64

package blocker

/*
#include <stdint.h>         // For int32_t
#include <semaphore.h>      // For sem_t

#define IP_BLOCKER_MAX_READERS (1 << 30)

typedef struct {
	sem_t Sem;
} ip_blocker_mutex_st;

typedef struct {
	ip_blocker_mutex_st W;        // held if there are pending writers
	sem_t WriterSem;              // semaphore for writers to wait for completing readers
	sem_t ReaderSem;              // semaphore for readers to wait for completing writers
	volatile int32_t ReaderCount; // number of pending readers
	volatile int32_t ReaderWait;  // number of departing readers
} ip_blocker_rwlock_st;

typedef struct {
	volatile size_t Base;
	volatile size_t Len;
} ip_blocker_ip_block_st;

typedef struct {
	uint32_t Version;
	volatile uint32_t Revision;

	ip_blocker_rwlock_st Lock;

	ip_blocker_ip_block_st IP4, IP6, IP6Route;
} ip_blocker_shm_st;
*/
import "C"

type mutex C.ip_blocker_mutex_st

type rwLock C.ip_blocker_rwlock_st

type ipBlock C.ip_blocker_ip_block_st

type shmHeader C.ip_blocker_shm_st

func (h *shmHeader) rwLocker() *rwLock {
	return (*rwLock)(&h.Lock)
}

func (h *shmHeader) setBlocks(ip4, ip4len, ip6, ip6len, ip6r, ip6rlen int) {
	h.IP4.Base = C.size_t(ip4)
	h.IP4.Len = C.size_t(ip4len)

	h.IP6.Base = C.size_t(ip6)
	h.IP6.Len = C.size_t(ip6len)

	h.IP6Route.Base = C.size_t(ip6r)
	h.IP6Route.Len = C.size_t(ip6rlen)
}

const (
	headerSize = C.sizeof_ip_blocker_shm_st

	rwLockMaxReaders = C.IP_BLOCKER_MAX_READERS
)
