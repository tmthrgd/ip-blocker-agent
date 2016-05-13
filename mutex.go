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
