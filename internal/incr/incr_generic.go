// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package incr

func incrBytes(b []byte) {
	for j := len(b) - 1; j >= 0; j-- {
		b[j]++

		if b[j] > 0 {
			break
		}
	}
}

func incrementBytesFallback(base, data []byte) {
	if len(data)%len(base) != 0 {
		panic("invalid data length")
	}

	x := append([]byte(nil), base...)

	for i := 0; i < len(data); i += len(base) {
		copy(data[i:], x)
		incrBytes(x)
	}
}
