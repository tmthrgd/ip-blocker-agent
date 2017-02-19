// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.
//
// This file is auto-generated - do not modify

// +build amd64,!gccgo,!appengine

#include "textflag.h"

DATA avx4IncBy<>+0x04(SB)/4, $0x00000001
DATA avx4IncBy<>+0x08(SB)/4, $0x00000002
DATA avx4IncBy<>+0x0c(SB)/4, $0x00000003
DATA avx4IncBy<>+0x10(SB)/4, $0x00000004
DATA avx4IncBy<>+0x14(SB)/4, $0x00000004
DATA avx4IncBy<>+0x18(SB)/4, $0x00000004
DATA avx4IncBy<>+0x1c(SB)/4, $0x00000004
GLOBL avx4IncBy<>(SB),RODATA,$32

DATA avx4BEShuf<>+0x00(SB)/4, $0x00010203
DATA avx4BEShuf<>+0x04(SB)/4, $0x04050607
DATA avx4BEShuf<>+0x08(SB)/4, $0x08090a0b
DATA avx4BEShuf<>+0x0c(SB)/4, $0x0c0d0e0f
GLOBL avx4BEShuf<>(SB),RODATA,$16

TEXT ·incrementBytes4Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), SI
	MOVQ data+8(FP), DI
	MOVQ len+16(FP), BX
	CMPQ BX, $16
	JB loop_from_si
	CMPB runtime·support_avx(SB), $1
	JNE loop_from_si
	VBROADCASTSS (SI), X0
	PSHUFB avx4BEShuf<>(SB), X0
	PADDL avx4IncBy<>(SB), X0
	VPSHUFB avx4BEShuf<>(SB), X0, X1
	MOVUPS X1, (DI)
	ADDQ $16, DI
	SUBQ $16, BX
	JZ ret
	CMPQ BX, $16
	JB loop_from_x0
	CMPQ BX, $64
	JB bigloop
hugeloop:
	PADDL avx4IncBy<>+0x10(SB), X0
	VPSHUFB avx4BEShuf<>(SB), X0, X1
	MOVUPS X1, (DI)
	PADDL avx4IncBy<>+0x10(SB), X0
	VPSHUFB avx4BEShuf<>(SB), X0, X1
	MOVUPS X1, 16(DI)
	PADDL avx4IncBy<>+0x10(SB), X0
	VPSHUFB avx4BEShuf<>(SB), X0, X1
	MOVUPS X1, 32(DI)
	PADDL avx4IncBy<>+0x10(SB), X0
	VPSHUFB avx4BEShuf<>(SB), X0, X1
	MOVUPS X1, 48(DI)
	ADDQ $64, DI
	SUBQ $64, BX
	JZ ret
	CMPQ BX, $64
	JAE hugeloop
	CMPQ BX, $16
	JB loop_from_x0
bigloop:
	PADDL avx4IncBy<>+0x10(SB), X0
	VPSHUFB avx4BEShuf<>(SB), X0, X1
	MOVUPS X1, (DI)
	ADDQ $16, DI
	SUBQ $16, BX
	JZ ret
	CMPQ BX, $16
	JAE bigloop
loop_from_x0:
	PEXTRD $3, X0, SI
	INCL SI
	BSWAPL SI
loop:
	MOVL SI, (DI)
	BSWAPL SI
	INCL SI
	BSWAPL SI
	ADDQ $4, DI
	SUBQ $4, BX
	JNZ loop
ret:
	RET
loop_from_si:
	MOVL (SI), SI
	JMP loop
