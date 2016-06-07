// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build linux

//go:generate sh -c "GOARCH=386 go tool cgo -godefs blocker.go | gofmt -r '(*rwLock)(x)->x' > blocker_linux_386.go"
//go:generate sh -c "GOARCH=amd64 go tool cgo -godefs blocker.go | gofmt -r '(*rwLock)(x)->x' > blocker_linux_amd64.go"

package blocker
