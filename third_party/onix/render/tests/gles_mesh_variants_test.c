#define main onix_gles_mesh_test_main
#include "gles_mesh_test.c"
#undef main

static GLuint load_program(const char *path) {
 FILE *file=fopen(path,"rb");if(!file)exit(1);
 fseek(file,0,SEEK_END);long length=ftell(file);rewind(file);
 char *source=calloc(length+1,1);if(!source || fread(source,1,length,file)!=(size_t)length)exit(1);fclose(file);
 GLuint program=glCreateProgram(),vs=compile_shader(GL_VERTEX_SHADER,mesh_test_vertex),fs=compile_shader(GL_FRAGMENT_SHADER,source);free(source);
 glAttachShader(program,vs);glAttachShader(program,fs);glLinkProgram(program);
 GLint linked;glGetProgramiv(program,GL_LINK_STATUS,&linked);
 if(!linked){char log[4096];glGetProgramInfoLog(program,sizeof(log),NULL,log);fprintf(stderr,"%s: %s\n",path,log);exit(1);}
 glDeleteShader(vs);glDeleteShader(fs);return program;
}
static void setup(GLuint program) {
 glUseProgram(program);
 uniform4(program,"testNormal",0,0,1,0);uniform4(program,"testWorldPosition",0,0,.25,0);
 uniform4(program,"cameraPosition",0,0,2,1);uniform4(program,"cameraForward",0,0,-1,16);
 uniform4(program,"testSurface",1,1,1,0);uniform4(program,"testSurface1",1,1,1,0);
 uniform4(program,"testSurface2",0,0,0,0);uniform4(program,"testSurface3",0,0,0,0);
 uniform4(program,"ambientLight",.2,.2,.2,1);uniform4(program,"directionalLight",1,1,1,0);
 uniform4(program,"lightDirection",0,0,-1,0);uniform4(program,"lightMeta",0,0,0,0);
 uniform4(program,"renderParams",1,0,0,0);
}
int main(int argc,char **argv) {
 if(argc!=5)return 1;
 EGLDisplay display=eglGetDisplay(EGL_DEFAULT_DISPLAY);if(!eglInitialize(display,NULL,NULL))return 77;
 eglBindAPI(EGL_OPENGL_ES_API);
 EGLint attrs[]={EGL_SURFACE_TYPE,EGL_PBUFFER_BIT,EGL_RENDERABLE_TYPE,EGL_OPENGL_ES3_BIT,EGL_RED_SIZE,8,EGL_GREEN_SIZE,8,EGL_BLUE_SIZE,8,EGL_ALPHA_SIZE,8,EGL_NONE};
 EGLConfig config;EGLint count;if(!eglChooseConfig(display,attrs,&config,1,&count)||!count)return 77;
 EGLint surfaceAttrs[]={EGL_WIDTH,4,EGL_HEIGHT,4,EGL_NONE},contextAttrs[]={EGL_CONTEXT_CLIENT_VERSION,3,EGL_NONE};
 EGLSurface surface=eglCreatePbufferSurface(display,config,surfaceAttrs);EGLContext context=eglCreateContext(display,config,EGL_NO_CONTEXT,contextAttrs);
 if(!eglMakeCurrent(display,surface,surface,context))return 77;
 printf("GLES variant device: %s / %s\n",glGetString(GL_RENDERER),glGetString(GL_VERSION));
 GLuint programs[4];for(int i=0;i<4;i++){programs[i]=load_program(argv[i+1]);setup(programs[i]);}
 GLuint vao,texture;glGenVertexArrays(1,&vao);glBindVertexArray(vao);
 glGenTextures(1,&texture);glBindTexture(GL_TEXTURE_2D,texture);unsigned char white[]={255,255,255,255};
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 int failed=0,comparisons=0;
 for(int variant=1;variant<4;variant++) for(int mode=0;mode<3;mode++) for(int material=0;material<3;material++) for(int flags=0;flags<4;flags++) {
  int values[2];
  for(int side=0;side<2;side++) {
   GLuint program=programs[side?variant:0];glUseProgram(program);
   float pass=variant==3?(mode==0?1:mode==1?2:5):(mode==0?0:mode==1?3:6);
   uniform4(program,"renderParams",1,pass,0,0);
   float features=flags==3?(4+8+16+32+64+128):0;
   uniform4(program,"testMaterial",.15f+material*.4f,1.5,.5f,512+(flags%3)+(variant==2?256:features));
   uniform4(program,"testEmission",.1,.2,.3,material*.5f);
   values[side]=draw_pixel();
  }
  if(abs(values[0]-values[1])>1){fprintf(stderr,"variant %d mode %d material %d flags %d: full=%d compact=%d\n",variant,mode,material,flags,values[0],values[1]);failed=1;}
  comparisons++;
 }
 // Both programs must preserve flat/background/exclusion and all three SSAO
 // kernels under perspective and rotated orthographic camera transforms.
 for(int i=0;i<2;i++){glUseProgram(programs[i]);setup(programs[i]);failed|=ssao_pixels(programs[i]);}
 printf("GLES variants: %d full/compact comparisons and SSAO %s\n",comparisons,failed?"FAILED":"passed");
 for(int i=0;i<4;i++)glDeleteProgram(programs[i]);glDeleteTextures(1,&texture);glDeleteVertexArrays(1,&vao);
 eglMakeCurrent(display,EGL_NO_SURFACE,EGL_NO_SURFACE,EGL_NO_CONTEXT);eglDestroyContext(display,context);eglDestroySurface(display,surface);eglTerminate(display);
 return failed;
}
