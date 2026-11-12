//go:build android && cgo && !onix_opengl

package render

func NewAndroidDriver(surface NativeSurface) (Driver, error) {
	return NewVulkanDriver(surface)
}
