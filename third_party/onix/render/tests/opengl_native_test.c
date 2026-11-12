#include "../gl_api.h"
#include "../opengl_context.h"
#include <stdio.h>
#include <stdlib.h>

static void require(int success, const char *operation) {
    if (!success) {
        fprintf(stderr, "%s (GL error 0x%x)\n", operation, glGetError());
        exit(1);
    }
}
static GLuint shader(GLenum kind, const char *path) {
    FILE *file = fopen(path, "rb");
    require(file != NULL, path);
    fseek(file, 0, SEEK_END);
    long length = ftell(file);
    rewind(file);
    char *source = calloc(length + 1, 1);
    require(source && fread(source, 1, length, file) == length, "read shader");
    fclose(file);
    GLuint value = glCreateShader(kind);
    glShaderSource(value, 1, (const char **)&source, NULL);
    glCompileShader(value);
    free(source);
    GLint compiled;
    glGetShaderiv(value, GL_COMPILE_STATUS, &compiled);
    if (!compiled) {
        char log[8192];
        glGetShaderInfoLog(value, sizeof(log), NULL, log);
        fprintf(stderr, "%s: %s\n", path, log);
        exit(1);
    }
    return value;
}
int main(int argc, char **argv) {
    HWND window = CreateWindowExW(0, L"STATIC", L"Onix OpenGL regression", WS_OVERLAPPEDWINDOW, 0, 0, 32, 32,
                                 NULL, NULL, GetModuleHandleW(NULL), NULL);
    OnixGLContext *context = onix_gl_context_create(4, NULL, (uintptr_t)window);
    if (!context) { fprintf(stderr, "create OpenGL context\n"); return 1; }
    for (int i = 1; i + 1 < argc; i += 2) {
        GLuint vertex = shader(GL_VERTEX_SHADER, argv[i]), fragment = shader(GL_FRAGMENT_SHADER, argv[i + 1]);
        GLuint program = glCreateProgram();
        glAttachShader(program, vertex); glAttachShader(program, fragment); glLinkProgram(program);
        GLint linked;
        glGetProgramiv(program, GL_LINK_STATUS, &linked);
        if (!linked) {
            char log[8192]; glGetProgramInfoLog(program, sizeof(log), NULL, log);
            fprintf(stderr, "%s: %s\n", argv[i], log); return 1;
        }
        glDeleteProgram(program); glDeleteShader(vertex); glDeleteShader(fragment);
    }
    GLuint texture, target;
    glGenTextures(1, &texture); glBindTexture(GL_TEXTURE_2D, texture);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, 16, 16, 0, GL_RGBA, GL_UNSIGNED_BYTE, NULL);
    glGenFramebuffers(1, &target); glBindFramebuffer(GL_FRAMEBUFFER, target);
    glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, texture, 0);
    require(glCheckFramebufferStatus(GL_FRAMEBUFFER) == GL_FRAMEBUFFER_COMPLETE, "create target");
    glClearColor(1, 0, 0, 1); glClearDepthf(0); glClear(GL_COLOR_BUFFER_BIT);
    unsigned char pixel[4]; glReadPixels(8, 8, 1, 1, GL_RGBA, GL_UNSIGNED_BYTE, pixel);
    require(pixel[0] == 255 && pixel[1] == 0 && pixel[3] == 255, "target readback");
    require(glGetError() == GL_NO_ERROR, "OpenGL calls");
    glDeleteFramebuffers(1, &target); glDeleteTextures(1, &texture);
    onix_gl_context_destroy(context); DestroyWindow(window);
    puts("OpenGL builtin shader compilation and target readback passed");
    return 0;
}
