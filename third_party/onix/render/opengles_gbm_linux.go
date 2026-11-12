//go:build linux && !android && cgo && onix_drm && (onix_opengl || (linux && !android && !onix_vulkan))

package render

/*
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <stdint.h>

static EGLDisplay onix_egl_get_gbm_display(uintptr_t device) {
	return eglGetPlatformDisplay(EGL_PLATFORM_GBM_KHR, (void *)device, NULL);
}
*/
import "C"

import "errors"

func NewGBMOpenGLESDriver(device, surface uintptr, width, height int, afterPresent func() error) (Driver, error) {
	if device == 0 || surface == 0 {
		return nil, errors.New("render: invalid GBM display")
	}
	driver, err := newOpenGLESDriver(C.onix_egl_get_gbm_display(C.uintptr_t(device)), surface, width, height)
	if err != nil {
		return nil, err
	}
	driver.(*openGLESDriver).afterPresent = afterPresent
	return driver, nil
}
