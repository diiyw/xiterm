//go:build cgo && (onix_vulkan || android) && (windows || linux || android || (darwin && !ios)) && !onix_directx && !onix_metal && !onix_opengl && !onix_webgpu && !onix_webgl

package render

/*
#include "native_gpu.h"
*/
import "C"

// The packer writes a mip chain as one flat payload and the Vulkan uploader has
// to find every level inside it, so both sides compute the same block layout.
// They are separate implementations in separate languages with no link-time
// check, and a disagreement would not fail loudly: it would upload level 1
// onwards shifted by a block or two, showing up as corruption that grows down
// the chain.
//
// These two wrappers exist so a test can assert the two agree. They are not
// called on any production path; the driver passes the level count down and
// lets the native side lay out the payload.
func nativeTextureChainSize(format TextureFormat, width, height, levels int) int {
	return int(C.onix_gpu_texture_chain_size(
		C.uint32_t(format), C.uint32_t(width), C.uint32_t(height), C.uint32_t(levels)))
}

func nativeTextureMipLevels(width, height int) int {
	return int(C.onix_gpu_texture_mip_level_count(C.uint32_t(width), C.uint32_t(height)))
}
