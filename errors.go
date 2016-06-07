// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import "errors"

var (
	// ErrClosed will be returned on attempts to call
	// methods on a closed server or client.
	ErrClosed = errors.New("shared memory closed")

	// ErrAlreadyBatching will be returned on attempts to call
	// (*Server).Batch() when the *Server is already
	// batching.
	ErrAlreadyBatching = errors.New("already batching")

	// ErrNotBatching will be returned on attempts to call
	// (*Server).Commit() when (*Server).Batch() has
	// not previously been called.
	ErrNotBatching = errors.New("not batching")

	// ErrInvalidSharedMemory will be returned in most Client
	// functions if the backing shared memory is invalid at
	// the time of the call.
	ErrInvalidSharedMemory = errors.New("invalid shared memory")

	errRangeTooLarge = errors.New("range too large")
)

// LockReleaseFailedError records that a lock could not
// be released and any error that was occuring.
//
// LockReleaseFailedError are serious errors and may mean
// that shared memory is in an invalid state and no further
// writes to shared memory by the server process will be
// possible.
type LockReleaseFailedError struct {
	Err error
}

func (e LockReleaseFailedError) Error() string {
	if e.Err == nil {
		return "failed to release read lock"
	}

	return "failed to release read lock: " + e.Err.Error()
}
