//go:build linux && cgo && (onix_opengl || (linux && !android && !onix_vulkan)) && !android && !onix_drm

#include "opengl_context.h"
#include <GL/glx.h>
#include <GL/glxext.h>
#include <stdlib.h>

struct OnixGLContext {
    Display *display;
    Window window;
    GLXContext context;
};
OnixGLContext *onix_gl_context_create(int kind, void *display, uintptr_t window) {
    if (kind != 1 || !display || !window)
        return NULL;
    XWindowAttributes attributes;
    if (!XGetWindowAttributes(display, window, &attributes))
        return NULL;
    int count = 0;
    GLXFBConfig *configs = glXGetFBConfigs(display, XScreenNumberOfScreen(attributes.screen), &count);
    GLXFBConfig config = NULL;
    for (int i = 0; i < count; i++) {
        int visual = 0, doublebuffer = 0;
        glXGetFBConfigAttrib(display, configs[i], GLX_VISUAL_ID, &visual);
        glXGetFBConfigAttrib(display, configs[i], GLX_DOUBLEBUFFER, &doublebuffer);
        if ((VisualID)visual == XVisualIDFromVisual(attributes.visual) && doublebuffer) {
            config = configs[i];
            break;
        }
    }
    XFree(configs);
    if (!config)
        return NULL;
    PFNGLXCREATECONTEXTATTRIBSARBPROC create =
        (PFNGLXCREATECONTEXTATTRIBSARBPROC)glXGetProcAddressARB((const GLubyte *)"glXCreateContextAttribsARB");
    if (!create)
        return NULL;
    const int context_attributes[] = {
        GLX_CONTEXT_MAJOR_VERSION_ARB,    3,   GLX_CONTEXT_MINOR_VERSION_ARB, 3, GLX_CONTEXT_PROFILE_MASK_ARB,
        GLX_CONTEXT_CORE_PROFILE_BIT_ARB, None};
    OnixGLContext *ctx = calloc(1, sizeof(*ctx));
    if (!ctx)
        return NULL;
    ctx->display = display;
    ctx->window = window;
    ctx->context = create(display, config, NULL, True, context_attributes);
    if (!ctx->context || !glXMakeCurrent(display, window, ctx->context)) {
        onix_gl_context_destroy(ctx);
        return NULL;
    }
    return ctx;
}
int onix_gl_context_swap(OnixGLContext *ctx) {
    glXSwapBuffers(ctx->display, ctx->window);
    return 1;
}
void onix_gl_context_vsync(OnixGLContext *ctx, int enabled) {
    PFNGLXSWAPINTERVALEXTPROC swap =
        (PFNGLXSWAPINTERVALEXTPROC)glXGetProcAddressARB((const GLubyte *)"glXSwapIntervalEXT");
    if (swap)
        swap(ctx->display, ctx->window, enabled);
}
void onix_gl_context_current(OnixGLContext *ctx) {
    if (glXGetCurrentContext() != ctx->context)
        glXMakeCurrent(ctx->display, ctx->window, ctx->context);
}
void onix_gl_context_resize(OnixGLContext *ctx, int width, int height) {}
void onix_gl_context_destroy(OnixGLContext *ctx) {
    if (!ctx)
        return;
    if (ctx->context) {
        if (glXGetCurrentContext() == ctx->context)
            glXMakeCurrent(ctx->display, None, NULL);
        glXDestroyContext(ctx->display, ctx->context);
    }
    free(ctx);
}
