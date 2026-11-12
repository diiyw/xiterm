//go:build windows && cgo && onix_opengl

#include "gl_api.h"
PFNGLACTIVETEXTUREPROC onix_ptr_glActiveTexture;
PFNGLATTACHSHADERPROC onix_ptr_glAttachShader;
PFNGLBINDBUFFERPROC onix_ptr_glBindBuffer;
PFNGLBINDFRAMEBUFFERPROC onix_ptr_glBindFramebuffer;
PFNGLBINDRENDERBUFFERPROC onix_ptr_glBindRenderbuffer;
PFNGLBINDTEXTUREPROC onix_ptr_glBindTexture;
PFNGLBINDVERTEXARRAYPROC onix_ptr_glBindVertexArray;
PFNGLBLENDCOLORPROC onix_ptr_glBlendColor;
PFNGLBLENDFUNCSEPARATEPROC onix_ptr_glBlendFuncSeparate;
PFNGLBLITFRAMEBUFFERPROC onix_ptr_glBlitFramebuffer;
PFNGLBUFFERDATAPROC onix_ptr_glBufferData;
PFNGLBUFFERSUBDATAPROC onix_ptr_glBufferSubData;
PFNGLCHECKFRAMEBUFFERSTATUSPROC onix_ptr_glCheckFramebufferStatus;
PFNGLCLEARPROC onix_ptr_glClear;
PFNGLCLEARBUFFERUIVPROC onix_ptr_glClearBufferuiv;
PFNGLCLEARCOLORPROC onix_ptr_glClearColor;
PFNGLCLEARDEPTHPROC onix_ptr_glClearDepthf;
PFNGLCLIENTWAITSYNCPROC onix_ptr_glClientWaitSync;
PFNGLCOMPILESHADERPROC onix_ptr_glCompileShader;
PFNGLCOMPRESSEDTEXIMAGE2DPROC onix_ptr_glCompressedTexImage2D;
PFNGLCREATEPROGRAMPROC onix_ptr_glCreateProgram;
PFNGLCREATESHADERPROC onix_ptr_glCreateShader;
PFNGLDELETEBUFFERSPROC onix_ptr_glDeleteBuffers;
PFNGLDELETEFRAMEBUFFERSPROC onix_ptr_glDeleteFramebuffers;
PFNGLDELETEPROGRAMPROC onix_ptr_glDeleteProgram;
PFNGLDELETERENDERBUFFERSPROC onix_ptr_glDeleteRenderbuffers;
PFNGLDELETESHADERPROC onix_ptr_glDeleteShader;
PFNGLDELETESYNCPROC onix_ptr_glDeleteSync;
PFNGLDELETETEXTURESPROC onix_ptr_glDeleteTextures;
PFNGLDELETEVERTEXARRAYSPROC onix_ptr_glDeleteVertexArrays;
PFNGLDEPTHFUNCPROC onix_ptr_glDepthFunc;
PFNGLDEPTHMASKPROC onix_ptr_glDepthMask;
PFNGLDISABLEPROC onix_ptr_glDisable;
PFNGLDRAWELEMENTSINSTANCEDPROC onix_ptr_glDrawElementsInstanced;
PFNGLENABLEPROC onix_ptr_glEnable;
PFNGLENABLEVERTEXATTRIBARRAYPROC onix_ptr_glEnableVertexAttribArray;
PFNGLFENCESYNCPROC onix_ptr_glFenceSync;
PFNGLFRAMEBUFFERRENDERBUFFERPROC onix_ptr_glFramebufferRenderbuffer;
PFNGLFRAMEBUFFERTEXTURE2DPROC onix_ptr_glFramebufferTexture2D;
PFNGLGENBUFFERSPROC onix_ptr_glGenBuffers;
PFNGLGENFRAMEBUFFERSPROC onix_ptr_glGenFramebuffers;
PFNGLGENRENDERBUFFERSPROC onix_ptr_glGenRenderbuffers;
PFNGLGENTEXTURESPROC onix_ptr_glGenTextures;
PFNGLGENVERTEXARRAYSPROC onix_ptr_glGenVertexArrays;
PFNGLGENERATEMIPMAPPROC onix_ptr_glGenerateMipmap;
PFNGLGETERRORPROC onix_ptr_glGetError;
PFNGLGETINTEGERVPROC onix_ptr_glGetIntegerv;
PFNGLGETPROGRAMINFOLOGPROC onix_ptr_glGetProgramInfoLog;
PFNGLGETPROGRAMIVPROC onix_ptr_glGetProgramiv;
PFNGLGETSHADERINFOLOGPROC onix_ptr_glGetShaderInfoLog;
PFNGLGETSHADERIVPROC onix_ptr_glGetShaderiv;
PFNGLGETSTRINGPROC onix_ptr_glGetString;
PFNGLGETSTRINGIPROC onix_ptr_glGetStringi;
PFNGLGETUNIFORMLOCATIONPROC onix_ptr_glGetUniformLocation;
PFNGLLINKPROGRAMPROC onix_ptr_glLinkProgram;
PFNGLMAPBUFFERRANGEPROC onix_ptr_glMapBufferRange;
PFNGLUNMAPBUFFERPROC onix_ptr_glUnmapBuffer;
PFNGLPIXELSTOREIPROC onix_ptr_glPixelStorei;
PFNGLREADPIXELSPROC onix_ptr_glReadPixels;
PFNGLRENDERBUFFERSTORAGEPROC onix_ptr_glRenderbufferStorage;
PFNGLSCISSORPROC onix_ptr_glScissor;
PFNGLSHADERSOURCEPROC onix_ptr_glShaderSource;
PFNGLTEXIMAGE2DPROC onix_ptr_glTexImage2D;
PFNGLTEXPARAMETERIPROC onix_ptr_glTexParameteri;
PFNGLTEXSUBIMAGE2DPROC onix_ptr_glTexSubImage2D;
PFNGLUNIFORM1IPROC onix_ptr_glUniform1i;
PFNGLUNIFORM4FVPROC onix_ptr_glUniform4fv;
PFNGLUNIFORMMATRIX4FVPROC onix_ptr_glUniformMatrix4fv;
PFNGLUSEPROGRAMPROC onix_ptr_glUseProgram;
PFNGLVERTEXATTRIBDIVISORPROC onix_ptr_glVertexAttribDivisor;
PFNGLVERTEXATTRIBIPOINTERPROC onix_ptr_glVertexAttribIPointer;
PFNGLVERTEXATTRIBPOINTERPROC onix_ptr_glVertexAttribPointer;
PFNGLVIEWPORTPROC onix_ptr_glViewport;

int onix_gl_load(void) {
    HMODULE library = GetModuleHandleA("opengl32.dll");
    onix_ptr_glActiveTexture = (PFNGLACTIVETEXTUREPROC)wglGetProcAddress("glActiveTexture");
    if ((uintptr_t)onix_ptr_glActiveTexture <= 3 || (intptr_t)onix_ptr_glActiveTexture == -1)
        onix_ptr_glActiveTexture = (PFNGLACTIVETEXTUREPROC)GetProcAddress(library, "glActiveTexture");
    if (!onix_ptr_glActiveTexture)
        return 0;
    onix_ptr_glAttachShader = (PFNGLATTACHSHADERPROC)wglGetProcAddress("glAttachShader");
    if ((uintptr_t)onix_ptr_glAttachShader <= 3 || (intptr_t)onix_ptr_glAttachShader == -1)
        onix_ptr_glAttachShader = (PFNGLATTACHSHADERPROC)GetProcAddress(library, "glAttachShader");
    if (!onix_ptr_glAttachShader)
        return 0;
    onix_ptr_glBindBuffer = (PFNGLBINDBUFFERPROC)wglGetProcAddress("glBindBuffer");
    if ((uintptr_t)onix_ptr_glBindBuffer <= 3 || (intptr_t)onix_ptr_glBindBuffer == -1)
        onix_ptr_glBindBuffer = (PFNGLBINDBUFFERPROC)GetProcAddress(library, "glBindBuffer");
    if (!onix_ptr_glBindBuffer)
        return 0;
    onix_ptr_glBindFramebuffer = (PFNGLBINDFRAMEBUFFERPROC)wglGetProcAddress("glBindFramebuffer");
    if ((uintptr_t)onix_ptr_glBindFramebuffer <= 3 || (intptr_t)onix_ptr_glBindFramebuffer == -1)
        onix_ptr_glBindFramebuffer = (PFNGLBINDFRAMEBUFFERPROC)GetProcAddress(library, "glBindFramebuffer");
    if (!onix_ptr_glBindFramebuffer)
        return 0;
    onix_ptr_glBindRenderbuffer = (PFNGLBINDRENDERBUFFERPROC)wglGetProcAddress("glBindRenderbuffer");
    if ((uintptr_t)onix_ptr_glBindRenderbuffer <= 3 || (intptr_t)onix_ptr_glBindRenderbuffer == -1)
        onix_ptr_glBindRenderbuffer = (PFNGLBINDRENDERBUFFERPROC)GetProcAddress(library, "glBindRenderbuffer");
    if (!onix_ptr_glBindRenderbuffer)
        return 0;
    onix_ptr_glBindTexture = (PFNGLBINDTEXTUREPROC)wglGetProcAddress("glBindTexture");
    if ((uintptr_t)onix_ptr_glBindTexture <= 3 || (intptr_t)onix_ptr_glBindTexture == -1)
        onix_ptr_glBindTexture = (PFNGLBINDTEXTUREPROC)GetProcAddress(library, "glBindTexture");
    if (!onix_ptr_glBindTexture)
        return 0;
    onix_ptr_glBindVertexArray = (PFNGLBINDVERTEXARRAYPROC)wglGetProcAddress("glBindVertexArray");
    if ((uintptr_t)onix_ptr_glBindVertexArray <= 3 || (intptr_t)onix_ptr_glBindVertexArray == -1)
        onix_ptr_glBindVertexArray = (PFNGLBINDVERTEXARRAYPROC)GetProcAddress(library, "glBindVertexArray");
    if (!onix_ptr_glBindVertexArray)
        return 0;
    onix_ptr_glBlendColor = (PFNGLBLENDCOLORPROC)wglGetProcAddress("glBlendColor");
    if ((uintptr_t)onix_ptr_glBlendColor <= 3 || (intptr_t)onix_ptr_glBlendColor == -1)
        onix_ptr_glBlendColor = (PFNGLBLENDCOLORPROC)GetProcAddress(library, "glBlendColor");
    if (!onix_ptr_glBlendColor)
        return 0;
    onix_ptr_glBlendFuncSeparate = (PFNGLBLENDFUNCSEPARATEPROC)wglGetProcAddress("glBlendFuncSeparate");
    if ((uintptr_t)onix_ptr_glBlendFuncSeparate <= 3 || (intptr_t)onix_ptr_glBlendFuncSeparate == -1)
        onix_ptr_glBlendFuncSeparate = (PFNGLBLENDFUNCSEPARATEPROC)GetProcAddress(library, "glBlendFuncSeparate");
    if (!onix_ptr_glBlendFuncSeparate)
        return 0;
    onix_ptr_glBlitFramebuffer = (PFNGLBLITFRAMEBUFFERPROC)wglGetProcAddress("glBlitFramebuffer");
    if ((uintptr_t)onix_ptr_glBlitFramebuffer <= 3 || (intptr_t)onix_ptr_glBlitFramebuffer == -1)
        onix_ptr_glBlitFramebuffer = (PFNGLBLITFRAMEBUFFERPROC)GetProcAddress(library, "glBlitFramebuffer");
    if (!onix_ptr_glBlitFramebuffer)
        return 0;
    onix_ptr_glBufferData = (PFNGLBUFFERDATAPROC)wglGetProcAddress("glBufferData");
    if ((uintptr_t)onix_ptr_glBufferData <= 3 || (intptr_t)onix_ptr_glBufferData == -1)
        onix_ptr_glBufferData = (PFNGLBUFFERDATAPROC)GetProcAddress(library, "glBufferData");
    if (!onix_ptr_glBufferData)
        return 0;
    onix_ptr_glBufferSubData = (PFNGLBUFFERSUBDATAPROC)wglGetProcAddress("glBufferSubData");
    if ((uintptr_t)onix_ptr_glBufferSubData <= 3 || (intptr_t)onix_ptr_glBufferSubData == -1)
        onix_ptr_glBufferSubData = (PFNGLBUFFERSUBDATAPROC)GetProcAddress(library, "glBufferSubData");
    if (!onix_ptr_glBufferSubData)
        return 0;
    onix_ptr_glCheckFramebufferStatus = (PFNGLCHECKFRAMEBUFFERSTATUSPROC)wglGetProcAddress("glCheckFramebufferStatus");
    if ((uintptr_t)onix_ptr_glCheckFramebufferStatus <= 3 || (intptr_t)onix_ptr_glCheckFramebufferStatus == -1)
        onix_ptr_glCheckFramebufferStatus =
            (PFNGLCHECKFRAMEBUFFERSTATUSPROC)GetProcAddress(library, "glCheckFramebufferStatus");
    if (!onix_ptr_glCheckFramebufferStatus)
        return 0;
    onix_ptr_glClear = (PFNGLCLEARPROC)wglGetProcAddress("glClear");
    if ((uintptr_t)onix_ptr_glClear <= 3 || (intptr_t)onix_ptr_glClear == -1)
        onix_ptr_glClear = (PFNGLCLEARPROC)GetProcAddress(library, "glClear");
    if (!onix_ptr_glClear)
        return 0;
    onix_ptr_glClearBufferuiv = (PFNGLCLEARBUFFERUIVPROC)wglGetProcAddress("glClearBufferuiv");
    if ((uintptr_t)onix_ptr_glClearBufferuiv <= 3 || (intptr_t)onix_ptr_glClearBufferuiv == -1)
        onix_ptr_glClearBufferuiv = (PFNGLCLEARBUFFERUIVPROC)GetProcAddress(library, "glClearBufferuiv");
    if (!onix_ptr_glClearBufferuiv)
        return 0;
    onix_ptr_glClearColor = (PFNGLCLEARCOLORPROC)wglGetProcAddress("glClearColor");
    if ((uintptr_t)onix_ptr_glClearColor <= 3 || (intptr_t)onix_ptr_glClearColor == -1)
        onix_ptr_glClearColor = (PFNGLCLEARCOLORPROC)GetProcAddress(library, "glClearColor");
    if (!onix_ptr_glClearColor)
        return 0;
    onix_ptr_glClearDepthf = (PFNGLCLEARDEPTHPROC)wglGetProcAddress("glClearDepth");
    if ((uintptr_t)onix_ptr_glClearDepthf <= 3 || (intptr_t)onix_ptr_glClearDepthf == -1)
        onix_ptr_glClearDepthf = (PFNGLCLEARDEPTHPROC)GetProcAddress(library, "glClearDepth");
    if (!onix_ptr_glClearDepthf)
        return 0;
    onix_ptr_glClientWaitSync = (PFNGLCLIENTWAITSYNCPROC)wglGetProcAddress("glClientWaitSync");
    if ((uintptr_t)onix_ptr_glClientWaitSync <= 3 || (intptr_t)onix_ptr_glClientWaitSync == -1)
        onix_ptr_glClientWaitSync = (PFNGLCLIENTWAITSYNCPROC)GetProcAddress(library, "glClientWaitSync");
    if (!onix_ptr_glClientWaitSync)
        return 0;
    onix_ptr_glCompileShader = (PFNGLCOMPILESHADERPROC)wglGetProcAddress("glCompileShader");
    if ((uintptr_t)onix_ptr_glCompileShader <= 3 || (intptr_t)onix_ptr_glCompileShader == -1)
        onix_ptr_glCompileShader = (PFNGLCOMPILESHADERPROC)GetProcAddress(library, "glCompileShader");
    if (!onix_ptr_glCompileShader)
        return 0;
    onix_ptr_glCompressedTexImage2D = (PFNGLCOMPRESSEDTEXIMAGE2DPROC)wglGetProcAddress("glCompressedTexImage2D");
    if ((uintptr_t)onix_ptr_glCompressedTexImage2D <= 3 || (intptr_t)onix_ptr_glCompressedTexImage2D == -1)
        onix_ptr_glCompressedTexImage2D =
            (PFNGLCOMPRESSEDTEXIMAGE2DPROC)GetProcAddress(library, "glCompressedTexImage2D");
    if (!onix_ptr_glCompressedTexImage2D)
        return 0;
    onix_ptr_glCreateProgram = (PFNGLCREATEPROGRAMPROC)wglGetProcAddress("glCreateProgram");
    if ((uintptr_t)onix_ptr_glCreateProgram <= 3 || (intptr_t)onix_ptr_glCreateProgram == -1)
        onix_ptr_glCreateProgram = (PFNGLCREATEPROGRAMPROC)GetProcAddress(library, "glCreateProgram");
    if (!onix_ptr_glCreateProgram)
        return 0;
    onix_ptr_glCreateShader = (PFNGLCREATESHADERPROC)wglGetProcAddress("glCreateShader");
    if ((uintptr_t)onix_ptr_glCreateShader <= 3 || (intptr_t)onix_ptr_glCreateShader == -1)
        onix_ptr_glCreateShader = (PFNGLCREATESHADERPROC)GetProcAddress(library, "glCreateShader");
    if (!onix_ptr_glCreateShader)
        return 0;
    onix_ptr_glDeleteBuffers = (PFNGLDELETEBUFFERSPROC)wglGetProcAddress("glDeleteBuffers");
    if ((uintptr_t)onix_ptr_glDeleteBuffers <= 3 || (intptr_t)onix_ptr_glDeleteBuffers == -1)
        onix_ptr_glDeleteBuffers = (PFNGLDELETEBUFFERSPROC)GetProcAddress(library, "glDeleteBuffers");
    if (!onix_ptr_glDeleteBuffers)
        return 0;
    onix_ptr_glDeleteFramebuffers = (PFNGLDELETEFRAMEBUFFERSPROC)wglGetProcAddress("glDeleteFramebuffers");
    if ((uintptr_t)onix_ptr_glDeleteFramebuffers <= 3 || (intptr_t)onix_ptr_glDeleteFramebuffers == -1)
        onix_ptr_glDeleteFramebuffers = (PFNGLDELETEFRAMEBUFFERSPROC)GetProcAddress(library, "glDeleteFramebuffers");
    if (!onix_ptr_glDeleteFramebuffers)
        return 0;
    onix_ptr_glDeleteProgram = (PFNGLDELETEPROGRAMPROC)wglGetProcAddress("glDeleteProgram");
    if ((uintptr_t)onix_ptr_glDeleteProgram <= 3 || (intptr_t)onix_ptr_glDeleteProgram == -1)
        onix_ptr_glDeleteProgram = (PFNGLDELETEPROGRAMPROC)GetProcAddress(library, "glDeleteProgram");
    if (!onix_ptr_glDeleteProgram)
        return 0;
    onix_ptr_glDeleteRenderbuffers = (PFNGLDELETERENDERBUFFERSPROC)wglGetProcAddress("glDeleteRenderbuffers");
    if ((uintptr_t)onix_ptr_glDeleteRenderbuffers <= 3 || (intptr_t)onix_ptr_glDeleteRenderbuffers == -1)
        onix_ptr_glDeleteRenderbuffers = (PFNGLDELETERENDERBUFFERSPROC)GetProcAddress(library, "glDeleteRenderbuffers");
    if (!onix_ptr_glDeleteRenderbuffers)
        return 0;
    onix_ptr_glDeleteShader = (PFNGLDELETESHADERPROC)wglGetProcAddress("glDeleteShader");
    if ((uintptr_t)onix_ptr_glDeleteShader <= 3 || (intptr_t)onix_ptr_glDeleteShader == -1)
        onix_ptr_glDeleteShader = (PFNGLDELETESHADERPROC)GetProcAddress(library, "glDeleteShader");
    if (!onix_ptr_glDeleteShader)
        return 0;
    onix_ptr_glDeleteSync = (PFNGLDELETESYNCPROC)wglGetProcAddress("glDeleteSync");
    if ((uintptr_t)onix_ptr_glDeleteSync <= 3 || (intptr_t)onix_ptr_glDeleteSync == -1)
        onix_ptr_glDeleteSync = (PFNGLDELETESYNCPROC)GetProcAddress(library, "glDeleteSync");
    if (!onix_ptr_glDeleteSync)
        return 0;
    onix_ptr_glDeleteTextures = (PFNGLDELETETEXTURESPROC)wglGetProcAddress("glDeleteTextures");
    if ((uintptr_t)onix_ptr_glDeleteTextures <= 3 || (intptr_t)onix_ptr_glDeleteTextures == -1)
        onix_ptr_glDeleteTextures = (PFNGLDELETETEXTURESPROC)GetProcAddress(library, "glDeleteTextures");
    if (!onix_ptr_glDeleteTextures)
        return 0;
    onix_ptr_glDeleteVertexArrays = (PFNGLDELETEVERTEXARRAYSPROC)wglGetProcAddress("glDeleteVertexArrays");
    if ((uintptr_t)onix_ptr_glDeleteVertexArrays <= 3 || (intptr_t)onix_ptr_glDeleteVertexArrays == -1)
        onix_ptr_glDeleteVertexArrays = (PFNGLDELETEVERTEXARRAYSPROC)GetProcAddress(library, "glDeleteVertexArrays");
    if (!onix_ptr_glDeleteVertexArrays)
        return 0;
    onix_ptr_glDepthFunc = (PFNGLDEPTHFUNCPROC)wglGetProcAddress("glDepthFunc");
    if ((uintptr_t)onix_ptr_glDepthFunc <= 3 || (intptr_t)onix_ptr_glDepthFunc == -1)
        onix_ptr_glDepthFunc = (PFNGLDEPTHFUNCPROC)GetProcAddress(library, "glDepthFunc");
    if (!onix_ptr_glDepthFunc)
        return 0;
    onix_ptr_glDepthMask = (PFNGLDEPTHMASKPROC)wglGetProcAddress("glDepthMask");
    if ((uintptr_t)onix_ptr_glDepthMask <= 3 || (intptr_t)onix_ptr_glDepthMask == -1)
        onix_ptr_glDepthMask = (PFNGLDEPTHMASKPROC)GetProcAddress(library, "glDepthMask");
    if (!onix_ptr_glDepthMask)
        return 0;
    onix_ptr_glDisable = (PFNGLDISABLEPROC)wglGetProcAddress("glDisable");
    if ((uintptr_t)onix_ptr_glDisable <= 3 || (intptr_t)onix_ptr_glDisable == -1)
        onix_ptr_glDisable = (PFNGLDISABLEPROC)GetProcAddress(library, "glDisable");
    if (!onix_ptr_glDisable)
        return 0;
    onix_ptr_glDrawElementsInstanced = (PFNGLDRAWELEMENTSINSTANCEDPROC)wglGetProcAddress("glDrawElementsInstanced");
    if ((uintptr_t)onix_ptr_glDrawElementsInstanced <= 3 || (intptr_t)onix_ptr_glDrawElementsInstanced == -1)
        onix_ptr_glDrawElementsInstanced =
            (PFNGLDRAWELEMENTSINSTANCEDPROC)GetProcAddress(library, "glDrawElementsInstanced");
    if (!onix_ptr_glDrawElementsInstanced)
        return 0;
    onix_ptr_glEnable = (PFNGLENABLEPROC)wglGetProcAddress("glEnable");
    if ((uintptr_t)onix_ptr_glEnable <= 3 || (intptr_t)onix_ptr_glEnable == -1)
        onix_ptr_glEnable = (PFNGLENABLEPROC)GetProcAddress(library, "glEnable");
    if (!onix_ptr_glEnable)
        return 0;
    onix_ptr_glEnableVertexAttribArray =
        (PFNGLENABLEVERTEXATTRIBARRAYPROC)wglGetProcAddress("glEnableVertexAttribArray");
    if ((uintptr_t)onix_ptr_glEnableVertexAttribArray <= 3 || (intptr_t)onix_ptr_glEnableVertexAttribArray == -1)
        onix_ptr_glEnableVertexAttribArray =
            (PFNGLENABLEVERTEXATTRIBARRAYPROC)GetProcAddress(library, "glEnableVertexAttribArray");
    if (!onix_ptr_glEnableVertexAttribArray)
        return 0;
    onix_ptr_glFenceSync = (PFNGLFENCESYNCPROC)wglGetProcAddress("glFenceSync");
    if ((uintptr_t)onix_ptr_glFenceSync <= 3 || (intptr_t)onix_ptr_glFenceSync == -1)
        onix_ptr_glFenceSync = (PFNGLFENCESYNCPROC)GetProcAddress(library, "glFenceSync");
    if (!onix_ptr_glFenceSync)
        return 0;
    onix_ptr_glFramebufferRenderbuffer =
        (PFNGLFRAMEBUFFERRENDERBUFFERPROC)wglGetProcAddress("glFramebufferRenderbuffer");
    if ((uintptr_t)onix_ptr_glFramebufferRenderbuffer <= 3 || (intptr_t)onix_ptr_glFramebufferRenderbuffer == -1)
        onix_ptr_glFramebufferRenderbuffer =
            (PFNGLFRAMEBUFFERRENDERBUFFERPROC)GetProcAddress(library, "glFramebufferRenderbuffer");
    if (!onix_ptr_glFramebufferRenderbuffer)
        return 0;
    onix_ptr_glFramebufferTexture2D = (PFNGLFRAMEBUFFERTEXTURE2DPROC)wglGetProcAddress("glFramebufferTexture2D");
    if ((uintptr_t)onix_ptr_glFramebufferTexture2D <= 3 || (intptr_t)onix_ptr_glFramebufferTexture2D == -1)
        onix_ptr_glFramebufferTexture2D =
            (PFNGLFRAMEBUFFERTEXTURE2DPROC)GetProcAddress(library, "glFramebufferTexture2D");
    if (!onix_ptr_glFramebufferTexture2D)
        return 0;
    onix_ptr_glGenBuffers = (PFNGLGENBUFFERSPROC)wglGetProcAddress("glGenBuffers");
    if ((uintptr_t)onix_ptr_glGenBuffers <= 3 || (intptr_t)onix_ptr_glGenBuffers == -1)
        onix_ptr_glGenBuffers = (PFNGLGENBUFFERSPROC)GetProcAddress(library, "glGenBuffers");
    if (!onix_ptr_glGenBuffers)
        return 0;
    onix_ptr_glGenFramebuffers = (PFNGLGENFRAMEBUFFERSPROC)wglGetProcAddress("glGenFramebuffers");
    if ((uintptr_t)onix_ptr_glGenFramebuffers <= 3 || (intptr_t)onix_ptr_glGenFramebuffers == -1)
        onix_ptr_glGenFramebuffers = (PFNGLGENFRAMEBUFFERSPROC)GetProcAddress(library, "glGenFramebuffers");
    if (!onix_ptr_glGenFramebuffers)
        return 0;
    onix_ptr_glGenRenderbuffers = (PFNGLGENRENDERBUFFERSPROC)wglGetProcAddress("glGenRenderbuffers");
    if ((uintptr_t)onix_ptr_glGenRenderbuffers <= 3 || (intptr_t)onix_ptr_glGenRenderbuffers == -1)
        onix_ptr_glGenRenderbuffers = (PFNGLGENRENDERBUFFERSPROC)GetProcAddress(library, "glGenRenderbuffers");
    if (!onix_ptr_glGenRenderbuffers)
        return 0;
    onix_ptr_glGenTextures = (PFNGLGENTEXTURESPROC)wglGetProcAddress("glGenTextures");
    if ((uintptr_t)onix_ptr_glGenTextures <= 3 || (intptr_t)onix_ptr_glGenTextures == -1)
        onix_ptr_glGenTextures = (PFNGLGENTEXTURESPROC)GetProcAddress(library, "glGenTextures");
    if (!onix_ptr_glGenTextures)
        return 0;
    onix_ptr_glGenVertexArrays = (PFNGLGENVERTEXARRAYSPROC)wglGetProcAddress("glGenVertexArrays");
    if ((uintptr_t)onix_ptr_glGenVertexArrays <= 3 || (intptr_t)onix_ptr_glGenVertexArrays == -1)
        onix_ptr_glGenVertexArrays = (PFNGLGENVERTEXARRAYSPROC)GetProcAddress(library, "glGenVertexArrays");
    if (!onix_ptr_glGenVertexArrays)
        return 0;
    onix_ptr_glGenerateMipmap = (PFNGLGENERATEMIPMAPPROC)wglGetProcAddress("glGenerateMipmap");
    if ((uintptr_t)onix_ptr_glGenerateMipmap <= 3 || (intptr_t)onix_ptr_glGenerateMipmap == -1)
        onix_ptr_glGenerateMipmap = (PFNGLGENERATEMIPMAPPROC)GetProcAddress(library, "glGenerateMipmap");
    if (!onix_ptr_glGenerateMipmap)
        return 0;
    onix_ptr_glGetError = (PFNGLGETERRORPROC)wglGetProcAddress("glGetError");
    if ((uintptr_t)onix_ptr_glGetError <= 3 || (intptr_t)onix_ptr_glGetError == -1)
        onix_ptr_glGetError = (PFNGLGETERRORPROC)GetProcAddress(library, "glGetError");
    if (!onix_ptr_glGetError)
        return 0;
    onix_ptr_glGetIntegerv = (PFNGLGETINTEGERVPROC)wglGetProcAddress("glGetIntegerv");
    if ((uintptr_t)onix_ptr_glGetIntegerv <= 3 || (intptr_t)onix_ptr_glGetIntegerv == -1)
        onix_ptr_glGetIntegerv = (PFNGLGETINTEGERVPROC)GetProcAddress(library, "glGetIntegerv");
    if (!onix_ptr_glGetIntegerv)
        return 0;
    onix_ptr_glGetProgramInfoLog = (PFNGLGETPROGRAMINFOLOGPROC)wglGetProcAddress("glGetProgramInfoLog");
    if ((uintptr_t)onix_ptr_glGetProgramInfoLog <= 3 || (intptr_t)onix_ptr_glGetProgramInfoLog == -1)
        onix_ptr_glGetProgramInfoLog = (PFNGLGETPROGRAMINFOLOGPROC)GetProcAddress(library, "glGetProgramInfoLog");
    if (!onix_ptr_glGetProgramInfoLog)
        return 0;
    onix_ptr_glGetProgramiv = (PFNGLGETPROGRAMIVPROC)wglGetProcAddress("glGetProgramiv");
    if ((uintptr_t)onix_ptr_glGetProgramiv <= 3 || (intptr_t)onix_ptr_glGetProgramiv == -1)
        onix_ptr_glGetProgramiv = (PFNGLGETPROGRAMIVPROC)GetProcAddress(library, "glGetProgramiv");
    if (!onix_ptr_glGetProgramiv)
        return 0;
    onix_ptr_glGetShaderInfoLog = (PFNGLGETSHADERINFOLOGPROC)wglGetProcAddress("glGetShaderInfoLog");
    if ((uintptr_t)onix_ptr_glGetShaderInfoLog <= 3 || (intptr_t)onix_ptr_glGetShaderInfoLog == -1)
        onix_ptr_glGetShaderInfoLog = (PFNGLGETSHADERINFOLOGPROC)GetProcAddress(library, "glGetShaderInfoLog");
    if (!onix_ptr_glGetShaderInfoLog)
        return 0;
    onix_ptr_glGetShaderiv = (PFNGLGETSHADERIVPROC)wglGetProcAddress("glGetShaderiv");
    if ((uintptr_t)onix_ptr_glGetShaderiv <= 3 || (intptr_t)onix_ptr_glGetShaderiv == -1)
        onix_ptr_glGetShaderiv = (PFNGLGETSHADERIVPROC)GetProcAddress(library, "glGetShaderiv");
    if (!onix_ptr_glGetShaderiv)
        return 0;
    onix_ptr_glGetString = (PFNGLGETSTRINGPROC)wglGetProcAddress("glGetString");
    if ((uintptr_t)onix_ptr_glGetString <= 3 || (intptr_t)onix_ptr_glGetString == -1)
        onix_ptr_glGetString = (PFNGLGETSTRINGPROC)GetProcAddress(library, "glGetString");
    if (!onix_ptr_glGetString)
        return 0;
    onix_ptr_glGetStringi = (PFNGLGETSTRINGIPROC)wglGetProcAddress("glGetStringi");
    if ((uintptr_t)onix_ptr_glGetStringi <= 3 || (intptr_t)onix_ptr_glGetStringi == -1)
        onix_ptr_glGetStringi = (PFNGLGETSTRINGIPROC)GetProcAddress(library, "glGetStringi");
    if (!onix_ptr_glGetStringi)
        return 0;
    onix_ptr_glGetUniformLocation = (PFNGLGETUNIFORMLOCATIONPROC)wglGetProcAddress("glGetUniformLocation");
    if ((uintptr_t)onix_ptr_glGetUniformLocation <= 3 || (intptr_t)onix_ptr_glGetUniformLocation == -1)
        onix_ptr_glGetUniformLocation = (PFNGLGETUNIFORMLOCATIONPROC)GetProcAddress(library, "glGetUniformLocation");
    if (!onix_ptr_glGetUniformLocation)
        return 0;
    onix_ptr_glLinkProgram = (PFNGLLINKPROGRAMPROC)wglGetProcAddress("glLinkProgram");
    if ((uintptr_t)onix_ptr_glLinkProgram <= 3 || (intptr_t)onix_ptr_glLinkProgram == -1)
        onix_ptr_glLinkProgram = (PFNGLLINKPROGRAMPROC)GetProcAddress(library, "glLinkProgram");
    if (!onix_ptr_glLinkProgram)
        return 0;
    onix_ptr_glMapBufferRange = (PFNGLMAPBUFFERRANGEPROC)wglGetProcAddress("glMapBufferRange");
    if ((uintptr_t)onix_ptr_glMapBufferRange <= 3 || (intptr_t)onix_ptr_glMapBufferRange == -1)
        onix_ptr_glMapBufferRange = (PFNGLMAPBUFFERRANGEPROC)GetProcAddress(library, "glMapBufferRange");
    if (!onix_ptr_glMapBufferRange)
        return 0;
    onix_ptr_glUnmapBuffer = (PFNGLUNMAPBUFFERPROC)wglGetProcAddress("glUnmapBuffer");
    if ((uintptr_t)onix_ptr_glUnmapBuffer <= 3 || (intptr_t)onix_ptr_glUnmapBuffer == -1)
        onix_ptr_glUnmapBuffer = (PFNGLUNMAPBUFFERPROC)GetProcAddress(library, "glUnmapBuffer");
    if (!onix_ptr_glUnmapBuffer)
        return 0;
    onix_ptr_glPixelStorei = (PFNGLPIXELSTOREIPROC)wglGetProcAddress("glPixelStorei");
    if ((uintptr_t)onix_ptr_glPixelStorei <= 3 || (intptr_t)onix_ptr_glPixelStorei == -1)
        onix_ptr_glPixelStorei = (PFNGLPIXELSTOREIPROC)GetProcAddress(library, "glPixelStorei");
    if (!onix_ptr_glPixelStorei)
        return 0;
    onix_ptr_glReadPixels = (PFNGLREADPIXELSPROC)wglGetProcAddress("glReadPixels");
    if ((uintptr_t)onix_ptr_glReadPixels <= 3 || (intptr_t)onix_ptr_glReadPixels == -1)
        onix_ptr_glReadPixels = (PFNGLREADPIXELSPROC)GetProcAddress(library, "glReadPixels");
    if (!onix_ptr_glReadPixels)
        return 0;
    onix_ptr_glRenderbufferStorage = (PFNGLRENDERBUFFERSTORAGEPROC)wglGetProcAddress("glRenderbufferStorage");
    if ((uintptr_t)onix_ptr_glRenderbufferStorage <= 3 || (intptr_t)onix_ptr_glRenderbufferStorage == -1)
        onix_ptr_glRenderbufferStorage = (PFNGLRENDERBUFFERSTORAGEPROC)GetProcAddress(library, "glRenderbufferStorage");
    if (!onix_ptr_glRenderbufferStorage)
        return 0;
    onix_ptr_glScissor = (PFNGLSCISSORPROC)wglGetProcAddress("glScissor");
    if ((uintptr_t)onix_ptr_glScissor <= 3 || (intptr_t)onix_ptr_glScissor == -1)
        onix_ptr_glScissor = (PFNGLSCISSORPROC)GetProcAddress(library, "glScissor");
    if (!onix_ptr_glScissor)
        return 0;
    onix_ptr_glShaderSource = (PFNGLSHADERSOURCEPROC)wglGetProcAddress("glShaderSource");
    if ((uintptr_t)onix_ptr_glShaderSource <= 3 || (intptr_t)onix_ptr_glShaderSource == -1)
        onix_ptr_glShaderSource = (PFNGLSHADERSOURCEPROC)GetProcAddress(library, "glShaderSource");
    if (!onix_ptr_glShaderSource)
        return 0;
    onix_ptr_glTexImage2D = (PFNGLTEXIMAGE2DPROC)wglGetProcAddress("glTexImage2D");
    if ((uintptr_t)onix_ptr_glTexImage2D <= 3 || (intptr_t)onix_ptr_glTexImage2D == -1)
        onix_ptr_glTexImage2D = (PFNGLTEXIMAGE2DPROC)GetProcAddress(library, "glTexImage2D");
    if (!onix_ptr_glTexImage2D)
        return 0;
    onix_ptr_glTexParameteri = (PFNGLTEXPARAMETERIPROC)wglGetProcAddress("glTexParameteri");
    if ((uintptr_t)onix_ptr_glTexParameteri <= 3 || (intptr_t)onix_ptr_glTexParameteri == -1)
        onix_ptr_glTexParameteri = (PFNGLTEXPARAMETERIPROC)GetProcAddress(library, "glTexParameteri");
    if (!onix_ptr_glTexParameteri)
        return 0;
    onix_ptr_glTexSubImage2D = (PFNGLTEXSUBIMAGE2DPROC)wglGetProcAddress("glTexSubImage2D");
    if ((uintptr_t)onix_ptr_glTexSubImage2D <= 3 || (intptr_t)onix_ptr_glTexSubImage2D == -1)
        onix_ptr_glTexSubImage2D = (PFNGLTEXSUBIMAGE2DPROC)GetProcAddress(library, "glTexSubImage2D");
    if (!onix_ptr_glTexSubImage2D)
        return 0;
    onix_ptr_glUniform1i = (PFNGLUNIFORM1IPROC)wglGetProcAddress("glUniform1i");
    if ((uintptr_t)onix_ptr_glUniform1i <= 3 || (intptr_t)onix_ptr_glUniform1i == -1)
        onix_ptr_glUniform1i = (PFNGLUNIFORM1IPROC)GetProcAddress(library, "glUniform1i");
    if (!onix_ptr_glUniform1i)
        return 0;
    onix_ptr_glUniform4fv = (PFNGLUNIFORM4FVPROC)wglGetProcAddress("glUniform4fv");
    if ((uintptr_t)onix_ptr_glUniform4fv <= 3 || (intptr_t)onix_ptr_glUniform4fv == -1)
        onix_ptr_glUniform4fv = (PFNGLUNIFORM4FVPROC)GetProcAddress(library, "glUniform4fv");
    if (!onix_ptr_glUniform4fv)
        return 0;
    onix_ptr_glUniformMatrix4fv = (PFNGLUNIFORMMATRIX4FVPROC)wglGetProcAddress("glUniformMatrix4fv");
    if ((uintptr_t)onix_ptr_glUniformMatrix4fv <= 3 || (intptr_t)onix_ptr_glUniformMatrix4fv == -1)
        onix_ptr_glUniformMatrix4fv = (PFNGLUNIFORMMATRIX4FVPROC)GetProcAddress(library, "glUniformMatrix4fv");
    if (!onix_ptr_glUniformMatrix4fv)
        return 0;
    onix_ptr_glUseProgram = (PFNGLUSEPROGRAMPROC)wglGetProcAddress("glUseProgram");
    if ((uintptr_t)onix_ptr_glUseProgram <= 3 || (intptr_t)onix_ptr_glUseProgram == -1)
        onix_ptr_glUseProgram = (PFNGLUSEPROGRAMPROC)GetProcAddress(library, "glUseProgram");
    if (!onix_ptr_glUseProgram)
        return 0;
    onix_ptr_glVertexAttribDivisor = (PFNGLVERTEXATTRIBDIVISORPROC)wglGetProcAddress("glVertexAttribDivisor");
    if ((uintptr_t)onix_ptr_glVertexAttribDivisor <= 3 || (intptr_t)onix_ptr_glVertexAttribDivisor == -1)
        onix_ptr_glVertexAttribDivisor = (PFNGLVERTEXATTRIBDIVISORPROC)GetProcAddress(library, "glVertexAttribDivisor");
    if (!onix_ptr_glVertexAttribDivisor)
        return 0;
    onix_ptr_glVertexAttribIPointer = (PFNGLVERTEXATTRIBIPOINTERPROC)wglGetProcAddress("glVertexAttribIPointer");
    if ((uintptr_t)onix_ptr_glVertexAttribIPointer <= 3 || (intptr_t)onix_ptr_glVertexAttribIPointer == -1)
        onix_ptr_glVertexAttribIPointer =
            (PFNGLVERTEXATTRIBIPOINTERPROC)GetProcAddress(library, "glVertexAttribIPointer");
    if (!onix_ptr_glVertexAttribIPointer)
        return 0;
    onix_ptr_glVertexAttribPointer = (PFNGLVERTEXATTRIBPOINTERPROC)wglGetProcAddress("glVertexAttribPointer");
    if ((uintptr_t)onix_ptr_glVertexAttribPointer <= 3 || (intptr_t)onix_ptr_glVertexAttribPointer == -1)
        onix_ptr_glVertexAttribPointer = (PFNGLVERTEXATTRIBPOINTERPROC)GetProcAddress(library, "glVertexAttribPointer");
    if (!onix_ptr_glVertexAttribPointer)
        return 0;
    onix_ptr_glViewport = (PFNGLVIEWPORTPROC)wglGetProcAddress("glViewport");
    if ((uintptr_t)onix_ptr_glViewport <= 3 || (intptr_t)onix_ptr_glViewport == -1)
        onix_ptr_glViewport = (PFNGLVIEWPORTPROC)GetProcAddress(library, "glViewport");
    if (!onix_ptr_glViewport)
        return 0;
    return 1;
}
