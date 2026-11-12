package render

import (
	"slices"
	"unsafe"
)

const spriteCompactInstanceBytes = 100
const spriteCustomInstanceBytes = int(unsafe.Sizeof(Instance2D{}))

// Upload scratch is shared across batches and devices, and never retained by
// the GPU. Built-in sprites omit Custom[0:2]; custom shaders keep every lane.
var spriteUploadBuffers = bufferPool[byte]{maxCapacity: 4 << 20}

func spriteInstanceStride(custom bool) int {
	if custom {
		return spriteCustomInstanceBytes
	}
	return spriteCompactInstanceBytes
}

// Vulkan and WebGPU expose FirstInstance to user vertex shaders. Leave enough
// leading space to bind their full-layout stream without changing that index.
func spriteInstanceOffset(offset uint64, draw SpriteDraw) uint64 {
	if draw.Shader.Valid() {
		return max(offset, uint64(draw.FirstInstance)*uint64(spriteCustomInstanceBytes))
	}
	return offset
}

func spriteAttributeOffset(location int, custom bool) int {
	offset := [...]int{0, 12, 24, 36, 48, 64, 80, 96, 112, 128}[location-2]
	if !custom && location >= 9 {
		offset -= 32
	}
	return offset
}

func appendSpriteInstances(dst []byte, instances []Instance2D, custom bool) []byte {
	if len(instances) == 0 {
		return dst
	}
	source := unsafe.Slice((*byte)(unsafe.Pointer(&instances[0])), len(instances)*spriteCustomInstanceBytes)
	if custom {
		return append(dst, source...)
	}
	start := len(dst)
	dst = slices.Grow(dst, len(instances)*spriteCompactInstanceBytes)[:start+len(instances)*spriteCompactInstanceBytes]
	for index := range instances {
		from := source[index*spriteCustomInstanceBytes:]
		to := dst[start+index*spriteCompactInstanceBytes:]
		copy(to[:64], from[:64])
		copy(to[64:100], from[96:132])
	}
	return dst
}

func appendSpriteDrawInstances(dst []byte, instances []Instance2D, draw SpriteDraw) []byte {
	offset := int(spriteInstanceOffset(uint64(len(dst)), draw))
	if offset > len(dst) {
		start := len(dst)
		dst = slices.Grow(dst, offset-start)[:offset]
		clear(dst[start:])
	}
	return appendSpriteInstances(dst, instances[draw.FirstInstance:uint64(draw.FirstInstance)+uint64(draw.InstanceCount)], draw.Shader.Valid())
}
