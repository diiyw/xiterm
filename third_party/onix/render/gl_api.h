#ifndef ONIX_GL_API_H
#define ONIX_GL_API_H
#if defined(ONIX_DESKTOP_GL)
#if defined(__APPLE__)
#define GL_SILENCE_DEPRECATION
#include <OpenGL/gl3.h>
#define glClearDepthf(value) glClearDepth(value)
#else
#define GL_GLEXT_PROTOTYPES
#include <GL/glcorearb.h>
#if !defined(_WIN32)
#define glClearDepthf(value) glClearDepth(value)
#endif
#endif
#else
#include <GLES3/gl3.h>
#endif
#if defined(_WIN32) && defined(ONIX_DESKTOP_GL)
#include <windows.h>
extern PFNGLACTIVETEXTUREPROC onix_ptr_glActiveTexture;
static inline void onix_call_glActiveTexture(GLenum texture) { onix_ptr_glActiveTexture(texture); }
#define glActiveTexture onix_call_glActiveTexture
extern PFNGLATTACHSHADERPROC onix_ptr_glAttachShader;
static inline void onix_call_glAttachShader(GLuint program, GLuint shader) { onix_ptr_glAttachShader(program, shader); }
#define glAttachShader onix_call_glAttachShader
extern PFNGLBINDBUFFERPROC onix_ptr_glBindBuffer;
static inline void onix_call_glBindBuffer(GLenum target, GLuint buffer) { onix_ptr_glBindBuffer(target, buffer); }
#define glBindBuffer onix_call_glBindBuffer
extern PFNGLBINDFRAMEBUFFERPROC onix_ptr_glBindFramebuffer;
static inline void onix_call_glBindFramebuffer(GLenum target, GLuint framebuffer) {
    onix_ptr_glBindFramebuffer(target, framebuffer);
}
#define glBindFramebuffer onix_call_glBindFramebuffer
extern PFNGLBINDRENDERBUFFERPROC onix_ptr_glBindRenderbuffer;
static inline void onix_call_glBindRenderbuffer(GLenum target, GLuint renderbuffer) {
    onix_ptr_glBindRenderbuffer(target, renderbuffer);
}
#define glBindRenderbuffer onix_call_glBindRenderbuffer
extern PFNGLBINDTEXTUREPROC onix_ptr_glBindTexture;
static inline void onix_call_glBindTexture(GLenum target, GLuint texture) { onix_ptr_glBindTexture(target, texture); }
#define glBindTexture onix_call_glBindTexture
extern PFNGLBINDVERTEXARRAYPROC onix_ptr_glBindVertexArray;
static inline void onix_call_glBindVertexArray(GLuint array) { onix_ptr_glBindVertexArray(array); }
#define glBindVertexArray onix_call_glBindVertexArray
extern PFNGLBLENDCOLORPROC onix_ptr_glBlendColor;
static inline void onix_call_glBlendColor(GLfloat red, GLfloat green, GLfloat blue, GLfloat alpha) {
    onix_ptr_glBlendColor(red, green, blue, alpha);
}
#define glBlendColor onix_call_glBlendColor
extern PFNGLBLENDFUNCSEPARATEPROC onix_ptr_glBlendFuncSeparate;
static inline void onix_call_glBlendFuncSeparate(GLenum sfactorRGB, GLenum dfactorRGB, GLenum sfactorAlpha,
                                                 GLenum dfactorAlpha) {
    onix_ptr_glBlendFuncSeparate(sfactorRGB, dfactorRGB, sfactorAlpha, dfactorAlpha);
}
#define glBlendFuncSeparate onix_call_glBlendFuncSeparate
extern PFNGLBLITFRAMEBUFFERPROC onix_ptr_glBlitFramebuffer;
static inline void onix_call_glBlitFramebuffer(GLint srcX0, GLint srcY0, GLint srcX1, GLint srcY1, GLint dstX0,
                                               GLint dstY0, GLint dstX1, GLint dstY1, GLbitfield mask, GLenum filter) {
    onix_ptr_glBlitFramebuffer(srcX0, srcY0, srcX1, srcY1, dstX0, dstY0, dstX1, dstY1, mask, filter);
}
#define glBlitFramebuffer onix_call_glBlitFramebuffer
extern PFNGLBUFFERDATAPROC onix_ptr_glBufferData;
static inline void onix_call_glBufferData(GLenum target, GLsizeiptr size, const void *data, GLenum usage) {
    onix_ptr_glBufferData(target, size, data, usage);
}
#define glBufferData onix_call_glBufferData
extern PFNGLBUFFERSUBDATAPROC onix_ptr_glBufferSubData;
static inline void onix_call_glBufferSubData(GLenum target, GLintptr offset, GLsizeiptr size, const void *data) {
    onix_ptr_glBufferSubData(target, offset, size, data);
}
#define glBufferSubData onix_call_glBufferSubData
extern PFNGLCHECKFRAMEBUFFERSTATUSPROC onix_ptr_glCheckFramebufferStatus;
static inline GLenum onix_call_glCheckFramebufferStatus(GLenum target) {
    return onix_ptr_glCheckFramebufferStatus(target);
}
#define glCheckFramebufferStatus onix_call_glCheckFramebufferStatus
extern PFNGLCLEARPROC onix_ptr_glClear;
static inline void onix_call_glClear(GLbitfield mask) { onix_ptr_glClear(mask); }
#define glClear onix_call_glClear
extern PFNGLCLEARBUFFERUIVPROC onix_ptr_glClearBufferuiv;
static inline void onix_call_glClearBufferuiv(GLenum buffer, GLint drawbuffer, const GLuint *value) {
    onix_ptr_glClearBufferuiv(buffer, drawbuffer, value);
}
#define glClearBufferuiv onix_call_glClearBufferuiv
extern PFNGLCLEARCOLORPROC onix_ptr_glClearColor;
static inline void onix_call_glClearColor(GLfloat red, GLfloat green, GLfloat blue, GLfloat alpha) {
    onix_ptr_glClearColor(red, green, blue, alpha);
}
#define glClearColor onix_call_glClearColor
extern PFNGLCLEARDEPTHPROC onix_ptr_glClearDepthf;
static inline void onix_call_glClearDepthf(GLfloat d) { onix_ptr_glClearDepthf(d); }
#define glClearDepthf onix_call_glClearDepthf
extern PFNGLCLIENTWAITSYNCPROC onix_ptr_glClientWaitSync;
static inline GLenum onix_call_glClientWaitSync(GLsync sync, GLbitfield flags, GLuint64 timeout) {
    return onix_ptr_glClientWaitSync(sync, flags, timeout);
}
#define glClientWaitSync onix_call_glClientWaitSync
extern PFNGLCOMPILESHADERPROC onix_ptr_glCompileShader;
static inline void onix_call_glCompileShader(GLuint shader) { onix_ptr_glCompileShader(shader); }
#define glCompileShader onix_call_glCompileShader
extern PFNGLCOMPRESSEDTEXIMAGE2DPROC onix_ptr_glCompressedTexImage2D;
static inline void onix_call_glCompressedTexImage2D(GLenum target, GLint level, GLenum internalformat, GLsizei width,
                                                    GLsizei height, GLint border, GLsizei imageSize, const void *data) {
    onix_ptr_glCompressedTexImage2D(target, level, internalformat, width, height, border, imageSize, data);
}
#define glCompressedTexImage2D onix_call_glCompressedTexImage2D
extern PFNGLCREATEPROGRAMPROC onix_ptr_glCreateProgram;
static inline GLuint onix_call_glCreateProgram(void) { return onix_ptr_glCreateProgram(); }
#define glCreateProgram onix_call_glCreateProgram
extern PFNGLCREATESHADERPROC onix_ptr_glCreateShader;
static inline GLuint onix_call_glCreateShader(GLenum type) { return onix_ptr_glCreateShader(type); }
#define glCreateShader onix_call_glCreateShader
extern PFNGLDELETEBUFFERSPROC onix_ptr_glDeleteBuffers;
static inline void onix_call_glDeleteBuffers(GLsizei n, const GLuint *buffers) { onix_ptr_glDeleteBuffers(n, buffers); }
#define glDeleteBuffers onix_call_glDeleteBuffers
extern PFNGLDELETEFRAMEBUFFERSPROC onix_ptr_glDeleteFramebuffers;
static inline void onix_call_glDeleteFramebuffers(GLsizei n, const GLuint *framebuffers) {
    onix_ptr_glDeleteFramebuffers(n, framebuffers);
}
#define glDeleteFramebuffers onix_call_glDeleteFramebuffers
extern PFNGLDELETEPROGRAMPROC onix_ptr_glDeleteProgram;
static inline void onix_call_glDeleteProgram(GLuint program) { onix_ptr_glDeleteProgram(program); }
#define glDeleteProgram onix_call_glDeleteProgram
extern PFNGLDELETERENDERBUFFERSPROC onix_ptr_glDeleteRenderbuffers;
static inline void onix_call_glDeleteRenderbuffers(GLsizei n, const GLuint *renderbuffers) {
    onix_ptr_glDeleteRenderbuffers(n, renderbuffers);
}
#define glDeleteRenderbuffers onix_call_glDeleteRenderbuffers
extern PFNGLDELETESHADERPROC onix_ptr_glDeleteShader;
static inline void onix_call_glDeleteShader(GLuint shader) { onix_ptr_glDeleteShader(shader); }
#define glDeleteShader onix_call_glDeleteShader
extern PFNGLDELETESYNCPROC onix_ptr_glDeleteSync;
static inline void onix_call_glDeleteSync(GLsync sync) { onix_ptr_glDeleteSync(sync); }
#define glDeleteSync onix_call_glDeleteSync
extern PFNGLDELETETEXTURESPROC onix_ptr_glDeleteTextures;
static inline void onix_call_glDeleteTextures(GLsizei n, const GLuint *textures) {
    onix_ptr_glDeleteTextures(n, textures);
}
#define glDeleteTextures onix_call_glDeleteTextures
extern PFNGLDELETEVERTEXARRAYSPROC onix_ptr_glDeleteVertexArrays;
static inline void onix_call_glDeleteVertexArrays(GLsizei n, const GLuint *arrays) {
    onix_ptr_glDeleteVertexArrays(n, arrays);
}
#define glDeleteVertexArrays onix_call_glDeleteVertexArrays
extern PFNGLDEPTHFUNCPROC onix_ptr_glDepthFunc;
static inline void onix_call_glDepthFunc(GLenum func) { onix_ptr_glDepthFunc(func); }
#define glDepthFunc onix_call_glDepthFunc
extern PFNGLDEPTHMASKPROC onix_ptr_glDepthMask;
static inline void onix_call_glDepthMask(GLboolean flag) { onix_ptr_glDepthMask(flag); }
#define glDepthMask onix_call_glDepthMask
extern PFNGLDISABLEPROC onix_ptr_glDisable;
static inline void onix_call_glDisable(GLenum cap) { onix_ptr_glDisable(cap); }
#define glDisable onix_call_glDisable
extern PFNGLDRAWELEMENTSINSTANCEDPROC onix_ptr_glDrawElementsInstanced;
static inline void onix_call_glDrawElementsInstanced(GLenum mode, GLsizei count, GLenum type, const void *indices,
                                                     GLsizei instancecount) {
    onix_ptr_glDrawElementsInstanced(mode, count, type, indices, instancecount);
}
#define glDrawElementsInstanced onix_call_glDrawElementsInstanced
extern PFNGLENABLEPROC onix_ptr_glEnable;
static inline void onix_call_glEnable(GLenum cap) { onix_ptr_glEnable(cap); }
#define glEnable onix_call_glEnable
extern PFNGLENABLEVERTEXATTRIBARRAYPROC onix_ptr_glEnableVertexAttribArray;
static inline void onix_call_glEnableVertexAttribArray(GLuint index) { onix_ptr_glEnableVertexAttribArray(index); }
#define glEnableVertexAttribArray onix_call_glEnableVertexAttribArray
extern PFNGLFENCESYNCPROC onix_ptr_glFenceSync;
static inline GLsync onix_call_glFenceSync(GLenum condition, GLbitfield flags) {
    return onix_ptr_glFenceSync(condition, flags);
}
#define glFenceSync onix_call_glFenceSync
extern PFNGLFRAMEBUFFERRENDERBUFFERPROC onix_ptr_glFramebufferRenderbuffer;
static inline void onix_call_glFramebufferRenderbuffer(GLenum target, GLenum attachment, GLenum renderbuffertarget,
                                                       GLuint renderbuffer) {
    onix_ptr_glFramebufferRenderbuffer(target, attachment, renderbuffertarget, renderbuffer);
}
#define glFramebufferRenderbuffer onix_call_glFramebufferRenderbuffer
extern PFNGLFRAMEBUFFERTEXTURE2DPROC onix_ptr_glFramebufferTexture2D;
static inline void onix_call_glFramebufferTexture2D(GLenum target, GLenum attachment, GLenum textarget, GLuint texture,
                                                    GLint level) {
    onix_ptr_glFramebufferTexture2D(target, attachment, textarget, texture, level);
}
#define glFramebufferTexture2D onix_call_glFramebufferTexture2D
extern PFNGLGENBUFFERSPROC onix_ptr_glGenBuffers;
static inline void onix_call_glGenBuffers(GLsizei n, GLuint *buffers) { onix_ptr_glGenBuffers(n, buffers); }
#define glGenBuffers onix_call_glGenBuffers
extern PFNGLGENFRAMEBUFFERSPROC onix_ptr_glGenFramebuffers;
static inline void onix_call_glGenFramebuffers(GLsizei n, GLuint *framebuffers) {
    onix_ptr_glGenFramebuffers(n, framebuffers);
}
#define glGenFramebuffers onix_call_glGenFramebuffers
extern PFNGLGENRENDERBUFFERSPROC onix_ptr_glGenRenderbuffers;
static inline void onix_call_glGenRenderbuffers(GLsizei n, GLuint *renderbuffers) {
    onix_ptr_glGenRenderbuffers(n, renderbuffers);
}
#define glGenRenderbuffers onix_call_glGenRenderbuffers
extern PFNGLGENTEXTURESPROC onix_ptr_glGenTextures;
static inline void onix_call_glGenTextures(GLsizei n, GLuint *textures) { onix_ptr_glGenTextures(n, textures); }
#define glGenTextures onix_call_glGenTextures
extern PFNGLGENVERTEXARRAYSPROC onix_ptr_glGenVertexArrays;
static inline void onix_call_glGenVertexArrays(GLsizei n, GLuint *arrays) { onix_ptr_glGenVertexArrays(n, arrays); }
#define glGenVertexArrays onix_call_glGenVertexArrays
extern PFNGLGENERATEMIPMAPPROC onix_ptr_glGenerateMipmap;
static inline void onix_call_glGenerateMipmap(GLenum target) { onix_ptr_glGenerateMipmap(target); }
#define glGenerateMipmap onix_call_glGenerateMipmap
extern PFNGLGETERRORPROC onix_ptr_glGetError;
static inline GLenum onix_call_glGetError(void) { return onix_ptr_glGetError(); }
#define glGetError onix_call_glGetError
extern PFNGLGETINTEGERVPROC onix_ptr_glGetIntegerv;
static inline void onix_call_glGetIntegerv(GLenum pname, GLint *data) { onix_ptr_glGetIntegerv(pname, data); }
#define glGetIntegerv onix_call_glGetIntegerv
extern PFNGLGETPROGRAMINFOLOGPROC onix_ptr_glGetProgramInfoLog;
static inline void onix_call_glGetProgramInfoLog(GLuint program, GLsizei bufSize, GLsizei *length, GLchar *infoLog) {
    onix_ptr_glGetProgramInfoLog(program, bufSize, length, infoLog);
}
#define glGetProgramInfoLog onix_call_glGetProgramInfoLog
extern PFNGLGETPROGRAMIVPROC onix_ptr_glGetProgramiv;
static inline void onix_call_glGetProgramiv(GLuint program, GLenum pname, GLint *params) {
    onix_ptr_glGetProgramiv(program, pname, params);
}
#define glGetProgramiv onix_call_glGetProgramiv
extern PFNGLGETSHADERINFOLOGPROC onix_ptr_glGetShaderInfoLog;
static inline void onix_call_glGetShaderInfoLog(GLuint shader, GLsizei bufSize, GLsizei *length, GLchar *infoLog) {
    onix_ptr_glGetShaderInfoLog(shader, bufSize, length, infoLog);
}
#define glGetShaderInfoLog onix_call_glGetShaderInfoLog
extern PFNGLGETSHADERIVPROC onix_ptr_glGetShaderiv;
static inline void onix_call_glGetShaderiv(GLuint shader, GLenum pname, GLint *params) {
    onix_ptr_glGetShaderiv(shader, pname, params);
}
#define glGetShaderiv onix_call_glGetShaderiv
extern PFNGLGETSTRINGPROC onix_ptr_glGetString;
static inline const GLubyte *onix_call_glGetString(GLenum name) { return onix_ptr_glGetString(name); }
#define glGetString onix_call_glGetString
extern PFNGLGETSTRINGIPROC onix_ptr_glGetStringi;
static inline const GLubyte *onix_call_glGetStringi(GLenum name, GLuint index) {
    return onix_ptr_glGetStringi(name, index);
}
#define glGetStringi onix_call_glGetStringi
extern PFNGLGETUNIFORMLOCATIONPROC onix_ptr_glGetUniformLocation;
static inline GLint onix_call_glGetUniformLocation(GLuint program, const GLchar *name) {
    return onix_ptr_glGetUniformLocation(program, name);
}
#define glGetUniformLocation onix_call_glGetUniformLocation
extern PFNGLLINKPROGRAMPROC onix_ptr_glLinkProgram;
static inline void onix_call_glLinkProgram(GLuint program) { onix_ptr_glLinkProgram(program); }
#define glLinkProgram onix_call_glLinkProgram
extern PFNGLMAPBUFFERRANGEPROC onix_ptr_glMapBufferRange;
static inline void *onix_call_glMapBufferRange(GLenum target, GLintptr offset, GLsizeiptr length, GLbitfield access) {
    return onix_ptr_glMapBufferRange(target, offset, length, access);
}
#define glMapBufferRange onix_call_glMapBufferRange
extern PFNGLUNMAPBUFFERPROC onix_ptr_glUnmapBuffer;
static inline GLboolean onix_call_glUnmapBuffer(GLenum target) { return onix_ptr_glUnmapBuffer(target); }
#define glUnmapBuffer onix_call_glUnmapBuffer
extern PFNGLPIXELSTOREIPROC onix_ptr_glPixelStorei;
static inline void onix_call_glPixelStorei(GLenum pname, GLint param) { onix_ptr_glPixelStorei(pname, param); }
#define glPixelStorei onix_call_glPixelStorei
extern PFNGLREADPIXELSPROC onix_ptr_glReadPixels;
static inline void onix_call_glReadPixels(GLint x, GLint y, GLsizei width, GLsizei height, GLenum format, GLenum type,
                                          void *pixels) {
    onix_ptr_glReadPixels(x, y, width, height, format, type, pixels);
}
#define glReadPixels onix_call_glReadPixels
extern PFNGLRENDERBUFFERSTORAGEPROC onix_ptr_glRenderbufferStorage;
static inline void onix_call_glRenderbufferStorage(GLenum target, GLenum internalformat, GLsizei width,
                                                   GLsizei height) {
    onix_ptr_glRenderbufferStorage(target, internalformat, width, height);
}
#define glRenderbufferStorage onix_call_glRenderbufferStorage
extern PFNGLSCISSORPROC onix_ptr_glScissor;
static inline void onix_call_glScissor(GLint x, GLint y, GLsizei width, GLsizei height) {
    onix_ptr_glScissor(x, y, width, height);
}
#define glScissor onix_call_glScissor
extern PFNGLSHADERSOURCEPROC onix_ptr_glShaderSource;
static inline void onix_call_glShaderSource(GLuint shader, GLsizei count, const GLchar *const *string,
                                            const GLint *length) {
    onix_ptr_glShaderSource(shader, count, string, length);
}
#define glShaderSource onix_call_glShaderSource
extern PFNGLTEXIMAGE2DPROC onix_ptr_glTexImage2D;
static inline void onix_call_glTexImage2D(GLenum target, GLint level, GLint internalformat, GLsizei width,
                                          GLsizei height, GLint border, GLenum format, GLenum type,
                                          const void *pixels) {
    onix_ptr_glTexImage2D(target, level, internalformat, width, height, border, format, type, pixels);
}
#define glTexImage2D onix_call_glTexImage2D
extern PFNGLTEXPARAMETERIPROC onix_ptr_glTexParameteri;
static inline void onix_call_glTexParameteri(GLenum target, GLenum pname, GLint param) {
    onix_ptr_glTexParameteri(target, pname, param);
}
#define glTexParameteri onix_call_glTexParameteri
extern PFNGLTEXSUBIMAGE2DPROC onix_ptr_glTexSubImage2D;
static inline void onix_call_glTexSubImage2D(GLenum target, GLint level, GLint xoffset, GLint yoffset, GLsizei width,
                                             GLsizei height, GLenum format, GLenum type, const void *pixels) {
    onix_ptr_glTexSubImage2D(target, level, xoffset, yoffset, width, height, format, type, pixels);
}
#define glTexSubImage2D onix_call_glTexSubImage2D
extern PFNGLUNIFORM1IPROC onix_ptr_glUniform1i;
static inline void onix_call_glUniform1i(GLint location, GLint v0) { onix_ptr_glUniform1i(location, v0); }
#define glUniform1i onix_call_glUniform1i
extern PFNGLUNIFORM4FVPROC onix_ptr_glUniform4fv;
static inline void onix_call_glUniform4fv(GLint location, GLsizei count, const GLfloat *value) {
    onix_ptr_glUniform4fv(location, count, value);
}
#define glUniform4fv onix_call_glUniform4fv
extern PFNGLUNIFORMMATRIX4FVPROC onix_ptr_glUniformMatrix4fv;
static inline void onix_call_glUniformMatrix4fv(GLint location, GLsizei count, GLboolean transpose,
                                                const GLfloat *value) {
    onix_ptr_glUniformMatrix4fv(location, count, transpose, value);
}
#define glUniformMatrix4fv onix_call_glUniformMatrix4fv
extern PFNGLUSEPROGRAMPROC onix_ptr_glUseProgram;
static inline void onix_call_glUseProgram(GLuint program) { onix_ptr_glUseProgram(program); }
#define glUseProgram onix_call_glUseProgram
extern PFNGLVERTEXATTRIBDIVISORPROC onix_ptr_glVertexAttribDivisor;
static inline void onix_call_glVertexAttribDivisor(GLuint index, GLuint divisor) {
    onix_ptr_glVertexAttribDivisor(index, divisor);
}
#define glVertexAttribDivisor onix_call_glVertexAttribDivisor
extern PFNGLVERTEXATTRIBIPOINTERPROC onix_ptr_glVertexAttribIPointer;
static inline void onix_call_glVertexAttribIPointer(GLuint index, GLint size, GLenum type, GLsizei stride,
                                                    const void *pointer) {
    onix_ptr_glVertexAttribIPointer(index, size, type, stride, pointer);
}
#define glVertexAttribIPointer onix_call_glVertexAttribIPointer
extern PFNGLVERTEXATTRIBPOINTERPROC onix_ptr_glVertexAttribPointer;
static inline void onix_call_glVertexAttribPointer(GLuint index, GLint size, GLenum type, GLboolean normalized,
                                                   GLsizei stride, const void *pointer) {
    onix_ptr_glVertexAttribPointer(index, size, type, normalized, stride, pointer);
}
#define glVertexAttribPointer onix_call_glVertexAttribPointer
extern PFNGLVIEWPORTPROC onix_ptr_glViewport;
static inline void onix_call_glViewport(GLint x, GLint y, GLsizei width, GLsizei height) {
    onix_ptr_glViewport(x, y, width, height);
}
#define glViewport onix_call_glViewport
#endif
#endif
