//go:build darwin && cgo && onix_opengl && !ios

#include "opengl_context.h"
#import <AppKit/AppKit.h>
#include <stdlib.h>
struct OnixGLContext {
    NSOpenGLContext *context;
};
OnixGLContext *onix_gl_context_create(int kind, void *display, uintptr_t window) {
    if (!window)
        return NULL;
    NSOpenGLPixelFormatAttribute attributes[] = {NSOpenGLPFAOpenGLProfile,
                                                 NSOpenGLProfileVersion4_1Core,
                                                 NSOpenGLPFAAccelerated,
                                                 NSOpenGLPFADoubleBuffer,
                                                 NSOpenGLPFAColorSize,
                                                 24,
                                                 NSOpenGLPFAAlphaSize,
                                                 8,
                                                 NSOpenGLPFADepthSize,
                                                 24,
                                                 0};
    NSOpenGLPixelFormat *format = [[NSOpenGLPixelFormat alloc] initWithAttributes:attributes];
    if (!format)
        return NULL;
    OnixGLContext *ctx = calloc(1, sizeof(*ctx));
    if (!ctx) {
        [format release];
        return NULL;
    }
    ctx->context = [[NSOpenGLContext alloc] initWithFormat:format shareContext:nil];
    [format release];
    if (!ctx->context) {
        free(ctx);
        return NULL;
    }
    [ctx->context setView:(NSView *)window];
    GLint opaque = 0;
    [ctx->context setValues:&opaque forParameter:NSOpenGLCPSurfaceOpacity];
    [ctx->context makeCurrentContext];
    return ctx;
}
int onix_gl_context_swap(OnixGLContext *ctx) {
    [ctx->context flushBuffer];
    return 1;
}
void onix_gl_context_vsync(OnixGLContext *ctx, int enabled) {
    GLint interval = enabled;
    [ctx->context setValues:&interval forParameter:NSOpenGLCPSwapInterval];
}
void onix_gl_context_current(OnixGLContext *ctx) {
    if ([NSOpenGLContext currentContext] != ctx->context)
        [ctx->context makeCurrentContext];
}
void onix_gl_context_resize(OnixGLContext *ctx, int width, int height) { [ctx->context update]; }
void onix_gl_context_destroy(OnixGLContext *ctx) {
    if (!ctx)
        return;
    if ([NSOpenGLContext currentContext] == ctx->context)
        [NSOpenGLContext clearCurrentContext];
    [ctx->context clearDrawable];
    [ctx->context release];
    free(ctx);
}
