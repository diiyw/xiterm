#define main onix_metal_lazy_test_main
#include "metal_lazy_test.m"
#undef main
#include <math.h>

static int render_sample(OnixMetalDriver *driver, OnixMetalInstance3D instance, uint32_t shader, uint32_t depth_texture) {
    OnixMetalVertex3D vertices[3] = {{.position={-.9,-.9,.25},.normal={0,0,1}}, {.position={.9,-.9,.25},.normal={0,0,1}}, {.position={0,.9,.25},.normal={0,0,1}}};
    uint32_t indices[]={0,1,2}, deformation[]={0};
    memcpy(instance.model,identity,sizeof(identity));
    instance.uv_transform[0]=instance.uv_transform[1]=1;
    OnixNativeMeshDraw3D draw={.shader=shader,.texture=white_texture,.index_count=3,.instance_count=1};
    draw.extra_textures[6]=depth_texture;
    require(onix_metal_begin_frame(driver,blue),"begin variant sample");
    require(onix_metal_draw_meshes(driver,&mesh_uniforms,vertices,3,indices,3,&instance,1,deformation,1,&draw,1),"draw variant sample");
    uint8_t pixels[32*256]; finish_frame(driver,pixels);
    return pixels[16*256+16*4+2];
}

int main(void) {
 @autoreleasepool {
    OnixTestLayer *layer=[[OnixTestLayer alloc] init];
    OnixMetalDriver *driver=onix_metal_create(layer,32,32);
    require(driver!=NULL,"create variant driver");
    for(int variant=0;variant<ONIX_MESH_VARIANT_COUNT;variant++) require(driver->mesh_libraries[variant]==nil,"2D startup avoids all mesh libraries");
    const uint8_t white[]={255,255,255,255};
    white_texture=onix_metal_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,white,sizeof(white),0,0);
    memcpy(mesh_uniforms.view_projection,identity,sizeof(identity));
    memcpy(mesh_uniforms.occlusion_inverse,identity,sizeof(identity));
    mesh_uniforms.camera_position[2]=2;mesh_uniforms.camera_position[3]=1;
    mesh_uniforms.ambient_light[0]=mesh_uniforms.ambient_light[1]=mesh_uniforms.ambient_light[2]=1;
    mesh_uniforms.lighting[0]=0;mesh_uniforms.lighting[4]=1;mesh_uniforms.lighting[10]=-1;mesh_uniforms.lighting[11]=16;
    OnixMetalInstance3D instance={.base_color={.7,.5,.3,1},.material={.8,1.5,.5,512},.custom={{1,1,1,0},{1,1,1,0}}};
    int original=render_sample(driver,instance,0,0);
    require(original>100,"basic PBR produces lit pixels");
    require(driver->mesh_variants[ONIX_MESH_BASIC-1][0]!=nil && driver->mesh_libraries[ONIX_MESH_FULL]==nil,"basic PBR avoids full library and pipeline");
    uint32_t full=onix_metal_create_mesh_shader(driver,onix_metal_mesh_sources[ONIX_MESH_FULL],strlen(onix_metal_mesh_sources[ONIX_MESH_FULL]));
    require(full!=0,"create full reference shader");
    int comparisons=0;
    for(int metallic=0;metallic<3;metallic++) for(int roughness=0;roughness<3;roughness++) for(int pass=0;pass<2;pass++) {
        instance.emissive_metallic[3]=metallic*.5f; instance.material[0]=.15f+roughness*.4f;
        mesh_uniforms.lighting[5]=pass?3:0;
        int reference=render_sample(driver,instance,full,0), specialized=render_sample(driver,instance,0,0);
        require(abs(reference-specialized)<=1,"basic matches full PBR in lit and capture passes"); comparisons++;
    }
    mesh_uniforms.lighting[5]=0;instance.emissive_metallic[3]=0;instance.material[0]=.8f;
	for (int sided = 0; sided < 2; sided++) for (int mirrored = 0; mirrored < 2; mirrored++) for (int target_flip = 0; target_flip < 2; target_flip++) {
		instance.material[3] = (sided ? 512 : 0) + (mirrored ? 1024 : 0);
		mesh_uniforms.lighting[7] = target_flip;
		int reference = render_sample(driver, instance, full, 0), culled = render_sample(driver, instance, 0, 0);
		require(abs(reference-culled)<=1, "hardware face culling matches fragment rejection, including mirrored targets and double sides");
	}
	instance.material[3]=512;mesh_uniforms.lighting[7]=0;
    uint8_t depths[32*32*4];
    for(int i=0;i<32*32;i++){depths[4*i]=127;depths[4*i+1]=255;depths[4*i+2]=255;depths[4*i+3]=255;}
    uint32_t depth=onix_metal_create_texture_with_format(driver,32,32,1,ONIX_TEXTURE_FORMAT_RGBA8,0,depths,sizeof(depths),0,0);
    mesh_uniforms.light_meta[2]=1;mesh_uniforms.light_meta[3]=.025;
    int clear=render_sample(driver,instance,0,depth);
    mesh_uniforms.lighting[6]=1;
    for(int samples=8;samples<=32;samples*=2) {
        mesh_uniforms.lighting[11]=samples;
        int full_ao=render_sample(driver,instance,full,depth), basic_ao=render_sample(driver,instance,0,depth);
        require(abs(full_ao-basic_ao)<=1 && basic_ao<clear-10,"all SSAO kernels darken contact and match full PBR");
        printf("Metal SSAO %d taps: clear=%d full=%d basic=%d\n",samples,clear,full_ao,basic_ao);
    }
    mesh_uniforms.lighting[6]=0;
    for(int pass=1;pass<=5;pass++) {
        if(pass==3 || pass==4)continue;
        mesh_uniforms.lighting[5]=pass;
        int reference=render_sample(driver,instance,full,0), specialized=render_sample(driver,instance,0,0);
        require(abs(reference-specialized)<=1,"packed depth matches full shader for all depth passes");
    }
    require(driver->mesh_variants[ONIX_MESH_DEPTH-1][0]!=nil,"depth pipeline selected");
    mesh_uniforms.lighting[5]=0;instance.material[3]=512+256;
    int reference=render_sample(driver,instance,full,0), specialized=render_sample(driver,instance,0,0);
    require(abs(reference-specialized)<=1,"unlit matches full shader");
    require(driver->mesh_variants[ONIX_MESH_UNLIT-1][0]!=nil,"unlit pipeline selected");
    instance.material[3]=512;instance.custom[0][3]=.5;
    render_sample(driver,instance,0,0);
    require(driver->mesh_opaque_pipeline!=nil,"advanced material selects full PBR");
    printf("Metal variants: %d basic/full comparisons, depth, unlit, advanced selection and lazy libraries passed\n",comparisons);
    onix_metal_destroy(driver);[layer release];
 }
 return 0;
}
