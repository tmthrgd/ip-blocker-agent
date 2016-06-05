// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build !linux

package blocker

/*
#cgo LDFLAGS: -lrt

#include <stdlib.h>          // For free
#include <sys/mman.h>        // For shm_*
*/
import "C"

import (
	"os"
	"unsafe"
)

func shmOpen(name string, flag int, perm os.FileMode) (*os.File, error) {
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	fd, err := C.shm_open(nameC, C.int(flag), C.mode_t(perms))
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), name), file
}

func shmUnlink(name string) error {
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	_, err := C.shm_unlink(nameC)
	return err
}
