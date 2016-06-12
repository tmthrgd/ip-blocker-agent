// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package incr

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

func TestEqual(t *testing.T) {
	if err := quick.Check(func(base []byte, l int) bool {
		data1 := make([]byte, l)
		IncrementBytes(base, data1)

		data2 := make([]byte, l)
		incrementBytesFallback(base, data2)

		if bytes.Equal(data1, data2) {
			return true
		}

		t.Logf("IncrementBytes        (%x, ..): %x", base, data1)
		t.Logf("incrementBytesFallback(%x, ..): %x", base, data2)
		return false
	}, &quick.Config{
		Values: func(args []reflect.Value, rand *rand.Rand) {
			base := make([]byte, 4<<uint(rand.Intn(3)))
			rand.Read(base)
			args[0] = reflect.ValueOf(base)

			args[1] = reflect.ValueOf(rand.Intn(1+4096/len(base)) * len(base))
		},
	}); err != nil {
		t.Error(err)
	}
}

func Test16Overflow(t *testing.T) {
	for _, l := range [...]int{32, 160} {
		base := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfa}
		data1 := make([]byte, l)
		data2 := make([]byte, l)

		IncrementBytes(base, data1)
		incrementBytesFallback(base, data2)

		if !bytes.Equal(data1, data2) {
			t.Logf("IncrementBytes        (%x, ..): %x", base, data1)
			t.Logf("incrementBytesFallback(%x, ..): %x", base, data2)
			t.Errorf("failed on input %v, 160", base)
		}

		base = []byte{1, 2, 3, 4, 5, 6, 7, 8, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe}

		IncrementBytes(base, data1)
		incrementBytesFallback(base, data2)

		if !bytes.Equal(data1, data2) {
			t.Logf("IncrementBytes        (%x, ..): %x", base, data1)
			t.Logf("incrementBytesFallback(%x, ..): %x", base, data2)
			t.Errorf("failed on input %v, 160", base)
		}

		base = []byte{1, 2, 3, 4, 5, 6, 7, 8, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

		IncrementBytes(base, data1)
		incrementBytesFallback(base, data2)

		if !bytes.Equal(data1, data2) {
			t.Logf("IncrementBytes        (%x, ..): %x", base, data1)
			t.Logf("incrementBytesFallback(%x, ..): %x", base, data2)
			t.Errorf("failed on input %v, 160", base)
		}
	}
}

func benchmarkGo(b *testing.B, k, l int) {
	base := make([]byte, k)
	dest := make([]byte, l)

	for i := 0; i < b.N; i++ {
		incrementBytesFallback(base, dest)
	}
}

func Benchmark4Go0(b *testing.B) {
	benchmarkGo(b, 4, 0)
}

func Benchmark4Go4(b *testing.B) {
	benchmarkGo(b, 4, 4)
}

func Benchmark4Go8(b *testing.B) {
	benchmarkGo(b, 4, 8)
}

func Benchmark4Go16(b *testing.B) {
	benchmarkGo(b, 4, 16)
}

func Benchmark4Go32(b *testing.B) {
	benchmarkGo(b, 4, 32)
}

func Benchmark4Go64(b *testing.B) {
	benchmarkGo(b, 4, 64)
}

func Benchmark4Go1024(b *testing.B) {
	benchmarkGo(b, 4, 1024)
}

func Benchmark4Go2048(b *testing.B) {
	benchmarkGo(b, 4, 2048)
}

func Benchmark4Go4096(b *testing.B) {
	benchmarkGo(b, 4, 4096)
}

func Benchmark4Go65536(b *testing.B) {
	benchmarkGo(b, 4, 65536)
}

func Benchmark8Go0(b *testing.B) {
	benchmarkGo(b, 8, 0)
}

func Benchmark8Go8(b *testing.B) {
	benchmarkGo(b, 8, 8)
}

func Benchmark8Go16(b *testing.B) {
	benchmarkGo(b, 8, 16)
}

func Benchmark8Go32(b *testing.B) {
	benchmarkGo(b, 8, 32)
}

func Benchmark8Go64(b *testing.B) {
	benchmarkGo(b, 8, 64)
}

func Benchmark8Go1024(b *testing.B) {
	benchmarkGo(b, 8, 1024)
}

func Benchmark8Go2048(b *testing.B) {
	benchmarkGo(b, 8, 2048)
}

func Benchmark8Go4096(b *testing.B) {
	benchmarkGo(b, 8, 4096)
}

func Benchmark8Go65536(b *testing.B) {
	benchmarkGo(b, 8, 65536)
}

func Benchmark16Go0(b *testing.B) {
	benchmarkGo(b, 16, 0)
}

func Benchmark16Go16(b *testing.B) {
	benchmarkGo(b, 16, 16)
}

func Benchmark16Go32(b *testing.B) {
	benchmarkGo(b, 16, 32)
}

func Benchmark16Go48(b *testing.B) {
	benchmarkGo(b, 16, 48)
}

func Benchmark16Go64(b *testing.B) {
	benchmarkGo(b, 16, 64)
}

func Benchmark16Go1024(b *testing.B) {
	benchmarkGo(b, 16, 1024)
}

func Benchmark16Go2048(b *testing.B) {
	benchmarkGo(b, 16, 2048)
}

func Benchmark16Go4096(b *testing.B) {
	benchmarkGo(b, 16, 4096)
}

func Benchmark16Go65536(b *testing.B) {
	benchmarkGo(b, 16, 65536)
}

func benchmark(b *testing.B, k, l int) {
	base := make([]byte, k)
	dest := make([]byte, l)

	for i := 0; i < b.N; i++ {
		IncrementBytes(base, dest)
	}
}

func Benchmark4Opt0(b *testing.B) {
	benchmark(b, 4, 0)
}

func Benchmark4Opt4(b *testing.B) {
	benchmark(b, 4, 4)
}

func Benchmark4Opt8(b *testing.B) {
	benchmark(b, 4, 8)
}

func Benchmark4Opt16(b *testing.B) {
	benchmark(b, 4, 16)
}

func Benchmark4Opt32(b *testing.B) {
	benchmark(b, 4, 32)
}

func Benchmark4Opt64(b *testing.B) {
	benchmark(b, 4, 64)
}

func Benchmark4Opt1024(b *testing.B) {
	benchmark(b, 4, 1024)
}

func Benchmark4Opt2048(b *testing.B) {
	benchmark(b, 4, 2048)
}

func Benchmark4Opt4096(b *testing.B) {
	benchmark(b, 4, 4096)
}

func Benchmark4Opt65536(b *testing.B) {
	benchmark(b, 4, 65536)
}

func Benchmark8Opt0(b *testing.B) {
	benchmark(b, 8, 0)
}

func Benchmark8Opt8(b *testing.B) {
	benchmark(b, 8, 8)
}

func Benchmark8Opt16(b *testing.B) {
	benchmark(b, 8, 16)
}

func Benchmark8Opt32(b *testing.B) {
	benchmark(b, 8, 32)
}

func Benchmark8Opt64(b *testing.B) {
	benchmark(b, 8, 64)
}

func Benchmark8Opt1024(b *testing.B) {
	benchmark(b, 8, 1024)
}

func Benchmark8Opt2048(b *testing.B) {
	benchmark(b, 8, 2048)
}

func Benchmark8Opt4096(b *testing.B) {
	benchmark(b, 8, 4096)
}

func Benchmark8Opt65536(b *testing.B) {
	benchmark(b, 8, 65536)
}

func Benchmark16Opt0(b *testing.B) {
	benchmark(b, 16, 0)
}

func Benchmark16Opt16(b *testing.B) {
	benchmark(b, 16, 16)
}

func Benchmark16Opt32(b *testing.B) {
	benchmark(b, 16, 32)
}

func Benchmark16Opt48(b *testing.B) {
	benchmark(b, 16, 48)
}

func Benchmark16Opt64(b *testing.B) {
	benchmark(b, 16, 64)
}

func Benchmark16Opt1024(b *testing.B) {
	benchmark(b, 16, 1024)
}

func Benchmark16Opt2048(b *testing.B) {
	benchmark(b, 16, 2048)
}

func Benchmark16Opt4096(b *testing.B) {
	benchmark(b, 16, 4096)
}

func Benchmark16Opt65536(b *testing.B) {
	benchmark(b, 16, 65536)
}
