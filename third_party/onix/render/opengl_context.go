//go:build cgo && (onix_opengl || (linux && !android && !onix_vulkan)) && !android && !ios && !onix_drm && (windows || linux || darwin)

package render

/*
#cgo CFLAGS: -DONIX_DESKTOP_GL
#cgo windows LDFLAGS: -lopengl32 -lgdi32
#cgo linux LDFLAGS: -lGL -lX11
#cgo darwin LDFLAGS: -framework OpenGL -framework AppKit
#include "opengl_context.h"
*/
import "C"
import (
	"errors"
	"strings"
)

type glContext struct{ native *C.OnixGLContext }

func NewOpenGLDriver(surface NativeSurface) (Driver, error) {
	if surface.Window == 0 || surface.Width <= 0 || surface.Height <= 0 {
		return nil, errors.New("render: invalid OpenGL surface")
	}
	native := C.onix_gl_context_create(C.int(surface.Kind), surface.Display, C.uintptr_t(surface.Window))
	if native == nil {
		return nil, errors.New("render: could not create an OpenGL 3.3 core context")
	}
	return initializeGLDriver(&openGLESDriver{glContext: glContext{native: native}, width: surface.Width, height: surface.Height, viewport: Rect{Width: int32(surface.Width), Height: int32(surface.Height)}, frameIndex: glesFramesInFlight - 1})
}
func platformGLShaderSource(source string) string {
	return strings.NewReplacer("#version 300 es", "#version 330 core", "precision highp float;", "", "precision highp int;", "", "precision mediump float;", "", "precision mediump int;", "", "precision lowp float;", "", "precision lowp int;", "", "highp ", "", "mediump ", "", "lowp ", "").Replace(source)
}
func (d *openGLESDriver) swapContextBuffers() error {
	if C.onix_gl_context_swap(d.native) == 0 {
		return errors.New("render: OpenGL swap failed")
	}
	return nil
}
func (d *openGLESDriver) setContextVSync(enabled bool) {
	C.onix_gl_context_vsync(d.native, C.int(boolInt(enabled)))
}
func (d *openGLESDriver) makeContextCurrent() {
	if d.native != nil {
		C.onix_gl_context_current(d.native)
	}
}
func (d *openGLESDriver) resizeContext(width, height int) {
	C.onix_gl_context_resize(d.native, C.int(width), C.int(height))
}
func (d *openGLESDriver) destroyContext() {
	if d.native != nil {
		C.onix_gl_context_destroy(d.native)
		d.native = nil
	}
}
