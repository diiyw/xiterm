#include "textflag.h"

// applyViewAffineKernel composes view * model for strided Instance2D models.
// The Go arm64 compiler evaluates each dot product as a rounded second product
// followed by FMLA, so this kernel preserves the same operation order.
TEXT ·applyViewAffineKernel(SB), NOSPLIT, $0-32
	MOVD models+0(FP), R0
	MOVD count+8(FP), R1
	CBZ R1, done
	MOVD stride+16(FP), R2
	MOVD view+24(FP), R3

	FMOVS 0(R3), F4
	VDUP V4.S[0], V4.S4
	FMOVS 4(R3), F5
	VDUP V5.S[0], V5.S4
	FMOVS 8(R3), F6
	FMOVS 12(R3), F7
	VDUP V7.S[0], V7.S4
	FMOVS 16(R3), F8
	VDUP V8.S[0], V8.S4
	FMOVS 20(R3), F9

loop:
	VLD1 (R0), [V0.S4]
	ADD $12, R0, R4
	VLD1 (R4), [V1.S4]

	// FMUL V1.S4, V5.S4, V2.S4. Go's assembler does not expose this
	// vector form, so keep the instruction encoding next to its operands.
	WORD $0x6e25dc22
	VFMLA V4.S4, V0.S4, V2.S4
	WORD $0x6e28dc2a // FMUL V1.S4, V8.S4, V10.S4
	VFMLA V7.S4, V0.S4, V10.S4

	VST1 [V2.S2], (R0)
	VDUP V2.S[2], V12.S4
	FADDS F6, F12, F12
	FMOVS F12, 8(R0)
	VST1 [V10.S2], (R4)
	VDUP V10.S[2], V12.S4
	FADDS F9, F12, F12
	FMOVS F12, 8(R4)

	ADD R2, R0, R0
	SUB $1, R1, R1
	CBNZ R1, loop

done:
	RET
