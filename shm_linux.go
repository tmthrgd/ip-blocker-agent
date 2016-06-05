// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"os"
	"syscall"
)

const devShm = "/dev/shm/"

func shmOpen(name string, flag int, perm os.FileMode) (*os.File, error) {
	fileName := name

	for len(name) != 0 && name[0] == '/' {
		name = name[1:]
	}

	if len(name) == 0 {
		return nil, syscall.EINVAL
	}

	o := uint32(perm.Perm())
	if perm&os.ModeSetuid != 0 {
		o |= syscall.S_ISUID
	}
	if perm&os.ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if perm&os.ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}

	fd, err := syscall.Open(devShm + name, flag|syscall.O_CLOEXEC, o)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), fileName), nil
}

func shmUnlink(name string) error {
	for len(name) != 0 && name[0] == '/' {
		name = name[1:]
	}

	if len(name) == 0 {
		return syscall.EINVAL
	}

	return syscall.Unlink(devShm + name)
}
