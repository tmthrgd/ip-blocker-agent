// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.
//
// This file is auto-generated - do not modify

// +build amd64,!gccgo,!appengine

#include "textflag.h"

DATA avx8IncBy<>+0x08(SB)/8, $0x0000000000000001
DATA avx8IncBy<>+0x10(SB)/8, $0x0000000000000002
DATA avx8IncBy<>+0x18(SB)/8, $0x0000000000000002
GLOBL avx8IncBy<>(SB),RODATA,$32

DATA avx8BEShuf<>+0x00(SB)/8, $0x0001020304050607
DATA avx8BEShuf<>+0x08(SB)/8, $0x08090a0b0c0d0e0f
GLOBL avx8BEShuf<>(SB),RODATA,$16

TEXT ·incrementBytes8Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), SI
	MOVQ data+8(FP), DI
	MOVQ len+16(FP), BX
	CMPQ BX, $16
	JB loop_from_si
	CMPB runtime·support_avx(SB), $1
	JNE loop_from_si
	MOVQ (SI), X0
	MOVLHPS X0, X0
	PSHUFB avx8BEShuf<>(SB), X0
	PADDQ avx8IncBy<>(SB), X0
	VPSHUFB avx8BEShuf<>(SB), X0, X1
	MOVUPS X1, (DI)
	ADDQ $16, DI
	SUBQ $16, BX
	JZ ret
	CMPQ BX, $16
	JB loop_from_x0
	CMPQ BX, $64
	JB bigloop
hugeloop:
	PADDQ avx8IncBy<>+0x10(SB), X0
	VPSHUFB avx8BEShuf<>(SB), X0, X1
	MOVUPS X1, (DI)
	PADDQ avx8IncBy<>+0x10(SB), X0
	VPSHUFB avx8BEShuf<>(SB), X0, X1
	MOVUPS X1, 16(DI)
	PADDQ avx8IncBy<>+0x10(SB), X0
	VPSHUFB avx8BEShuf<>(SB), X0, X1
	MOVUPS X1, 32(DI)
	PADDQ avx8IncBy<>+0x10(SB), X0
	VPSHUFB avx8BEShuf<>(SB), X0, X1
	MOVUPS X1, 48(DI)
	ADDQ $64, DI
	SUBQ $64, BX
	JZ ret
	CMPQ BX, $64
	JAE hugeloop
	CMPQ BX, $16
	JB loop_from_x0
bigloop:
	PADDQ avx8IncBy<>+0x10(SB), X0
	VPSHUFB avx8BEShuf<>(SB), X0, X1
	MOVUPS X1, (DI)
	ADDQ $16, DI
	SUBQ $16, BX
	JZ ret
	CMPQ BX, $16
	JAE bigloop
loop_from_x0:
	PEXTRQ $1, X0, SI
	INCQ SI
	BSWAPQ SI
loop:
	MOVQ SI, (DI)
	BSWAPQ SI
	INCQ SI
	BSWAPQ SI
	ADDQ $8, DI
	SUBQ $8, BX
	JNZ loop
ret:
	RET
loop_from_si:
	MOVQ (SI), SI
	JMP loop
