//go:build !amd64 && !arm64

package graphics

import "github.com/onix-engine/onix/render"

func applyViewAffineBatchArch(instances []render.Instance2D, view [6]float32) {
	applyViewAffineBatchGo(instances, view)
}
