// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

func addIntToBytes(x []byte, y int) (overflow bool) {
	for i := len(x) - 1; i >= 0 && y > 0; i-- {
		c := int(x[i]) + (y & 0xff)
		y >>= 8

		if c >= 1<<8 {
			y++

			c -= 1 << 8
		}

		x[i] = byte(c)
	}

	return y != 0
}
