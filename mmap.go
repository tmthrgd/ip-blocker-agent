// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"syscall"
	"unsafe"
)

func mmap(fd int, offset int64, length, prot, flags int) (addr unsafe.Pointer, err error) {
	data, err := syscall.Mmap(fd, offset, length, prot, flags)
	if err != nil {
		return nil, err
	}

	return unsafe.Pointer(&data[0]), nil
}

func munmap(addr unsafe.Pointer, length int) (err error) {
	data := (*[1 << 30]byte)(addr)[:length:length]
	return syscall.Munmap(data)
}
