// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build amd64,!gccgo,!appengine

#include "textflag.h"

DATA avx8IncBy<>+0x00(SB)/8, $0x00
DATA avx8IncBy<>+0x08(SB)/8, $0x01
DATA avx8IncBy<>+0x10(SB)/8, $0x02
DATA avx8IncBy<>+0x18(SB)/8, $0x02
GLOBL avx8IncBy<>(SB), RODATA, $32

DATA avx8BEShuf<>+0x00(SB)/8, $0x0001020304050607
DATA avx8BEShuf<>+0x08(SB)/8, $0x08090a0b0c0d0e0f
GLOBL avx8BEShuf<>(SB), RODATA, $16

// func incrementBytes8Asm(*byte, *byte, uint64)
TEXT ·incrementBytes8Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), AX
	MOVQ data+8(FP), DI
	MOVQ len+16(FP), BX

	CMPQ BX, $16
	JB loop_from_ax

	CMPB runtime·support_avx(SB), $1
	JNE loop_from_ax

	MOVQ $avx8BEShuf<>(SB), DX

	MOVQ 0(AX), X0
	MOVLHPS X0, X0
	PSHUFB avx8BEShuf<>(SB), X0

	PADDQ avx8IncBy<>(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(DI)

	ADDQ $16, DI
	SUBQ $16, BX
	JZ ret

	CMPQ BX, $16
	JB loop_from_x0

	CMPQ BX, $64
	JB bigloop

hugeloop:
	PADDQ avx8IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(DI)

	PADDQ avx8IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 16(DI)

	PADDQ avx8IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 32(DI)

	PADDQ avx8IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 48(DI)

	ADDQ $64, DI
	SUBQ $64, BX
	JZ ret

	CMPQ BX, $64
	JAE hugeloop

	CMPQ BX, $16
	JB loop_from_x0

bigloop:
	PADDQ avx8IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(DI)

	ADDQ $16, DI
	SUBQ $16, BX
	JZ ret

	CMPQ BX, $16
	JAE bigloop

loop_from_x0:
	PEXTRQ $1, X0, AX

	INCQ AX
	BSWAPQ AX

loop:
	MOVQ AX, 0(DI)

	BSWAPQ AX
	INCQ AX
	BSWAPQ AX

	ADDQ $8, DI
	SUBQ $8, BX
	JNZ loop

ret:
	RET

loop_from_ax:
	MOVQ 0(AX), AX
	JMP loop
