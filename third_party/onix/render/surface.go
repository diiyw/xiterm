package render

import "unsafe"

type NativeSurfaceKind int

const (
	NativeSurfaceXlib NativeSurfaceKind = iota + 1
	NativeSurfaceWayland
	NativeSurfaceAndroid
	NativeSurfaceWin32
	NativeSurfaceDRM
	NativeSurfaceMetal
)

type NativeSurface struct {
	Display        unsafe.Pointer
	Kind           NativeSurfaceKind
	Window         uintptr
	Width          int
	Height         int
	DRMFD          int
	DRMConnectorID uint32
	DRMRefreshRate uint32 // Millihertz; zero selects the highest rate at Width x Height.
	Transparent    bool
}
