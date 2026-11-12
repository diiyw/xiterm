#ifndef ONIX_OPENGL_CONTEXT_H
#define ONIX_OPENGL_CONTEXT_H
#include <stdint.h>
typedef struct OnixGLContext OnixGLContext;
OnixGLContext *onix_gl_context_create(int kind, void *display, uintptr_t window);
int onix_gl_context_swap(OnixGLContext *context);
void onix_gl_context_vsync(OnixGLContext *context, int enabled);
void onix_gl_context_current(OnixGLContext *context);
void onix_gl_context_resize(OnixGLContext *context, int width, int height);
void onix_gl_context_destroy(OnixGLContext *context);
#endif
