//go:build linux && cgo && onix_drm && onix_vulkan && !onix_opengl

package window

/*
#include "drm_native.h"
*/
import "C"

import "github.com/onix-engine/onix/render"

func newDRMDriver(display *C.OnixDRMDisplay, width, height int) (render.Driver, error) {
	if C.onix_drm_save_outputs(display) == 0 {
		return nil, drmError("save outputs")
	}
	driver, err := render.NewVulkanDriver(render.NativeSurface{
		Kind:           render.NativeSurfaceDRM,
		Width:          width,
		Height:         height,
		DRMFD:          int(C.onix_drm_fd(display)),
		DRMConnectorID: uint32(C.onix_drm_connector_id(display)),
		DRMRefreshRate: uint32(C.onix_drm_refresh_rate(display)),
	})
	if err == nil {
		C.onix_drm_restore_on_destroy(display)
	}
	return driver, err
}

func drmNativeHandle(*C.OnixDRMDisplay) uintptr { return 0 }
