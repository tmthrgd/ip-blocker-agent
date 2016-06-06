// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import "github.com/tmthrgd/go-sem"

// type mutex C.ip_blocker_mutex_st
// 	see blocker.go

func (m *mutex) Create() {
	sem := (*sem.Semaphore)(&m.Sem)
	if err := sem.Init(1); err != nil {
		panic(err)
	}
}

func (m *mutex) Lock() {
	sem := (*sem.Semaphore)(&m.Sem)
	if err := sem.Wait(); err != nil {
		panic(err)
	}
}

func (m *mutex) Unlock() {
	sem := (*sem.Semaphore)(&m.Sem)
	if err := sem.Post(); err != nil {
		panic(err)
	}
}
