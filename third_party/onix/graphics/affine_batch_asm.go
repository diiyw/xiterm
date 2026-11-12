//go:build amd64 || arm64

package graphics

import (
	"unsafe"

	"github.com/onix-engine/onix/render"
)

func applyViewAffineBatchArch(instances []render.Instance2D, view [6]float32) {
	if len(instances) == 0 {
		return
	}
	applyViewAffineKernel(
		&instances[0].Model[0],
		uintptr(len(instances)),
		unsafe.Sizeof(instances[0]),
		&view,
	)
}

//go:noescape
func applyViewAffineKernel(models *float32, count, stride uintptr, view *[6]float32)
