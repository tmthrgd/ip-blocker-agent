// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// +build amd64,!gccgo,!appengine

#include "textflag.h"

DATA avxIncBy<>+0x00(SB)/4, $0x00
DATA avxIncBy<>+0x04(SB)/4, $0x01
DATA avxIncBy<>+0x08(SB)/4, $0x02
DATA avxIncBy<>+0x0c(SB)/4, $0x03
GLOBL avxIncBy<>(SB), RODATA, $16

DATA avxIncBy2<>+0x00(SB)/4, $0x04
DATA avxIncBy2<>+0x04(SB)/4, $0x04
DATA avxIncBy2<>+0x08(SB)/4, $0x04
DATA avxIncBy2<>+0x0c(SB)/4, $0x04
GLOBL avxIncBy2<>(SB), RODATA, $16

DATA avxBEShuf<>+0x00(SB)/4, $0x00010203
DATA avxBEShuf<>+0x04(SB)/4, $0x04050607
DATA avxBEShuf<>+0x08(SB)/4, $0x08090a0b
DATA avxBEShuf<>+0x0c(SB)/4, $0x0c0d0e0f
GLOBL avxBEShuf<>(SB), RODATA, $16

// func incrementBytes4Asm(*byte, *byte, uint64)
TEXT ·incrementBytes4Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), AX
	MOVQ data+8(FP), BX
	MOVQ len+16(FP), CX

	CMPQ CX, $16
	JB loop_from_ax

	CMPB runtime·support_avx(SB), $1
	JNE loop_from_ax

	MOVQ $avxBEShuf<>(SB), DX

	// VBROADCASTSS 0(AX), X0
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x18; BYTE $0x00
	PSHUFB avxBEShuf<>(SB), X0

	PADDD avxIncBy<>(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(BX)

	ADDQ $16, BX
	SUBQ $16, CX
	JZ ret

	CMPQ CX, $16
	JB loop_from_x0

bigloop:
	PADDD avxIncBy2<>(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(BX)

	ADDQ $16, BX
	SUBQ $16, CX
	JZ ret

	CMPQ CX, $16
	JGE bigloop

loop_from_x0:
	PSHUFD $0xff, X0, X0
	MOVL X0, AX

	INCL AX
	BSWAPL AX

loop:
	MOVL AX, 0(BX)

	BSWAPL AX
	INCL AX
	BSWAPL AX

	ADDQ $4, BX
	SUBQ $4, CX
	JNZ loop

ret:
	RET

loop_from_ax:
	MOVL 0(AX), AX
	JMP loop

DATA avx8IncBy<>+0x00(SB)/8, $0x00
DATA avx8IncBy<>+0x08(SB)/8, $0x01
GLOBL avx8IncBy<>(SB), RODATA, $16

DATA avx8IncBy2<>+0x00(SB)/8, $0x02
DATA avx8IncBy2<>+0x08(SB)/8, $0x02
GLOBL avx8IncBy2<>(SB), RODATA, $16

DATA avx8BEShuf<>+0x00(SB)/8, $0x0001020304050607
DATA avx8BEShuf<>+0x08(SB)/8, $0x08090a0b0c0d0e0f
GLOBL avx8BEShuf<>(SB), RODATA, $16

// func incrementBytes8Asm(*byte, *byte, uint64)
TEXT ·incrementBytes8Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), AX
	MOVQ data+8(FP), BX
	MOVQ len+16(FP), CX

	CMPQ CX, $16
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
	MOVUPS X1, 0(BX)

	ADDQ $16, BX
	SUBQ $16, CX
	JZ ret

	CMPQ CX, $16
	JB loop_from_x0

bigloop:
	PADDQ avx8IncBy2<>(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(BX)

	ADDQ $16, BX
	SUBQ $16, CX
	JZ ret

	CMPQ CX, $16
	JGE bigloop

loop_from_x0:
	MOVHLPS X0, X0
	MOVQ X0, AX

	INCQ AX
	BSWAPQ AX

loop:
	MOVQ AX, 0(BX)

	BSWAPQ AX
	INCQ AX
	BSWAPQ AX

	ADDQ $8, BX
	SUBQ $8, CX
	JNZ loop

ret:
	RET

loop_from_ax:
	MOVQ 0(AX), AX
	JMP loop

DATA avx16IncBy<>+0x00(SB)/8, $0x00
DATA avx16IncBy<>+0x08(SB)/8, $0x01
GLOBL avx16IncBy<>(SB), RODATA, $16

DATA avx16IncBy2<>+0x00(SB)/8, $0x02
DATA avx16IncBy2<>+0x08(SB)/8, $0x02
GLOBL avx16IncBy2<>(SB), RODATA, $16

DATA avx16Overflow<>+0x00(SB)/8, $0
DATA avx16Overflow<>+0x08(SB)/8, $0xffffffffffffffff
GLOBL avx16Overflow<>(SB), RODATA, $16

DATA avx16Overflow2<>+0x00(SB)/8, $0xffffffffffffffff
DATA avx16Overflow2<>+0x08(SB)/8, $0xffffffffffffffff
GLOBL avx16Overflow2<>(SB), RODATA, $16

DATA avx16BEShuf<>+0x00(SB)/8, $0x0001020304050607
DATA avx16BEShuf<>+0x08(SB)/8, $0x08090a0b0c0d0e0f
GLOBL avx16BEShuf<>(SB), RODATA, $16

DATA avx16QOne<>+0x00(SB)/8, $0x00
DATA avx16QOne<>+0x08(SB)/8, $0x01
GLOBL avx16QOne<>(SB), RODATA, $16

DATA avx16QOne2<>+0x00(SB)/8, $0x01
DATA avx16QOne2<>+0x08(SB)/8, $0x01
GLOBL avx16QOne2<>(SB), RODATA, $16

// func incrementBytes16Asm(*byte, *byte, uint64)
TEXT ·incrementBytes16Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), AX
	MOVQ data+8(FP), BX
	MOVQ len+16(FP), CX

	CMPQ CX, $32
	JB loop_from_ax

	CMPB runtime·support_avx(SB), $1
	JNE loop_from_ax

	MOVQ $avx16Overflow<>(SB), DX
	MOVQ $avx16Overflow2<>(SB), R10
	MOVQ $avx16BEShuf<>(SB), R9
	MOVQ $avx16QOne2<>(SB), R11

	MOVQ 0(AX), X1
	MOVLHPS X1, X1

	MOVQ 8(AX), X0
	MOVLHPS X0, X0

	PSHUFB avx16BEShuf<>(SB), X0
	PSHUFB avx16BEShuf<>(SB), X1

	// VPCMPEQQ (DX), X0, X2
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x29; BYTE $0x12
	PAND avx16QOne<>(SB), X2
	PADDQ X2, X1

	PADDQ avx16IncBy<>(SB), X0

	// VPSHUFB 0(R9), X0, X3
	BYTE $0xc4; BYTE $0xc2; BYTE $0x79; BYTE $0x00; BYTE $0x19
	// VPSHUFB 0(R9), X1, X4
	BYTE $0xc4; BYTE $0xc2; BYTE $0x71; BYTE $0x00; BYTE $0x21

	// VPUNPCKLQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6c; BYTE $0xd3
	MOVUPS X2, 0(BX)

	// VPUNPCKHQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6d; BYTE $0xd3
	MOVUPS X2, 16(BX)

	ADDQ $32, BX
	SUBQ $32, CX
	JZ ret

	CMPQ CX, $32
	JB loop_from_x0x1

bigloop:
	// VPOR (R11), X0, X2
	BYTE $0xc4; BYTE $0xc1; BYTE $0x79; BYTE $0xeb; BYTE $0x13
	// PCMPEQQ (R10), X2
	BYTE $0x66; BYTE $0x41; BYTE $0x0f; BYTE $0x38; BYTE $0x29; BYTE $0x12
	PAND avx16QOne2<>(SB), X2
	PADDQ X2, X1

	PADDQ avx16IncBy2<>(SB), X0

	// VPSHUFB 0(R9), X0, X3
	BYTE $0xc4; BYTE $0xc2; BYTE $0x79; BYTE $0x00; BYTE $0x19
	// VPSHUFB 0(R9), X1, X4
	BYTE $0xc4; BYTE $0xc2; BYTE $0x71; BYTE $0x00; BYTE $0x21

	// VPUNPCKLQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6c; BYTE $0xd3
	MOVUPS X2, 0(BX)

	// VPUNPCKHQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6d; BYTE $0xd3
	MOVUPS X2, 16(BX)

	ADDQ $32, BX
	SUBQ $32, CX
	JZ ret

	CMPQ CX, $32
	JGE bigloop

loop_from_x0x1:
	MOVHLPS X0, X0
	MOVHLPS X1, X1

	MOVQ X0, AX
	MOVQ X1, DX

	ADDQ $1, AX
	JNC skiped_high_1

	INCQ DX

skiped_high_1:
	BSWAPQ DX
	BSWAPQ AX

loop:
	MOVQ DX, 0(BX)
	MOVQ AX, 8(BX)

	BSWAPQ AX
	ADDQ $1, AX
	JNC skiped_high_2

	BSWAPQ DX
	INCQ DX
	BSWAPQ DX

skiped_high_2:
	BSWAPQ AX

	ADDQ $16, BX
	SUBQ $16, CX
	JNZ loop

ret:
	RET

loop_from_ax:
	MOVQ 0(AX), DX
	MOVQ 8(AX), AX
	JMP loop
