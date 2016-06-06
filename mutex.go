// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

//#include "ngx_ip_blocker_shm.h"
import "C"

import "github.com/tmthrgd/go-sem"

type mutex C.ngx_ip_blocker_mutex_st

func (m *mutex) Create() {
	sem := (*sem.Semaphore)(&m.sem)
	if err := sem.Init(1); err != nil {
		panic(err)
	}
}

func (m *mutex) Lock() {
	sem := (*sem.Semaphore)(&m.sem)
	if err := sem.Wait(); err != nil {
		panic(err)
	}
}

func (m *mutex) Unlock() {
	sem := (*sem.Semaphore)(&m.sem)
	if err := sem.Post(); err != nil {
		panic(err)
	}
}
