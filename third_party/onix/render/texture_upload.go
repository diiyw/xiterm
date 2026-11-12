package render

// packTexturePixels copies a validated batch using the total size accumulated
// while building native update descriptors. Allocate once instead of growing
// the pixel array for each update; repeated uploads reuse the same storage.
func packTexturePixels(buffer []byte, updates []TextureUpdate, length int) []byte {
	if cap(buffer) < length {
		// Keep growth headroom for varying batches without copying old pixels.
		buffer = make([]byte, length, max(length, cap(buffer)+cap(buffer)/2))
	} else {
		buffer = buffer[:length]
	}
	offset := 0
	for _, update := range updates {
		offset += copy(buffer[offset:], update.Pixels)
	}
	return buffer
}
