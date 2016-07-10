// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build amd64,!gccgo,!appengine

#include "textflag.h"

DATA avx4IncBy<>+0x00(SB)/4, $0x00
DATA avx4IncBy<>+0x04(SB)/4, $0x01
DATA avx4IncBy<>+0x08(SB)/4, $0x02
DATA avx4IncBy<>+0x0c(SB)/4, $0x03
DATA avx4IncBy<>+0x10(SB)/4, $0x04
DATA avx4IncBy<>+0x14(SB)/4, $0x04
DATA avx4IncBy<>+0x18(SB)/4, $0x04
DATA avx4IncBy<>+0x1c(SB)/4, $0x04
GLOBL avx4IncBy<>(SB), RODATA, $32

DATA avx4BEShuf<>+0x00(SB)/4, $0x00010203
DATA avx4BEShuf<>+0x04(SB)/4, $0x04050607
DATA avx4BEShuf<>+0x08(SB)/4, $0x08090a0b
DATA avx4BEShuf<>+0x0c(SB)/4, $0x0c0d0e0f
GLOBL avx4BEShuf<>(SB), RODATA, $16

// func incrementBytes4Asm(*byte, *byte, uint64)
TEXT ·incrementBytes4Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), AX
	MOVQ data+8(FP), DI
	MOVQ len+16(FP), BX

	CMPQ BX, $16
	JB loop_from_ax

	CMPB runtime·support_avx(SB), $1
	JNE loop_from_ax

	MOVQ $avx4BEShuf<>(SB), DX

	// VBROADCASTSS 0(AX), X0
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x18; BYTE $0x00
	PSHUFB avx4BEShuf<>(SB), X0

	PADDD avx4IncBy<>(SB), X0

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
	PADDD avx4IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(DI)

	PADDD avx4IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 16(DI)

	PADDD avx4IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 32(DI)

	PADDD avx4IncBy<>+16(SB), X0

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
	PADDD avx4IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(DI)

	ADDQ $16, DI
	SUBQ $16, BX
	JZ ret

	CMPQ BX, $16
	JAE bigloop

loop_from_x0:
	PEXTRD $3, X0, AX

	INCL AX
	BSWAPL AX

loop:
	MOVL AX, 0(DI)

	BSWAPL AX
	INCL AX
	BSWAPL AX

	ADDQ $4, DI
	SUBQ $4, BX
	JNZ loop

ret:
	RET

loop_from_ax:
	MOVL 0(AX), AX
	JMP loop
