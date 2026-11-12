//go:build linux && cgo && onix_drm && (onix_opengl || (linux && !android && !onix_vulkan))

package window

/*
#include "drm_native.h"
*/
import "C"

import "github.com/onix-engine/onix/render"

func newDRMDriver(display *C.OnixDRMDisplay, width, height int) (render.Driver, error) {
	if C.onix_drm_create_gbm(display) == 0 {
		return nil, drmError("initialize GBM")
	}
	return render.NewGBMOpenGLESDriver(
		uintptr(C.onix_drm_gbm_device(display)),
		uintptr(C.onix_drm_gbm_surface(display)),
		width, height,
		func() error {
			if C.onix_drm_present(display) == 0 {
				return drmError("present")
			}
			return nil
		},
	)
}

func drmNativeHandle(display *C.OnixDRMDisplay) uintptr {
	return uintptr(C.onix_drm_gbm_surface(display))
}
