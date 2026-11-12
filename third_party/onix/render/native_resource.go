package render

// Native backends keep small resource metadata in generation slots. GPU
// ownership remains with the backend's native resource and retirement queues.
type nativeMesh struct {
	native      uint32
	deformation meshDeformationLayout
}

type nativeRenderTarget struct {
	native uint32
	color  TextureHandle
}
