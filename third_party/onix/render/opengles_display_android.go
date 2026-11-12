//go:build android && cgo && onix_opengl

package render

/*
#include <EGL/egl.h>

static EGLDisplay onix_egl_get_default_display(void) {
	return eglGetDisplay(EGL_DEFAULT_DISPLAY);
}
*/
import "C"

func NewOpenGLESDriver(window uintptr, width, height int) (Driver, error) {
	return newOpenGLESDriver(C.onix_egl_get_default_display(), window, width, height)
}
