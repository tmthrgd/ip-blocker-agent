// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build ignore

package main

import "github.com/tmthrgd/asm"

const header = `// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.
//
// This file is auto-generated - do not modify

// +build amd64,!gccgo,!appengine
`

func incrementBytes48(a *asm.Asm, incrBy, beShuf asm.Data, movIntoX0, movSi, padd, inc, bswap, pextr func(...asm.Operand), size int) {
	base := a.Argument("base", 8)
	data := a.Argument("data", 8)
	length := a.Argument("len", 8)

	a.Start()

	loop := a.NewLabel("loop")
	loopFrom := loop.Suffix("from")
	loopFromSI := loopFrom.Suffix("si")
	loopFromX0 := loopFrom.Suffix("x0")
	bigloop := a.NewLabel("bigloop")
	hugeloop := a.NewLabel("hugeloop")
	ret := a.NewLabel("ret")

	si, di, cx := asm.SI, asm.DI, asm.BX

	a.Movq(si, base)
	a.Movq(di, data)
	a.Movq(cx, length)

	a.Cmpq(asm.Constant(16), cx)
	a.Jb(loopFromSI)

	a.Cmpb(asm.Constant(1), asm.Data("runtime·support_avx"))
	a.Jne(loopFromSI)

	movIntoX0(asm.X0, asm.Address(si))
	a.Pshufb(asm.X0, beShuf)

	padd(asm.X0, incrBy)

	a.Vpshufb(asm.X1, asm.X0, beShuf)
	a.Movups(asm.Address(di), asm.X1)

	a.Addq(di, asm.Constant(16))
	a.Subq(cx, asm.Constant(16))
	a.Jz(ret)

	a.Cmpq(asm.Constant(16), cx)
	a.Jb(loopFromX0)

	a.Cmpq(asm.Constant(64), cx)
	a.Jb(bigloop)

	a.Label(hugeloop)

	for i := 0; i < 64; i += 16 {
		padd(asm.X0, incrBy.Offset(16))

		a.Vpshufb(asm.X1, asm.X0, beShuf)
		a.Movups(asm.Address(di, i), asm.X1)
	}

	a.Addq(di, asm.Constant(64))
	a.Subq(cx, asm.Constant(64))
	a.Jz(ret)

	a.Cmpq(asm.Constant(64), cx)
	a.Jae(hugeloop)

	a.Cmpq(asm.Constant(16), cx)
	a.Jb(loopFromX0)

	a.Label(bigloop)

	padd(asm.X0, incrBy.Offset(16))

	a.Vpshufb(asm.X1, asm.X0, beShuf)
	a.Movups(asm.Address(di), asm.X1)

	a.Addq(di, asm.Constant(16))
	a.Subq(cx, asm.Constant(16))
	a.Jz(ret)

	a.Cmpq(asm.Constant(16), cx)
	a.Jae(bigloop)

	a.Label(loopFromX0)

	pextr(si, asm.X0, asm.Constant((16/size)-1))

	inc(si)
	bswap(si)

	a.Label(loop)

	movSi(asm.Address(di), si)

	bswap(si)
	inc(si)
	bswap(si)

	a.Addq(di, asm.Constant(size))
	a.Subq(cx, asm.Constant(size))
	a.Jnz(loop)

	a.Label(ret)

	a.Ret()

	a.Label(loopFromSI)

	movSi(si, asm.Address(si))
	a.Jmp(loop)
}

func incrementBytes4Asm(a *asm.Asm) {
	incrBy := a.Data32("avx4IncBy", []uint32{
		0, 1, 2, 3,
		4, 4, 4, 4,
	})
	beShuf := a.Data32("avx4BEShuf", []uint32{
		0x00010203,
		0x04050607,
		0x08090a0b,
		0x0c0d0e0f,
	})

	a.NewFunction("incrementBytes4Asm")
	a.NoSplit()

	incrementBytes48(a, incrBy, beShuf, a.Vbroadcastss, a.Movl, a.Paddl, a.Incl, a.Bswapl, a.Pextrd, 4)
}

func incrementBytes8Asm(a *asm.Asm) {
	incrBy := a.Data64("avx8IncBy", []uint64{
		0, 1,
		2, 2,
	})
	beShuf := a.Data64("avx8BEShuf", []uint64{
		0x0001020304050607,
		0x08090a0b0c0d0e0f,
	})

	a.NewFunction("incrementBytes8Asm")
	a.NoSplit()

	incrementBytes48(a, incrBy, beShuf, func(ops ...asm.Operand) {
		if len(ops) != 2 {
			panic("wrong number of operands")
		}

		a.Movq(ops[0], ops[1])
		a.Movlhps(ops[0], ops[0])
	}, a.Movq, a.Paddq, a.Incq, a.Bswapq, a.Pextrq, 8)
}

func incrementBytes16Asm(a *asm.Asm) {
	incrBy := a.Data64("avx16IncBy", []uint64{
		0, 1,
		2, 2,
	})
	overflow := a.Data64("avx16Overflow", []uint64{
		0,
		^uint64(0),
		^uint64(0),
		^uint64(0),
	})
	beShuf := a.Data64("avx16BEShuf", []uint64{
		0x0001020304050607,
		0x08090a0b0c0d0e0f,
	})
	qOne := a.Data64("avx16QOne", []uint64{
		0, 1,
		1, 1,
	})

	a.NewFunction("incrementBytes16Asm")
	a.NoSplit()

	base := a.Argument("base", 8)
	data := a.Argument("data", 8)
	length := a.Argument("len", 8)

	a.Start()

	loop := a.NewLabel("loop")
	loopFrom := loop.Suffix("from")
	loopFromSI := loopFrom.Suffix("si")
	loopFromX0X1 := loopFrom.Suffix("x0x1")
	bigloop := a.NewLabel("bigloop")
	incrementDX := a.NewLabel("increment_dx")
	incrementedDX := a.NewLabel("incremented_dx")
	ret := a.NewLabel("ret")

	si, di, cx := asm.SI, asm.DI, asm.BX

	a.Movq(si, base)
	a.Movq(di, data)
	a.Movq(cx, length)

	a.Cmpq(asm.Constant(32), cx)
	a.Jb(loopFromSI)

	a.Cmpb(asm.Constant(1), asm.Data("runtime·support_avx"))
	a.Jne(loopFromSI)

	a.Movq(asm.R15, overflow.Address())

	a.Movq(asm.X1, asm.Address(si))
	a.Movlhps(asm.X1, asm.X1)

	a.Movq(asm.X0, asm.Address(si, 8))
	a.Movlhps(asm.X0, asm.X0)

	a.Pshufb(asm.X0, beShuf)
	a.Pshufb(asm.X1, beShuf)

	a.Vpcmpeqq(asm.X2, asm.X0, asm.Address(asm.R15))
	a.Pand(asm.X2, qOne)
	a.Paddq(asm.X1, asm.X2)

	a.Paddq(asm.X0, incrBy)

	a.Vpshufb(asm.X3, asm.X0, beShuf)
	a.Vpshufb(asm.X4, asm.X1, beShuf)

	a.Vpunpcklqdq(asm.X2, asm.X4, asm.X3)
	a.Movups(asm.Address(di), asm.X2)

	a.Vpunpckhqdq(asm.X2, asm.X4, asm.X3)
	a.Movups(asm.Address(di, 16), asm.X2)

	a.Addq(di, asm.Constant(32))
	a.Subq(cx, asm.Constant(32))
	a.Jz(ret)

	a.Cmpq(asm.Constant(32), cx)
	a.Jb(loopFromX0X1)

	a.Label(bigloop)

	a.Vpor(asm.X2, asm.X0, qOne.Offset(16))
	a.Pcmpeqq(asm.X2, asm.Address(asm.R15, 16))
	a.Pand(asm.X2, qOne.Offset(16))
	a.Paddq(asm.X1, asm.X2)

	a.Paddq(asm.X0, incrBy.Offset(16))

	a.Vpshufb(asm.X3, asm.X0, beShuf)
	a.Vpshufb(asm.X4, asm.X1, beShuf)

	a.Vpunpcklqdq(asm.X2, asm.X4, asm.X3)
	a.Movups(asm.Address(di), asm.X2)

	a.Vpunpckhqdq(asm.X2, asm.X4, asm.X3)
	a.Movups(asm.Address(di, 16), asm.X2)

	a.Addq(di, asm.Constant(32))
	a.Subq(cx, asm.Constant(32))
	a.Jz(ret)

	a.Cmpq(asm.Constant(32), cx)
	a.Jae(bigloop)

	a.Label(loopFromX0X1)

	a.Pextrq(asm.AX, asm.X0, asm.Constant(1))
	a.Pextrq(asm.DX, asm.X1, asm.Constant(1))

	a.Addq(asm.AX, asm.Constant(1))
	a.Adcq(asm.DX, asm.Constant(0))
	a.Bswapq(asm.DX)
	a.Bswapq(asm.AX)

	a.Label(loop)

	a.Movq(asm.Address(di, 0), asm.DX)
	a.Movq(asm.Address(di, 8), asm.AX)

	a.Bswapq(asm.AX)
	a.Addq(asm.AX, asm.Constant(1))
	a.Jc(incrementDX)
	a.Label(incrementedDX)
	a.Bswapq(asm.AX)

	a.Addq(di, asm.Constant(16))
	a.Subq(cx, asm.Constant(16))
	a.Jnz(loop)

	a.Label(ret)

	a.Ret()

	a.Label(loopFromSI)

	a.Movq(asm.DX, asm.Address(si, 0))
	a.Movq(asm.AX, asm.Address(si, 8))
	a.Jmp(loop)

	a.Label(incrementDX)

	a.Bswapq(asm.DX)
	a.Incq(asm.DX)
	a.Bswapq(asm.DX)

	a.Jmp(incrementedDX)
}

func main() {
	if err := asm.Do("incr4_amd64.s", header, incrementBytes4Asm); err != nil {
		panic(err)
	}

	if err := asm.Do("incr8_amd64.s", header, incrementBytes8Asm); err != nil {
		panic(err)
	}

	if err := asm.Do("incr16_amd64.s", header, incrementBytes16Asm); err != nil {
		panic(err)
	}
}
