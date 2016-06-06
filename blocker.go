// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// Package blocker is an efficient shared memory IP
// blocking system for nginx.
//
// See https://github.com/tmthrgd/nginx-ip-blocker
package blocker

//#include "ngx_ip_blocker_shm.h"
import "C"

import (
	"errors"
	"unsafe"
)

// ErrClosed will be returned on attempts to call
// methods on a closed server or client.
var ErrClosed = errors.New("shared memory closed")

const headerSize = unsafe.Sizeof(C.ngx_ip_blocker_shm_st{})
