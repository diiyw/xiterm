#include <EGL/egl.h>
#include <GLES3/gl3.h>
#include <stdio.h>
#include <stdlib.h>
#include <math.h>
#include <float.h>
#include "display_reference.h"

static GLuint compile_shader(GLenum kind, const char *source) {
	GLuint shader = glCreateShader(kind);
	glShaderSource(shader, 1, &source, NULL);
	glCompileShader(shader);
	GLint compiled;
	glGetShaderiv(shader, GL_COMPILE_STATUS, &compiled);
	if (!compiled) {
		char log[4096];
		glGetShaderInfoLog(shader, sizeof(log), NULL, log);
		fprintf(stderr, "compile shader: %s\n", log);
		exit(1);
	}
	return shader;
}

static void uniform4(GLuint program, const char *name, float x, float y, float z, float w) {
	glUniform4f(glGetUniformLocation(program, name), x, y, z, w);
}

static unsigned char pixel_alpha;
static float environment_encoding;
static int draw_pixel(void) {
	glClearColor(0,0,0,1);
	glClear(GL_COLOR_BUFFER_BIT);
	glDrawArrays(GL_TRIANGLES, 0, 3);
	unsigned char rgba[4];
	glReadPixels(0, 0, 1, 1, GL_RGBA, GL_UNSIGNED_BYTE, rgba);
	if (glGetError() != GL_NO_ERROR) {
		fprintf(stderr, "invalid rendered pixel\n");
		exit(1);
	}
	pixel_alpha=rgba[3];
	return rgba[0];
}

static int pixel(GLuint program, int count, float direction) {
 uniform4(program,"lightMeta",(float)count,0,0,0);
 uniform4(program,"lightShadows[8]",0,0,0,environment_encoding);
 uniform4(program,"lightDirections[0]",0,direction,0,1);
 return draw_pixel();
}
static float scene_radiance(int byte) {
 float srgb=(float)byte/255.0f;
 float linear=srgb<=0.04045f?srgb/12.92f:powf((srgb+0.055f)/1.055f,2.4f);
 return linear/(1.0f-linear);
}
static void shadow_map_uniforms(GLuint program,int index,const float *matrix,float x,float y,float w,float h,float near,float far) {
 char name[64];
 snprintf(name,sizeof(name),"shadowMatrices[%d]",index);
 glUniformMatrix4fv(glGetUniformLocation(program,name),1,GL_FALSE,matrix);
 snprintf(name,sizeof(name),"shadowRects[%d]",index);
 uniform4(program,name,x,y,w,h);
 snprintf(name,sizeof(name),"shadowParams[%d]",index);
 uniform4(program,name,0.0003f,0,near,far);
}

static int shadow_atlas_pixels(GLuint program,GLuint texture) {
 int failed=0;
 unsigned char depths[12*8*4];
 const float center[16]={[14]=.5f,[15]=1};
 const float axes[6][3]={{1,0,0},{-1,0,0},{0,1,0},{0,-1,0},{0,0,1},{0,0,-1}};
 for(int face=0;face<6;face++) {
  // CPU atlas rects are top-left; GLES texture row zero is the bottom row.
  int ox=face%3*4,oy=(1-face/3)*4;
  unsigned int depth=(unsigned int)((face%2?.25f:.75f)*16777215.0f);
  for(int y=0;y<4;y++) for(int x=0;x<4;x++) {
   int offset=((oy+y)*12+ox+x)*4;
   depths[offset]=depth>>16;depths[offset+1]=depth>>8;depths[offset+2]=depth;depths[offset+3]=255;
  }
  shadow_map_uniforms(program,face,center,face%3/3.0f,face/3/2.0f,1/3.0f,.5f,0,0);
 }
 glActiveTexture(GL_TEXTURE1);glBindTexture(GL_TEXTURE_2D,texture);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,12,8,0,GL_RGBA,GL_UNSIGNED_BYTE,depths);
 uniform4(program,"testMaterial",.8f,1.0f,.5f,0);
 uniform4(program,"testSurface",1,1,0,0);
 uniform4(program,"lightMeta",1,0,0,0);
 uniform4(program,"lightDirections[0]",0,-1,0,2);
 uniform4(program,"lightPosition[0]",0,0,0,1);
 uniform4(program,"lightColors[0]",1,1,1,0);
 uniform4(program,"lightParams[0]",10,50,.9f,.5f);
 for(int face=0;face<6;face++) {
  const float *axis=axes[face];
  uniform4(program,"testWorldPosition",axis[0],axis[1],axis[2],0);
  uniform4(program,"testNormal",-axis[0],-axis[1],-axis[2],0);
  uniform4(program,"cameraPosition",-axis[0],-axis[1],-axis[2],1);
  uniform4(program,"lightShadows[0]",0,0,0,0);
  int clear=draw_pixel();
  uniform4(program,"lightShadows[0]",1,6,0,0);
  int shaded=draw_pixel();
  if(clear<50 || (face%2?abs(clear-shaded)>1:shaded>1)) {
   fprintf(stderr,"point shadow face %d: clear=%d shaded=%d\n",face,clear,shaded);failed=1;
  }
 }
 uniform4(program,"testWorldPosition",0,-1,0,0);
 uniform4(program,"testNormal",0,1,0,0);
 uniform4(program,"cameraPosition",0,2,2,1);
 uniform4(program,"lightRight[0]",1,0,0,1);
 uniform4(program,"lightUp[0]",0,0,1,1);
 for(int kind=3;kind<=5;kind++) {
  uniform4(program,"lightDirections[0]",0,-1,0,kind);
  uniform4(program,"lightShadows[0]",1,1,0,0);
  int blocked=draw_pixel();
  uniform4(program,"lightShadows[0]",2,1,0,0);
  int clear=draw_pixel();
  if(blocked>1 || clear<50) {fprintf(stderr,"spot shadow kind %d: blocked=%d clear=%d\n",kind,blocked,clear);failed=1;}
 }
 uniform4(program,"lightDirections[0]",0,-1,0,1);
 uniform4(program,"lightColors[1]",1,1,1,0);
 uniform4(program,"lightDirections[1]",0,-1,0,1);
 uniform4(program,"lightShadows[0]",0,0,0,0);
 int reference=draw_pixel();
 uniform4(program,"lightMeta",2,0,0,0);
 uniform4(program,"lightShadows[0]",1,1,0,0);
 uniform4(program,"lightShadows[1]",2,1,0,0);
 int independent=draw_pixel();
 if(abs(reference-independent)>1) {fprintf(stderr,"independent light shadows: reference=%d combined=%d\n",reference,independent);failed=1;}
 uniform4(program,"lightMeta",1,0,0,0);
 uniform4(program,"cameraPosition",0,0,2,1);
 uniform4(program,"cameraForward",0,0,-1,0);
 uniform4(program,"shadowParams[0]",.0003f,0,0,4);
 uniform4(program,"shadowParams[1]",.0003f,0,4,8);
 uniform4(program,"lightShadows[0]",1,2,.25f,0);
 const float distances[]={2,3.5f,4,6,9},expected[]={0,.5f,1,1,1};
 for(int i=0;i<5;i++) {
  uniform4(program,"testWorldPosition",0,0,2-distances[i],0);
  float visibility=scene_radiance(draw_pixel())/scene_radiance(reference);
  if(fabsf(visibility-expected[i])>.018f) {fprintf(stderr,"cascade distance %g visibility=%g expected=%g\n",distances[i],visibility,expected[i]);failed=1;}
 }
 // Fade the last, occluded cascade into unshadowed lighting at the distance limit.
 uniform4(program,"lightShadows[0]",2,2,.25f,0);
 uniform4(program,"shadowParams[1]",.0003f,0,0,4);
 uniform4(program,"shadowParams[2]",.0003f,0,4,8);
 uniform4(program,"testWorldPosition",0,0,-5.5f,0);
 float fade=scene_radiance(draw_pixel())/scene_radiance(reference);
 if(fabsf(fade-.5f)>.018f) {fprintf(stderr,"last cascade fade=%g\n",fade);failed=1;}
 // PCF must stay inside a tile, including the exact projection boundary.
 const float identity[16]={[0]=1,[5]=1,[10]=1,[15]=1};
 shadow_map_uniforms(program,0,identity,0,0,1/3.0f,.5f,0,0);
 shadow_map_uniforms(program,1,identity,1/3.0f,0,1/3.0f,.5f,0,0);
 uniform4(program,"testWorldPosition",1,0,.5f,0);
 uniform4(program,"lightShadows[0]",1,1,0,0);
 int blockedEdge=draw_pixel();
 uniform4(program,"testWorldPosition",-1,0,.5f,0);
 uniform4(program,"lightShadows[0]",2,1,0,0);
 int clearEdge=draw_pixel();
 if(blockedEdge>1 || abs(clearEdge-reference)>1) {fprintf(stderr,"atlas PCF bleed: blocked=%d clear=%d reference=%d\n",blockedEdge,clearEdge,reference);failed=1;}
 uniform4(program,"lightShadows[0]",0,0,0,0);
 uniform4(program,"lightShadows[1]",0,0,0,0);
 uniform4(program,"testWorldPosition",0,0,0,0);
 uniform4(program,"cameraPosition",0,2,2,1);
 uniform4(program,"testSurface",1,1,1,0);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,0);
 unsigned char white[4]={255,255,255,255};
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
 printf("shadow atlas: six point faces, three spot shapes, independent lights, cascade blending and PCF isolation %s\n",failed?"FAILED":"passed");
 return failed;
}

static int shadow_pixels(GLuint program, GLuint texture) {
 // Use diffuse-only lighting so radiance ratios directly measure visibility.
 uniform4(program,"testMaterial",0.8f,1.0f,0.5f,0);
 uniform4(program,"testSurface",1,1,0,0);
 uniform4(program,"testWorldPosition",0,0,0.5f,0);
 uniform4(program,"lightShadows[0]",0,0,0,0);
 float lit=scene_radiance(pixel(program,1,-1));
 unsigned char depths[8*8*4];
 glActiveTexture(GL_TEXTURE1);glBindTexture(GL_TEXTURE_2D,texture);
 int failed=0;
 for (int axis=0;axis<2;axis++) {
  for (int y=0;y<8;y++) for (int x=0;x<8;x++) {
   unsigned int depth=(unsigned int)(((axis?y:x)<4?0.75f:0.25f)*16777215.0f);
   int i=(y*8+x)*4;
   depths[i]=depth>>16;depths[i+1]=depth>>8;depths[i+2]=depth;depths[i+3]=255;
  }
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,8,8,0,GL_RGBA,GL_UNSIGNED_BYTE,depths);
  uniform4(program,"lightShadows[0]",1,1,0,0);
  uniform4(program,"shadowParams[0]",0.0003f,0,0,0);
  float previous=-1;
  // Cross a texel boundary in tenths of a texel. Nearest PCF gives plateaus.
  for (int step=0;step<=10;step++) {
   float fraction=step/10.0f;
   float coordinate=(3.5f+fraction)/8.0f*2.0f-1.0f;
   uniform4(program,"testWorldPosition",axis?0:coordinate,axis?coordinate:0,0.5f,0);
   float visibility=scene_radiance(draw_pixel())/lit;
   float expected=(1.0f+fraction)/3.0f;
   if (fabsf(visibility-expected)>0.018f || visibility<=previous) {
    fprintf(stderr,"shadow edge axis=%d step=%d: visibility=%g expected=%g\n",axis,step,visibility,expected);
    failed=1;
   }
   previous=visibility;
  }
 }
 // A receiver that slopes in both shadow-map axes must not shadow itself.
 // A real occluder 0.1 closer to the light must still block it (reversed Z).
 glViewport(0,0,4,4);
 uniform4(program,"testShadowPlane",0.25f,-0.125f,0.75f,0);
 for (int occluder=0;occluder<2;occluder++) {
  for (int y=0;y<8;y++) for (int x=0;x<8;x++) {
   float z=0.5f+0.25f*((x+0.5f)/4.0f-1)-0.125f*((y+0.5f)/4.0f-1)+0.1f*occluder;
   unsigned int depth=(unsigned int)(z*16777215.0f);
   int i=(y*8+x)*4;
   depths[i]=depth>>16;depths[i+1]=depth>>8;depths[i+2]=depth;depths[i+3]=255;
  }
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,8,8,0,GL_RGBA,GL_UNSIGNED_BYTE,depths);
  for (int step=0;step<8;step++) {
   float x=step/32.0f,y=-step/64.0f;
   uniform4(program,"testWorldPosition",x,y,0.5f+0.25f*x-0.125f*y,1);
   float visibility=scene_radiance(draw_pixel())/lit;
   if (fabsf(visibility-(1-occluder))>0.018f) {
    fprintf(stderr,"shadow slope occluder=%d step=%d: visibility=%g\n",occluder,step,visibility);
    failed=1;
   }
  }
 }
 uniform4(program,"testWorldPosition",0,0,0,0);
 glViewport(0,0,1,1);
 uniform4(program,"testShadowPlane",0,0,0,0);
 uniform4(program,"lightShadows[0]",0,0,0,0);
 uniform4(program,"testMaterial",0.8f,1.5f,0.5f,0);
 uniform4(program,"testSurface",1,1,1,0);
 // Later material tests reuse this texture as a one-pixel data map.
 unsigned char white[4]={255,255,255,255};
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
 printf("shadow filtering: continuous edges and receiver plane %s\n",failed?"FAILED":"passed");
 return failed;
}
static int photometry_pixels(GLuint program) {
 GLuint whiteTexture;glGenTextures(1,&whiteTexture);glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,whiteTexture);
 unsigned char white[]={255,255,255,255};
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 const float pi=3.14159265359f;
 uniform4(program,"testMaterial",0.8f,1.0f,0.5f,0);
 uniform4(program,"testSurface",1,1,1,0);
 uniform4(program,"testWorldPosition",0,0,0,0);
 uniform4(program,"cameraPosition",0,10,0,1);
 uniform4(program,"ambientLight",0,0,0,0);
 uniform4(program,"environment",0,0,0,0);
 uniform4(program,"lightShadows[0]",0,0,0,0);
 uniform4(program,"renderParams",1,0,0,0);
 uniform4(program,"lightMeta",1,1,0,0);
 uniform4(program,"lightColors[0]",1,1,1,0);
 uniform4(program,"lightDirections[0]",0,-1,0,2);
 uniform4(program,"lightPosition[0]",0,1,0,1);
 uniform4(program,"lightParams[0]",100,pi,0,0);
 float pointNear=scene_radiance(draw_pixel());
 uniform4(program,"lightPosition[0]",0,2,0,1);
 float pointFar=scene_radiance(draw_pixel());
 int failed=fabsf(pointNear-1)>0.025f || fabsf(pointNear/pointFar-4)>0.15f;
 // Full 60 degree cone: on axis is illuminated, 45 degrees is outside.
 uniform4(program,"lightDirections[0]",0,-1,0,3);
 uniform4(program,"lightParams[0]",100,pi,0.9659258f,0.8660254f);
 int cone=draw_pixel();
 uniform4(program,"testWorldPosition",2,0,0,0);
 int outside=draw_pixel();
 failed|=cone<120 || outside!=0;
 // Pyramid and box use rotated local axes, and box is constant with depth.
 uniform4(program,"lightRight[0]",1,0,0,0.5f);
 uniform4(program,"lightUp[0]",0,0,1,0.25f);
 uniform4(program,"lightDirections[0]",0,-1,0,4);
 uniform4(program,"testWorldPosition",0,0,0,0);
 int pyramid=draw_pixel();
 uniform4(program,"testWorldPosition",0,0,1,0);
 int pyramidOutside=draw_pixel();
 uniform4(program,"testWorldPosition",0,0,0,0);
 uniform4(program,"lightDirections[0]",0,-1,0,5);
 uniform4(program,"lightParams[0]",10,pi,0,0);
 int boxNear=draw_pixel();
 uniform4(program,"lightPosition[0]",0,5,0,1);
 int boxFar=draw_pixel();
 uniform4(program,"testWorldPosition",1,0,0,0);
 int boxOutside=draw_pixel();
 failed|=pyramid<120 || pyramidOutside!=0 || abs(boxNear-boxFar)>1 || boxOutside!=0;
 // Lambertian surface under finite emitters; compare to analytic integrals.
 uniform4(program,"testWorldPosition",0,0,0,0);
 uniform4(program,"lightPosition[0]",0,2,0,1);
 uniform4(program,"lightParams[0]",100,1,0,0);
 uniform4(program,"lightRight[0]",1,0,0,2);
 uniform4(program,"lightUp[0]",0,0,1,2);
 uniform4(program,"lightDirections[0]",0,-1,0,6);
 float rectangle=scene_radiance(draw_pixel());
 float expectedRect=4.0f/pi/sqrtf(5.0f)*atanf(1.0f/sqrtf(5.0f));
 uniform4(program,"lightDirections[0]",0,1,0,6);
 int backside=draw_pixel();
 uniform4(program,"lightDirections[0]",0,-1,0,7);
 uniform4(program,"lightRight[0]",1,0,0,1);
 float disc=scene_radiance(draw_pixel());
 uniform4(program,"lightDirections[0]",0,-1,0,8);
 uniform4(program,"lightRight[0]",1,0,0,2);
 float tube=scene_radiance(draw_pixel());
 float expectedTube=1.0f/(pi*sqrtf(5.0f));
 failed|=fabsf(rectangle/expectedRect-1)>0.06f || fabsf(disc/0.2f-1)>0.04f || fabsf(tube/expectedTube-1)>0.04f || backside!=0;
 // One EV of compensation doubles scene-linear output before tone mapping.
 uniform4(program,"renderParams",2,0,0,0);
 float compensated=scene_radiance(draw_pixel());
 failed|=fabsf(compensated/tube-2)>0.08f;
 printf("photometry: point=%.3f/%.3f, cone=%d/%d, pyramid=%d/%d, box=%d/%d/%d, area=%.3f/%.3f/%.3f, exposure=%.3f\n",pointNear,pointFar,cone,outside,pyramid,pyramidOutside,boxNear,boxFar,boxOutside,rectangle,disc,tube,compensated);
 glDeleteTextures(1,&whiteTexture);
 return failed;
}

static int ssao_pixels(GLuint program) {
 const float identity[16]={[0]=1,[5]=1,[10]=1,[15]=1};
 unsigned char depths[32*32*4];
 GLuint depthTexture,whiteTexture;
 unsigned char white[4]={255,255,255,255};
 glGenTextures(1,&whiteTexture);glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,whiteTexture);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 glGenTextures(1,&depthTexture);glActiveTexture(GL_TEXTURE2);glBindTexture(GL_TEXTURE_2D,depthTexture);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 glUniform1i(glGetUniformLocation(program,"screenDepthTex"),2);
 glUniformMatrix4fv(glGetUniformLocation(program,"occlusionInverseProjection"),1,GL_FALSE,identity);
 uniform4(program,"cameraPosition",0,0,2,1);
 uniform4(program,"testNormal",0,0,1,0);
 uniform4(program,"testWorldPosition",0,0,.25f,0);
 uniform4(program,"testMaterial",.8f,1.0f,.5f,0);
 uniform4(program,"testSurface",1,1,0,0);
 uniform4(program,"testEmission",0,0,0,0);
 uniform4(program,"environment",0,0,0,0);
 uniform4(program,"ambientLight",1,1,1,1);
 uniform4(program,"lightMeta",0,0,1,.025f);
 uniform4(program,"directionalLight",0,0,0,0);
 uniform4(program,"cameraForward",0,0,-1,16);
 uniform4(program,"renderParams",1,0,0,0);
 int reference=draw_pixel(),flat=0,contact=0,background=0,far=0;
 int failed=0;
 for(int scenario=0;scenario<4;scenario++) {
  unsigned int depth=(unsigned int)((scenario==0?.25f:scenario==3?.95f:.5f)*16777215.0f);
  for(int i=0;i<32*32;i++) {
   depths[i*4]=depth>>16;depths[i*4+1]=depth>>8;depths[i*4+2]=depth;depths[i*4+3]=scenario==2?0:255;
  }
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,32,32,0,GL_RGBA,GL_UNSIGNED_BYTE,depths);
  uniform4(program,"renderParams",1,0,1,0);
  uniform4(program,"lightMeta",0,0,scenario==3?.5f:1,.025f);
  int value=draw_pixel();
  if(scenario==0) flat=value;
  if(scenario==1) contact=value;
  if(scenario==2) background=value;
  if(scenario==3) far=value;
 }
 failed|=abs(reference-flat)>1 || contact>=reference-15 || abs(reference-background)>1 || abs(reference-far)>1;
 // All three kernels must darken nearby geometry, never an isolated plane.
 for(int i=0;i<32*32;i++) { depths[i*4]=127;depths[i*4+1]=255;depths[i*4+2]=255;depths[i*4+3]=255; }
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,32,32,0,GL_RGBA,GL_UNSIGNED_BYTE,depths);
 uniform4(program,"lightMeta",0,0,1,.025f);
 for(int count=8;count<=32;count*=2) {
  uniform4(program,"cameraForward",0,0,-1,count);
  failed|=draw_pixel()>=reference-10;
 }
 // Infinite reversed-Z perspective and a translated/rotated orthographic
 // camera must reconstruct the same nearby blocker in world distances.
 const float inversePerspective[16]={[0]=1,[5]=1,[11]=10,[14]=-1};
 glUniformMatrix4fv(glGetUniformLocation(program,"occlusionInverseProjection"),1,GL_FALSE,inversePerspective);
 uniform4(program,"testWorldPosition",0,0,-2,0);
 uniform4(program,"cameraPosition",0,0,0,1);
 for(int scenario=0;scenario<2;scenario++) {
  unsigned int depth=(unsigned int)((.1f/(scenario?1.75f:2.0f))*16777215.0f);
  for(int i=0;i<32*32;i++) {depths[i*4]=depth>>16;depths[i*4+1]=depth>>8;depths[i*4+2]=depth;depths[i*4+3]=255;}
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,32,32,0,GL_RGBA,GL_UNSIGNED_BYTE,depths);
  int value=draw_pixel();
  failed|=scenario?value>=reference-10:abs(value-reference)>1;
 }
 const float inverseRotated[16]={[1]=1,[4]=-1,[10]=1,[12]=3,[13]=4,[15]=1};
 glUniformMatrix4fv(glGetUniformLocation(program,"occlusionInverseProjection"),1,GL_FALSE,inverseRotated);
 uniform4(program,"testWorldPosition",3,4,.25f,0);
 uniform4(program,"cameraPosition",3,4,2,1);
 for(int i=0;i<32*32;i++) {depths[i*4]=127;depths[i*4+1]=255;depths[i*4+2]=255;depths[i*4+3]=255;}
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,32,32,0,GL_RGBA,GL_UNSIGNED_BYTE,depths);
 failed|=draw_pixel()>=reference-10;
 glUniformMatrix4fv(glGetUniformLocation(program,"occlusionInverseProjection"),1,GL_FALSE,identity);
 uniform4(program,"testWorldPosition",0,0,.25f,0);
 uniform4(program,"cameraPosition",0,0,2,1);
 uniform4(program,"testMaterial",.8f,1.0f,.5f,32768);
 failed|=abs(draw_pixel()-reference)>1;
 uniform4(program,"testMaterial",.8f,1.0f,.5f,2);
 failed|=abs(draw_pixel()-reference)>1;
 uniform4(program,"testMaterial",.8f,1.0f,.5f,0);
 // Direct light and emission are outside the AO multiplication.
 uniform4(program,"ambientLight",0,0,0,0);
 uniform4(program,"directionalLight",1,1,1,0);
 uniform4(program,"lightDirection",0,0,-1,0);
 uniform4(program,"renderParams",1,0,0,0);
 int direct=draw_pixel();
 uniform4(program,"renderParams",1,0,1,0);
 failed|=abs(draw_pixel()-direct)>1;
 uniform4(program,"directionalLight",0,0,0,0);
 uniform4(program,"testEmission",1,1,1,0);
 uniform4(program,"renderParams",1,0,0,0);
 int emission=draw_pixel();
 failed|=direct<50 || emission<100;
 uniform4(program,"renderParams",1,0,1,0);
 failed|=abs(draw_pixel()-emission)>1;
 printf("SSAO: reference=%d flat=%d contact=%d background=%d range=%d direct=%d emission=%d\n",reference,flat,contact,background,far,direct,emission);
 glDeleteTextures(1,&depthTexture);glDeleteTextures(1,&whiteTexture);
 return failed;
}

static int material_layer_pixels(GLuint program,GLuint texture) {
 int failed=0;
 GLuint baseTexture;glGenTextures(1,&baseTexture);glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,baseTexture);
 const unsigned char white[]={255,255,255,255};glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 glUniform1i(glGetUniformLocation(program,"tex"),0);
 uniform4(program,"testWorldPosition",0,0,0,0);uniform4(program,"testNormal",0,0,1,0);
 uniform4(program,"testTangent",1,0,0,1);uniform4(program,"testSurface",1,1,1,0);
 uniform4(program,"testSurface1",1,1,1,.6f);uniform4(program,"testSurface2",0,0,0,0);
 uniform4(program,"testSurface3",0,1,0,1);uniform4(program,"testMaterial",.8f,1.5f,.5f,4096);
 uniform4(program,"testEmission",.5f,.5f,.5f,0);uniform4(program,"cameraPosition",0,0,1,1);
 uniform4(program,"lightMeta",0,1,0,0);uniform4(program,"directionalLight",0,0,0,0);
 uniform4(program,"ambientLight",0,0,0,0);uniform4(program,"environment",0,0,0,0);
 uniform4(program,"renderParams",1,0,0,0);
 glActiveTexture(GL_TEXTURE1);glBindTexture(GL_TEXTURE_2D,texture);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 const char *names[]={"clearcoatTex","clearcoatRoughnessTex","clearcoatNormalTex","sheenColorTex","sheenRoughnessTex","anisotropyTex","normalTex"};
 for(int i=0;i<7;i++) {
  int unit=i==6 ? 1 : 10+i;
  glActiveTexture(GL_TEXTURE0+unit);glBindTexture(GL_TEXTURE_2D,texture);
  glUniform1i(glGetUniformLocation(program,names[i]),unit);
 }
 glActiveTexture(GL_TEXTURE1);
 int emission=draw_pixel();uniform4(program,"testSurface",1,1,1,1);int coatEmission=draw_pixel();
 float coatRatio=scene_radiance(coatEmission)/scene_radiance(emission);
 if(fabsf(coatRatio-.96f)>.02f) {fprintf(stderr,"clearcoat emission transmission=%f\n",coatRatio);failed=1;}
 unsigned char data[]={0,255,255,255};
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,data);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,4096+65536);int zeroCoat=draw_pixel();
 failed|=abs(zeroCoat-emission)>1;
 // Base normal does not affect the coat. Coat normal scale zero removes XY.
 data[0]=255;data[1]=128;data[2]=128;
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,data);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,4096+4);int baseNormal=draw_pixel();
 uniform4(program,"testMaterial",.8f,1.5f,.5f,4096+262144);int coatNormal=draw_pixel();
 uniform4(program,"testSurface3",0,1,0,0);int zeroScale=draw_pixel();
 failed|=abs(baseNormal-coatEmission)>1 || abs(zeroScale-coatEmission)>1 || coatNormal>=coatEmission-40;
 data[0]=data[1]=128;data[2]=0;
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,data);
 uniform4(program,"testSurface3",0,1,0,1);
 failed|=abs(draw_pixel()-coatEmission)>1;
 // Clearcoat roughness is G, and must multiply the authored factor.
 uniform4(program,"testSurface3",0,1,0,1);uniform4(program,"testEmission",0,0,0,0);
 uniform4(program,"directionalLight",1,1,1,0);uniform4(program,"lightDirection",0,0,-1,0);
 uniform4(program,"testSurface1",1,1,1,.8f);uniform4(program,"testMaterial",.8f,1.5f,.5f,4096+131072);
 int coatTexture=draw_pixel();uniform4(program,"testSurface1",1,1,1,.8f*128/255);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,4096);int coatFactor=draw_pixel();failed|=abs(coatTexture-coatFactor)>1;
 // Charlie sheen with its albedo scaling; color is RGB, roughness is Alpha.
 uniform4(program,"testSurface",1,1,0,0);uniform4(program,"testSurface1",1,1,1,0);
 uniform4(program,"cameraPosition",.9797959f,0,.2f,1);uniform4(program,"lightDirection",-.9797959f,0,-.2f,0);
 int plain=draw_pixel();uniform4(program,"testSurface2",.7f,.5f,.3f,.6f);int sheen=draw_pixel();
 data[0]=data[1]=data[2]=0;data[3]=255;glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,data);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,4096+524288);int zeroSheen=draw_pixel();failed|=abs(zeroSheen-plain)>1 || abs(sheen-plain)<3;
 data[0]=data[1]=data[2]=255;data[3]=128;glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,data);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,4096+1048576);int sheenTexture=draw_pixel();
 uniform4(program,"testSurface2",.7f,.5f,.3f,.6f*128/255);uniform4(program,"testMaterial",.8f,1.5f,.5f,4096);
 int sheenFactor=draw_pixel();failed|=abs(sheenTexture-sheenFactor)>1;
 // Anisotropic tangent rotation and the texture's blue strength channel.
 uniform4(program,"testSurface2",0,0,0,0);uniform4(program,"testSurface",1,1,1,0);
 uniform4(program,"testEmission",0,0,0,1);uniform4(program,"testMaterial",.45f,1.5f,.5f,4096);
 uniform4(program,"cameraPosition",.6f,0,.8f,1);uniform4(program,"lightDirection",0,0,-1,0);
 int isotropic=draw_pixel();uniform4(program,"testSurface3",.9f,1,0,1);int alongX=draw_pixel();
 uniform4(program,"testSurface3",.9f,0,1,1);int rotated=draw_pixel();
 uniform4(program,"testSurface3",.9f,1,0,1);uniform4(program,"testTangent",0,1,0,1);int alongY=draw_pixel();
 failed|=abs(rotated-alongY)>1 || abs(alongX-alongY)<10;
 uniform4(program,"testTangent",1,0,0,1);data[0]=255;data[1]=128;data[2]=0;data[3]=255;
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,data);
 uniform4(program,"testMaterial",.45f,1.5f,.5f,4096+2097152);int zeroAnisotropy=draw_pixel();failed|=abs(zeroAnisotropy-isotropic)>1;
 data[2]=128;glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,data);
 int anisotropyTexture=draw_pixel();uniform4(program,"testSurface3",.9f*128/255,1,0,1);
 uniform4(program,"testMaterial",.45f,1.5f,.5f,4096);int anisotropyFactor=draw_pixel();failed|=abs(anisotropyTexture-anisotropyFactor)>1;
 // Area quadrature must use the same layered BRDF as punctual lights.
 uniform4(program,"cameraPosition",0,0,1,1);
 uniform4(program,"lightMeta",1,1,0,0);uniform4(program,"lightDirections[0]",0,0,-1,6);
 uniform4(program,"lightPosition[0]",0,0,2,0);uniform4(program,"lightRight[0]",1,0,0,1);
 uniform4(program,"lightUp[0]",0,1,0,1);uniform4(program,"lightColors[0]",1,1,1,0);
 uniform4(program,"lightParams[0]",10,10,0,0);uniform4(program,"testSurface3",0,1,0,1);
 uniform4(program,"testEmission",0,0,0,0);uniform4(program,"testMaterial",.8f,1.5f,.5f,4096);
 int plainArea=draw_pixel();uniform4(program,"testSurface",1,1,1,1);uniform4(program,"testSurface1",1,1,1,.2f);int coatArea=draw_pixel();
 failed|=abs(coatArea-plainArea)<2;
 // Both extra layers also participate in image-based light.
 unsigned char atlas[4*8];for(int i=0;i<8;i++){atlas[i*4]=atlas[i*4+1]=atlas[i*4+2]=64;atlas[i*4+3]=255;}
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,8,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
 glUniform1i(glGetUniformLocation(program,"environmentTex"),1);
 uniform4(program,"environment",1,0,0,0);uniform4(program,"lightMeta",0,1,0,0);uniform4(program,"directionalLight",0,0,0,0);
 uniform4(program,"testEmission",0,0,0,0);uniform4(program,"testSurface",1,1,0,0);uniform4(program,"testSurface2",0,0,0,0);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,4096);uniform4(program,"renderParams",.1f,0,0,0);
 int plainIBL=draw_pixel();uniform4(program,"testSurface",1,1,0,1);int coatIBL=draw_pixel();
 uniform4(program,"testSurface",1,1,0,0);uniform4(program,"testSurface2",1,1,1,.8f);int sheenIBL=draw_pixel();
 // A white furnace preserves energy when the sheen covers a white diffuse
 // base; its indirect lobe replaces the energy removed by albedo scaling.
 failed|=coatIBL<=plainIBL || abs(sheenIBL-plainIBL)>1;
 printf("material layers: emission=%d/%d R=%d normals=%d/%d/%d G=%d/%d; sheen=%d/%d RGB=%d A=%d/%d; anisotropy=%d X=%d Y=%d/%d B=%d/%d/%d; area=%d/%d IBL=%d/%d/%d %s\n",emission,coatEmission,zeroCoat,baseNormal,coatNormal,zeroScale,coatTexture,coatFactor,plain,sheen,zeroSheen,sheenTexture,sheenFactor,isotropic,alongX,alongY,rotated,zeroAnisotropy,anisotropyTexture,anisotropyFactor,plainArea,coatArea,plainIBL,coatIBL,sheenIBL,failed?"FAILED":"passed");
 glDeleteTextures(1,&baseTexture);
 return failed;
}

static void encode_radiance(unsigned char *out,float r,float g,float b) {
 float exponent=ceilf(log2f(fmaxf(fmaxf(r,g),fmaxf(b,1e-30f))));
 out[0]=(unsigned char)roundf(r*exp2f(-exponent)*255);
 out[1]=(unsigned char)roundf(g*exp2f(-exponent)*255);
 out[2]=(unsigned char)roundf(b*exp2f(-exponent)*255);
 out[3]=(unsigned char)(exponent+128);
}

static int fresnel_pixels(GLuint program) {
 const float pi=3.14159265359f;
 int failed=0;
 GLuint textures[2];glGenTextures(2,textures);
 for(int i=0;i<2;i++) {
  glActiveTexture(GL_TEXTURE0+i);glBindTexture(GL_TEXTURE_2D,textures[i]);
  glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);
  glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 }
 unsigned char color[4]={0,0,0,255};
 glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,textures[0]);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,color);
 glUniform1i(glGetUniformLocation(program,"tex"),0);
 uniform4(program,"testWorldPosition",0,0,0,0);uniform4(program,"testNormal",0,0,1,0);
 uniform4(program,"testSurface",1,1,1,0);uniform4(program,"testSurface1",1,1,1,0);
 uniform4(program,"testSurface2",0,0,0,0);uniform4(program,"testSurface3",0,1,0,1);
 uniform4(program,"testEmission",0,0,0,0);uniform4(program,"cameraPosition",0,0,1,1);
 uniform4(program,"lightMeta",0,1,0,0);uniform4(program,"lightDirection",0,0,-1,0);
 uniform4(program,"ambientLight",0,0,0,0);uniform4(program,"environment",0,0,0,0);
 uniform4(program,"directionalLight",4*pi,4*pi,4*pi,0);uniform4(program,"renderParams",1,0,0,0);
 const float iors[]={1,1.33f,1.5f,2.42f,32,FLT_MAX,0};
 int values[7];
 // At normal incidence and roughness=1, GGX D*V=1/(4*pi).
 // A black base removes diffuse, so 4*pi lux produces exactly dielectric F0.
 for(int i=0;i<7;i++) {
  uniform4(program,"testMaterial",1,iors[i],.5f,4096);
  values[i]=draw_pixel();
  float ratio=iors[i]==0?1:1-2/(iors[i]+1),expected=ratio*ratio;
  float actual=scene_radiance(values[i]);
  if(fabsf(actual-expected)>.018f) {fprintf(stderr,"IOR %g: direct=%g expected=%g\n",iors[i],actual,expected);failed=1;}
 }
 // Clamp tinted F0 before applying the specular factor.
 uniform4(program,"testMaterial",1,4,.5f,4096);uniform4(program,"testSurface",1,1,.25f,0);
 uniform4(program,"testSurface1",4,4,4,0);
 int tinted=draw_pixel();failed|=fabsf(scene_radiance(tinted)-.25f)>.01f;
 // Split-sum IBL uses the same IOR; choose an atlas with LUT A=1, B=0.
 unsigned char atlas[8*4];for(int i=0;i<8;i++){atlas[i*4]=atlas[i*4+1]=atlas[i*4+2]=255;atlas[i*4+3]=16;}
 atlas[28]=255;atlas[29]=atlas[30]=0;atlas[31]=255;
 glActiveTexture(GL_TEXTURE1);glBindTexture(GL_TEXTURE_2D,textures[1]);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,8,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
 glUniform1i(glGetUniformLocation(program,"environmentTex"),1);
 uniform4(program,"directionalLight",0,0,0,0);uniform4(program,"environment",255.0f/256,0,.5f,.5f);
 uniform4(program,"testSurface",1,1,1,0);uniform4(program,"testSurface1",1,1,1,0);
 for(int i=0;i<7;i++) {
  uniform4(program,"testMaterial",1,iors[i],.5f,4096);
  failed|=abs(draw_pixel()-values[i])>1;
 }
 // Fractional metalness must attenuate only the dielectric diffuse lobe.
 color[0]=color[1]=color[2]=255;glActiveTexture(GL_TEXTURE0);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,color);
 // A uniform unit-radiance environment must preserve energy even when
 // the BRDF LUT records substantial single-scattering loss.
 atlas[28]=64;atlas[29]=13;
 glActiveTexture(GL_TEXTURE1);glBindTexture(GL_TEXTURE_2D,textures[1]);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,8,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
 for(int i=0;i<3;i++) {
  uniform4(program,"testEmission",0,0,0,i*.5f);
  uniform4(program,"testMaterial",.2f+i*.4f,1.5f,.5f,4096);
  if(fabsf(scene_radiance(draw_pixel())-1)>.02f) {fprintf(stderr,"IBL white furnace lost energy at metalness %g\n",i*.5f);failed=1;}
 }
 // A darker metal keeps its authored absorption while recovering the
 // analytic multiple-scattering lobe; compensation is not a brightness gain.
 color[0]=color[1]=color[2]=128;
 glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,textures[0]);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,color);
 float base=128/255.0f,a=64/255.0f,b=13/255.0f,missing=1-a-b;
 float single=base*a+b,average=base+(1-base)/21;
 float expected_metal=single+single*average*missing/(1-average*missing);
 failed|=fabsf(scene_radiance(draw_pixel())-expected_metal)>.01f;
 color[0]=color[1]=color[2]=255;
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,color);
 printf("IBL white-furnace energy and absorbing-metal multi-scattering checked\n");
 uniform4(program,"environment",0,0,0,0);uniform4(program,"testSurface",1,1,0,0);
 uniform4(program,"testEmission",0,0,0,.5f);uniform4(program,"testMaterial",1,1.5f,.5f,4096);
 uniform4(program,"ambientLight",1,1,1,0);
 int ambient=draw_pixel();failed|=fabsf(scene_radiance(ambient)-.5f)>.01f;
 uniform4(program,"ambientLight",0,0,0,0);uniform4(program,"directionalLight",4*pi,4*pi,4*pi,0);
 int mixed=draw_pixel();failed|=fabsf(scene_radiance(mixed)-2.5f)>.06f;
 // Full metal is independent of dielectric IOR and specular controls.
 uniform4(program,"testEmission",0,0,0,1);int metal=draw_pixel();
 uniform4(program,"testMaterial",1,32,.5f,4096);uniform4(program,"testSurface",1,1,1,0);
 failed|=abs(draw_pixel()-metal)>1;
 printf("IOR direct/IBL: air=%d water=%d default=%d diamond=%d large=%d max=%d infinite=%d; tint=%d fractional=%d/%d %s\n",values[0],values[1],values[2],values[3],values[4],values[5],values[6],tinted,ambient,mixed,failed?"FAILED":"passed");
 glDeleteTextures(2,textures);
 return failed;
}

static int ssr_pixels(GLuint program) {
 int failed=0;
 const float projection[16]={[0]=1,[5]=1,[11]=-1,[14]=.1f};
 const float inverse[16]={[0]=1,[5]=1,[11]=10,[14]=-1};
 const unsigned char white[4]={255,255,255,255};
 GLuint textures[3];glGenTextures(3,textures);
 for(int i=0;i<3;i++) {
  glActiveTexture(GL_TEXTURE0+i*2);glBindTexture(GL_TEXTURE_2D,textures[i]);
  glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);
  glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 }
 glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,textures[0]);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
 glUniform1i(glGetUniformLocation(program,"tex"),0);
 glUniform1i(glGetUniformLocation(program,"screenDepthTex"),2);
 glUniform1i(glGetUniformLocation(program,"shadowTex"),4);
 uniform4(program,"testNormal",.8f,0,.6f,0);
 uniform4(program,"testWorldPosition",0,0,-2,0);
 uniform4(program,"cameraPosition",0,0,0,1);
 uniform4(program,"cameraForward",0,0,-1,0);
 uniform4(program,"testSurface",1,1,1,0);
 uniform4(program,"testSurface1",1,1,1,.045f);
 uniform4(program,"testSurface2",0,0,0,0);
 uniform4(program,"testSurface3",0,1,0,1);
 uniform4(program,"testEmission",0,0,0,1);
 uniform4(program,"testMaterial",.045f,1.5f,.5f,0);
 uniform4(program,"environment",0,0,0,0);
 uniform4(program,"ambientLight",0,0,0,0);
 uniform4(program,"lightMeta",0,0,0,0);
 uniform4(program,"directionalLight",0,0,0,0);
 uniform4(program,"renderParams",1,4,0,0);
 glUniformMatrix4fv(glGetUniformLocation(program,"shadowMatrices[0]"),1,GL_FALSE,projection);
 glUniformMatrix4fv(glGetUniformLocation(program,"occlusionInverseProjection"),1,GL_FALSE,inverse);
 uniform4(program,"shadowParams[0]",4,.1f,1,.6f);
 uniform4(program,"shadowRects[0]",64,.1f,0,0);
 unsigned char depth[64*64*4],color[128*64*4];
 int reference=0,hit=0,miss=0,foreground=0,coat=0,ao=0;
 for(int scenario=0;scenario<3;scenario++) {
  for(int y=0;y<64;y++) for(int x=0;x<64;x++) {
   int i=(y*64+x)*4,target=x>=37;
   unsigned int z=(unsigned int)((.1f/(target?(scenario==2?1.0f:2.2f):2.0f))*16777215.0f);
   depth[i]=z>>16;depth[i+1]=z>>8;depth[i+2]=z;depth[i+3]=(scenario==1&&target)?0:255;
   encode_radiance(color+i,target?4:.05f,target?0:.05f,target?0:.05f);
  }
  glActiveTexture(GL_TEXTURE2);glBindTexture(GL_TEXTURE_2D,textures[1]);
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,64,64,0,GL_RGBA,GL_UNSIGNED_BYTE,depth);
  glActiveTexture(GL_TEXTURE4);glBindTexture(GL_TEXTURE_2D,textures[2]);
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,64,64,0,GL_RGBA,GL_UNSIGNED_BYTE,color);
  if(scenario==0) {
   uniform4(program,"shadowParams[0]",4,.1f,0,.6f);reference=draw_pixel();
   uniform4(program,"shadowParams[0]",4,.1f,1,.6f);hit=draw_pixel();
   for(int steps=32;steps<=128;steps*=2) {
    uniform4(program,"shadowRects[0]",steps,.1f,0,0);
    if(abs(draw_pixel()-hit)>2) {fprintf(stderr,"SSR steps %d changed hit\n",steps);failed=1;}
   }
   uniform4(program,"testMaterial",.045f,1.5f,.5f,4194304);
   failed|=abs(draw_pixel()-reference)>1;
   uniform4(program,"testMaterial",.8f,1.5f,.5f,0);
   failed|=abs(draw_pixel()-reference)>1;
   uniform4(program,"testMaterial",.045f,1.5f,.5f,0);
   uniform4(program,"testNormal",-.8f,0,.6f,0);
   failed|=abs(draw_pixel()-reference)>1;
   uniform4(program,"testNormal",.8f,0,.6f,0);
   uniform4(program,"shadowParams[0]",.3f,.1f,1,.6f);
   failed|=abs(draw_pixel()-reference)>1;
   uniform4(program,"shadowParams[0]",4,.1f,1,.6f);
   uniform4(program,"testEmission",0,0,0,0);
   // Dielectric SSR must use the authored IOR, while capture radiance is
   // independent of the receiver's material. Infinite IOR matches white metal.
   const float iors[]={1,1.33f,1.5f,2.42f,0};
   int dielectric[5];
   for(int i=0;i<5;i++) {
    uniform4(program,"testMaterial",.045f,iors[i],.5f,0);
    dielectric[i]=draw_pixel();
   }
   failed|=dielectric[0]>=dielectric[1] || dielectric[1]>=dielectric[2] || dielectric[2]>=dielectric[3] || dielectric[3]>=dielectric[4] || abs(dielectric[4]-hit)>1;
   printf("SSR IOR: air=%d water=%d default=%d diamond=%d infinite=%d\n",dielectric[0],dielectric[1],dielectric[2],dielectric[3],dielectric[4]);
   uniform4(program,"testMaterial",.045f,1.5f,.5f,0);
   uniform4(program,"testSurface",1,1,0,1);coat=draw_pixel();
   failed|=coat<=reference+10;
   uniform4(program,"testEmission",0,0,0,1);
   uniform4(program,"testSurface",1,1,1,0);
   // Preserve per-pixel AO in the right tile without filtering RGBE bytes.
   for(int y=63;y>=0;y--) for(int x=0;x<64;x++) {
    int from=(y*64+x)*4,to=(y*128+x)*4;
    for(int c=0;c<4;c++)color[to+c]=color[from+c];
   }
   for(int y=0;y<64;y++)for(int x=64;x<128;x++) {
    int i=(y*128+x)*4;color[i]=0;color[i+1]=0;color[i+2]=0;color[i+3]=255;
   }
   glActiveTexture(GL_TEXTURE4);glBindTexture(GL_TEXTURE_2D,textures[2]);
   glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,128,64,0,GL_RGBA,GL_UNSIGNED_BYTE,color);
   uniform4(program,"shadowRects[0]",64,.1f,1,0);ao=draw_pixel();
   failed|=abs(ao-reference)>1;
   uniform4(program,"shadowRects[0]",64,.1f,0,0);
  } else if(scenario==1) {miss=draw_pixel();} else {foreground=draw_pixel();}
 }
 failed|=hit<220 || reference>80 || abs(miss-reference)>1 || abs(foreground-reference)>1;
 // Replace the captured IBL lobe instead of adding a second environment.
 GLuint environment;glGenTextures(1,&environment);
 unsigned char atlas[8*4];
 for(int row=0;row<8;row++) {atlas[row*4]=atlas[row*4+1]=atlas[row*4+2]=255;atlas[row*4+3]=32;}
 atlas[28]=230;atlas[29]=4;atlas[30]=0;atlas[31]=255;
 glActiveTexture(GL_TEXTURE6);glBindTexture(GL_TEXTURE_2D,environment);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,8,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
 glUniform1i(glGetUniformLocation(program,"environmentTex"),6);
 uniform4(program,"environment",3,0,1,1);
 float environment_lobe=(16*32.0f/255)*3*(234.0f/255);
 int replacement[2];
 for(int scenario=0;scenario<2;scenario++) {
  float ambient_occlusion=scenario?64.0f/255:1;
  for(int y=0;y<64;y++)for(int x=0;x<64;x++) {
   int i=(y*64+x)*4,target=x>=37;
   unsigned int z=(unsigned int)((.1f/(target?2.2f:2.0f))*16777215.0f);
   depth[i]=z>>16;depth[i+1]=z>>8;depth[i+2]=z;depth[i+3]=255;
   float base=.05f+environment_lobe*ambient_occlusion;
   encode_radiance(color+(y*(scenario?128:64)+x)*4,target?4:base,target?0:base,target?0:base);
   if(scenario) {int j=(y*128+x+64)*4;color[j]=64;color[j+1]=color[j+2]=0;color[j+3]=255;}
  }
  glActiveTexture(GL_TEXTURE2);glBindTexture(GL_TEXTURE_2D,textures[1]);
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,64,64,0,GL_RGBA,GL_UNSIGNED_BYTE,depth);
  glActiveTexture(GL_TEXTURE4);glBindTexture(GL_TEXTURE_2D,textures[2]);
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,scenario?128:64,64,0,GL_RGBA,GL_UNSIGNED_BYTE,color);
  uniform4(program,"shadowRects[0]",64,.1f,scenario,0);
  replacement[scenario]=draw_pixel();
  unsigned char result[4];glReadPixels(0,0,1,1,GL_RGBA,GL_UNSIGNED_BYTE,result);
  if(result[1]>66 || result[1]<58 || (scenario?(replacement[1]<180 || replacement[1]>191):abs(replacement[0]-hit)>2)) {fprintf(stderr,"SSR IBL replacement AO=%g pixel=%u,%u,%u\n",ambient_occlusion,result[0],result[1],result[2]);failed=1;}
 }
 uniform4(program,"environment",0,0,0,0);
 glDeleteTextures(1,&environment);
 // The capture stage preserves high radiance independently of exposure.
 uniform4(program,"testNormal",0,0,1,0);
 uniform4(program,"testEmission",64,.5f,0,0);
 uniform4(program,"renderParams",.001f,3,0,0);
 uniform4(program,"testSurface",1,1,0,0);
 int encoded=draw_pixel();
 unsigned char rgba[4];glReadPixels(0,0,1,1,GL_RGBA,GL_UNSIGNED_BYTE,rgba);
 failed|=encoded!=255 || pixel_alpha!=134 || rgba[1]!=2;
	uniform4(program,"testMaterial",.8f,-1,.5f,0);
 draw_pixel();failed|=pixel_alpha!=0;
 printf("SSR: base=%d hit=%d miss=%d foreground=%d coat=%d AO=%d IBL=%d/%d; RGBE=%d/%d %s\n",reference,hit,miss,foreground,coat,ao,replacement[0],replacement[1],encoded,rgba[3],failed?"FAILED":"passed");
 glDeleteTextures(3,textures);
 return failed;
}



static int display_byte(double radiance,int curve) {
 double mapped=curve?fmin(1.0,fmax(0.0,((radiance/0.6)*(radiance/0.6+0.0245786)-0.000090537)/((radiance/0.6)*(0.983729*radiance/0.6+0.4329510)+0.238081))):radiance/(radiance+1);
 double srgb=mapped<=.0031308?mapped*12.92:1.055*pow(mapped,1.0/2.4)-.055;
 return (int)lround(srgb*255);
}
static void camera_fog(GLuint program,float red,float green,float blue,float start,float end,float density,float mode,float curve) {
 const float values[]={red,green,blue,start,end,density,mode,curve};
 for(int i=0;i<8;i++) {char name[48];snprintf(name,sizeof(name),"lightShadows[%d]",i);uniform4(program,name,0,0,0,values[i]);}
}
static int panorama_pixels(GLuint program,GLuint original) {
 GLuint panorama;glGenTextures(1,&panorama);glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,panorama);
 unsigned char pixels[64*32*4];
 for(int y=0;y<32;y++) for(int x=0;x<64;x++) {
  int i=(y*64+x)*4;pixels[i]=x%2?255:0;pixels[i+1]=0;pixels[i+2]=x%2?0:255;pixels[i+3]=128;
 }
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,64,32,0,GL_RGBA,GL_UNSIGNED_BYTE,pixels);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_LINEAR);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_LINEAR);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,2048+512+8388608);
 uniform4(program,"renderParams",1,0,0,0);
 float angle=-3.14159265359f/64;
 uniform4(program,"testSurface3",cosf(angle),0,sinf(angle),1);
 int failed=0;
 for(int test=0;test<3;test++) {
  uniform4(program,"environment",1,test==1?3.14159265359f/32:0,0,0);
  if(test==2) uniform4(program,"testSurface3",-1,0,0,1);
  pixel(program,0,-1);unsigned char rgba[4];glReadPixels(0,0,1,1,GL_RGBA,GL_UNSIGNED_BYTE,rgba);
  int red=test==0?188:test==1?0:156,blue=test==0?0:test==1?188:156;
  if(abs(rgba[0]-red)>1 || rgba[1]!=0 || abs(rgba[2]-blue)>1 || rgba[3]!=255) {fprintf(stderr,"panorama detail/rotation/seam case %d: %u/%u/%u/%u\n",test,rgba[0],rgba[1],rgba[2],rgba[3]);failed=1;}
 }
 glDeleteTextures(1,&panorama);glBindTexture(GL_TEXTURE_2D,original);
 printf("Original panorama detail, independent sky texture, rotation and longitude seam %s\n",failed?"FAILED":"passed");
 return failed;
}
static int camera_color_pixels(GLuint program) {
 GLuint texture;glGenTextures(1,&texture);glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,texture);
 const unsigned char white[]={255,255,255,255},half[]={255,255,255,128};
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_LINEAR);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_LINEAR);
 glUniform1i(glGetUniformLocation(program,"tex"),0);
 glViewport(0,0,1,1);glDisable(GL_BLEND);
 uniform4(program,"ambientLight",0,0,0,0);uniform4(program,"directionalLight",0,0,0,0);uniform4(program,"environment",0,0,0,0);uniform4(program,"lightMeta",0,0,0,0);
 uniform4(program,"testMaterial",.8f,1.5f,.5f,512+4096);uniform4(program,"testNormal",0,0,1,0);uniform4(program,"testSurface",1,1,0,0);uniform4(program,"testSurface1",1,1,1,0);uniform4(program,"testSurface2",0,0,0,0);uniform4(program,"testSurface3",0,1,0,1);
 uniform4(program,"testWorldPosition",0,0,-4,0);uniform4(program,"cameraPosition",0,0,0,1);uniform4(program,"cameraForward",0,0,-1,0);
 const float radiance[]={0,.001f,.18f,1,4,100,FLT_MAX};
 int failed=0;
 for(int curve=0;curve<2;curve++) for(int i=0;i<7;i++) {
  camera_fog(program,0,0,0,0,0,0,0,(float)curve);uniform4(program,"renderParams",1,0,0,0);uniform4(program,"testEmission",radiance[i],radiance[i],radiance[i],0);
  int value=draw_pixel(),expected=display_byte(radiance[i],curve);
  if(abs(value-expected)>1) {fprintf(stderr,"tone curve %d radiance %g: %d expected %d\n",curve,radiance[i],value,expected);failed=1;}
 }
 // Saturated colors exercise the ACES input/output matrices, not just the
 // neutral-axis curve (which cannot detect a transposed color transform).
 const float colors[][3]={{1,0,0},{0,1,0},{0,0,1},{.03f,.2f,3},{4,1,.05f}};
 for(int i=0;i<5;i++) {
  unsigned char expected[3],actual[4];aces_display_reference(colors[i],expected);
  camera_fog(program,0,0,0,0,0,0,0,1);
  uniform4(program,"testEmission",colors[i][0],colors[i][1],colors[i][2],0);
  draw_pixel();glReadPixels(0,0,1,1,GL_RGBA,GL_UNSIGNED_BYTE,actual);
  for(int c=0;c<3;c++) { if(abs(actual[c]-expected[c])>1) {fprintf(stderr,"ACES color %d channel %d: %d expected %d\n",i,c,actual[c],expected[c]);failed=1;} }
 }
 // Linear fog mixes radiance, including premultiplied transparent surfaces.
 uniform4(program,"testEmission",0,0,1,0);
 const float distances[]={1,2,4,6,10};
 for(int mode=1;mode<3;mode++) for(int i=0;i<5;i++) {
  camera_fog(program,1,0,0,2,6,.25f,(float)mode,0);uniform4(program,"testWorldPosition",0,0,-distances[i],0);
  double delta=fmax(distances[i]-2,0),factor=mode==1?fmin(delta/4,1):1-exp(-delta*delta*.0625);
  int value=draw_pixel(),expected=display_byte(factor,0);
  if(abs(value-expected)>1) {fprintf(stderr,"fog mode %d distance %g: %d expected %d\n",mode,distances[i],value,expected);failed=1;}
 }
 camera_fog(program,1,0,0,2,6,.25f,1,0);uniform4(program,"testWorldPosition",99,0,-4,0);uniform4(program,"cameraPosition",0,0,0,-1);
 failed|=abs(draw_pixel()-display_byte(.5,0))>1;
 // Fully fogged unlit geometry meets lit geometry's exposed fog color.
 uniform4(program,"cameraPosition",0,0,0,1);uniform4(program,"testWorldPosition",0,0,-10,0);uniform4(program,"testMaterial",.8f,1.5f,.5f,512+256);
 failed|=abs(draw_pixel()-188)>1;
 uniform4(program,"testMaterial",.8f,1.5f,.5f,512+4096+2);glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,half);
 failed|=abs(draw_pixel()-94)>1 || abs(pixel_alpha-128)>1;
 // RGBE capture and depth keep the pre-display scene; FXAA also bypasses fog.
 uniform4(program,"renderParams",999,3,0,0);failed|=draw_pixel()!=0 || pixel_alpha!=128;
 uniform4(program,"renderParams",999,7,0,0);failed|=draw_pixel()!=255 || pixel_alpha!=128;
 camera_fog(program,0,0,0,0,0,0,0,0);
 printf("Reinhard/ACES fit, huge HDR values, linear/exp2 fog, orthographic distance, unlit/transparent fog and capture/FXAA isolation %s\n",failed?"FAILED":"passed");
 glDeleteTextures(1,&texture);
 return failed;
}

static int fxaa_pixels(GLuint program) {
 const int size=16;
 GLuint texture,target,fbo;
 glGenTextures(1,&texture);glGenTextures(1,&target);glGenFramebuffers(1,&fbo);
 glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,target);
 glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,size,size,0,GL_RGBA,GL_UNSIGNED_BYTE,NULL);
 glBindFramebuffer(GL_FRAMEBUFFER,fbo);glFramebufferTexture2D(GL_FRAMEBUFFER,GL_COLOR_ATTACHMENT0,GL_TEXTURE_2D,target,0);
 if(glCheckFramebufferStatus(GL_FRAMEBUFFER)!=GL_FRAMEBUFFER_COMPLETE) return 1;
 glBindTexture(GL_TEXTURE_2D,texture);
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_LINEAR);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_LINEAR);
 // Intentional repeat verifies that the shader clamps every edge fetch.
 glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_WRAP_S,GL_REPEAT);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_WRAP_T,GL_REPEAT);
 glUniform1i(glGetUniformLocation(program,"tex"),0);
 glUniform1f(glGetUniformLocation(program,"testUVMode"),1);
 uniform4(program,"renderParams",999,7,0,0);
 glDisable(GL_BLEND);glViewport(0,0,size,size);
 int failed=0,smoothed=0,alpha_smoothed=0;
 unsigned char data[size*size*4],output[size*size*4];
 for(int test=0;test<4;test++) {
  for(int y=0;y<size;y++) for(int x=0;x<size;x++) {
   int index=(y*size+x)*4,covered=x>y;
   data[index]=test==0?73:test==1?(covered?255:0):test==2?0:(y<size/2?255:0);
   data[index+1]=test==0?111:test==3?(y<size/2?0:255):data[index];
   data[index+2]=test==0?149:test==3?0:data[index];
   data[index+3]=test==2?(covered?255:0):test==3?128:255;
  }
  glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,size,size,0,GL_RGBA,GL_UNSIGNED_BYTE,data);
  glDrawArrays(GL_TRIANGLES,0,3);glReadPixels(0,0,size,size,GL_RGBA,GL_UNSIGNED_BYTE,output);
  if(test==0) for(int i=0;i<size*size*4;i++) failed|=abs(output[i]-data[i])>1;
  if(test==1 || test==2) for(int i=0;i<size*size;i++) {
   int value=output[i*4+(test==1?0:3)];
   if(value>5 && value<250) { if(test==1) smoothed++; else alpha_smoothed++; }
   if(test==2) failed|=output[i*4]!=0 || output[i*4+1]!=0 || output[i*4+2]!=0;
  }
  if(test==3) failed|=output[0]!=255 || output[1]!=0 || output[(size*size-1)*4]!=0 || output[(size*size-1)*4+1]!=255 || abs(output[3]-128)>1;
 }
 failed|=smoothed==0 || alpha_smoothed==0 || glGetError()!=GL_NO_ERROR;
 printf("FXAA flat color, diagonal edge (%d pixels), black transparent edge (%d pixels), orientation and clamped boundaries %s\n",smoothed,alpha_smoothed,failed?"FAILED":"passed");
 glUniform1f(glGetUniformLocation(program,"testUVMode"),0);
 glBindFramebuffer(GL_FRAMEBUFFER,0);glDeleteFramebuffers(1,&fbo);glDeleteTextures(1,&texture);glDeleteTextures(1,&target);
 return failed;
}

static const char *mesh_test_vertex = "#version 300 es\n"
		"out vec3 _vs2fs_location0; out vec3 _vs2fs_location1; out vec2 _vs2fs_location2;\n"
		"out vec4 _vs2fs_location3; out vec4 _vs2fs_location4; flat out vec4 _vs2fs_location5;\n"
		"flat out vec4 _vs2fs_location6; flat out vec4 _vs2fs_location7; flat out vec4 _vs2fs_location8; flat out vec4 _vs2fs_location9; out vec4 _vs2fs_location10;\n"
		"uniform float testUVMode; uniform vec4 testMaterial; uniform vec4 testSurface; uniform vec4 testWorldPosition; uniform vec4 testShadowPlane; uniform vec4 testNormal; uniform vec4 testEmission; uniform vec4 testSurface1; uniform vec4 testSurface2; uniform vec4 testSurface3; uniform vec4 testTangent;\n"
		"void main(){ vec2 p=vec2(float((gl_VertexID<<1)&2),float(gl_VertexID&2));\n"
		"gl_Position=vec4(p*2.0-1.0,0,1); _vs2fs_location0=testWorldPosition.xyz;\n"
		"vec2 plane=(p*2.0-1.0+testShadowPlane.z)*testWorldPosition.w; _vs2fs_location0+=vec3(plane,dot(plane,testShadowPlane.xy));\n"
		"_vs2fs_location1=testNormal.xyz; _vs2fs_location2=mix(vec2(0.5),p,testUVMode);\n"
		"_vs2fs_location3=vec4(1); _vs2fs_location4=testEmission;\n"
		"_vs2fs_location5=testMaterial; _vs2fs_location6=testSurface; _vs2fs_location7=testSurface1; _vs2fs_location8=testSurface2; _vs2fs_location9=testSurface3; _vs2fs_location10=testTangent; }\n";

int main(int argc, char **argv) {
	if (argc != 2) return 1;
	EGLDisplay display = eglGetDisplay(EGL_DEFAULT_DISPLAY);
	if (!eglInitialize(display, NULL, NULL)) {
		fprintf(stderr, "no offscreen EGL display\n");
		return 77;
	}
	eglBindAPI(EGL_OPENGL_ES_API);
	EGLint attributes[] = {EGL_SURFACE_TYPE, EGL_PBUFFER_BIT, EGL_RENDERABLE_TYPE, EGL_OPENGL_ES3_BIT,
		EGL_RED_SIZE, 8, EGL_GREEN_SIZE, 8, EGL_BLUE_SIZE, 8, EGL_ALPHA_SIZE, 8, EGL_NONE};
	EGLConfig config;
	EGLint count;
	if (!eglChooseConfig(display, attributes, &config, 1, &count) || count == 0) return 77;
	EGLint surface_attributes[] = {EGL_WIDTH, 4, EGL_HEIGHT, 4, EGL_NONE};
	EGLint context_attributes[] = {EGL_CONTEXT_CLIENT_VERSION, 3, EGL_NONE};
	EGLSurface surface = eglCreatePbufferSurface(display, config, surface_attributes);
	EGLContext context = eglCreateContext(display, config, EGL_NO_CONTEXT, context_attributes);
	if (!eglMakeCurrent(display, surface, surface, context)) return 77;

	FILE *file = fopen(argv[1], "rb");
	if (file == NULL) return 1;
	fseek(file, 0, SEEK_END);
	long length = ftell(file);
	rewind(file);
	char *fragment = calloc((size_t)length + 1, 1);
	if (fragment == NULL || fread(fragment, 1, (size_t)length, file) != (size_t)length) return 1;
	fclose(file);
	const char *vertex = mesh_test_vertex;
	GLuint program = glCreateProgram();
	GLuint vs = compile_shader(GL_VERTEX_SHADER, vertex), fs = compile_shader(GL_FRAGMENT_SHADER, fragment);
	free(fragment);
	glAttachShader(program, vs);
	glAttachShader(program, fs);
	glLinkProgram(program);
	GLint linked;
	glGetProgramiv(program, GL_LINK_STATUS, &linked);
	if (!linked) {
		char log[4096];
		glGetProgramInfoLog(program, sizeof(log), NULL, log);
		fprintf(stderr, "link shader: %s\n", log);
		return 1;
	}
	glUseProgram(program);
	GLuint texture, vao;
	glGenVertexArrays(1, &vao);
	glBindVertexArray(vao);
	glGenTextures(1, &texture);
	glBindTexture(GL_TEXTURE_2D, texture);
	unsigned char white[] = {255, 255, 255, 255};
	glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, 1, 1, 0, GL_RGBA, GL_UNSIGNED_BYTE, white);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST);
	uniform4(program,"testMaterial",0.8,1.5f,0.5,0);
	uniform4(program,"testSurface",1,1,1,0);
	uniform4(program,"testSurface1",1,1,1,0);
	uniform4(program,"testSurface3",0,1,0,1);
	uniform4(program,"testNormal",0,1,0,0);
	uniform4(program, "cameraPosition", 0, 2, 2, 1);
	uniform4(program, "lightDirection", 0, -1, 0, 0);
	uniform4(program, "directionalLight", 1, 1, 1, 1);
	uniform4(program, "lightColors[0]", 1, 1, 1, 1);
	glViewport(0, 0, 1, 1);
	int single = pixel(program, 0, -1);
	int front = pixel(program, 1, -1);
	int back = pixel(program, 1, 1);
	printf("directional light: single=%d, array=%d, away=%d\n", single, front, back);
	int failed = single < 100 || abs(front - single) > 1 || back > 1;
	// A sampled shadow blocks the directional light while leaving ambient light.
	GLuint dataTexture;
	glGenTextures(1,&dataTexture);glActiveTexture(GL_TEXTURE1);glBindTexture(GL_TEXTURE_2D,dataTexture);
	glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,white);
	glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MIN_FILTER,GL_NEAREST);glTexParameteri(GL_TEXTURE_2D,GL_TEXTURE_MAG_FILTER,GL_NEAREST);
	glUniform1i(glGetUniformLocation(program,"shadowTex"),1);
	float identity[16]={1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1};
	glUniformMatrix4fv(glGetUniformLocation(program,"shadowMatrices[0]"),1,GL_FALSE,identity);
	uniform4(program,"shadowRects[0]",0,0,1,1);
	uniform4(program,"shadowParams[0]",0.0003,0,0,0);
	uniform4(program,"lightShadows[0]",1,1,0,0);
	int shadowed=pixel(program,1,-1);
	failed |= shadowed>1;
	failed |= shadow_pixels(program,dataTexture);
	failed |= shadow_atlas_pixels(program,dataTexture);
	uniform4(program,"lightShadows[0]",0,0,0,0);
	// sRGB texture sampling must happen before lighting; unlit output roundtrips.
	unsigned char gray[4]={64,64,64,255};
	glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,texture);
	glTexImage2D(GL_TEXTURE_2D,0,GL_SRGB8_ALPHA8,1,1,0,GL_RGBA,GL_UNSIGNED_BYTE,gray);
	uniform4(program,"directionalLight",0,0,0,1);uniform4(program,"ambientLight",1,1,1,1);
	uniform4(program,"testSurface",1,1,0,0);
	int grayLit=pixel(program,0,-1);
	uniform4(program,"testMaterial",0.8,1.5f,0.5,256);
	int grayUnlit=pixel(program,0,-1);
	failed |= grayLit<59 || grayLit>64 || abs(grayUnlit-64)>1;
	// Occlusion affects indirect light, not emission or direct lighting.
	unsigned char black[4]={0,0,0,255};
	glActiveTexture(GL_TEXTURE1);glBindTexture(GL_TEXTURE_2D,dataTexture);
	glTexSubImage2D(GL_TEXTURE_2D,0,0,0,1,1,GL_RGBA,GL_UNSIGNED_BYTE,black);
	glUniform1i(glGetUniformLocation(program,"occlusionTex"),1);
	uniform4(program,"testMaterial",0.8,1.5f,0.5,16);
	int occluded=pixel(program,0,-1);
	failed |= occluded>1;
	printf("surface pixels: shadow=%d, sRGB lit=%d, unlit=%d, AO=%d\n",shadowed,grayLit,grayUnlit,occluded);
	// Only blended materials keep fractional alpha. Mask coverage stays opaque.
	gray[3]=64;
	glActiveTexture(GL_TEXTURE0);glBindTexture(GL_TEXTURE_2D,texture);
	glTexSubImage2D(GL_TEXTURE_2D,0,0,0,1,1,GL_RGBA,GL_UNSIGNED_BYTE,gray);
	uniform4(program,"testMaterial",0.8,1.5f,0.5,256);
	failed |= abs(pixel(program,0,-1)-64)>1 || pixel_alpha!=255;
	uniform4(program,"testMaterial",0.8,1.5f,0.5,257);
	failed |= pixel(program,0,-1)!=0;
	uniform4(program,"testMaterial",0.8,1.5f,0.2,257);
	failed |= abs(pixel(program,0,-1)-64)>1 || pixel_alpha!=255;
	uniform4(program,"testMaterial",0.8,1.5f,0.5,258);
	failed |= abs(pixel(program,0,-1)-16)>1 || pixel_alpha!=64;
	// Back faces require double-sided materials; mirrored transforms reverse winding.
	glFrontFace(GL_CW);
	uniform4(program,"testMaterial",0.8,1.5f,0.5,256);
	failed |= pixel(program,0,-1)!=0;
	uniform4(program,"testMaterial",0.8,1.5f,0.5,256+512);
	failed |= abs(pixel(program,0,-1)-64)>1;
	uniform4(program,"testMaterial",0.8,1.5f,0.5,256+1024);
	failed |= abs(pixel(program,0,-1)-64)>1;
	glFrontFace(GL_CCW);
	// A constant RGBM environment lights diffuse surfaces and the sky background.
	unsigned char atlas[16*64*4];
	for(int i=0;i<16*64;i++) {
		atlas[i*4]=atlas[i*4+1]=atlas[i*4+2]=64;atlas[i*4+3]=255;
	}
	glActiveTexture(GL_TEXTURE1);glBindTexture(GL_TEXTURE_2D,dataTexture);
	glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,16,64,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
	glUniform1i(glGetUniformLocation(program,"environmentTex"),1);
	uniform4(program,"environment",1,0,1.0/16,1.0/8);
	uniform4(program,"ambientLight",0,0,0,1);
	uniform4(program,"testMaterial",0.8,1.5f,0.5,0);
	int ibl=pixel(program,0,-1);
	uniform4(program,"testMaterial",0.8,1.5f,0.5,2048+512);
	int sky=pixel(program,0,-1);
	uniform4(program,"renderParams",2,0,0,0);
	int exposedSky=pixel(program,0,-1);
	failed |= ibl<110 || ibl>118 || sky<230 || sky>232 || exposedSky<=sky;
	printf("environment pixels: IBL=%d, sky=%d, exposure=%d; alpha and face coverage checked\n",ibl,sky,exposedSky);
	// RGBE uses per-texel exponents; bright IBL and sky remain above 16.
    for(int i=0;i<16*56;i++) {
        atlas[i*4]=atlas[i*4+1]=atlas[i*4+2]=64;atlas[i*4+3]=135;
    }
    glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,16,64,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
    environment_encoding=1;
    uniform4(program,"renderParams",0.03125,0,0,0);
    int hdrSky=pixel(program,0,-1);
    uniform4(program,"testMaterial",0.8,1.5f,0.5,0);
    int hdrIBL=pixel(program,0,-1);
    environment_encoding=0;
    for(int i=0;i<16*56;i++) atlas[i*4+3]=255;
    glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,16,64,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
    uniform4(program,"renderParams",0.25,0,0,0);
    int referenceIBL=pixel(program,0,-1);
    uniform4(program,"testMaterial",0.8,1.5f,0.5,2048+512);
    int referenceSky=pixel(program,0,-1);
    failed |= hdrSky<187 || hdrSky>189 || abs(hdrSky-referenceSky)>1 || abs(hdrIBL-referenceIBL)>1 || hdrIBL<50;
    printf("RGBE pixels: sky=%d/%d, IBL=%d/%d\n",hdrSky,referenceSky,hdrIBL,referenceIBL);
    // Longitude .5 lies halfway between columns 7 and 8. Mixing exponent
    // bytes first would return ~sqrt(32*0.25), instead of their linear mean.
    environment_encoding=1;
    for(int y=0;y<56;y++) for(int x=0;x<16;x++) {
        int i=(y*16+x)*4;
        atlas[i]=atlas[i+1]=atlas[i+2]=255;atlas[i+3]=(x<8)?133:126;
    }
    glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,16,64,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
    uniform4(program,"testWorldPosition",1,0,0,0);
    uniform4(program,"testSurface3",1,0,0,1);
    uniform4(program,"cameraPosition",0,0,0,1);
    uniform4(program,"renderParams",0.0625,0,0,0);
    int filtered=pixel(program,0,-1);
    failed |= filtered<187 || filtered>189;
    // Zero inset at the bottom pole must stay in the sky tile, even though
    // the next roughness tile contains very bright texels.
    for(int i=0;i<16*64;i++) {
        atlas[i*4]=atlas[i*4+1]=atlas[i*4+2]=255;atlas[i*4+3]=i<16*8?126:144;
    }
    glTexImage2D(GL_TEXTURE_2D,0,GL_RGBA8,16,64,0,GL_RGBA,GL_UNSIGNED_BYTE,atlas);
    uniform4(program,"testWorldPosition",0,-1,0,0);
    uniform4(program,"testSurface3",0,-1,0,1);
    uniform4(program,"environment",1,0,0,0);
    uniform4(program,"renderParams",1,0,0,0);
    int pole=pixel(program,0,-1);
    failed |= pole<123 || pole>125;
    printf("RGBE filtering: mixed exponents=%d, isolated pole=%d\n",filtered,pole);
    failed |= panorama_pixels(program,texture);
    environment_encoding=0;
    uniform4(program,"testWorldPosition",0,0,0,0);
    uniform4(program,"cameraPosition",0,2,2,1);
    uniform4(program,"testSurface3",0,1,0,1);
	// The analytic sky works without a lighting atlas. Grid opacity fades to
	// zero before its moving plane boundary and at grazing viewing angles.
	uniform4(program,"renderParams",1,0,0,0);
	uniform4(program,"environment",0,0,0,0);
	uniform4(program,"cameraPosition",0,-2,0,1);
	uniform4(program,"testSurface",0.1,0.2,0.3,0);
	uniform4(program,"testMaterial",0.8,1.5f,0.5,2048+512+16384);
	int gradient=pixel(program,0,-1);
	uniform4(program,"testMaterial",-1,1.0f,0,8192);
	uniform4(program,"testSurface",1,0,1,1);
	uniform4(program,"testWorldPosition",1,0,1,0);
	uniform4(program,"cameraPosition",1,2,1,1);
	int grid=pixel(program,0,-1);
	uniform4(program,"testWorldPosition",2,0,1,0);
	int distantGrid=pixel(program,0,-1);
	uniform4(program,"testWorldPosition",1,0,1,0);
	uniform4(program,"cameraPosition",3,0.01,1,1);
	int grazingGrid=pixel(program,0,-1);
	failed |= gradient<84 || gradient>86 || grid<60 || grid>62 || distantGrid!=0 || grazingGrid!=0;
	printf("viewport pixels: gradient=%d, grid=%d, distant=%d, grazing=%d\n",gradient,grid,distantGrid,grazingGrid);
	failed |= photometry_pixels(program);
	failed |= ssao_pixels(program);
	failed |= material_layer_pixels(program,dataTexture);
	failed |= fresnel_pixels(program);
	failed |= ssr_pixels(program);
	failed |= fxaa_pixels(program);
	failed |= camera_color_pixels(program);
	glDeleteTextures(1,&dataTexture);

	glDeleteTextures(1, &texture);
	glDeleteVertexArrays(1, &vao);
	glDeleteShader(vs);
	glDeleteShader(fs);
	glDeleteProgram(program);
	eglMakeCurrent(display, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
	eglDestroyContext(display, context);
	eglDestroySurface(display, surface);
	eglTerminate(display);
	return failed;
}
