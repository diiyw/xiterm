#include <EGL/egl.h>
#include <GLES3/gl3.h>
#include <stdio.h>
#include <stdlib.h>
#include "gles_frame_wrappers.inc"

static void require(int ok, const char *message) {
 if (!ok) {fprintf(stderr,"%s (GL 0x%x, EGL 0x%x)\n",message,glGetError(),eglGetError());exit(1);}
}
static void pixel(int x,int y,int r,int g,int b) {
 unsigned char rgba[4];glReadPixels(x,y,1,1,GL_RGBA,GL_UNSIGNED_BYTE,rgba);
 require(rgba[0]==r && rgba[1]==g && rgba[2]==b && rgba[3]==255,"unexpected surface pixel");
}
static GLuint shader(GLenum type,const char *source) {
 GLuint result=glCreateShader(type);glShaderSource(result,1,&source,NULL);glCompileShader(result);
 GLint ok;glGetShaderiv(result,GL_COMPILE_STATUS,&ok);require(ok,"compile depth probe shader");return result;
}
int main(void) {
 EGLDisplay display=eglGetDisplay(EGL_DEFAULT_DISPLAY);
 if (display==EGL_NO_DISPLAY || !eglInitialize(display,NULL,NULL)) return 77;
 const EGLint attrs[]={EGL_SURFACE_TYPE,EGL_PBUFFER_BIT,EGL_RENDERABLE_TYPE,EGL_OPENGL_ES3_BIT,EGL_RED_SIZE,8,EGL_GREEN_SIZE,8,EGL_BLUE_SIZE,8,EGL_ALPHA_SIZE,8,EGL_DEPTH_SIZE,0,EGL_SAMPLE_BUFFERS,0,EGL_NONE};
 EGLConfig config;EGLint count;require(eglChooseConfig(display,attrs,&config,1,&count) && count,"choose depthless config");
 const EGLint surface_attrs[]={EGL_WIDTH,16,EGL_HEIGHT,16,EGL_NONE},context_attrs[]={EGL_CONTEXT_CLIENT_VERSION,3,EGL_NONE};
 EGLSurface surface=eglCreatePbufferSurface(display,config,surface_attrs);
 EGLContext context=eglCreateContext(display,config,EGL_NO_CONTEXT,context_attrs);
 require(eglMakeCurrent(display,surface,surface,context),"create GLES3 pbuffer");
 GLint depth_bits;glGetIntegerv(GL_DEPTH_BITS,&depth_bits);require(depth_bits==0,"2D default surface must have no depth");
 const GLfloat red[]={1,0,0,1};
 for(int frame=0;frame<6;frame++) {onix_gl_begin_frame(0,red,1,0,0,0,0,0,0,16);pixel(8,8,255,0,0);}
 GLuint framebuffer,texture,depth;glGenFramebuffers(1,&framebuffer);glGenTextures(1,&texture);glGenRenderbuffers(1,&depth);
 glBindTexture(GL_TEXTURE_2D,texture);glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,16,16,0,GL_RGBA,GL_UNSIGNED_BYTE,NULL);
 glBindRenderbuffer(GL_RENDERBUFFER,depth);glRenderbufferStorage(GL_RENDERBUFFER,GL_DEPTH_COMPONENT24,16,16);
 glBindFramebuffer(GL_FRAMEBUFFER,framebuffer);glFramebufferTexture2D(GL_FRAMEBUFFER,GL_COLOR_ATTACHMENT0,GL_TEXTURE_2D,texture,0);glFramebufferRenderbuffer(GL_FRAMEBUFFER,GL_DEPTH_ATTACHMENT,GL_RENDERBUFFER,depth);
 require(glCheckFramebufferStatus(GL_FRAMEBUFFER)==GL_FRAMEBUFFER_COMPLETE,"create lazy surface FBO");
 GLuint vertex=shader(GL_VERTEX_SHADER,"#version 300 es\nuniform float z;void main(){vec2 p=vec2(float((gl_VertexID<<1)&2),float(gl_VertexID&2));gl_Position=vec4(p*2.0-1.0,z,1.0);}");
 GLuint fragment=shader(GL_FRAGMENT_SHADER,"#version 300 es\nprecision highp float;uniform vec4 rgba;out vec4 color;void main(){color=rgba;}");
 GLuint program=glCreateProgram();glAttachShader(program,vertex);glAttachShader(program,fragment);glLinkProgram(program);
 GLint linked;glGetProgramiv(program,GL_LINK_STATUS,&linked);require(linked,"link depth probe");glUseProgram(program);glViewport(0,0,16,16);
 for(int frame=0;frame<6;frame++) {
  if(frame) onix_gl_begin_frame(framebuffer,red,1,0,0,0,0,0,0,16);
  else {
   glEnable(GL_SCISSOR_TEST);glScissor(4,4,8,8);
   onix_gl_blit_surface(0,framebuffer,16,16,1);
   require(glIsEnabled(GL_SCISSOR_TEST),"upgrade restores scissor");
   glDisable(GL_SCISSOR_TEST);glDepthMask(GL_TRUE);glClearDepthf(0);glClear(GL_DEPTH_BUFFER_BIT);
  }
  glEnable(GL_DEPTH_TEST);glDepthFunc(GL_GREATER);glEnable(GL_SCISSOR_TEST);glScissor(4,4,8,8);
  glUniform1f(glGetUniformLocation(program,"z"),frame%2?.0f:.5f);glUniform4f(glGetUniformLocation(program,"rgba"),0,1,0,1);glDrawArrays(GL_TRIANGLES,0,3);
  // A target round trip must restore the owned surface, including its depth.
  glBindFramebuffer(GL_FRAMEBUFFER,0);onix_gl_end_render_target(framebuffer,1,1,4,4,8,8,16);
  glUniform1f(glGetUniformLocation(program,"z"),-.5f);glUniform4f(glGetUniformLocation(program,"rgba"),0,0,1,1);glDrawArrays(GL_TRIANGLES,0,3);
  glDisable(GL_SCISSOR_TEST);pixel(1,1,255,0,0);pixel(8,8,0,255,0);
  unsigned char rgba[4];onix_gl_read_render_target(framebuffer,8,8,1,1,GL_RGBA,GL_UNSIGNED_BYTE,rgba,framebuffer);
  GLint binding;glGetIntegerv(GL_FRAMEBUFFER_BINDING,&binding);require((GLuint)binding==framebuffer,"readback restores owned surface");
  glEnable(GL_SCISSOR_TEST);glScissor(0,0,1,1);onix_gl_blit_surface(framebuffer,0,16,16,1);
  require(glIsEnabled(GL_SCISSOR_TEST),"present restores scissor");pixel(15,15,255,0,0);pixel(8,8,0,255,0);glDisable(GL_SCISSOR_TEST);
  glBindFramebuffer(GL_FRAMEBUFFER,framebuffer);
  require(glGetError()==GL_NO_ERROR,"depth upgrade/present GL validation");
 }
 glDeleteProgram(program);glDeleteShader(vertex);glDeleteShader(fragment);glDeleteFramebuffers(1,&framebuffer);glDeleteRenderbuffers(1,&depth);glDeleteTextures(1,&texture);
 eglMakeCurrent(display,EGL_NO_SURFACE,EGL_NO_SURFACE,EGL_NO_CONTEXT);eglDestroyContext(display,context);eglDestroySurface(display,surface);eglTerminate(display);
 puts("GLES depthless 2D, color-preserving upgrade, reversed depth reset, target/readback restore, unclipped present and scissor preservation passed");return 0;
}
