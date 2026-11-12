#include "textflag.h"

// applyViewAffineKernel composes view * model for strided Instance2D models.
// Each row is evaluated as a four-lane SSE vector; only its first three lanes
// are stored, leaving TextureModel and the remaining instance data untouched.
TEXT ·applyViewAffineKernel(SB), NOSPLIT, $0-32
	MOVQ models+0(FP), AX
	MOVQ count+8(FP), CX
	TESTQ CX, CX
	JE done
	MOVQ stride+16(FP), DX
	MOVQ view+24(FP), SI

	MOVSS 0(SI), X4
	SHUFPS $0, X4, X4
	MOVSS 4(SI), X5
	SHUFPS $0, X5, X5
	PXOR X6, X6
	MOVSS 8(SI), X6
	PSLLDQ $8, X6

	MOVSS 12(SI), X7
	SHUFPS $0, X7, X7
	MOVSS 16(SI), X8
	SHUFPS $0, X8, X8
	PXOR X9, X9
	MOVSS 20(SI), X9
	PSLLDQ $8, X9

loop:
	MOVUPS 0(AX), X0
	MOVUPS 12(AX), X1

	MOVAPS X0, X2
	MULPS X4, X2
	MOVAPS X1, X3
	MULPS X5, X3
	ADDPS X3, X2
	ADDPS X6, X2

	MOVAPS X0, X10
	MULPS X7, X10
	MOVAPS X1, X11
	MULPS X8, X11
	ADDPS X11, X10
	ADDPS X9, X10

	MOVQ X2, 0(AX)
	PSHUFD $0xaa, X2, X3
	MOVSS X3, 8(AX)
	MOVQ X10, 12(AX)
	PSHUFD $0xaa, X10, X11
	MOVSS X11, 20(AX)

	ADDQ DX, AX
	DECQ CX
	JNZ loop

done:
	RET
