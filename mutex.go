// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package main

/*
#include <semaphore.h>       // For sem_*

#include "ngx_ip_blocker_shm.h"
*/
import "C"

type mutex C.ngx_ip_blocker_mutex_st

func (m *mutex) Create() {
	if _, err := C.sem_init(&m.sem, 1, 1); err != nil {
		panic(err)
	}
}

func (m *mutex) Lock() {
	if _, err := C.sem_wait(&m.sem); err != nil {
		panic(err)
	}
}

func (m *mutex) Unlock() {
	if _, err := C.sem_post(&m.sem); err != nil {
		panic(err)
	}
}
