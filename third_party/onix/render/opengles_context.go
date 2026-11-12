//go:build cgo && (onix_opengl || (linux && !android && !onix_vulkan)) && (android || (linux && onix_drm))

package render

/*
#cgo android LDFLAGS: -lEGL -lGLESv3 -landroid
#cgo linux LDFLAGS: -lEGL -lGLESv2
#include <EGL/egl.h>
#include <GLES3/gl3.h>
#if defined(__ANDROID__)
#include <android/native_window.h>
#endif
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

static EGLSurface onix_egl_create_window_surface(EGLDisplay display, EGLConfig config, uintptr_t window) {
	return eglCreateWindowSurface(display, config, (EGLNativeWindowType)window, NULL);
}

static EGLBoolean onix_egl_choose_config(EGLDisplay display, EGLConfig *config, EGLint *count) {
#if defined(__ANDROID__)
	const EGLint attributes[] = {
		EGL_SURFACE_TYPE, EGL_WINDOW_BIT,
		EGL_RENDERABLE_TYPE, EGL_OPENGL_ES3_BIT,
		EGL_RED_SIZE, 8,
		EGL_GREEN_SIZE, 8,
		EGL_BLUE_SIZE, 8,
		EGL_ALPHA_SIZE, 8,
		EGL_DEPTH_SIZE, 0,
		EGL_SAMPLE_BUFFERS, 0,
		EGL_NONE,
	};
#else
	const EGLint attributes[] = {
		EGL_SURFACE_TYPE, EGL_WINDOW_BIT,
		EGL_RENDERABLE_TYPE, EGL_OPENGL_ES3_BIT,
		EGL_RED_SIZE, 8,
		EGL_GREEN_SIZE, 8,
		EGL_BLUE_SIZE, 8,
		EGL_DEPTH_SIZE, 0,
		EGL_SAMPLE_BUFFERS, 0,
		EGL_NATIVE_VISUAL_ID, 0x34325258,
		EGL_NONE,
	};
#endif
	return eglChooseConfig(display, attributes, config, 1, count);
}

static void onix_native_window_set_buffers_geometry(uintptr_t window, int32_t format) {
#if defined(__ANDROID__)
	ANativeWindow_setBuffersGeometry((ANativeWindow *)window, 0, 0, format);
#else
	(void)window;
	(void)format;
#endif
}

*/
import "C"
import (
	"errors"
	"fmt"
)

type glContext struct {
	display C.EGLDisplay
	surface C.EGLSurface
	context C.EGLContext
}

func platformGLShaderSource(source string) string { return source }
func newOpenGLESDriver(display C.EGLDisplay, window uintptr, width, height int) (Driver, error) {
	if window == 0 || width <= 0 || height <= 0 {
		return nil, errors.New("render: invalid OpenGL ES surface")
	}
	driver := &openGLESDriver{
		width:      width,
		height:     height,
		viewport:   Rect{Width: int32(width), Height: int32(height)},
		frameIndex: glesFramesInFlight - 1,
	}
	driver.display = display
	var noDisplay C.EGLDisplay
	if driver.display == noDisplay || C.eglInitialize(driver.display, nil, nil) == C.EGL_FALSE {
		return nil, eglError("initialize display")
	}
	if C.eglBindAPI(C.EGL_OPENGL_ES_API) == C.EGL_FALSE {
		err := eglError("bind OpenGL ES API")
		driver.Destroy()
		return nil, err
	}
	var config C.EGLConfig
	var count C.EGLint
	if C.onix_egl_choose_config(driver.display, &config, &count) == C.EGL_FALSE || count == 0 {
		err := eglError("choose OpenGL ES 3 config")
		driver.Destroy()
		return nil, err
	}
	var visualID C.EGLint
	if C.eglGetConfigAttrib(driver.display, config, C.EGL_NATIVE_VISUAL_ID, &visualID) == C.EGL_TRUE {
		C.onix_native_window_set_buffers_geometry(C.uintptr_t(window), C.int32_t(visualID))
	}
	driver.surface = C.onix_egl_create_window_surface(driver.display, config, C.uintptr_t(window))
	var noSurface C.EGLSurface
	if driver.surface == noSurface {
		err := eglError("create window surface")
		driver.Destroy()
		return nil, err
	}
	contextAttributes := [...]C.EGLint{C.EGL_CONTEXT_CLIENT_VERSION, 3, C.EGL_NONE}
	var noContext C.EGLContext
	driver.context = C.eglCreateContext(driver.display, config, noContext, &contextAttributes[0])
	if driver.context == noContext || C.eglMakeCurrent(driver.display, driver.surface, driver.surface, driver.context) == C.EGL_FALSE {
		err := eglError("create OpenGL ES 3 context")
		driver.Destroy()
		return nil, err
	}
	var surfaceWidth, surfaceHeight C.EGLint
	if C.eglQuerySurface(driver.display, driver.surface, C.EGL_WIDTH, &surfaceWidth) == C.EGL_TRUE &&
		C.eglQuerySurface(driver.display, driver.surface, C.EGL_HEIGHT, &surfaceHeight) == C.EGL_TRUE && surfaceWidth > 0 && surfaceHeight > 0 {
		driver.width, driver.height = int(surfaceWidth), int(surfaceHeight)
		driver.viewport = Rect{Width: int32(surfaceWidth), Height: int32(surfaceHeight)}
	}
	return initializeGLDriver(driver)
}

func (d *openGLESDriver) swapContextBuffers() error {
	if C.eglSwapBuffers(d.display, d.surface) == C.EGL_FALSE {
		return eglError("present")
	}
	return nil
}
func (d *openGLESDriver) setContextVSync(enabled bool) {
	C.eglSwapInterval(d.display, C.EGLint(boolInt(enabled)))
}
func (d *openGLESDriver) resizeContext(width, height int) {}
func (d *openGLESDriver) makeContextCurrent() {
	var zero C.EGLContext
	if d.context != zero {
		C.eglMakeCurrent(d.display, d.surface, d.surface, d.context)
	}
}
func (d *openGLESDriver) destroyContext() {
	var display C.EGLDisplay
	var surface C.EGLSurface
	var context C.EGLContext
	if d.display != display {
		C.eglMakeCurrent(d.display, surface, surface, context)
		if d.context != context {
			C.eglDestroyContext(d.display, d.context)
		}
		if d.surface != surface {
			C.eglDestroySurface(d.display, d.surface)
		}
		C.eglTerminate(d.display)
		C.eglReleaseThread()
	}
	d.display, d.surface, d.context = display, surface, context
}
func eglError(operation string) error {
	return fmt.Errorf("render: OpenGL ES %s failed with EGL error %#x", operation, uint32(C.eglGetError()))
}
