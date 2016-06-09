// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"math/big"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

func incrBytes(b []byte) {
	for j := len(b) - 1; j >= 0; j-- {
		b[j]++

		if b[j] > 0 {
			break
		}
	}
}

func testAddition(t *testing.T, a *big.Int, b int, pad int) {
	aa := a.Bytes()
	aa = append(make([]byte, pad-len(aa)), aa...)

	if addIntToBytes(aa, b) {
		t.Skip("overflow")
	}

	got := new(big.Int).SetBytes(aa)

	bb := new(big.Int).SetInt64(int64(b))
	expect := bb.Add(bb, a)

	if expect.Cmp(got) != 0 {
		diff := new(big.Int).Sub(expect, got)
		t.Errorf("%s + %d expected %s, got %s, exp - got = %s", a, b, expect, got, diff)
	}
}

const maxInt = int(^uint(0) >> 1)

var maxIntBig = big.NewInt(int64(maxInt))

func TestAddition(t *testing.T) {
	t.Parallel()

	bigIntZero := big.NewInt(0)

	testAddition(t, big.NewInt(10000), 7321, 16)
	testAddition(t, maxIntBig, maxInt, 16)
	testAddition(t, maxIntBig, 0, 16)
	testAddition(t, bigIntZero, 0, 16)
	testAddition(t, big.NewInt(int64(maxInt>>1)), maxInt>>2, 16)

	testAddition(t, big.NewInt(10000), 7321, 8)
	testAddition(t, big.NewInt(10000), 7321, 4)
	testAddition(t, big.NewInt(10000), 7321, 2)
	testAddition(t, big.NewInt(70), 30, 1)
	testAddition(t, bigIntZero, 0, 0)

	one := big.NewInt(1)

	if err := quick.Check(func(x []byte, y int) bool {
		ox := new(big.Int).SetBytes(x)
		expect := new(big.Int).Add(ox, new(big.Int).SetInt64(int64(y)))

		overflow := addIntToBytes(x, y)
		got := new(big.Int).SetBytes(x)

		shouldOverflow := expect.Cmp(new(big.Int).Lsh(one, uint(len(x))*8)) != -1
		if shouldOverflow && overflow {
			return true
		}

		if overflow {
			t.Logf("overflowed on %s + %d", ox, y)
		}

		if shouldOverflow == overflow && expect.Cmp(got) == 0 {
			return true
		}

		diff := new(big.Int).Sub(expect, got)
		t.Logf("%s + %d expected %s, got %s, exp-got = %s", ox, y, expect, got, diff)
		return false
	}, &quick.Config{
		Values: func(args []reflect.Value, rand *rand.Rand) {
			x := make([]byte, rand.Int()%32+1)
			rand.Read(x)
			args[0] = reflect.ValueOf(x)

			args[1] = reflect.ValueOf(rand.Int() & int(^uint32(0)))
		},
	}); err != nil {
		t.Error(err)
	}
}

func benchmarkAddition(b *testing.B, l int) {
	var x []byte
	var y int

	switch l {
	case 1:
		x = big.NewInt(127).Bytes()
		y = 74
	case 2:
		x = big.NewInt(45621).Bytes()
		y = 13421
	case 4:
		x = big.NewInt(1073741824).Bytes()
		y = 805306496
	case 8:
		a := big.NewInt(1)
		a.Lsh(a, 63)
		a.Sub(a, big.NewInt(1))
		x = a.Bytes()
		y = 1610612736
	case 16:
		a := big.NewInt(1)
		a.Lsh(a, 127)
		a.Sub(a, big.NewInt(1))
		x = a.Bytes()
		y = 2013265920
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		addIntToBytes(x, y)
	}
}

func BenchmarkAddition16(b *testing.B) {
	benchmarkAddition(b, 16)
}

func BenchmarkAddition8(b *testing.B) {
	benchmarkAddition(b, 8)
}

func BenchmarkAddition4(b *testing.B) {
	benchmarkAddition(b, 4)
}

func BenchmarkAddition2(b *testing.B) {
	benchmarkAddition(b, 2)
}

func BenchmarkAddition1(b *testing.B) {
	benchmarkAddition(b, 1)
}
