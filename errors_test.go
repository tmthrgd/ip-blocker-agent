// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"errors"
	"testing"
)

func TestInvalidDataError(t *testing.T) {
	err := InvalidDataError{nil}
	if err.Error() != "invalid data" {
		t.Error("invalid error message")
	}

	err = InvalidDataError{errors.New("test error")}
	if err.Error() != "invalid data: test error" {
		t.Error("invalid error message")
	}
}

func TestLockReleaseFailedError(t *testing.T) {
	err := LockReleaseFailedError{nil}
	if err.Error() != "failed to release read lock" {
		t.Error("invalid error message")
	}

	err = LockReleaseFailedError{errors.New("test error")}
	if err.Error() != "failed to release read lock: test error" {
		t.Error("invalid error message")
	}
}
