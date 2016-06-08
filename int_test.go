// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package blocker

import (
	"bytes"
	"math/big"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

var intOne = big.NewInt(1)
var zeroIP [16]byte

func incrBytesBigInt(x *big.Int, ip []byte) {
	if x == nil {
		x = new(big.Int).SetBytes(ip)
	}

	b := x.Add(x, intOne).Bytes()

	if len(b) > len(ip) {
		copy(ip, zeroIP[:])
		x.SetInt64(0)
	} else {
		copy(ip[:len(b)], zeroIP[:])
		copy(ip[len(ip)-len(b):], b)
	}
}

func TestIncrement(t *testing.T) {
	t.Parallel()

	ip1 := make([]byte, 16)
	ip2 := make([]byte, 16)

	x := new(big.Int).SetBytes(ip2)

	for i := 0; i < 10000; i++ {
		incrBytes(ip1)
		incrBytesBigInt(x, ip2)
	}

	if !bytes.Equal(ip1, ip2) {
		t.Errorf("incrBytes and incrBytesBigInt produced different results:\n%x\n%x", []byte(ip1), []byte(ip2))
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

func subBytesBigInt(ip1, ip2 []byte) int {
	a := new(big.Int).SetBytes(ip1)
	b := new(big.Int).SetBytes(ip2)

	c := a.Sub(a, b)
	if c.Sign() != 1 {
		return 0
	}

	if c.Cmp(maxIntBig) >= 0 {
		return maxInt
	}

	return int(c.Int64())
}

func testSubtraction(t *testing.T, a, b *big.Int, expect int64, pad int) {
	ip1 := a.Bytes()
	ip1 = append(make([]byte, pad-len(ip1)), ip1...)
	ip2 := b.Bytes()
	ip2 = append(make([]byte, pad-len(ip2)), ip2...)

	if x := subBytesBigInt(ip1, ip2); int64(x) != expect {
		t.Errorf("subBytesBigInt failed for %s - %s, expected %d, got %d", a, b, expect, x)
	}

	if x := subBytes(ip1, ip2); int64(x) != expect {
		t.Errorf("subBytes failed for %s - %s, expected %d, got %d", a, b, expect, x)
	}

	if x := subBytes32(ip1, ip2); int64(x) != expect {
		const maxInt32 = int32(^uint32(0) >> 1)

		if x != maxInt || expect <= int64(maxInt32) {
			t.Errorf("subBytes32 failed for %s - %s, expected %d, got %d", a, b, expect, x)
		}
	}
}

func TestSubtraction(t *testing.T) {
	t.Parallel()

	bigIntZero := big.NewInt(0)

	testSubtraction(t, big.NewInt(10000), big.NewInt(7321), 10000-7321, 16)
	testSubtraction(t, maxIntBig, maxIntBig, 0, 16)
	testSubtraction(t, maxIntBig, bigIntZero, int64(maxInt), 16)
	testSubtraction(t, bigIntZero, bigIntZero, 0, 16)
	testSubtraction(t, big.NewInt(int64(maxInt>>1)), big.NewInt(int64(maxInt>>2)), int64(maxInt>>2+1), 16)

	const maxInt64 = int64(^uint64(0) >> 1)
	testSubtraction(t, big.NewInt(maxInt64), big.NewInt(maxInt64), 0, 16)

	maxInt64Lsh1 := new(big.Int).Lsh(big.NewInt(maxInt64), 1)
	testSubtraction(t, maxInt64Lsh1, maxInt64Lsh1, 0, 16)
	testSubtraction(t, maxInt64Lsh1, big.NewInt(maxInt64), int64(maxInt), 16)
	testSubtraction(t, maxInt64Lsh1, bigIntZero, int64(maxInt), 16)
	testSubtraction(t, maxInt64Lsh1, bigIntZero, int64(maxInt), 16)

	if maxInt64 <= int64(maxInt) {
		testSubtraction(t, big.NewInt(maxInt64), big.NewInt(maxInt64>>1), int64(maxInt64>>1+1), 16)
		testSubtraction(t, big.NewInt(maxInt64), bigIntZero, int64(maxInt64), 16)
		testSubtraction(t, big.NewInt(maxInt64>>1), bigIntZero, int64(maxInt64>>1), 16)
	} else {
		testSubtraction(t, big.NewInt(maxInt64), big.NewInt(maxInt64>>1), int64(maxInt), 16)
		testSubtraction(t, big.NewInt(maxInt64), bigIntZero, int64(maxInt), 16)
		testSubtraction(t, big.NewInt(maxInt64>>1), bigIntZero, int64(maxInt), 16)
	}

	testSubtraction(t, big.NewInt(10000), big.NewInt(7321), 10000-7321, 8)
	testSubtraction(t, big.NewInt(10000), big.NewInt(7321), 10000-7321, 4)
	testSubtraction(t, big.NewInt(10000), big.NewInt(7321), 10000-7321, 2)
	testSubtraction(t, big.NewInt(70), big.NewInt(30), 40, 1)
	testSubtraction(t, bigIntZero, bigIntZero, 0, 0)

	for _, size := range [...]int{16, 8, 4, 2, 1} {
		ip1 := make([]byte, size)
		ip2 := make([]byte, size)

		rand.Read(ip1)

		for i := 0; i < 10000; i++ {
			a := subBytesBigInt(ip1, ip2)
			b := subBytes(ip1, ip2)
			if a != b {
				t.Errorf("subBytesBigInt (%d) and subBytes (%d) differ", a, b)
			}

			c := subBytes32(ip1, ip2)
			if a != c {
				const maxInt32 = int32(^uint32(0) >> 1)

				if c != maxInt || a <= int(maxInt32) {
					t.Errorf("subBytesBigInt (%d) and subBytes32 (%d) differ", a, c)
				}
			}

			rand.Read(ip2)
			ip1, ip2 = ip2, ip1
		}
	}
}

func testSubtractionDifferentLengths(t *testing.T, sub func(x, y []byte) int) {
	var x [16]byte
	var y [20]byte

	defer func() {
		if err := recover(); err != nil && err != "different lengths" {
			panic(err)
		}
	}()

	sub(x[:], y[:])
	t.Error("did not panic on different sizes")
}

func TestSubtractionDifferentLengths(t *testing.T) {
	testSubtractionDifferentLengths(t, subBytes32)
	testSubtractionDifferentLengths(t, subBytes64)
}

func testSubtractionInvalidLength(t *testing.T, l int, sub func(x, y []byte) int) {
	x := make([]byte, l)

	defer func() {
		if err := recover(); err != nil && err != "invalid length" {
			panic(err)
		}
	}()

	sub(x, x)
	t.Error("did not panic on invalid length")
}

func TestSubtractionInvalidLength(t *testing.T) {
	testSubtractionInvalidLength(t, 3, subBytes32)
	testSubtractionInvalidLength(t, 3, subBytes64)
	testSubtractionInvalidLength(t, 5, subBytes32)
	testSubtractionInvalidLength(t, 5, subBytes64)
	testSubtractionInvalidLength(t, 6, subBytes32)
	testSubtractionInvalidLength(t, 6, subBytes64)
	testSubtractionInvalidLength(t, 7, subBytes32)
	testSubtractionInvalidLength(t, 7, subBytes64)
}

func benchmarkManualIncrement(b *testing.B, size int) {
	ip := make([]byte, size)

	for i := 0; i < b.N; i++ {
		incrBytes(ip)
	}
}

func benchmarkBigIntIncrement(b *testing.B, size int) {
	ip := make([]byte, size)
	x := new(big.Int).SetBytes(ip)

	for i := 0; i < b.N; i++ {
		incrBytesBigInt(x, ip)
	}
}

func BenchmarkManualIncrement16(b *testing.B) {
	benchmarkManualIncrement(b, 16)
}

func BenchmarkBigIntIncrement16(b *testing.B) {
	benchmarkBigIntIncrement(b, 16)
}

func BenchmarkManualIncrement8(b *testing.B) {
	benchmarkManualIncrement(b, 8)
}

func BenchmarkBigIntIncrement8(b *testing.B) {
	benchmarkBigIntIncrement(b, 8)
}

func BenchmarkManualIncrement4(b *testing.B) {
	benchmarkManualIncrement(b, 4)
}

func BenchmarkBigIntIncrement4(b *testing.B) {
	benchmarkBigIntIncrement(b, 4)
}

func BenchmarkManualIncrement2(b *testing.B) {
	benchmarkManualIncrement(b, 2)
}

func BenchmarkBigIntIncrement2(b *testing.B) {
	benchmarkBigIntIncrement(b, 2)
}

func BenchmarkManualIncrement1(b *testing.B) {
	benchmarkManualIncrement(b, 1)
}

func BenchmarkBigIntIncrement1(b *testing.B) {
	benchmarkBigIntIncrement(b, 1)
}

func benchmarkSubtraction(b *testing.B, sub func(_, _ []byte) int, pad int) {
	var ip1, ip2 []byte

	if pad == 1 {
		ip1 = big.NewInt(250).Bytes()
		ip2 = big.NewInt(161).Bytes()
	} else {
		ip1 = big.NewInt(10000).Bytes()
		ip1 = append(make([]byte, pad-len(ip1)), ip1...)
		ip2 = big.NewInt(7321).Bytes()
		ip2 = append(make([]byte, pad-len(ip2)), ip2...)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sub(ip1, ip2)
	}
}

func BenchmarkManualSubtraction16(b *testing.B) {
	benchmarkSubtraction(b, subBytes, 16)
}

func BenchmarkBigIntSubtraction16(b *testing.B) {
	benchmarkSubtraction(b, subBytesBigInt, 16)
}

func BenchmarkManualSubtraction8(b *testing.B) {
	benchmarkSubtraction(b, subBytes, 8)
}

func BenchmarkBigIntSubtraction8(b *testing.B) {
	benchmarkSubtraction(b, subBytesBigInt, 8)
}

func BenchmarkManualSubtraction4(b *testing.B) {
	benchmarkSubtraction(b, subBytes, 4)
}

func BenchmarkBigIntSubtraction4(b *testing.B) {
	benchmarkSubtraction(b, subBytesBigInt, 4)
}

func BenchmarkManualSubtraction2(b *testing.B) {
	benchmarkSubtraction(b, subBytes, 2)
}

func BenchmarkBigIntSubtraction2(b *testing.B) {
	benchmarkSubtraction(b, subBytesBigInt, 2)
}

func BenchmarkManualSubtraction1(b *testing.B) {
	benchmarkSubtraction(b, subBytes, 1)
}

func BenchmarkBigIntSubtraction1(b *testing.B) {
	benchmarkSubtraction(b, subBytesBigInt, 1)
}
