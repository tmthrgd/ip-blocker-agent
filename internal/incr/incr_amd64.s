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
	MOVQ len+16(FP), CX

	CMPQ CX, $16
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
	SUBQ $16, CX
	JZ ret

	CMPQ CX, $16
	JB loop_from_x0

	CMPQ CX, $64
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
	SUBQ $64, CX
	JZ ret

	CMPQ CX, $64
	JGE hugeloop

	CMPQ CX, $16
	JB loop_from_x0

bigloop:
	PADDD avx4IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(DI)

	ADDQ $16, DI
	SUBQ $16, CX
	JZ ret

	CMPQ CX, $16
	JGE bigloop

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
	SUBQ $4, CX
	JNZ loop

ret:
	RET

loop_from_ax:
	MOVL 0(AX), AX
	JMP loop

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
	MOVUPS X1, 0(DI)

	ADDQ $16, DI
	SUBQ $16, CX
	JZ ret

	CMPQ CX, $16
	JB loop_from_x0

	CMPQ CX, $64
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
	SUBQ $64, CX
	JZ ret

	CMPQ CX, $64
	JGE hugeloop

	CMPQ CX, $16
	JB loop_from_x0

bigloop:
	PADDQ avx8IncBy<>+16(SB), X0

	// VPSHUFB 0(DX), X0, X1
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x00; BYTE $0x0a
	MOVUPS X1, 0(DI)

	ADDQ $16, DI
	SUBQ $16, CX
	JZ ret

	CMPQ CX, $16
	JGE bigloop

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
	SUBQ $8, CX
	JNZ loop

ret:
	RET

loop_from_ax:
	MOVQ 0(AX), AX
	JMP loop

DATA avx16IncBy<>+0x00(SB)/8, $0x00
DATA avx16IncBy<>+0x08(SB)/8, $0x01
DATA avx16IncBy<>+0x10(SB)/8, $0x02
DATA avx16IncBy<>+0x18(SB)/8, $0x02
GLOBL avx16IncBy<>(SB), RODATA, $32

DATA avx16Overflow<>+0x00(SB)/8, $0
DATA avx16Overflow<>+0x08(SB)/8, $0xffffffffffffffff
DATA avx16Overflow<>+0x10(SB)/8, $0xffffffffffffffff
DATA avx16Overflow<>+0x18(SB)/8, $0xffffffffffffffff
GLOBL avx16Overflow<>(SB), RODATA, $32

DATA avx16BEShuf<>+0x00(SB)/8, $0x0001020304050607
DATA avx16BEShuf<>+0x08(SB)/8, $0x08090a0b0c0d0e0f
GLOBL avx16BEShuf<>(SB), RODATA, $16

DATA avx16QOne<>+0x00(SB)/8, $0x00
DATA avx16QOne<>+0x08(SB)/8, $0x01
DATA avx16QOne<>+0x10(SB)/8, $0x01
DATA avx16QOne<>+0x18(SB)/8, $0x01
GLOBL avx16QOne<>(SB), RODATA, $32

// func incrementBytes16Asm(*byte, *byte, uint64)
TEXT ·incrementBytes16Asm(SB),NOSPLIT,$0
	MOVQ base+0(FP), AX
	MOVQ data+8(FP), DI
	MOVQ len+16(FP), CX

	CMPQ CX, $32
	JB loop_from_ax

	CMPB runtime·support_avx(SB), $1
	JNE loop_from_ax

	MOVQ $avx16BEShuf<>(SB), R9
	MOVQ $avx16Overflow<>(SB), R10
	MOVQ $avx16QOne<>(SB), R11

	MOVQ 0(AX), X1
	MOVLHPS X1, X1

	MOVQ 8(AX), X0
	MOVLHPS X0, X0

	PSHUFB avx16BEShuf<>(SB), X0
	PSHUFB avx16BEShuf<>(SB), X1

	// VPCMPEQQ (R10), X0, X2
	BYTE $0xc4; BYTE $0xc2; BYTE $0x79; BYTE $0x29; BYTE $0x12
	PAND avx16QOne<>(SB), X2
	PADDQ X2, X1

	PADDQ avx16IncBy<>(SB), X0

	// VPSHUFB 0(R9), X0, X3
	BYTE $0xc4; BYTE $0xc2; BYTE $0x79; BYTE $0x00; BYTE $0x19
	// VPSHUFB 0(R9), X1, X4
	BYTE $0xc4; BYTE $0xc2; BYTE $0x71; BYTE $0x00; BYTE $0x21

	// VPUNPCKLQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6c; BYTE $0xd3
	MOVUPS X2, 0(DI)

	// VPUNPCKHQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6d; BYTE $0xd3
	MOVUPS X2, 16(DI)

	ADDQ $32, DI
	SUBQ $32, CX
	JZ ret

	CMPQ CX, $32
	JB loop_from_x0x1

bigloop:
	// VPOR 16(R11), X0, X2
	BYTE $0xc4; BYTE $0xc1; BYTE $0x79; BYTE $0xeb; BYTE $0x53; BYTE $0x10
	// PCMPEQQ 16(R10), X2
	BYTE $0x66; BYTE $0x41; BYTE $0x0f; BYTE $0x38; BYTE $0x29; BYTE $0x52; BYTE $0x10
	PAND avx16QOne<>+16(SB), X2
	PADDQ X2, X1

	PADDQ avx16IncBy<>+16(SB), X0

	// VPSHUFB 0(R9), X0, X3
	BYTE $0xc4; BYTE $0xc2; BYTE $0x79; BYTE $0x00; BYTE $0x19
	// VPSHUFB 0(R9), X1, X4
	BYTE $0xc4; BYTE $0xc2; BYTE $0x71; BYTE $0x00; BYTE $0x21

	// VPUNPCKLQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6c; BYTE $0xd3
	MOVUPS X2, 0(DI)

	// VPUNPCKHQDQ X3, X4, X2
	BYTE $0xc5; BYTE $0xd9; BYTE $0x6d; BYTE $0xd3
	MOVUPS X2, 16(DI)

	ADDQ $32, DI
	SUBQ $32, CX
	JZ ret

	CMPQ CX, $32
	JGE bigloop

loop_from_x0x1:
	PEXTRQ $1, X0, AX
	PEXTRQ $1, X1, DX

	ADDQ $1, AX
	ADCQ $0, DX
	BSWAPQ DX
	BSWAPQ AX

loop:
	MOVQ DX, 0(DI)
	MOVQ AX, 8(DI)

	BSWAPQ AX
	ADDQ $1, AX
	JC increment_dx
incremented_dx:
	BSWAPQ AX

	ADDQ $16, DI
	SUBQ $16, CX
	JNZ loop

ret:
	RET

loop_from_ax:
	MOVQ 0(AX), DX
	MOVQ 8(AX), AX
	JMP loop

increment_dx:
	BSWAPQ DX
	INCQ DX
	BSWAPQ DX

	JMP incremented_dx
