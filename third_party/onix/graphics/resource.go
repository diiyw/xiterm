package graphics

import "github.com/onix-engine/onix/render"

type ResourceUsage uint8

const (
	ResourceUsageCPU ResourceUsage = 1 << iota
	ResourceUsageGPU
	ResourceUsageDefault = ResourceUsageCPU | ResourceUsageGPU
)

func (u ResourceUsage) Has(flag ResourceUsage) bool {
	return u&flag == flag
}

type Resource[T any] struct {
	Data  T
	Usage ResourceUsage
}

func NewResource[T any](data T, usage ResourceUsage) Resource[T] {
	return Resource[T]{Data: data, Usage: usage}
}

type TextureOptions struct {
	// SRGB applies to color images, never normal or material data maps.
	SRGB             bool
	WrapU, WrapV     render.TextureAddressMode
	Mipmap           bool
	PremultiplyAlpha bool
	Usage            ResourceUsage
}
