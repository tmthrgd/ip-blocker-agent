// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.
//
// This file is auto-generated - do not modify

// +build amd64,!gccgo,!appengine

#include "textflag.h"

DATA avx16IncBy<>+0x08(SB)/8, $0x0000000000000001
DATA avx16IncBy<>+0x10(SB)/8, $0x0000000000000002
DATA avx16IncBy<>+0x18(SB)/8, $0x0000000000000002
GLOBL avx16IncBy<>(SB),RODATA,$32

DATA avx16Overflow<>+0x08(SB)/8, $0xffffffffffffffff
DATA avx16Overflow<>+0x10(SB)/8, $0xffffffffffffffff
DATA avx16Overflow<>+0x18(SB)/8, $0xffffffffffffffff
GLOBL avx16Overflow<>(SB),RODATA,$32

DATA avx16BEShuf<>+0x00(SB)/8, $0x0001020304050607
DATA avx16BEShuf<>+0x08(SB)/8, $0x08090a0b0c0d0e0f
GLOBL avx16BEShuf<>(SB),RODATA,$16

DATA avx16QOne<>+0x08(SB)/8, $0x0000000000000001
DATA avx16QOne<>+0x10(SB)/8, $0x0000000000000001
DATA avx16QOne<>+0x18(SB)/8, $0x0000000000000001
GLOBL avx16QOne<>(SB),RODATA,$32

TEXT ·incrementBytes16Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), SI
	MOVQ data+8(FP), DI
	MOVQ len+16(FP), BX
	CMPQ BX, $32
	JB loop_from_si
	CMPB runtime·support_avx(SB), $1
	JNE loop_from_si
	MOVQ $avx16Overflow<>(SB), R15
	MOVQ (SI), X1
	MOVLHPS X1, X1
	MOVQ 8(SI), X0
	MOVLHPS X0, X0
	PSHUFB avx16BEShuf<>(SB), X0
	PSHUFB avx16BEShuf<>(SB), X1
	// VPCMPEQQ (R15), X0, X2
	BYTE $0xc4; BYTE $0xc2; BYTE $0x79; BYTE $0x29; BYTE $0x17
	PAND avx16QOne<>(SB), X2
	PADDQ X2, X1
	PADDQ avx16IncBy<>(SB), X0
	VPSHUFB avx16BEShuf<>(SB), X0, X3
	VPSHUFB avx16BEShuf<>(SB), X1, X4
	// VPUNPCKLQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6c; BYTE $0xd3
	MOVUPS X2, (DI)
	// VPUNPCKHQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6d; BYTE $0xd3
	MOVUPS X2, 16(DI)
	ADDQ $32, DI
	SUBQ $32, BX
	JZ ret
	CMPQ BX, $32
	JB loop_from_x0x1
bigloop:
	VPOR avx16QOne<>+0x10(SB), X0, X2
	// PCMPEQQ 16(R15), X2
	BYTE $0x66; BYTE $0x41; BYTE $0x0f; BYTE $0x38; BYTE $0x29; BYTE $0x57; BYTE $0x10
	PAND avx16QOne<>+0x10(SB), X2
	PADDQ X2, X1
	PADDQ avx16IncBy<>+0x10(SB), X0
	VPSHUFB avx16BEShuf<>(SB), X0, X3
	VPSHUFB avx16BEShuf<>(SB), X1, X4
	// VPUNPCKLQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6c; BYTE $0xd3
	MOVUPS X2, (DI)
	// VPUNPCKHQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6d; BYTE $0xd3
	MOVUPS X2, 16(DI)
	ADDQ $32, DI
	SUBQ $32, BX
	JZ ret
	CMPQ BX, $32
	JAE bigloop
loop_from_x0x1:
	PEXTRQ $1, X0, AX
	PEXTRQ $1, X1, DX
	ADDQ $1, AX
	ADCQ $0, DX
	BSWAPQ DX
	BSWAPQ AX
loop:
	MOVQ DX, (DI)
	MOVQ AX, 8(DI)
	BSWAPQ AX
	ADDQ $1, AX
	JC increment_dx
incremented_dx:
	BSWAPQ AX
	ADDQ $16, DI
	SUBQ $16, BX
	JNZ loop
ret:
	RET
loop_from_si:
	MOVQ (SI), DX
	MOVQ 8(SI), AX
	JMP loop
increment_dx:
	BSWAPQ DX
	INCQ DX
	BSWAPQ DX
	JMP incremented_dx
