package graphics

import "sync"

// Only staging bytes belong in the pool; textures and their CPU shadows keep
// independent ownership. Drivers consume CreateTexture pixels before returning.
type textureUploadScratch struct {
	pixels []byte
}

const textureUploadScratchLimit = 16 << 20

var textureUploadScratchPool = sync.Pool{New: func() any { return &textureUploadScratch{} }}

func acquireTextureUploadScratch(length int) *textureUploadScratch {
	scratch := textureUploadScratchPool.Get().(*textureUploadScratch)
	if cap(scratch.pixels) < length {
		scratch.pixels = make([]byte, length)
	} else {
		scratch.pixels = scratch.pixels[:length]
	}
	return scratch
}

func releaseTextureUploadScratch(scratch *textureUploadScratch) {
	if cap(scratch.pixels) > textureUploadScratchLimit {
		scratch.pixels = nil
	}
	textureUploadScratchPool.Put(scratch)
}
