//go:build windows && cgo && onix_opengl

#include "opengl_context.h"
#include <GL/gl.h>
#include <stdint.h>
#include <stdlib.h>
#include <windows.h>

struct OnixGLContext {
    HWND window;
    HDC dc;
    HGLRC context;
};
typedef HGLRC(WINAPI *OnixCreateContext)(HDC, HGLRC, const int *);
typedef BOOL(WINAPI *OnixSwapInterval)(int);
int onix_gl_load(void);

OnixGLContext *onix_gl_context_create(int kind, void *display, uintptr_t window) {
    if (!window)
        return NULL;
    OnixGLContext *ctx = calloc(1, sizeof(*ctx));
    if (!ctx)
        return NULL;
    ctx->window = (HWND)window;
    ctx->dc = GetDC(ctx->window);
    PIXELFORMATDESCRIPTOR format = {.nSize = sizeof(format),
                                    .nVersion = 1,
                                    .dwFlags = PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER,
                                    .iPixelType = PFD_TYPE_RGBA,
                                    .cColorBits = 32,
                                    .cAlphaBits = 8,
                                    .cDepthBits = 24};
    int index = ChoosePixelFormat(ctx->dc, &format);
    if (!index || !SetPixelFormat(ctx->dc, index, &format))
        goto fail;
    HGLRC bootstrap = wglCreateContext(ctx->dc);
    if (!bootstrap || !wglMakeCurrent(ctx->dc, bootstrap)) {
        if (bootstrap)
            wglDeleteContext(bootstrap);
        goto fail;
    }
    OnixCreateContext create = (OnixCreateContext)wglGetProcAddress("wglCreateContextAttribsARB");
    const int attributes[] = {0x2091, 3, 0x2092, 3, 0x9126, 1, 0};
    ctx->context = create ? create(ctx->dc, NULL, attributes) : NULL;
    wglMakeCurrent(NULL, NULL);
    wglDeleteContext(bootstrap);
    if (!ctx->context || !wglMakeCurrent(ctx->dc, ctx->context) || !onix_gl_load())
        goto fail;
    return ctx;
fail:
    onix_gl_context_destroy(ctx);
    return NULL;
}

int onix_gl_context_swap(OnixGLContext *ctx) { return SwapBuffers(ctx->dc); }
void onix_gl_context_vsync(OnixGLContext *ctx, int enabled) {
    OnixSwapInterval interval = (OnixSwapInterval)wglGetProcAddress("wglSwapIntervalEXT");
    if (interval)
        interval(enabled);
}
void onix_gl_context_current(OnixGLContext *ctx) {
    if (wglGetCurrentContext() != ctx->context)
        wglMakeCurrent(ctx->dc, ctx->context);
}
void onix_gl_context_resize(OnixGLContext *ctx, int width, int height) {}
void onix_gl_context_destroy(OnixGLContext *ctx) {
    if (!ctx)
        return;
    if (ctx->context) {
        if (wglGetCurrentContext() == ctx->context)
            wglMakeCurrent(NULL, NULL);
        wglDeleteContext(ctx->context);
    }
    if (ctx->dc)
        ReleaseDC(ctx->window, ctx->dc);
    free(ctx);
}
