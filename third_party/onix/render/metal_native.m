//go:build cgo && (darwin || ios) && !onix_directx && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl


#include "metal_native.h"
#include "native_slots.h"
#include "native_buffer.h"
#include "native_upload.h"

#import <Foundation/Foundation.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#import <CoreVideo/CoreVideo.h>
#import <TargetConditionals.h>

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define ONIX_METAL_FRAMES_IN_FLIGHT 2
#define ONIX_METAL_INSTANCE_BUFFER_MINIMUM (64u * 1024u)
#define ONIX_MESH_BLEND_MODE_COUNT 4

typedef struct {
    id<MTLTexture> texture;
	CVMetalTextureRef external;
	uint32_t format;
	uint32_t sampler;
	uint32_t last_update;
	uint32_t free_next;
} OnixMetalTexture;

typedef struct {
	uint32_t color_texture;
	uint32_t color_format;
	id<MTLTexture> depth_texture;
	uint32_t width;
	uint32_t height;
	int initialized;
	uint32_t free_next;
} OnixMetalRenderTarget;

typedef struct {
	id<MTLLibrary> vertex_library, fragment_library;
	id<MTLRenderPipelineState> opaque;
	id<MTLRenderPipelineState> transparent[ONIX_MESH_BLEND_MODE_COUNT];
	uint32_t free_next;
} OnixMetalMeshPipeline;

typedef struct {
	id<MTLLibrary> vertex_library, fragment_library;
	id<MTLRenderPipelineState> pipelines[3];
	uint32_t free_next;
} OnixMetalMaterial2DPipeline;

typedef struct {
	id<MTLBuffer> vertices;
	id<MTLBuffer> indices;
	id<MTLBuffer> deformation;
	uint32_t vertex_count;
	uint32_t index_count;
	uint32_t free_next;
} OnixMetalMesh;

ONIX_NATIVE_SLOTS(onix_metal_textures, OnixMetalTexture, 64, UINT32_MAX)
ONIX_NATIVE_SLOTS(onix_metal_targets, OnixMetalRenderTarget, 8, UINT32_MAX)
ONIX_NATIVE_SLOTS(onix_metal_meshes, OnixMetalMesh, 16, UINT32_MAX)
ONIX_NATIVE_SLOTS(onix_metal_mesh_shaders, OnixMetalMeshPipeline, 8, UINT32_MAX)
ONIX_NATIVE_SLOTS(onix_metal_material2d_shaders, OnixMetalMaterial2DPipeline, 8, UINT32_MAX)

typedef struct {
	id<MTLBuffer> buffer;
	NSUInteger capacity, offset;
} OnixMetalUploadBlock;

typedef struct {
	OnixMetalUploadBlock *blocks;
	uint32_t count, capacity, index;
	NSUInteger bytes, used;
	OnixBufferRetention retention, table_retention;
	OnixNativeUploadCache cache;
} OnixMetalUploadFrame;

struct OnixMetalDriver {
    CAMetalLayer *layer;
    id<MTLDevice> device;
	id<MTLSamplerState> samplers[18];
	id<MTLCommandQueue> queue;
	CVMetalTextureCacheRef texture_cache;
	id<MTLLibrary> default_library;
	id<MTLLibrary> mesh_libraries[ONIX_MESH_VARIANT_COUNT];
	id<MTLRenderPipelineState> mesh_variants[ONIX_MESH_VARIANT_COUNT-1][1+ONIX_MESH_BLEND_MODE_COUNT];
	// Cache 2D variants by color/depth attachment, then sprite blend mode.
	id<MTLRenderPipelineState> sprite_pipelines[3][2];
	id<MTLRenderPipelineState> shape_pipelines[3];
	id<MTLRenderPipelineState> mesh_opaque_pipeline;
	id<MTLRenderPipelineState> mesh_picking_pipeline;
	id<MTLRenderPipelineState> mesh_transparent_pipelines[ONIX_MESH_BLEND_MODE_COUNT];
	OnixMetalMeshPipeline *mesh_pipelines;
	OnixNativeSlots mesh_shader_slots;
	OnixMetalMaterial2DPipeline *material2d_pipelines;
	OnixNativeSlots material2d_shader_slots;
	id<MTLDepthStencilState> depth_disabled;
	id<MTLDepthStencilState> mesh_opaque_depth;
	id<MTLDepthStencilState> mesh_transparent_depth;
	id<MTLTexture> depth_texture;
    id<MTLBuffer> quad_vertices;
    id<MTLBuffer> quad_indices;

	id<CAMetalDrawable> drawable;
	id<MTLCommandBuffer> command_buffer;
	id<MTLRenderCommandEncoder> encoder;
	uint32_t active_render_target;
	id<MTLCommandBuffer> inflight_commands[ONIX_METAL_FRAMES_IN_FLIGHT];
	NSMutableArray *frame_resources[ONIX_METAL_FRAMES_IN_FLIGHT];
	id<MTLBuffer> readback_buffer;
	NSUInteger readback_capacity, readback_used;
	OnixBufferRetention readback_retention;
	OnixMetalUploadFrame uploads[ONIX_METAL_FRAMES_IN_FLIGHT];
	MTLRenderPassDescriptor *surface_pass;
	MTLRenderPassDescriptor *target_pass;
	uint32_t frame_index;
    MTLViewport viewport;
    MTLScissorRect scissor;
	int scissor_enabled;
	int sprite_blend;
	float blend_constant[4];

	OnixMetalTexture *textures;
	OnixNativeSlots texture_slots;
	OnixMetalRenderTarget *render_targets;
	OnixNativeSlots target_slots;
	OnixMetalMesh *meshes;
	OnixNativeSlots mesh_slots;
};

void onix_metal_buffer_memory(OnixMetalDriver *driver, OnixBufferMemory *memory) {
	*memory = (OnixBufferMemory){0};
	if (driver == NULL) return;
	for (uint32_t index = 0; index < ONIX_METAL_FRAMES_IN_FLIGHT; index++) {
		OnixMetalUploadFrame *frame = &driver->uploads[index];
		memory->instances += frame->bytes;
		memory->scratch += (uint64_t)frame->capacity * sizeof(*frame->blocks) + onix_upload_cache_bytes(&frame->cache);
	}
	memory->readback = driver->readback_capacity;
}

static _Thread_local char onix_metal_error[1024];

static const char *onix_metal_source =
"#include <metal_stdlib>\n"
"using namespace metal;\n"
"struct SpriteIn {\n"
"  float3 position [[attribute(0)]];\n"
"  float2 texture_coord [[attribute(1)]];\n"
"  float3 model0 [[attribute(2)]];\n"
"  float3 model1 [[attribute(3)]];\n"
"  float3 texture0 [[attribute(4)]];\n"
"  float3 texture1 [[attribute(5)]];\n"
"  float4 color [[attribute(6)]];\n"
"  float4 custom0 [[attribute(7)]]; float4 custom1 [[attribute(8)]]; float4 custom2 [[attribute(9)]]; float4 custom3 [[attribute(10)]];\n"
"  float texture_slot [[attribute(11)]];\n"
"};\n"
"struct SpriteUniforms { float4x4 projection; float texture_coverage; float padding[3]; };\n"
"struct SpriteOut { float4 position [[position]]; float2 texture_coord [[user(loc0)]]; float4 color [[user(loc1)]]; float4 custom0 [[user(loc2), flat]]; float4 custom1 [[user(loc3), flat]]; float4 custom2 [[user(loc4), flat]]; float4 custom3 [[user(loc5), flat]]; };\n"
"vertex SpriteOut sprite_vertex(SpriteIn in [[stage_in]], constant SpriteUniforms &u [[buffer(2)]]) {\n"
"  SpriteOut out;\n"
"  float3 p = float3(in.position.xy, 1.0);\n"
"  out.position = u.projection * float4(dot(in.model0, p), dot(in.model1, p), 0.0, 1.0);\n"
"  float3 uv = float3(in.texture_coord, 1.0);\n"
"  out.texture_coord = float2(dot(in.texture0, uv), dot(in.texture1, uv));\n"
"  out.color = in.color; out.custom0=in.custom0; out.custom1=in.custom1; out.custom2=in.custom2; out.custom3=in.custom3; return out;\n"
"}\n"
"fragment float4 sprite_fragment(SpriteOut in [[stage_in]], texture2d<float> tex [[texture(0)]], constant SpriteUniforms &u [[buffer(2)]], sampler s [[sampler(0)]]) {\n"
"  float4 sampled = tex.sample(s, in.texture_coord);\n"
"  if (u.texture_coverage > 0.5f) sampled = float4(1.0f, 1.0f, 1.0f, sampled.r);\n"
"  float4 color = sampled * in.color;\n"
"  color.rgb *= color.a; return color;\n"
"}\n"
"struct MultiSpriteUniforms { float4x4 projection; float4 coverage; };\n"
"struct MultiSpriteOut { float4 position [[position]]; float2 uv; float2 position2D; float4 color; uint slot [[flat]]; float4 clipRect [[flat]]; float4 clipParams [[flat]]; };\n"
"vertex MultiSpriteOut sprite_multi_vertex(SpriteIn in [[stage_in]], constant MultiSpriteUniforms &u [[buffer(2)]]) {\n"
"  MultiSpriteOut out; float3 p=float3(in.position.xy,1.0f),uv=float3(in.texture_coord,1.0f);\n"
"  out.position2D=float2(dot(in.model0,p),dot(in.model1,p));\n"
"  out.position=u.projection*float4(out.position2D,0.0f,1.0f);\n"
"  out.clipRect=in.custom2; out.clipParams=in.custom3;\n"
"  out.uv=float2(dot(in.texture0,uv),dot(in.texture1,uv)); out.color=in.color; out.slot=uint(in.texture_slot); return out;\n"
"}\n"
"fragment float4 sprite_multi_fragment(MultiSpriteOut in [[stage_in]], constant MultiSpriteUniforms &u [[buffer(2)]], texture2d<float> tex [[texture(0)]], texture2d<float> tex1 [[texture(1)]], texture2d<float> tex2 [[texture(2)]], texture2d<float> tex3 [[texture(3)]], sampler s [[sampler(0)]], sampler s1 [[sampler(1)]], sampler s2 [[sampler(2)]], sampler s3 [[sampler(3)]]) {\n"
"  float2 dx=dfdx(in.uv),dy=dfdy(in.uv); float4 sampled;\n"
"  switch(in.slot) {\n"
"    case 1u: sampled=tex1.sample(s1,in.uv,gradient2d(dx,dy)); break;\n"
"    case 2u: sampled=tex2.sample(s2,in.uv,gradient2d(dx,dy)); break;\n"
"    case 3u: sampled=tex3.sample(s3,in.uv,gradient2d(dx,dy)); break;\n"
"    default: sampled=tex.sample(s,in.uv,gradient2d(dx,dy)); break;\n"
"  }\n"
"  if(u.coverage[in.slot]>0.5f) sampled=float4(1.0f,1.0f,1.0f,sampled.r);\n"
"  float4 color=sampled*in.color;\n"
"  float2 halfSize=(in.clipRect.zw-in.clipRect.xy)*0.5f;\n"
"  float radius=in.clipParams.x;\n"
"  float2 q=abs(in.position2D-(in.clipRect.xy+halfSize))-halfSize+radius;\n"
"  float distance=length(max(q,float2(0.0f)))+min(max(q.x,q.y),0.0f)-radius;\n"
"  float clipCoverage=clamp(0.5f-distance/max(fwidth(distance),0.0001f),0.0f,1.0f);\n"
"  color.a*=in.clipParams.y>0.5f ? clipCoverage : 1.0f;\n"
"  return float4(color.rgb*color.a,color.a);\n"
"}\n"
"struct ShapeIn {\n"
"  float3 position [[attribute(0)]];\n"
"  float2 local_coord [[attribute(1)]];\n"
"  float3 model0 [[attribute(2)]];\n"
"  float3 model1 [[attribute(3)]];\n"
"  float3 local0 [[attribute(4)]];\n"
"  float3 local1 [[attribute(5)]];\n"
"  float4 fill [[attribute(6)]];\n"
"  float4 border [[attribute(7)]];\n"
"  float4 params [[attribute(8)]];\n"
"  uint kind [[attribute(9)]];\n"
"};\n"
"struct ShapeOut { float4 position [[position]]; float2 local; float4 fill [[flat]]; float4 border [[flat]]; float4 params [[flat]]; uint kind [[flat]]; };\n"
"vertex ShapeOut shape_vertex(ShapeIn in [[stage_in]], constant SpriteUniforms &u [[buffer(2)]]) {\n"
"  ShapeOut out; float3 p=float3(in.position.xy,1.0); float3 uv=float3(in.local_coord,1.0);\n"
"  out.position=u.projection*float4(dot(in.model0,p),dot(in.model1,p),0.0,1.0);\n"
"  out.local=float2(dot(in.local0,uv),dot(in.local1,uv))*in.params.xy;\n"
"  out.fill=in.fill; out.border=in.border; out.params=in.params; out.kind=in.kind; return out;\n"
"}\n"
"float shape_sd_rounded_box(float2 point, float2 size, float radius) {\n"
"  radius=clamp(radius,0.0f,min(size.x,size.y)*0.5f); float2 q=abs(point)-size*0.5f+radius;\n"
"  return length(max(q,float2(0.0f)))+min(max(q.x,q.y),0.0f)-radius;\n"
"}\n"
"float shape_sd_segment(float2 point, float2 a, float2 b) { float2 pa=point-a; float2 ba=b-a; float h=clamp(dot(pa,ba)/dot(ba,ba),0.0f,1.0f); return length(pa-ba*h); }\n"
"float shape_coverage(float distance) { return clamp(0.5f-distance/max(fwidth(distance),0.0001f),0.0f,1.0f); }\n"
"float4 shape_premultiply(float4 value) { return float4(value.rgb*value.a,value.a); }\n"
"fragment float4 shape_fragment(ShapeOut in [[stage_in]]) {\n"
"  float2 size=in.params.xy;\n"
"  if (in.kind==4u) { float direction=in.params.w; float2 a=float2(size.x*0.5f,size.y*0.40f),b=float2(size.x*0.20f,size.y*0.73f),c=float2(size.x*0.80f,size.y*0.73f);\n"
"    if(direction>=0.5f && direction<1.5f){ a=float2(size.x*0.20f,size.y*0.20f); b=float2(size.x*0.80f,size.y*0.20f); c=float2(size.x*0.5f,size.y*0.50f); }\n"
"    else if(direction>=1.5f && direction<2.5f){ a=float2(size.x*0.73f,size.y*0.20f); b=float2(size.x*0.73f,size.y*0.80f); c=float2(size.x*0.40f,size.y*0.5f); }\n"
"    else if(direction>=2.5f){ a=float2(size.x*0.27f,size.y*0.20f); b=float2(size.x*0.27f,size.y*0.80f); c=float2(size.x*0.60f,size.y*0.5f); }\n"
"    float e0=(b.x-a.x)*(in.local.y-a.y)-(b.y-a.y)*(in.local.x-a.x), e1=(c.x-b.x)*(in.local.y-b.y)-(c.y-b.y)*(in.local.x-b.x), e2=(a.x-c.x)*(in.local.y-c.y)-(a.y-c.y)*(in.local.x-c.x);\n"
"    float orientation=sign((b.x-a.x)*(c.y-a.y)-(b.y-a.y)*(c.x-a.x)); float distance=(orientation>0.0f ? -min(min(e0,e1),e2) : max(max(e0,e1),e2))/max(length(b-a),1.0f); return shape_premultiply(in.fill)*step(distance,0.0f); }\n"
"  if (in.kind==2u) { float2 a=size*float2(0.18f,0.56f); float2 b=size*float2(0.42f,0.78f); float2 c=size*float2(0.84f,0.22f);\n"
"    float thickness=max(min(size.x,size.y)*0.14f,1.4f); float distance=min(shape_sd_segment(in.local,a,b),shape_sd_segment(in.local,b,c))-thickness*0.5f;\n"
"    return shape_premultiply(in.fill)*shape_coverage(distance); }\n"
"  float2 point=in.local-size*0.5f; float outer=shape_coverage(shape_sd_rounded_box(point,size,in.params.z));\n"
"  float border_width=min(max(in.params.w,0.0f),min(size.x,size.y)*0.5f);\n"
"  if (border_width<=0.0f) return shape_premultiply(in.fill)*outer;\n"
"  float2 inner_size=max(size-border_width*2.0f,float2(0.0f)); float inner=min(inner_size.x,inner_size.y)>0.0f ? shape_coverage(shape_sd_rounded_box(point,inner_size,max(in.params.z-border_width,0.0f))) : 0.0f;\n"
"  return mix(shape_premultiply(in.border),shape_premultiply(in.fill),inner)*outer;\n"
"}\n"
"struct MeshIn {\n"
"  float3 position [[attribute(0)]]; float3 normal [[attribute(1)]]; float2 texture_coord [[attribute(2)]]; float4 tangent [[attribute(15)]];\n"
"  float4 model0 [[attribute(3)]]; float4 model1 [[attribute(4)]]; float4 model2 [[attribute(5)]]; float4 model3 [[attribute(6)]];\n"
"  float4 base_color [[attribute(7)]]; float4 emissive_metallic [[attribute(8)]]; float4 material [[attribute(9)]]; float4 uv_transform [[attribute(10)]];\n"
"  float4 custom0 [[attribute(11)]]; float4 custom1 [[attribute(12)]]; float4 custom2 [[attribute(13)]]; float4 custom3 [[attribute(14)]];\n"
"};\n"
"struct MeshUniforms { float4x4 view_projection; float4 camera_position; float4 ambient_light; float4 light_direction; float4 directional_light; float4 light_position[16]; float4 light_directions[16]; float4 light_colors[16]; float4 light_params[16]; float4 light_meta; };\n"
"struct MeshOut { float4 position [[position]]; float3 world_position [[user(loc0)]]; float3 normal [[user(loc1)]]; float2 texture_coord [[user(loc2)]]; float4 base_color [[user(loc3)]]; float4 emissive_metallic [[user(loc4)]]; float4 material [[user(loc5), flat]]; float4 custom0 [[user(loc6), flat]]; float4 custom1 [[user(loc7), flat]]; float4 custom2 [[user(loc8), flat]]; float4 custom3 [[user(loc9), flat]]; float4 tangent [[user(loc10)]]; };\n"
"float3 mesh_unpack_rgb(float encoded) { uint packed=uint(encoded); return float3(float(packed&255u),float((packed>>8u)&255u),float((packed>>16u)&255u))/255.0f; }\n"
"float mesh_deformation_float(device const uint *data,uint offset){ return as_type<float>(data[offset]); }\n"
"float4x4 mesh_deformation_matrix(device const uint *data,uint offset){ return float4x4(\n"
"  float4(mesh_deformation_float(data,offset),mesh_deformation_float(data,offset+1u),mesh_deformation_float(data,offset+2u),mesh_deformation_float(data,offset+3u)),\n"
"  float4(mesh_deformation_float(data,offset+4u),mesh_deformation_float(data,offset+5u),mesh_deformation_float(data,offset+6u),mesh_deformation_float(data,offset+7u)),\n"
"  float4(mesh_deformation_float(data,offset+8u),mesh_deformation_float(data,offset+9u),mesh_deformation_float(data,offset+10u),mesh_deformation_float(data,offset+11u)),\n"
"  float4(mesh_deformation_float(data,offset+12u),mesh_deformation_float(data,offset+13u),mesh_deformation_float(data,offset+14u),mesh_deformation_float(data,offset+15u))); }\n"
"vertex MeshOut mesh_vertex(MeshIn in [[stage_in]], constant MeshUniforms &u [[buffer(2)]], device const uint *deformation [[buffer(3)]], device const uint *static_deformation [[buffer(5)]], constant uint &deformation_index [[buffer(4)]], uint vertex_id [[vertex_id]]) {\n"
"  MeshOut out; bool spine=in.material.x < -1.5f; float4 vertex_color=in.base_color; float4 vertex_custom0=in.custom0;\n"
"  if(spine) { vertex_color=float4(mesh_unpack_rgb(in.normal.x),in.normal.y); vertex_custom0.rgb=mesh_unpack_rgb(in.normal.z); }\n"
"  float3 position=in.position,normal=in.normal,tangent=in.tangent.xyz; float tangent_sign=in.tangent.w;\n"
"  if(!spine && deformation_index!=0u){ uint header=(deformation_index-1u)*8u; uint skin_offset=deformation[header],morph_offset=deformation[header+1u],joint_offset=deformation[header+2u],weight_offset=deformation[header+3u],vertex_count=deformation[header+4u],target_count=deformation[header+5u],joint_count=deformation[header+6u];\n"
"    if(morph_offset!=0u){ for(uint morph_target=0u;morph_target<target_count;morph_target++){ uint delta=morph_offset+(morph_target*vertex_count+vertex_id)*9u; float weight=mesh_deformation_float(deformation,weight_offset+morph_target); if(weight==0.0f) continue; position+=float3(mesh_deformation_float(static_deformation,delta),mesh_deformation_float(static_deformation,delta+1u),mesh_deformation_float(static_deformation,delta+2u))*weight; normal+=float3(mesh_deformation_float(static_deformation,delta+3u),mesh_deformation_float(static_deformation,delta+4u),mesh_deformation_float(static_deformation,delta+5u))*weight; tangent+=float3(mesh_deformation_float(static_deformation,delta+6u),mesh_deformation_float(static_deformation,delta+7u),mesh_deformation_float(static_deformation,delta+8u))*weight; } }\n"
"    if(skin_offset!=0u && joint_count!=0u){ float4 source_position=float4(position,1.0f),source_normal=float4(normal,0.0f),source_tangent=float4(tangent,0.0f); float4x4 skin_matrix=float4x4(0.0f); float skin_weight=0.0f; for(uint influence=0u;influence<4u;influence++){ float weight=mesh_deformation_float(static_deformation,skin_offset+vertex_id*8u+4u+influence); if(weight>0.0f){ uint joint=static_deformation[skin_offset+vertex_id*8u+influence]; if(joint<joint_count){ float4x4 matrix=mesh_deformation_matrix(deformation,joint_offset+joint*16u); skin_matrix+=matrix*weight; skin_weight+=weight; } } } if(skin_weight>0.0f){ position=(skin_matrix*source_position).xyz; float3x3 cofactor=float3x3(cross(skin_matrix[1].xyz,skin_matrix[2].xyz),cross(skin_matrix[2].xyz,skin_matrix[0].xyz),cross(skin_matrix[0].xyz,skin_matrix[1].xyz)); float flip=dot(skin_matrix[0].xyz,cofactor[0])<0.0f ? -1.0f : 1.0f; normal=normalize(cofactor*source_normal.xyz)*flip; tangent=(skin_matrix*source_tangent).xyz; tangent_sign*=flip; } }\n"
"  }\n"
"  float4x4 model=float4x4(in.model0,in.model1,in.model2,in.model3); float4 world=model*float4(position,1.0f);\n"
"  out.position=u.view_projection*world; if((uint(in.material.w+0.5f)&2048u)!=0u) out.position.z=0.000001f*out.position.w; out.world_position=world.xyz;\n"
"  float3x3 cofactor=float3x3(cross(in.model1.xyz,in.model2.xyz),cross(in.model2.xyz,in.model0.xyz),cross(in.model0.xyz,in.model1.xyz));\n"
"  out.normal=spine ? float3(0.0f,0.0f,1.0f) : normalize(cofactor*normal)*(dot(in.model0.xyz,cofactor[0])<0.0f ? -1.0f : 1.0f); out.tangent=float4((model*float4(tangent,0.0f)).xyz,tangent_sign*(dot(in.model0.xyz,cofactor[0])<0.0f ? -1.0f : 1.0f)); out.texture_coord=in.texture_coord*in.uv_transform.xy+in.uv_transform.zw;\n"
"  out.base_color=vertex_color; out.emissive_metallic=in.emissive_metallic; out.material=in.material;\n"
"  out.custom0=vertex_custom0; out.custom1=in.custom1; out.custom2=in.custom2; out.custom3=in.custom3; return out;\n"
"}\n"
"float mesh_distribution_ggx(float3 n,float3 h,float roughness){ float a=roughness*roughness,a2=a*a,nh=max(dot(n,h),0.0f),d=nh*nh*(a2-1.0f)+1.0f; return a2/max(M_PI_F*d*d,0.0001f); }\n"
"float mesh_geometry(float nv,float roughness){ float r=roughness+1.0f,k=r*r/8.0f; return nv/max(nv*(1.0f-k)+k,0.0001f); }\n"
"float3 mesh_fresnel(float value,float3 f0){ return f0+(1.0f-f0)*pow(1.0f-value,5.0f); }\n"

"struct MeshPickingOut { float4 position [[position]]; uint picking_id [[user(loc0), flat]]; };\n"
"vertex MeshPickingOut mesh_picking_vertex(MeshIn in [[stage_in]], constant MeshUniforms &u [[buffer(2)]]) { MeshPickingOut out; out.position=u.view_projection*float4x4(in.model0,in.model1,in.model2,in.model3)*float4(in.position,1.0f); out.picking_id=as_type<uint>(in.custom3.w); return out; }\n"
"fragment uint mesh_picking_fragment(MeshPickingOut in [[stage_in]]) { return in.picking_id; }\n";

static const char *onix_metal_mesh_sources[ONIX_MESH_VARIANT_COUNT] = {
#include "shaders/metal_mesh_fragment.inc"
,
#include "shaders/metal_mesh_unlit_fragment.inc"
,
#include "shaders/metal_mesh_depth_fragment.inc"
,
#include "shaders/metal_mesh_basic_fragment.inc"
};

static void onix_metal_set_error(const char *format, ...) {
    va_list arguments;
    va_start(arguments, format);
    vsnprintf(onix_metal_error, sizeof(onix_metal_error), format, arguments);
    va_end(arguments);
}

const char *onix_metal_last_error(void) {
    return onix_metal_error;
}

static MTLVertexDescriptor *onix_metal_sprite_vertex_descriptor(void) {
    MTLVertexDescriptor *descriptor = [MTLVertexDescriptor vertexDescriptor];
    descriptor.attributes[0].format = MTLVertexFormatFloat3;
    descriptor.attributes[0].offset = 0;
    descriptor.attributes[0].bufferIndex = 0;
    descriptor.attributes[1].format = MTLVertexFormatFloat2;
    descriptor.attributes[1].offset = 12;
    descriptor.attributes[1].bufferIndex = 0;
    descriptor.layouts[0].stride = 20;
    descriptor.layouts[0].stepFunction = MTLVertexStepFunctionPerVertex;

	const MTLVertexFormat formats[] = {
		MTLVertexFormatFloat3, MTLVertexFormatFloat3,
		MTLVertexFormatFloat3, MTLVertexFormatFloat3,
		MTLVertexFormatFloat4, MTLVertexFormatFloat4,
		MTLVertexFormatFloat4, MTLVertexFormatFloat4,
		MTLVertexFormatFloat4, MTLVertexFormatFloat,
	};
	const NSUInteger offsets[] = {0, 12, 24, 36, 48, 64, 80, 96, 112, 128};
	for (NSUInteger index = 0; index < 10; index++) {
        descriptor.attributes[index + 2].format = formats[index];
        descriptor.attributes[index + 2].offset = offsets[index];
        descriptor.attributes[index + 2].bufferIndex = 1;
    }
    descriptor.layouts[1].stride = sizeof(OnixMetalInstance);
    descriptor.layouts[1].stepFunction = MTLVertexStepFunctionPerInstance;
    descriptor.layouts[1].stepRate = 1;
    return descriptor;
}

static MTLVertexDescriptor *onix_metal_compact_sprite_vertex_descriptor(void) {
	MTLVertexDescriptor *descriptor = onix_metal_sprite_vertex_descriptor();
	for (NSUInteger attribute = 9; attribute < 12; attribute++) descriptor.attributes[attribute].offset -= 32;
	descriptor.layouts[1].stride = ONIX_SPRITE_COMPACT_BYTES;
	return descriptor;
}

static MTLVertexDescriptor *onix_metal_shape_vertex_descriptor(void) {
	MTLVertexDescriptor *descriptor = onix_metal_sprite_vertex_descriptor();
	const MTLVertexFormat formats[] = {
		MTLVertexFormatFloat3, MTLVertexFormatFloat3,
		MTLVertexFormatFloat3, MTLVertexFormatFloat3,
		MTLVertexFormatFloat4, MTLVertexFormatFloat4,
		MTLVertexFormatFloat4, MTLVertexFormatUInt,
	};
	const NSUInteger offsets[] = {0, 12, 24, 36, 48, 64, 80, 96};
	for (NSUInteger index = 0; index < 8; index++) {
		descriptor.attributes[index + 2].format = formats[index];
		descriptor.attributes[index + 2].offset = offsets[index];
		descriptor.attributes[index + 2].bufferIndex = 1;
	}
	descriptor.attributes[10].format = MTLVertexFormatInvalid;
	descriptor.attributes[11].format = MTLVertexFormatInvalid;
	descriptor.layouts[1].stride = sizeof(OnixMetalShapeInstance);
	return descriptor;
}

static MTLVertexDescriptor *onix_metal_mesh_vertex_descriptor(void) {
	MTLVertexDescriptor *descriptor = [MTLVertexDescriptor vertexDescriptor];
	const MTLVertexFormat vertex_formats[] = {MTLVertexFormatFloat3, MTLVertexFormatFloat3, MTLVertexFormatFloat2};
	const NSUInteger vertex_offsets[] = {0, 12, 24};
	for (NSUInteger index = 0; index < 3; index++) {
		descriptor.attributes[index].format = vertex_formats[index];
		descriptor.attributes[index].offset = vertex_offsets[index];
		descriptor.attributes[index].bufferIndex = 0;
	}
	descriptor.attributes[15].format = MTLVertexFormatFloat4;
	descriptor.attributes[15].offset = 32;
	descriptor.attributes[15].bufferIndex = 0;
	descriptor.layouts[0].stride = sizeof(OnixMetalVertex3D);
	descriptor.layouts[0].stepFunction = MTLVertexStepFunctionPerVertex;
	const NSUInteger instance_offsets[] = {0, 16, 32, 48, 64, 80, 96, 112, 128, 144, 160, 176};
	for (NSUInteger index = 0; index < 12; index++) {
		descriptor.attributes[index + 3].format = MTLVertexFormatFloat4;
		descriptor.attributes[index + 3].offset = instance_offsets[index];
		descriptor.attributes[index + 3].bufferIndex = 1;
	}
	descriptor.layouts[1].stride = sizeof(OnixMetalInstance3D);
	descriptor.layouts[1].stepFunction = MTLVertexStepFunctionPerInstance;
	descriptor.layouts[1].stepRate = 1;
	return descriptor;
}

static id<MTLRenderPipelineState> onix_metal_pipeline(
    OnixMetalDriver *driver,
	id<MTLLibrary> vertex_library,
	id<MTLLibrary> fragment_library,
	NSString *vertex_name,
	NSString *fragment_name,
	MTLVertexDescriptor *vertex_descriptor,
	int blend_mode,
	int depth) {
	id<MTLFunction> vertex = [vertex_library newFunctionWithName:vertex_name];
	id<MTLFunction> fragment = [fragment_library newFunctionWithName:fragment_name];
    if (vertex == nil || fragment == nil) {
        onix_metal_set_error("Metal shader function is missing");
        [vertex release];
        [fragment release];
        return nil;
    }
    MTLRenderPipelineDescriptor *descriptor = [[[MTLRenderPipelineDescriptor alloc] init] autorelease];
    descriptor.vertexFunction = vertex;
    descriptor.fragmentFunction = fragment;
    descriptor.vertexDescriptor = vertex_descriptor;
    BOOL picking = [vertex_name isEqualToString:@"mesh_picking_vertex"];
    descriptor.colorAttachments[0].pixelFormat = depth == 2 ? MTLPixelFormatR8Unorm : picking ? MTLPixelFormatR32Uint : MTLPixelFormatBGRA8Unorm;
	descriptor.depthAttachmentPixelFormat = depth == 1 ? MTLPixelFormatDepth32Float : MTLPixelFormatInvalid;
	descriptor.colorAttachments[0].blendingEnabled = picking ? NO : (blend_mode < 0 ? NO : YES);
    descriptor.colorAttachments[0].sourceRGBBlendFactor = MTLBlendFactorOne;
    descriptor.colorAttachments[0].destinationRGBBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
	descriptor.colorAttachments[0].sourceAlphaBlendFactor = MTLBlendFactorOne;
	descriptor.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
	if (picking) {
		descriptor.colorAttachments[0].sourceRGBBlendFactor = MTLBlendFactorOne;
		descriptor.colorAttachments[0].destinationRGBBlendFactor = MTLBlendFactorZero;
		descriptor.colorAttachments[0].sourceAlphaBlendFactor = MTLBlendFactorOne;
		descriptor.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorZero;
	}
	if (blend_mode == 1) {
		descriptor.colorAttachments[0].sourceRGBBlendFactor = MTLBlendFactorBlendColor;
		descriptor.colorAttachments[0].destinationRGBBlendFactor = MTLBlendFactorOneMinusSourceColor;
		descriptor.colorAttachments[0].sourceAlphaBlendFactor = MTLBlendFactorZero;
		descriptor.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOne;
	}
    NSError *error = nil;
    id<MTLRenderPipelineState> pipeline = [driver->device newRenderPipelineStateWithDescriptor:descriptor error:&error];
    if (pipeline == nil) {
        onix_metal_set_error("Metal pipeline creation failed: %s", error.localizedDescription.UTF8String);
    }
    [vertex release];
    [fragment release];
    return pipeline;
}

static id<MTLLibrary> onix_metal_mesh_library(OnixMetalDriver *driver, int variant) {
    if (driver->mesh_libraries[variant] == nil) {
        NSError *error = nil;
        NSString *source = [NSString stringWithUTF8String:onix_metal_mesh_sources[variant]];
        driver->mesh_libraries[variant] = [driver->device newLibraryWithSource:source options:nil error:&error];
        if (driver->mesh_libraries[variant] == nil) onix_metal_set_error("Metal mesh compilation failed: %s", error.localizedDescription.UTF8String);
    }
    return driver->mesh_libraries[variant];
}

uint32_t onix_metal_create_mesh_shader(OnixMetalDriver *driver, const char *source, size_t length) {
	if (driver == NULL || source == NULL || length == 0) {
		onix_metal_set_error("invalid Metal mesh shader");
		return 0;
	}
	@autoreleasepool {
		NSString *text = [[[NSString alloc] initWithBytes:source length:length encoding:NSUTF8StringEncoding] autorelease];
		if (text == nil) {
			onix_metal_set_error("Metal mesh shader is not valid UTF-8");
			return 0;
		}
		NSError *error = nil;
		id<MTLLibrary> library = [driver->device newLibraryWithSource:text options:nil error:&error];
		if (library == nil) {
			onix_metal_set_error("Metal shader compilation failed: %s", error.localizedDescription.UTF8String);
			return 0;
		}
		id<MTLFunction> custom_vertex = [library newFunctionWithName:@"mesh_vertex"];
		id<MTLFunction> custom_fragment = [library newFunctionWithName:@"mesh_fragment"];
		id<MTLLibrary> vertex_library = custom_vertex == nil ? driver->default_library : library;
		id<MTLLibrary> fragment_library = custom_fragment == nil && custom_vertex != nil ? onix_metal_mesh_library(driver, ONIX_MESH_FULL) : library;
		[custom_vertex release];
		[custom_fragment release];
		if ((custom_vertex == nil && custom_fragment == nil) || fragment_library == nil) {
			[library release];
			onix_metal_set_error("Metal mesh shader must export mesh_vertex or mesh_fragment");
			return 0;
		}
		uint32_t index;
		if (!onix_metal_mesh_shaders_acquire(&driver->mesh_pipelines, &driver->mesh_shader_slots, &index)) {
			[library release];
			onix_metal_set_error("failed to grow Metal mesh shader storage");
			return 0;
		}
		OnixMetalMeshPipeline result = {.vertex_library = [vertex_library retain], .fragment_library = [fragment_library retain]};
		[library release];
		onix_metal_mesh_shaders_insert(driver->mesh_pipelines, &driver->mesh_shader_slots, index, result);
		return index + 1;
	}
}

int onix_metal_destroy_mesh_shader(OnixMetalDriver *driver, uint32_t shader) {
	if (driver == NULL || shader == 0 || shader > driver->mesh_shader_slots.count) {
		onix_metal_set_error("invalid Metal mesh shader");
		return 0;
	}
	OnixMetalMeshPipeline *pipeline = &driver->mesh_pipelines[shader - 1];
	if (pipeline->vertex_library == nil) {
		onix_metal_set_error("Metal mesh shader is already destroyed");
		return 0;
	}
	uint32_t frame_index = driver->encoder != nil
		? driver->frame_index
		: (driver->frame_index + ONIX_METAL_FRAMES_IN_FLIGHT - 1) % ONIX_METAL_FRAMES_IN_FLIGHT;
	if (pipeline->opaque != nil) [driver->frame_resources[frame_index] addObject:pipeline->opaque];
	for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) if (pipeline->transparent[blend] != nil) [driver->frame_resources[frame_index] addObject:pipeline->transparent[blend]];
	[pipeline->vertex_library release];
	[pipeline->fragment_library release];
	[pipeline->opaque release];
	for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) [pipeline->transparent[blend] release];
	onix_metal_mesh_shaders_recycle(driver->mesh_pipelines, &driver->mesh_shader_slots, shader - 1);
	return 1;
}

uint32_t onix_metal_create_material2d_shader(OnixMetalDriver *driver, const char *source, size_t length) {
	if (driver == NULL || source == NULL || length == 0) {
		onix_metal_set_error("invalid Metal Material2D shader");
		return 0;
	}
	@autoreleasepool {
		NSString *text = [[[NSString alloc] initWithBytes:source length:length encoding:NSUTF8StringEncoding] autorelease];
		if (text == nil) {
			onix_metal_set_error("Metal Material2D shader is not valid UTF-8");
			return 0;
		}
		NSError *error = nil;
		id<MTLLibrary> library = [driver->device newLibraryWithSource:text options:nil error:&error];
		if (library == nil) {
			onix_metal_set_error("Metal Material2D shader compilation failed: %s", error.localizedDescription.UTF8String);
			return 0;
		}
		id<MTLFunction> custom_vertex = [library newFunctionWithName:@"sprite_vertex"];
		id<MTLFunction> custom_fragment = [library newFunctionWithName:@"sprite_fragment"];
		id<MTLLibrary> vertex_library = custom_vertex == nil ? driver->default_library : library;
		id<MTLLibrary> fragment_library = custom_fragment == nil ? driver->default_library : library;
		[custom_vertex release];
		[custom_fragment release];
		if (vertex_library == driver->default_library && fragment_library == driver->default_library) {
			[library release];
			onix_metal_set_error("Metal Material2D shader must export sprite_vertex or sprite_fragment");
			return 0;
		}
		OnixMetalMaterial2DPipeline result = {
			.vertex_library = [vertex_library retain], .fragment_library = [fragment_library retain],
		};
		for (uint32_t depth = 0; depth < 2; depth++) {
			result.pipelines[depth] = onix_metal_pipeline(driver, vertex_library, fragment_library, @"sprite_vertex", @"sprite_fragment", onix_metal_sprite_vertex_descriptor(), 0, depth);
			if (result.pipelines[depth] == nil) break;
		}
		[library release];
		if (result.pipelines[0] == nil || result.pipelines[1] == nil) {
			for (uint32_t depth = 0; depth < 3; depth++) [result.pipelines[depth] release];
			[result.vertex_library release];
			[result.fragment_library release];
			return 0;
		}
		uint32_t index;
		if (!onix_metal_material2d_shaders_acquire(&driver->material2d_pipelines, &driver->material2d_shader_slots, &index)) {
			for (uint32_t depth = 0; depth < 3; depth++) [result.pipelines[depth] release];
			[result.vertex_library release];
			[result.fragment_library release];
			onix_metal_set_error("failed to grow Metal Material2D shader storage");
			return 0;
		}
		onix_metal_material2d_shaders_insert(driver->material2d_pipelines, &driver->material2d_shader_slots, index, result);
		return index + 1;
	}
}

int onix_metal_destroy_material2d_shader(OnixMetalDriver *driver, uint32_t shader) {
	if (driver == NULL || shader == 0 || shader > driver->material2d_shader_slots.count || driver->material2d_pipelines[shader - 1].pipelines[0] == nil) {
		onix_metal_set_error("invalid Metal Material2D shader");
		return 0;
	}
	uint32_t frame_index = driver->encoder != nil
		? driver->frame_index
		: (driver->frame_index + ONIX_METAL_FRAMES_IN_FLIGHT - 1) % ONIX_METAL_FRAMES_IN_FLIGHT;
	for (uint32_t depth = 0; depth < 3; depth++) {
		id<MTLRenderPipelineState> pipeline = driver->material2d_pipelines[shader - 1].pipelines[depth];
		if (pipeline != nil) [driver->frame_resources[frame_index] addObject:pipeline];
		[pipeline release];
	}
	[driver->material2d_pipelines[shader - 1].vertex_library release];
	[driver->material2d_pipelines[shader - 1].fragment_library release];
	onix_metal_material2d_shaders_recycle(driver->material2d_pipelines, &driver->material2d_shader_slots, shader - 1);
	return 1;
}

static int onix_metal_create_depth_texture(OnixMetalDriver *driver, uint32_t width, uint32_t height) {
	MTLTextureDescriptor *descriptor = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatDepth32Float width:width height:height mipmapped:NO];
	descriptor.usage = MTLTextureUsageRenderTarget;
	descriptor.storageMode = MTLStorageModePrivate;
	id<MTLTexture> texture = [driver->device newTextureWithDescriptor:descriptor];
	if (texture == nil) {
		onix_metal_set_error("Metal depth texture allocation failed");
		return 0;
	}
	[driver->depth_texture release];
	driver->depth_texture = texture;
	return 1;
}

// Resume the surface without discarding any preceding 2D or 3D draws.
static int onix_metal_resume_surface(OnixMetalDriver *driver, MTLLoadAction depth_load) {
	MTLRenderPassDescriptor *pass = driver->surface_pass;
	pass.colorAttachments[0].texture = driver->drawable.texture;
	pass.colorAttachments[0].loadAction = MTLLoadActionLoad;
	pass.colorAttachments[0].storeAction = MTLStoreActionStore;
	pass.depthAttachment.texture = driver->depth_texture;
	pass.depthAttachment.loadAction = depth_load;
	pass.depthAttachment.clearDepth = 0.0;
	pass.depthAttachment.storeAction = MTLStoreActionStore;
	driver->encoder = [[driver->command_buffer renderCommandEncoderWithDescriptor:pass] retain];
	pass.colorAttachments[0].texture = nil;
	pass.depthAttachment.texture = nil;
	if (driver->encoder == nil) {
		onix_metal_set_error("Metal surface encoder restoration failed");
		return 0;
	}
	return 1;
}

static int onix_metal_has_depth(OnixMetalDriver *driver) {
	return driver->active_render_target == 0 ? driver->depth_texture != nil
		: driver->render_targets[driver->active_render_target - 1].depth_texture != nil;
}

static int onix_metal_2d_attachment(OnixMetalDriver *driver) {
	if (driver->active_render_target != 0 && driver->render_targets[driver->active_render_target - 1].color_format == 2) return 2;
	return onix_metal_has_depth(driver);
}

static int onix_metal_require_depth(OnixMetalDriver *driver) {
	if (driver->active_render_target != 0 && !onix_metal_has_depth(driver)) {
		onix_metal_set_error("Metal mesh drawing requires a render target with depth");
		return 0;
	}
	if (driver->mesh_opaque_depth == nil) {
		MTLDepthStencilDescriptor *descriptor = [[[MTLDepthStencilDescriptor alloc] init] autorelease];
		descriptor.depthCompareFunction = MTLCompareFunctionGreater;
		descriptor.depthWriteEnabled = YES;
		driver->mesh_opaque_depth = [driver->device newDepthStencilStateWithDescriptor:descriptor];
		if (driver->mesh_opaque_depth == nil) {
			onix_metal_set_error("Metal depth state allocation failed");
			return 0;
		}
	}
	if (driver->mesh_transparent_depth == nil) {
		MTLDepthStencilDescriptor *descriptor = [[[MTLDepthStencilDescriptor alloc] init] autorelease];
		descriptor.depthCompareFunction = MTLCompareFunctionGreater;
		descriptor.depthWriteEnabled = NO;
		driver->mesh_transparent_depth = [driver->device newDepthStencilStateWithDescriptor:descriptor];
		if (driver->mesh_transparent_depth == nil) {
			onix_metal_set_error("Metal depth state allocation failed");
			return 0;
		}
	}
	if (driver->active_render_target != 0 || driver->depth_texture != nil) return 1;
	if (!onix_metal_create_depth_texture(driver, driver->drawable.texture.width, driver->drawable.texture.height)) return 0;
	[driver->encoder endEncoding];
	[driver->encoder release];
	driver->encoder = nil;
	return onix_metal_resume_surface(driver, MTLLoadActionClear);
}

static id<MTLRenderPipelineState> onix_metal_mesh_pipeline(OnixMetalDriver *driver, int transparent, uint32_t blend, int picking, int variant) {
	id<MTLRenderPipelineState> *pipeline = picking ? &driver->mesh_picking_pipeline
		: variant != ONIX_MESH_FULL ? &driver->mesh_variants[variant-1][transparent ? 1+blend : 0]
		: (transparent ? &driver->mesh_transparent_pipelines[blend] : &driver->mesh_opaque_pipeline);
	if (*pipeline == nil) {
		id<MTLLibrary> fragment = picking ? driver->default_library : onix_metal_mesh_library(driver, variant);
		if (fragment == nil) return nil;
		*pipeline = onix_metal_pipeline(driver, driver->default_library, fragment,
			picking ? @"mesh_picking_vertex" : @"mesh_vertex", picking ? @"mesh_picking_fragment" : @"mesh_fragment",
			onix_metal_mesh_vertex_descriptor(), transparent ? (int)blend : -1, 1);
	}
	return *pipeline;
}

OnixMetalDriver *onix_metal_create(void *layer_pointer, uint32_t width, uint32_t height) {
    @autoreleasepool {
        onix_metal_error[0] = '\0';
        CAMetalLayer *layer = (CAMetalLayer *)layer_pointer;
        if (layer == nil || width == 0 || height == 0) {
            onix_metal_set_error("invalid CAMetalLayer");
            return NULL;
        }
        OnixMetalDriver *driver = calloc(1, sizeof(OnixMetalDriver));
        if (driver == NULL) {
            onix_metal_set_error("failed to allocate Metal driver");
            return NULL;
        }
        driver->layer = [layer retain];
        driver->device = MTLCreateSystemDefaultDevice();
        if (driver->device == nil) {
            onix_metal_set_error("Metal device is unavailable");
            onix_metal_destroy(driver);
            return NULL;
        }
        driver->layer.device = driver->device;
        driver->layer.pixelFormat = MTLPixelFormatBGRA8Unorm;
        driver->layer.framebufferOnly = YES;
        driver->layer.maximumDrawableCount = ONIX_METAL_FRAMES_IN_FLIGHT;
        driver->layer.drawableSize = CGSizeMake(width, height);
        MTLSamplerAddressMode modes[] = {MTLSamplerAddressModeClampToEdge, MTLSamplerAddressModeRepeat, MTLSamplerAddressModeMirrorRepeat};
        MTLSamplerDescriptor *sampler = [[MTLSamplerDescriptor alloc] init];
        sampler.minFilter = MTLSamplerMinMagFilterLinear;
        sampler.magFilter = MTLSamplerMinMagFilterLinear;
        for (int mipmap = 0; mipmap < 2; mipmap++) {
            sampler.mipFilter = mipmap ? MTLSamplerMipFilterLinear : MTLSamplerMipFilterNotMipmapped;
            for (int u = 0; u < 3; u++) {
                for (int v = 0; v < 3; v++) {
                    sampler.sAddressMode = modes[u];
                    sampler.tAddressMode = modes[v];
                    int index = mipmap * 9 + u * 3 + v;
                    driver->samplers[index] = [driver->device newSamplerStateWithDescriptor:sampler];
                    if (driver->samplers[index] == nil) {
                        [sampler release];
                        onix_metal_set_error("Metal sampler allocation failed");
                        onix_metal_destroy(driver);
                        return NULL;
                    }
                }
            }
        }
        [sampler release];
        driver->queue = [driver->device newCommandQueue];
		driver->surface_pass = [[MTLRenderPassDescriptor alloc] init];
		driver->target_pass = [[MTLRenderPassDescriptor alloc] init];
		if (CVMetalTextureCacheCreate(NULL, NULL, driver->device, NULL, &driver->texture_cache) != kCVReturnSuccess) {
			onix_metal_set_error("Metal CoreVideo texture cache allocation failed");
			onix_metal_destroy(driver);
			return NULL;
		}
		for (uint32_t index = 0; index < ONIX_METAL_FRAMES_IN_FLIGHT; index++) {
			driver->frame_resources[index] = [[NSMutableArray alloc] init];
		}
		if (driver->surface_pass == nil || driver->target_pass == nil || driver->queue == nil || driver->frame_resources[0] == nil || driver->frame_resources[1] == nil) {
			onix_metal_set_error("Metal command queue allocation failed");
			onix_metal_destroy(driver);
			return NULL;
		}

        NSError *error = nil;
        NSString *source = [NSString stringWithUTF8String:onix_metal_source];
        id<MTLLibrary> library = [driver->device newLibraryWithSource:source options:nil error:&error];
        if (library == nil) {
            onix_metal_set_error("Metal shader compilation failed: %s", error.localizedDescription.UTF8String);
            onix_metal_destroy(driver);
            return NULL;
        }
	driver->default_library = library;
	driver->sprite_pipelines[0][0] = onix_metal_pipeline(driver, library, library, @"sprite_multi_vertex", @"sprite_multi_fragment", onix_metal_compact_sprite_vertex_descriptor(), 0, 0);
	driver->shape_pipelines[0] = onix_metal_pipeline(driver, library, library, @"shape_vertex", @"shape_fragment", onix_metal_shape_vertex_descriptor(), 0, 0);
	if (driver->sprite_pipelines[0][0] == nil || driver->shape_pipelines[0] == nil) {
            onix_metal_destroy(driver);
            return NULL;
        }
		MTLDepthStencilDescriptor *depth_disabled = [[[MTLDepthStencilDescriptor alloc] init] autorelease];
		depth_disabled.depthCompareFunction = MTLCompareFunctionAlways;
		depth_disabled.depthWriteEnabled = NO;
		driver->depth_disabled = [driver->device newDepthStencilStateWithDescriptor:depth_disabled];
		if (driver->depth_disabled == nil) {
			onix_metal_set_error("Metal disabled depth state allocation failed");
			onix_metal_destroy(driver);
			return NULL;
		}
        const float vertices[] = {
            0, 0, 0, 0, 0,
            1, 0, 0, 1, 0,
            1, 1, 0, 1, 1,
            0, 1, 0, 0, 1,
        };
        const uint16_t indices[] = {0, 1, 2, 0, 2, 3};
        driver->quad_vertices = [driver->device newBufferWithBytes:vertices length:sizeof(vertices) options:MTLResourceStorageModeShared];
        driver->quad_indices = [driver->device newBufferWithBytes:indices length:sizeof(indices) options:MTLResourceStorageModeShared];
		if (driver->quad_vertices == nil || driver->quad_indices == nil) {
			onix_metal_set_error("Metal geometry allocation failed");
			onix_metal_destroy(driver);
			return NULL;
		}
        driver->viewport = (MTLViewport){0, 0, width, height, 0, 1};
        return driver;
    }
}

const char *onix_metal_device_name(OnixMetalDriver *driver) {
	return driver == NULL || driver->device == nil ? "" : driver->device.name.UTF8String;
}

void onix_metal_destroy(OnixMetalDriver *driver) {
    if (driver == NULL) return;
	if (driver->encoder != nil) {
		[driver->encoder endEncoding];
		[driver->encoder release];
	}
	if (driver->command_buffer != nil) {
		[driver->command_buffer commit];
		[driver->command_buffer waitUntilCompleted];
		[driver->command_buffer release];
	}
	[driver->drawable release];
	for (uint32_t index = 0; index < ONIX_METAL_FRAMES_IN_FLIGHT; index++) {
		if (driver->inflight_commands[index] != nil) {
			[driver->inflight_commands[index] waitUntilCompleted];
			[driver->inflight_commands[index] release];
		}
		OnixMetalUploadFrame *frame = &driver->uploads[index];
		for (uint32_t block = 0; block < frame->count; block++) [frame->blocks[block].buffer release];
		free(frame->blocks);
		onix_upload_cache_destroy(&frame->cache);
	}
	for (uint32_t index = 0; index < driver->target_slots.count; index++) {
		[driver->render_targets[index].depth_texture release];
	}
    for (uint32_t index = 0; index < driver->texture_slots.count; index++) {
		[driver->textures[index].texture release];
		if (driver->textures[index].external != NULL) CFRelease(driver->textures[index].external);
	}
	if (driver->texture_cache != NULL) CFRelease(driver->texture_cache);
	[driver->readback_buffer release];
	[driver->quad_vertices release];
    [driver->quad_indices release];
	for (uint32_t depth = 0; depth < 3; depth++) {
		for (uint32_t blend = 0; blend < 2; blend++) [driver->sprite_pipelines[depth][blend] release];
		[driver->shape_pipelines[depth] release];
	}
    for (uint32_t variant = 0; variant < ONIX_MESH_VARIANT_COUNT; variant++) {
        [driver->mesh_libraries[variant] release];
        if (variant) for (uint32_t blend = 0; blend <= ONIX_MESH_BLEND_MODE_COUNT; blend++) [driver->mesh_variants[variant-1][blend] release];
    }
	[driver->mesh_opaque_pipeline release];
	[driver->mesh_picking_pipeline release];
	for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) [driver->mesh_transparent_pipelines[blend] release];
	[driver->default_library release];
	for (uint32_t index = 0; index < driver->mesh_shader_slots.count; index++) {
		[driver->mesh_pipelines[index].vertex_library release];
		[driver->mesh_pipelines[index].fragment_library release];
		[driver->mesh_pipelines[index].opaque release];
		for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) [driver->mesh_pipelines[index].transparent[blend] release];
	}
	for (uint32_t index = 0; index < driver->material2d_shader_slots.count; index++) {
		for (uint32_t depth = 0; depth < 3; depth++) [driver->material2d_pipelines[index].pipelines[depth] release];
		[driver->material2d_pipelines[index].vertex_library release];
		[driver->material2d_pipelines[index].fragment_library release];
	}
	for (uint32_t index = 0; index < driver->mesh_slots.count; index++) {
		[driver->meshes[index].vertices release];
		[driver->meshes[index].indices release];
		[driver->meshes[index].deformation release];
	}
	[driver->mesh_opaque_depth release];
	[driver->mesh_transparent_depth release];
	[driver->depth_disabled release];
	[driver->depth_texture release];
	[driver->surface_pass release];
	[driver->target_pass release];
	for (uint32_t index = 0; index < ONIX_METAL_FRAMES_IN_FLIGHT; index++) {
		[driver->frame_resources[index] release];
	}
    [driver->queue release];
    for (int i = 0; i < 18; i++) [driver->samplers[i] release];
    [driver->device release];
	[driver->layer release];
	free(driver->textures);
	free(driver->render_targets);
	free(driver->meshes);
	free(driver->mesh_pipelines);
	free(driver->material2d_pipelines);
	free(driver);
}

// Idle cleanup is independent of frame count. It waits for every in-flight
// slot before releasing transient buffers or resources retired by that slot.
int onix_metal_trim_idle(OnixMetalDriver *driver) {
 if (driver == NULL) return 1;
 if (driver->command_buffer != nil || driver->encoder != nil) {
  onix_metal_set_error("cannot trim buffers inside a Metal frame"); return 0;
 }
 @autoreleasepool {
  for (uint32_t index=0; index<ONIX_METAL_FRAMES_IN_FLIGHT; index++) {
   if (driver->inflight_commands[index] != nil) {
    [driver->inflight_commands[index] waitUntilCompleted];
    [driver->inflight_commands[index] release];
    driver->inflight_commands[index]=nil;
   }
   [driver->frame_resources[index] removeAllObjects];
   OnixMetalUploadFrame *frame=&driver->uploads[index];
   for (uint32_t block=0; block<frame->count; block++) [frame->blocks[block].buffer release];
   free(frame->blocks); onix_upload_cache_destroy(&frame->cache);
   *frame=(OnixMetalUploadFrame){0};
  }
  [driver->readback_buffer release]; driver->readback_buffer=nil;
  driver->readback_capacity=0; driver->readback_used=0;
  driver->readback_retention=(OnixBufferRetention){0};
 }
 return 1;
}

int onix_metal_resize(OnixMetalDriver *driver, uint32_t width, uint32_t height) {
    if (driver == NULL || width == 0 || height == 0) return driver != NULL;
	for (uint32_t index = 0; index < ONIX_METAL_FRAMES_IN_FLIGHT; index++) {
		if (driver->inflight_commands[index] != nil) [driver->inflight_commands[index] waitUntilCompleted];
	}
    driver->layer.drawableSize = CGSizeMake(width, height);
	return driver->depth_texture == nil || onix_metal_create_depth_texture(driver, width, height);
}

void onix_metal_set_vsync(OnixMetalDriver *driver, int enabled) {
#if TARGET_OS_OSX
	if (driver != NULL) driver->layer.displaySyncEnabled = enabled != 0;
#else
	(void)driver;
	(void)enabled;
#endif
}

// The caller has completed this slot's command buffer before releasing storage.
static void onix_metal_reset_uploads(OnixMetalUploadFrame *frame) {
	onix_upload_cache_reset(&frame->cache);
	NSUInteger budget = (NSUInteger)onix_buffer_trim_capacity(&frame->retention,
		frame->bytes, frame->used, ONIX_METAL_INSTANCE_BUFFER_MINIMUM);
	NSUInteger retained = 0;
	uint32_t keep = 0;
	for (uint32_t index = 0; index < frame->count; index++) {
		OnixMetalUploadBlock block = frame->blocks[index];
		if (budget && block.capacity > budget - retained) {
			[block.buffer release];
			continue;
		}
		block.offset = 0;
		retained += block.capacity;
		frame->blocks[keep++] = block;
	}
	frame->count = keep;
	frame->blocks = onix_buffer_trim_array(frame->blocks, &frame->capacity, keep,
		sizeof(*frame->blocks), 4, &frame->table_retention);
	frame->bytes = retained;
	frame->used = 0;
	frame->index = 0;
}

static void onix_metal_trim_readback(OnixMetalDriver *driver) {
	if (onix_buffer_trim_capacity(&driver->readback_retention, driver->readback_capacity, driver->readback_used, 64u * 1024u)) {
		[driver->readback_buffer release];
		driver->readback_buffer = nil;
		driver->readback_capacity = 0;
	}
	driver->readback_used = 0;
}

int onix_metal_begin_frame(OnixMetalDriver *driver, const float clear_color[4]) {
	@autoreleasepool {
		if (driver == NULL || driver->encoder != nil || driver->active_render_target != 0) return driver != NULL;
		onix_metal_trim_readback(driver);
		uint32_t frame_index = driver->frame_index;
		if (driver->inflight_commands[frame_index] != nil) {
			[driver->inflight_commands[frame_index] waitUntilCompleted];
			[driver->inflight_commands[frame_index] release];
			driver->inflight_commands[frame_index] = nil;
			[driver->frame_resources[frame_index] removeAllObjects];
		}
		onix_metal_reset_uploads(&driver->uploads[frame_index]);
		driver->drawable = [[driver->layer nextDrawable] retain];
        if (driver->drawable == nil) {
            onix_metal_set_error("CAMetalLayer returned no drawable");
            return 0;
        }
		driver->command_buffer = [[driver->queue commandBuffer] retain];
		if (driver->command_buffer == nil) {
			onix_metal_set_error("Metal command buffer allocation failed");
			[driver->drawable release];
			driver->drawable = nil;
			return 0;
		}
        MTLRenderPassDescriptor *pass = driver->surface_pass;
        pass.colorAttachments[0].texture = driver->drawable.texture;
        pass.colorAttachments[0].loadAction = MTLLoadActionClear;
        pass.colorAttachments[0].storeAction = MTLStoreActionStore;
        pass.colorAttachments[0].clearColor = MTLClearColorMake(clear_color[0], clear_color[1], clear_color[2], clear_color[3]);
		pass.depthAttachment.texture = driver->depth_texture;
		pass.depthAttachment.loadAction = MTLLoadActionClear;
		pass.depthAttachment.storeAction = MTLStoreActionStore;
		pass.depthAttachment.clearDepth = 0.0;
		driver->encoder = [[driver->command_buffer renderCommandEncoderWithDescriptor:pass] retain];
		pass.colorAttachments[0].texture = nil;
		pass.depthAttachment.texture = nil;
        if (driver->encoder == nil) {
            onix_metal_set_error("Metal render encoder creation failed");
			[driver->command_buffer release];
			[driver->drawable release];
			driver->command_buffer = nil;
			driver->drawable = nil;
            return 0;
        }
        return 1;
    }
}

void onix_metal_set_sprite_blend(OnixMetalDriver *driver, int mode, float red, float green, float blue, float alpha) {
	if (driver == NULL) return;
	driver->sprite_blend = mode == 1;
	driver->blend_constant[0] = red;
	driver->blend_constant[1] = green;
	driver->blend_constant[2] = blue;
	driver->blend_constant[3] = alpha;
}

void onix_metal_set_viewport(OnixMetalDriver *driver, int32_t x, int32_t y, int32_t width, int32_t height) {
    if (driver == NULL || width <= 0 || height <= 0) return;
    driver->viewport = (MTLViewport){x, y, width, height, 0, 1};
}

void onix_metal_set_scissor(OnixMetalDriver *driver, int enabled, int32_t x, int32_t y, int32_t width, int32_t height) {
    if (driver == NULL) return;
    driver->scissor_enabled = enabled;
    if (enabled) {
		int64_t right = (int64_t)x + width;
		int64_t bottom = (int64_t)y + height;
		int64_t left = x > 0 ? x : 0;
		int64_t top = y > 0 ? y : 0;
		if (right < left) right = left;
		if (bottom < top) bottom = top;
        driver->scissor = (MTLScissorRect){
			(NSUInteger)left,
			(NSUInteger)top,
			(NSUInteger)(right - left),
			(NSUInteger)(bottom - top),
        };
    }
}

static void onix_metal_apply_dynamic_state(OnixMetalDriver *driver) {
	[driver->encoder setBlendColorRed:driver->blend_constant[0] green:driver->blend_constant[1] blue:driver->blend_constant[2] alpha:driver->blend_constant[3]];
    [driver->encoder setViewport:driver->viewport];
	NSUInteger max_width = (NSUInteger)driver->layer.drawableSize.width;
	NSUInteger max_height = (NSUInteger)driver->layer.drawableSize.height;
	if (driver->active_render_target > 0 && driver->active_render_target <= driver->target_slots.count) {
		OnixMetalRenderTarget *target = &driver->render_targets[driver->active_render_target - 1];
		max_width = target->width;
		max_height = target->height;
	}
    if (driver->scissor_enabled) {
        MTLScissorRect scissor = driver->scissor;
		if (scissor.x > max_width) scissor.x = max_width;
		if (scissor.y > max_height) scissor.y = max_height;
        if (scissor.x + scissor.width > max_width) scissor.width = max_width > scissor.x ? max_width - scissor.x : 0;
        if (scissor.y + scissor.height > max_height) scissor.height = max_height > scissor.y ? max_height - scissor.y : 0;
        [driver->encoder setScissorRect:scissor];
    } else {
		[driver->encoder setScissorRect:(MTLScissorRect){0, 0, max_width, max_height}];
    }
}

int onix_metal_present(OnixMetalDriver *driver) {
	if (driver == NULL || driver->encoder == nil || driver->active_render_target != 0) {
		if (driver != NULL && driver->active_render_target != 0) onix_metal_set_error("cannot present while a Metal render target is active");
		return driver != NULL && driver->active_render_target == 0;
	}
	[driver->encoder endEncoding];
	[driver->command_buffer presentDrawable:driver->drawable];
	[driver->command_buffer commit];
	driver->inflight_commands[driver->frame_index] = driver->command_buffer;
	[driver->encoder release];
	[driver->drawable release];
	driver->encoder = nil;
	driver->command_buffer = nil;
	driver->drawable = nil;
	driver->frame_index = (driver->frame_index + 1) % ONIX_METAL_FRAMES_IN_FLIGHT;
	return 1;
}

static uint32_t onix_metal_texture_slot(OnixMetalDriver *driver) {
	uint32_t slot;
	return onix_metal_textures_acquire(&driver->textures, &driver->texture_slots, &slot) ? slot : UINT32_MAX;
}

static void onix_metal_generate_mipmaps(OnixMetalDriver *driver, id<MTLTexture> texture) {
	if (texture.mipmapLevelCount <= 1) return;
	id<MTLCommandBuffer> command = [driver->queue commandBuffer];
	id<MTLBlitCommandEncoder> blit = [command blitCommandEncoder];
	[blit generateMipmapsForTexture:texture];
	[blit endEncoding];
	[command commit];
}

static uint32_t onix_metal_texture_mip_levels(uint32_t width, uint32_t height) {
	uint32_t levels = 1;
	while (width > 1 || height > 1) {
		width = width > 1 ? width / 2 : 1;
		height = height > 1 ? height / 2 : 1;
		levels++;
	}
	return levels;
}

uint32_t onix_metal_create_texture_with_format(OnixMetalDriver *driver, uint32_t width, uint32_t height, uint32_t levels, uint32_t format, int mipmap, const uint8_t *pixels, size_t length, uint32_t wrap_u, uint32_t wrap_v) {
	int compressed = format >= ONIX_TEXTURE_FORMAT_BC1;
	if (levels == 0) levels = 1;
	if (wrap_u > 2 || wrap_v > 2 || mipmap < 0 || mipmap > 1 || driver == NULL || width == 0 || height == 0 || pixels == NULL ||
		format >= ONIX_TEXTURE_FORMAT_COUNT || levels > onix_metal_texture_mip_levels(width, height)) {
        onix_metal_set_error("invalid Metal texture data");
        return 0;
    }
	size_t bytes_per_pixel = format == ONIX_TEXTURE_FORMAT_R8 ? 1 : 4;
	MTLPixelFormat pixel_format = MTLPixelFormatRGBA8Unorm;
	size_t expected = (size_t)width * height * bytes_per_pixel;
	switch (format) {
	case ONIX_TEXTURE_FORMAT_R8: pixel_format = MTLPixelFormatR8Unorm; break;
	case ONIX_TEXTURE_FORMAT_SRGBA8: pixel_format = MTLPixelFormatRGBA8Unorm_sRGB; break;
	case ONIX_TEXTURE_FORMAT_BC1: pixel_format = MTLPixelFormatBC1_RGBA; break;
	case ONIX_TEXTURE_FORMAT_BC1_SRGB: pixel_format = MTLPixelFormatBC1_RGBA_sRGB; break;
	case ONIX_TEXTURE_FORMAT_BC3: pixel_format = MTLPixelFormatBC3_RGBA; break;
	case ONIX_TEXTURE_FORMAT_BC3_SRGB: pixel_format = MTLPixelFormatBC3_RGBA_sRGB; break;
	case ONIX_TEXTURE_FORMAT_BC4: pixel_format = MTLPixelFormatBC4_RUnorm; break;
	case ONIX_TEXTURE_FORMAT_BC5: pixel_format = MTLPixelFormatBC5_RGUnorm; break;
	case ONIX_TEXTURE_FORMAT_BC7: pixel_format = MTLPixelFormatBC7_RGBAUnorm; break;
	case ONIX_TEXTURE_FORMAT_BC7_SRGB: pixel_format = MTLPixelFormatBC7_RGBAUnorm_sRGB; break;
	case ONIX_TEXTURE_FORMAT_ASTC_4X4: pixel_format = MTLPixelFormatASTC_4x4_LDR; break;
	case ONIX_TEXTURE_FORMAT_ASTC_4X4_SRGB: pixel_format = MTLPixelFormatASTC_4x4_sRGB; break;
	}
	if (compressed) {
		uint32_t block_bytes = (format == ONIX_TEXTURE_FORMAT_BC1 || format == ONIX_TEXTURE_FORMAT_BC1_SRGB || format == ONIX_TEXTURE_FORMAT_BC4) ? 8 : 16;
		expected = 0;
		uint32_t level_width = width, level_height = height;
		for (uint32_t level = 0; level < levels; level++) {
			expected += (size_t)((level_width + 3) / 4) * ((level_height + 3) / 4) * block_bytes;
			level_width = level_width > 1 ? level_width / 2 : 1;
			level_height = level_height > 1 ? level_height / 2 : 1;
		}
	}
	if (length != expected) {
		onix_metal_set_error("invalid Metal texture data");
		return 0;
	}
	MTLTextureDescriptor *descriptor = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:pixel_format width:width height:height mipmapped:levels > 1];
	descriptor.mipmapLevelCount = levels;
    descriptor.usage = MTLTextureUsageShaderRead;
    id<MTLTexture> texture = [driver->device newTextureWithDescriptor:descriptor];
    if (texture == nil) {
        onix_metal_set_error("Metal texture allocation failed");
        return 0;
	}
	if (compressed) {
		size_t offset = 0;
		uint32_t level_width = width, level_height = height;
		uint32_t block_bytes = (format == ONIX_TEXTURE_FORMAT_BC1 || format == ONIX_TEXTURE_FORMAT_BC1_SRGB || format == ONIX_TEXTURE_FORMAT_BC4) ? 8 : 16;
		for (uint32_t level = 0; level < levels; level++) {
			NSUInteger bytes_per_row = (NSUInteger)((level_width + 3) / 4) * block_bytes;
			size_t level_size = (size_t)bytes_per_row * ((level_height + 3) / 4);
			[texture replaceRegion:MTLRegionMake2D(0, 0, level_width, level_height) mipmapLevel:level withBytes:pixels + offset bytesPerRow:bytes_per_row];
			offset += level_size;
			level_width = level_width > 1 ? level_width / 2 : 1;
			level_height = level_height > 1 ? level_height / 2 : 1;
		}
	} else {
		[texture replaceRegion:MTLRegionMake2D(0, 0, width, height) mipmapLevel:0 withBytes:pixels bytesPerRow:width * bytes_per_pixel];
		onix_metal_generate_mipmaps(driver, texture);
	}
	uint32_t slot = onix_metal_texture_slot(driver);
	if (slot == UINT32_MAX) {
		[texture release];
		onix_metal_set_error("Metal texture table allocation failed");
		return 0;
	}
	onix_metal_textures_insert(driver->textures, &driver->texture_slots, slot, (OnixMetalTexture){.texture = texture, .format = format, .sampler = (uint32_t)mipmap*9+wrap_u*3+wrap_v});
	return slot + 1;
}

uint32_t onix_metal_create_external_cv_texture(OnixMetalDriver *driver, uintptr_t handle, uint32_t width, uint32_t height) {
	if (driver == NULL || driver->texture_cache == NULL || handle == 0 || width == 0 || height == 0) {
		onix_metal_set_error("invalid CoreVideo external texture");
		return 0;
	}
	CVPixelBufferRef pixel_buffer = (CVPixelBufferRef)handle;
	if (CVPixelBufferGetWidth(pixel_buffer) != width || CVPixelBufferGetHeight(pixel_buffer) != height ||
		CVPixelBufferGetPixelFormatType(pixel_buffer) != kCVPixelFormatType_32BGRA) {
		onix_metal_set_error("CoreVideo texture must be a matching BGRA pixel buffer");
		return 0;
	}
	CVMetalTextureRef external = NULL;
	CVReturn result = CVMetalTextureCacheCreateTextureFromImage(
		NULL, driver->texture_cache, pixel_buffer, NULL, MTLPixelFormatBGRA8Unorm,
		width, height, 0, &external);
	if (result != kCVReturnSuccess || external == NULL) {
		onix_metal_set_error("create Metal texture from CoreVideo failed (%d)", (int)result);
		return 0;
	}
	id<MTLTexture> texture = CVMetalTextureGetTexture(external);
	uint32_t slot = onix_metal_texture_slot(driver);
	if (texture == nil || slot == UINT32_MAX) {
		CFRelease(external);
		onix_metal_set_error("Metal external texture table allocation failed");
		return 0;
	}
	onix_metal_textures_insert(driver->textures, &driver->texture_slots, slot, (OnixMetalTexture){.texture = [texture retain], .format = ONIX_TEXTURE_FORMAT_RGBA8, .external = external});
	return slot + 1;
}

int onix_metal_refresh_external_texture(OnixMetalDriver *driver, uint32_t handle) {
	if (driver == NULL || handle == 0 || handle > driver->texture_slots.count ||
		driver->textures[handle - 1].texture == nil || driver->textures[handle - 1].external == NULL) {
		onix_metal_set_error("invalid Metal external texture refresh");
		return 0;
	}
	return 1;
}

int onix_metal_update_textures(OnixMetalDriver *driver, const OnixMetalTextureUpdate *updates, uint32_t update_count, const uint8_t *pixels, size_t length) {
	if (driver == NULL || updates == NULL || update_count == 0 || pixels == NULL) {
		onix_metal_set_error("invalid Metal texture updates");
		return 0;
	}
	for (uint32_t index = 0; index < update_count; index++) {
		const OnixMetalTextureUpdate *update = &updates[index];
		if (update->texture == 0 || update->texture > driver->texture_slots.count || update->x < 0 || update->y < 0 || update->width == 0 || update->height == 0 ||
			update->data_offset > length || update->data_length > length - update->data_offset) {
			onix_metal_set_error("invalid Metal texture update");
			return 0;
		}
		id<MTLTexture> texture = driver->textures[update->texture - 1].texture;
		size_t bytes_per_pixel = driver->textures[update->texture - 1].format == ONIX_TEXTURE_FORMAT_R8 ? 1 : 4;
		if (update->data_length != (uint64_t)update->width * update->height * bytes_per_pixel ||
			texture == nil || (uint64_t)(uint32_t)update->x + update->width > texture.width ||
			(uint64_t)(uint32_t)update->y + update->height > texture.height) {
			onix_metal_set_error("Metal texture update is outside the image");
			return 0;
		}
		driver->textures[update->texture - 1].last_update = index;
	}
	for (uint32_t index = 0; index < update_count; index++) {
		const OnixMetalTextureUpdate *update = &updates[index];
		id<MTLTexture> texture = driver->textures[update->texture - 1].texture;
		size_t bytes_per_pixel = driver->textures[update->texture - 1].format == ONIX_TEXTURE_FORMAT_R8 ? 1 : 4;
		[texture replaceRegion:MTLRegionMake2D(update->x, update->y, update->width, update->height)
			mipmapLevel:0
			withBytes:pixels + (size_t)update->data_offset
			bytesPerRow:update->width * bytes_per_pixel];
		if (driver->textures[update->texture - 1].last_update == index) onix_metal_generate_mipmaps(driver, texture);
	}
	return 1;
}

int onix_metal_destroy_texture(OnixMetalDriver *driver, uint32_t handle) {
	if (driver == NULL || handle == 0 || handle > driver->texture_slots.count || driver->textures[handle - 1].texture == nil) {
		onix_metal_set_error("invalid Metal texture destruction");
		return 0;
	}
	uint32_t frame_index = driver->encoder != nil
		? driver->frame_index
		: (driver->frame_index + ONIX_METAL_FRAMES_IN_FLIGHT - 1) % ONIX_METAL_FRAMES_IN_FLIGHT;
	[driver->frame_resources[frame_index] addObject:driver->textures[handle - 1].texture];
	[driver->textures[handle - 1].texture release];
	if (driver->textures[handle - 1].external != NULL) {
		CFRelease(driver->textures[handle - 1].external);
		driver->textures[handle - 1].external = NULL;
	}
	onix_metal_textures_recycle(driver->textures, &driver->texture_slots, handle - 1);
	return 1;
}

static int onix_metal_render_target_slot(OnixMetalDriver *driver, uint32_t *slot) {
	if (onix_metal_targets_acquire(&driver->render_targets, &driver->target_slots, slot)) return 1;
	onix_metal_set_error("failed to acquire Metal render target slot");
	return 0;
}

static void onix_metal_retire_target_depth(OnixMetalDriver *driver, OnixMetalRenderTarget *target) {
	if (target->depth_texture == nil) return;
	uint32_t frame_index = driver->encoder != nil
		? driver->frame_index
		: (driver->frame_index + ONIX_METAL_FRAMES_IN_FLIGHT - 1) % ONIX_METAL_FRAMES_IN_FLIGHT;
	[driver->frame_resources[frame_index] addObject:target->depth_texture];
	[target->depth_texture release];
	target->depth_texture = nil;
}

static int onix_metal_create_render_target_resources(
	OnixMetalDriver *driver,
	uint32_t width,
	uint32_t height,
	uint32_t color_format,
	int depth,
	OnixMetalRenderTarget *target) {
	if (width == 0 || height == 0 || color_format > 2 || (color_format == 2 && depth)) {
		onix_metal_set_error("unsupported Metal render target descriptor");
		return 0;
	}
	MTLPixelFormat pixel_format = color_format == 2 ? MTLPixelFormatR8Unorm : color_format == 0 ? MTLPixelFormatBGRA8Unorm : MTLPixelFormatR32Uint;
	MTLTextureDescriptor *color_descriptor = [MTLTextureDescriptor
		texture2DDescriptorWithPixelFormat:pixel_format width:width height:height mipmapped:NO];
	color_descriptor.usage = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
	color_descriptor.storageMode = MTLStorageModePrivate;
	id<MTLTexture> color = [driver->device newTextureWithDescriptor:color_descriptor];
	if (color == nil) {
		onix_metal_set_error("Metal render target color allocation failed");
		return 0;
	}
	id<MTLTexture> depth_texture = nil;
	if (depth) {
		MTLTextureDescriptor *depth_descriptor = [MTLTextureDescriptor
			texture2DDescriptorWithPixelFormat:MTLPixelFormatDepth32Float width:width height:height mipmapped:NO];
		depth_descriptor.usage = MTLTextureUsageRenderTarget;
		depth_descriptor.storageMode = MTLStorageModePrivate;
		depth_texture = [driver->device newTextureWithDescriptor:depth_descriptor];
		if (depth_texture == nil) {
			[color release];
			onix_metal_set_error("Metal render target depth allocation failed");
			return 0;
		}
	}
	uint32_t texture_slot = onix_metal_texture_slot(driver);
	if (texture_slot == UINT32_MAX) {
		[color release];
		[depth_texture release];
		onix_metal_set_error("Metal render target texture table allocation failed");
		return 0;
	}
	onix_metal_textures_insert(driver->textures, &driver->texture_slots, texture_slot, (OnixMetalTexture){.texture = color, .format = color_format == 2 ? ONIX_TEXTURE_FORMAT_R8 : ONIX_TEXTURE_FORMAT_RGBA8});
	*target = (OnixMetalRenderTarget){
		.color_texture = texture_slot + 1,
		.color_format = color_format,
		.depth_texture = depth_texture,
		.width = width,
		.height = height,
	};
	return 1;
}

static void onix_metal_destroy_render_target_resources(OnixMetalDriver *driver, OnixMetalRenderTarget *target) {
	if (target->color_texture != 0) onix_metal_destroy_texture(driver, target->color_texture);
	onix_metal_retire_target_depth(driver, target);
	target->color_texture = 0;
	target->width = 0;
	target->height = 0;
	target->initialized = 0;
}

uint32_t onix_metal_create_render_target(
	OnixMetalDriver *driver,
	uint32_t width,
	uint32_t height,
	uint32_t color_format,
	int depth,
	uint32_t *color_texture) {
	if (driver == NULL || color_texture == NULL || driver->active_render_target != 0) {
		onix_metal_set_error("invalid Metal render target creation");
		return 0;
	}
	uint32_t slot;
	if (!onix_metal_render_target_slot(driver, &slot)) return 0;
	OnixMetalRenderTarget target;
	if (!onix_metal_create_render_target_resources(driver, width, height, color_format, depth, &target)) return 0;
	onix_metal_targets_insert(driver->render_targets, &driver->target_slots, slot, target);
	*color_texture = target.color_texture;
	return slot + 1;
}

int onix_metal_resize_render_target(
	OnixMetalDriver *driver,
	uint32_t handle,
	uint32_t width,
	uint32_t height,
	uint32_t color_format,
	int depth,
	uint32_t *color_texture) {
	if (driver == NULL || handle == 0 || handle > driver->target_slots.count || color_texture == NULL || driver->active_render_target != 0) {
		onix_metal_set_error("invalid Metal render target resize");
		return 0;
	}
	OnixMetalRenderTarget *target = &driver->render_targets[handle - 1];
	if (target->color_texture == 0) {
		onix_metal_set_error("Metal render target is unavailable");
		return 0;
	}
	OnixMetalRenderTarget replacement;
	if (!onix_metal_create_render_target_resources(driver, width, height, color_format, depth, &replacement)) return 0;
	onix_metal_destroy_render_target_resources(driver, target);
	*target = replacement;
	*color_texture = replacement.color_texture;
	return 1;
}

int onix_metal_destroy_render_target(OnixMetalDriver *driver, uint32_t handle) {
	if (driver == NULL || handle == 0 || handle > driver->target_slots.count || driver->active_render_target == handle) {
		onix_metal_set_error("invalid Metal render target destruction");
		return 0;
	}
	OnixMetalRenderTarget *target = &driver->render_targets[handle - 1];
	if (target->color_texture == 0) {
		onix_metal_set_error("Metal render target is already destroyed");
		return 0;
	}
	onix_metal_destroy_render_target_resources(driver, target);
	onix_metal_targets_recycle(driver->render_targets, &driver->target_slots, handle - 1);
	return 1;
}

int onix_metal_begin_render_target(
	OnixMetalDriver *driver,
	uint32_t handle,
	const float clear_color[4],
	double clear_depth,
	int clear_color_enabled,
	int clear_depth_enabled) {
	if (driver == NULL || driver->encoder == nil || driver->command_buffer == nil || driver->active_render_target != 0 ||
		handle == 0 || handle > driver->target_slots.count || clear_color == NULL) {
		onix_metal_set_error("invalid Metal render target begin");
		return 0;
	}
	OnixMetalRenderTarget *target = &driver->render_targets[handle - 1];
	if (target->color_texture == 0 || target->color_texture > driver->texture_slots.count || driver->textures[target->color_texture - 1].texture == nil) {
		onix_metal_set_error("Metal render target is unavailable");
		return 0;
	}
	[driver->encoder endEncoding];
	[driver->encoder release];
	driver->encoder = nil;
	if (!target->initialized) {
		clear_color_enabled = 1;
		clear_depth_enabled = 1;
	}
	MTLRenderPassDescriptor *pass = driver->target_pass;
	pass.colorAttachments[0].texture = driver->textures[target->color_texture - 1].texture;
	pass.colorAttachments[0].loadAction = clear_color_enabled ? MTLLoadActionClear : MTLLoadActionLoad;
	pass.colorAttachments[0].storeAction = MTLStoreActionStore;
	pass.colorAttachments[0].clearColor = MTLClearColorMake(clear_color[0], clear_color[1], clear_color[2], clear_color[3]);
	if (target->depth_texture != nil) {
		pass.depthAttachment.texture = target->depth_texture;
		pass.depthAttachment.loadAction = clear_depth_enabled ? MTLLoadActionClear : MTLLoadActionLoad;
		pass.depthAttachment.storeAction = MTLStoreActionStore;
		pass.depthAttachment.clearDepth = clear_depth;
	}
	driver->encoder = [[driver->command_buffer renderCommandEncoderWithDescriptor:pass] retain];
	pass.colorAttachments[0].texture = nil;
	pass.depthAttachment.texture = nil;
	if (driver->encoder == nil) {
		onix_metal_resume_surface(driver, MTLLoadActionLoad);
		onix_metal_set_error("Metal render target encoder creation failed");
		return 0;
	}
	driver->active_render_target = handle;
	target->initialized = 1;
	return 1;
}

int onix_metal_end_render_target(OnixMetalDriver *driver) {
	if (driver == NULL || driver->encoder == nil || driver->command_buffer == nil || driver->active_render_target == 0) {
		onix_metal_set_error("invalid Metal render target end");
		return 0;
	}
	[driver->encoder endEncoding];
	[driver->encoder release];
	driver->encoder = nil;
	if (!onix_metal_resume_surface(driver, MTLLoadActionLoad)) return 0;
	driver->active_render_target = 0;
	return 1;
}

int onix_metal_read_render_target(
	OnixMetalDriver *driver,
	uint32_t handle,
	int32_t x,
	int32_t y,
	uint32_t width,
	uint32_t height,
	uint8_t *destination,
	size_t length) {
	if (driver == NULL || driver->encoder != nil || driver->command_buffer != nil || handle == 0 ||
		handle > driver->target_slots.count || destination == NULL || width == 0 || height == 0 ||
		x < 0 || y < 0) {
		onix_metal_set_error("invalid Metal render target readback");
		return 0;
	}
	OnixMetalRenderTarget *target = &driver->render_targets[handle - 1];
	if (length != (size_t)width * height * (target->color_format == 2 ? 1 : 4) || target->color_texture == 0 || (uint64_t)(uint32_t)x + width > target->width ||
		(uint64_t)(uint32_t)y + height > target->height || target->color_texture > driver->texture_slots.count ||
		driver->textures[target->color_texture - 1].texture == nil) {
		onix_metal_set_error("Metal render target readback is outside the image");
		return 0;
	}
	@autoreleasepool {
		NSUInteger row_bytes = (NSUInteger)width * (target->color_format == 2 ? 1u : 4u);
		NSUInteger bytes_per_row = (row_bytes + 255u) & ~(NSUInteger)255u;
		NSUInteger required = bytes_per_row * height;
		if (required > driver->readback_used) driver->readback_used = required;
		if (required > driver->readback_capacity) {
			NSUInteger capacity = driver->readback_capacity == 0 ? 256 : driver->readback_capacity;
			while (capacity < required) capacity = capacity <= NSUIntegerMax / 2 ? capacity * 2 : required;
			id<MTLBuffer> replacement = [driver->device newBufferWithLength:capacity options:MTLResourceStorageModeShared];
			if (replacement == nil) { onix_metal_set_error("Metal render target readback buffer allocation failed"); return 0; }
			[driver->readback_buffer release];
			driver->readback_buffer = replacement;
			driver->readback_capacity = capacity;
		}
		id<MTLBuffer> buffer = driver->readback_buffer;
		id<MTLCommandBuffer> command = [driver->queue commandBuffer];
		id<MTLBlitCommandEncoder> blit = [command blitCommandEncoder];
		[blit copyFromTexture:driver->textures[target->color_texture - 1].texture
			sourceSlice:0 sourceLevel:0
			sourceOrigin:MTLOriginMake((NSUInteger)x, (NSUInteger)y, 0)
			sourceSize:MTLSizeMake(width, height, 1)
			toBuffer:buffer destinationOffset:0 destinationBytesPerRow:bytes_per_row destinationBytesPerImage:bytes_per_row * height];
		[blit endEncoding];
		[command commit];
		[command waitUntilCompleted];
		if (command.status == MTLCommandBufferStatusError) {
			onix_metal_set_error("Metal render target readback command failed");
			return 0;
		}
		for (uint32_t row = 0; row < height; row++) {
			memcpy(destination + (size_t)row * row_bytes, (const uint8_t *)buffer.contents + (size_t)row * bytes_per_row, row_bytes);
		}
	}
	return 1;
}

uint32_t onix_metal_create_mesh(
	OnixMetalDriver *driver,
	const OnixMetalVertex3D *vertices,
	uint32_t vertex_count,
	const uint32_t *indices,
	uint32_t index_count) {
	if (driver == NULL || vertices == NULL || vertex_count == 0 || indices == NULL || index_count == 0) {
		onix_metal_set_error("invalid Metal mesh data");
		return 0;
	}
	uint32_t slot;
	if (!onix_metal_meshes_acquire(&driver->meshes, &driver->mesh_slots, &slot)) {
		onix_metal_set_error("failed to acquire Metal mesh slot");
		return 0;
	}
	OnixMetalMesh mesh = {
		.vertices = [driver->device newBufferWithBytes:vertices length:(NSUInteger)vertex_count * sizeof(OnixMetalVertex3D) options:MTLResourceStorageModeShared],
		.indices = [driver->device newBufferWithBytes:indices length:(NSUInteger)index_count * sizeof(uint32_t) options:MTLResourceStorageModeShared],
		.vertex_count = vertex_count,
		.index_count = index_count,
	};
	if (mesh.vertices == nil || mesh.indices == nil) {
		[mesh.vertices release];
		[mesh.indices release];
		onix_metal_set_error("Metal mesh allocation failed");
		return 0;
	}
	onix_metal_meshes_insert(driver->meshes, &driver->mesh_slots, slot, mesh);
	return slot + 1;
}

int onix_metal_set_mesh_deformation(OnixMetalDriver *driver, uint32_t handle, const uint32_t *words, uint32_t word_count, uint32_t vertex_count) {
	if (driver == NULL || handle == 0 || handle > driver->mesh_slots.count || words == NULL || word_count == 0) { onix_metal_set_error("invalid static mesh deformation"); return 0; }
	OnixMetalMesh *mesh = &driver->meshes[handle-1];
	if (mesh->vertices == nil || mesh->deformation != nil || mesh->vertex_count != vertex_count) { onix_metal_set_error("invalid static mesh deformation resource"); return 0; }
	mesh->deformation = [driver->device newBufferWithBytes:words length:(NSUInteger)word_count*4 options:MTLResourceStorageModeShared];
	if (mesh->deformation == nil) { onix_metal_set_error("static mesh deformation allocation failed"); return 0; }
	return 1;
}

int onix_metal_destroy_mesh(OnixMetalDriver *driver, uint32_t handle) {
	if (driver == NULL || handle == 0 || handle > driver->mesh_slots.count || driver->meshes[handle - 1].vertices == nil) {
		onix_metal_set_error("invalid Metal mesh destruction");
		return 0;
	}
	OnixMetalMesh *mesh = &driver->meshes[handle - 1];
	uint32_t frame_index = driver->encoder != nil
		? driver->frame_index
		: (driver->frame_index + ONIX_METAL_FRAMES_IN_FLIGHT - 1) % ONIX_METAL_FRAMES_IN_FLIGHT;
	[driver->frame_resources[frame_index] addObject:mesh->vertices];
	[driver->frame_resources[frame_index] addObject:mesh->indices];
	[mesh->vertices release];
	[mesh->indices release];
	if (mesh->deformation != nil) { [driver->frame_resources[frame_index] addObject:mesh->deformation]; [mesh->deformation release]; }
	onix_metal_meshes_recycle(driver->meshes, &driver->mesh_slots, handle - 1);
	return 1;
}

static int onix_metal_upload(OnixMetalDriver *driver, const void *data, NSUInteger size, NSUInteger alignment, int cached, OnixNativeUpload *upload) {
	OnixMetalUploadFrame *frame = &driver->uploads[driver->frame_index];
	uint64_t hash = cached ? onix_upload_hash(data, size) : 0;
	if (cached && onix_upload_cache_find(&frame->cache, data, size, hash, alignment, upload)) return 1;
	for (uint32_t index = frame->index; index < frame->count; index++) {
		OnixMetalUploadBlock *block = &frame->blocks[index];
		NSUInteger offset = (block->offset + alignment - 1) & ~(alignment - 1);
		if (offset <= block->capacity && size <= block->capacity - offset) {
			frame->index = index;
			frame->used += offset + size - block->offset;
			block->offset = offset + size;
			void *destination = (uint8_t *)block->buffer.contents + offset;
			if (data != NULL) memcpy(destination, data, size);
			*upload = (OnixNativeUpload){.resource = (uintptr_t)block->buffer, .offset = offset, .hash = hash, .size = size, .data = destination};
			if (cached && !onix_upload_cache_insert(&frame->cache, *upload)) { onix_metal_set_error("Metal upload cache allocation failed"); return 0; }
			return 1;
		}
	}
	if (frame->count == frame->capacity) {
		uint32_t capacity = frame->capacity ? frame->capacity * 2 : 4;
		if (capacity < frame->capacity || (uint64_t)capacity * sizeof(*frame->blocks) > SIZE_MAX) return 0;
		OnixMetalUploadBlock *blocks = realloc(frame->blocks, (size_t)capacity * sizeof(*blocks));
		if (blocks == NULL) { onix_metal_set_error("Metal upload table allocation failed"); return 0; }
		frame->blocks = blocks;
		frame->capacity = capacity;
	}
	NSUInteger capacity = (NSUInteger)onix_buffer_block_capacity(size, ONIX_METAL_INSTANCE_BUFFER_MINIMUM,
		frame->count ? frame->blocks[frame->count - 1].capacity : 0, 1024u * 1024u);
	if (!capacity) return 0;
	id<MTLBuffer> buffer = [driver->device newBufferWithLength:capacity options:MTLResourceStorageModeShared];
	if (buffer == nil) { onix_metal_set_error("Metal upload buffer allocation failed"); return 0; }
	frame->index = frame->count;
	frame->blocks[frame->count++] = (OnixMetalUploadBlock){.buffer = buffer, .capacity = capacity};
	frame->bytes += capacity;
	return onix_metal_upload(driver, data, size, alignment, cached, upload);
}

int onix_metal_draw_sprites(OnixMetalDriver *driver, const OnixMetalSpriteUniforms *uniforms, const OnixMetalInstance *instances, uint32_t instance_count, const OnixNativeSpriteDraw *draws, uint32_t draw_count) {
	if (driver == NULL || driver->encoder == nil || uniforms == NULL || instances == NULL || instance_count == 0 || draws == NULL || draw_count == 0) {
		onix_metal_set_error("invalid Metal sprite draw");
		return 0;
	}
	NSUInteger byte_length = 0;
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeSpriteDraw *draw = &draws[index];
		if (draw->shader && byte_length < (NSUInteger)draw->first_instance * ONIX_SPRITE_CUSTOM_BYTES) byte_length = (NSUInteger)draw->first_instance * ONIX_SPRITE_CUSTOM_BYTES;
		byte_length += (NSUInteger)draw->instance_count * onix_sprite_stride(draw->shader);
		if (draw->texture == 0 || draw->texture > driver->texture_slots.count || driver->textures[draw->texture - 1].texture == nil ||
			draw->instance_count == 0 || (uint64_t)draw->first_instance + (uint64_t)draw->instance_count > instance_count ||
			(draw->shader > 0 && (draw->shader > driver->material2d_shader_slots.count || driver->material2d_pipelines[draw->shader - 1].pipelines[0] == nil))) {
			onix_metal_set_error("invalid Metal sprite draw range");
			return 0;
		}
		for (uint32_t slot = 0; slot < 3; slot++) {
			uint32_t texture = draw->additional_textures[slot];
			if (texture != 0 && (texture > driver->texture_slots.count || driver->textures[texture - 1].texture == nil)) {
				onix_metal_set_error("invalid Metal sprite texture slot");
				return 0;
			}
		}
	}
	OnixNativeUpload instance_upload;
	if (!onix_metal_upload(driver, NULL, byte_length, 16, 0, &instance_upload)) return 0;
	id<MTLBuffer> instance_buffer = (id<MTLBuffer>)(uintptr_t)instance_upload.resource;
	NSUInteger instance_offset = instance_upload.offset;
	NSUInteger packed_offset = 0;
	[driver->encoder setDepthStencilState:driver->depth_disabled];
    onix_metal_apply_dynamic_state(driver);
    [driver->encoder setVertexBuffer:driver->quad_vertices offset:0 atIndex:0];
	id<MTLRenderPipelineState> pipeline = nil;
	uint32_t depth = onix_metal_2d_attachment(driver);
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeSpriteDraw *draw = &draws[index];
		id<MTLRenderPipelineState> next_pipeline;
		if (draw->shader != 0) {
			OnixMetalMaterial2DPipeline *custom = &driver->material2d_pipelines[draw->shader - 1];
			if (custom->pipelines[depth] == nil) {
				custom->pipelines[depth] = onix_metal_pipeline(driver, custom->vertex_library, custom->fragment_library, @"sprite_vertex", @"sprite_fragment", onix_metal_sprite_vertex_descriptor(), 0, depth);
			}
			next_pipeline = custom->pipelines[depth];
			if (next_pipeline == nil) return 0;
		} else {
			id<MTLRenderPipelineState> *cached = &driver->sprite_pipelines[depth][driver->sprite_blend];
			if (*cached == nil) {
				*cached = onix_metal_pipeline(driver, driver->default_library, driver->default_library, @"sprite_multi_vertex", @"sprite_multi_fragment", onix_metal_compact_sprite_vertex_descriptor(), driver->sprite_blend, depth);
			}
			next_pipeline = *cached;
			if (next_pipeline == nil) return 0;
		}
		if (pipeline != next_pipeline) {
			pipeline = next_pipeline;
			[driver->encoder setRenderPipelineState:pipeline];
		}
		if (draw->shader && packed_offset < (NSUInteger)draw->first_instance * ONIX_SPRITE_CUSTOM_BYTES) packed_offset = (NSUInteger)draw->first_instance * ONIX_SPRITE_CUSTOM_BYTES;
		onix_sprite_copy((uint8_t *)instance_upload.data + packed_offset, instances + draw->first_instance, draw->instance_count, draw->shader);
		[driver->encoder setVertexBuffer:instance_buffer offset:instance_offset + packed_offset atIndex:1];
		packed_offset += (NSUInteger)draw->instance_count * onix_sprite_stride(draw->shader);
		OnixMetalTexture *texture = &driver->textures[draw->texture - 1];
		OnixMetalSpriteUniforms draw_uniforms = *uniforms;
		draw_uniforms.texture_coverage[0] = texture->format == ONIX_TEXTURE_FORMAT_R8 ? 1.0f : 0.0f;
		if (draw->shader == 0) {
			for (uint32_t slot = 1; slot < 4; slot++) {
				uint32_t handle = draw->additional_textures[slot - 1];
				OnixMetalTexture *extra = handle == 0 ? texture : &driver->textures[handle - 1];
				draw_uniforms.texture_coverage[slot] = extra->format == ONIX_TEXTURE_FORMAT_R8 ? 1.0f : 0.0f;
				[driver->encoder setFragmentTexture:extra->texture atIndex:slot];
		[driver->encoder setFragmentSamplerState:driver->samplers[extra->sampler] atIndex:slot];
			}
		}
		[driver->encoder setVertexBytes:&draw_uniforms length:sizeof(draw_uniforms) atIndex:2];
		[driver->encoder setFragmentBytes:&draw_uniforms length:sizeof(draw_uniforms) atIndex:2];
		id<MTLTexture> base_texture = texture->texture;
		[driver->encoder setFragmentTexture:base_texture atIndex:0];
		[driver->encoder setFragmentSamplerState:driver->samplers[driver->textures[draw->texture-1].sampler] atIndex:0];
		[driver->encoder setVertexTexture:base_texture atIndex:0];
		[driver->encoder drawIndexedPrimitives:MTLPrimitiveTypeTriangle indexCount:6 indexType:MTLIndexTypeUInt16 indexBuffer:driver->quad_indices indexBufferOffset:0 instanceCount:draw->instance_count];
	}
	return 1;
}

int onix_metal_draw_shapes(OnixMetalDriver *driver, const OnixMetalSpriteUniforms *uniforms, const OnixMetalShapeInstance *instances, uint32_t instance_count) {
	if (driver == NULL || driver->encoder == nil || uniforms == NULL || instances == NULL || instance_count == 0) {
		onix_metal_set_error("invalid Metal shape draw");
		return 0;
	}
	NSUInteger byte_length = (NSUInteger)instance_count * sizeof(OnixMetalShapeInstance);
	OnixNativeUpload instance_upload;
	if (!onix_metal_upload(driver, instances, byte_length, 16, 0, &instance_upload)) return 0;
	id<MTLBuffer> instance_buffer = (id<MTLBuffer>)(uintptr_t)instance_upload.resource;
	NSUInteger instance_offset = instance_upload.offset;
	uint32_t depth = onix_metal_2d_attachment(driver);
	if (driver->shape_pipelines[depth] == nil) {
		driver->shape_pipelines[depth] = onix_metal_pipeline(driver, driver->default_library, driver->default_library, @"shape_vertex", @"shape_fragment", onix_metal_shape_vertex_descriptor(), 0, depth);
	}
	if (driver->shape_pipelines[depth] == nil) return 0;
	[driver->encoder setRenderPipelineState:driver->shape_pipelines[depth]];
	[driver->encoder setDepthStencilState:driver->depth_disabled];
	onix_metal_apply_dynamic_state(driver);
	[driver->encoder setVertexBuffer:driver->quad_vertices offset:0 atIndex:0];
	[driver->encoder setVertexBuffer:instance_buffer offset:instance_offset atIndex:1];
	[driver->encoder setVertexBytes:uniforms length:sizeof(*uniforms) atIndex:2];
	[driver->encoder drawIndexedPrimitives:MTLPrimitiveTypeTriangle indexCount:6 indexType:MTLIndexTypeUInt16 indexBuffer:driver->quad_indices indexBufferOffset:0 instanceCount:instance_count];
	return 1;
}

int onix_metal_draw_meshes(
	OnixMetalDriver *driver,
	const OnixNativeMeshUniforms *uniforms,
	const OnixMetalVertex3D *vertices,
	uint32_t vertex_count,
	const uint32_t *indices,
	uint32_t index_count,
	const OnixMetalInstance3D *instances,
	uint32_t instance_count,
	const uint32_t *deformation_words,
	uint32_t deformation_word_count,
	const OnixNativeMeshDraw3D *draws,
	uint32_t draw_count) {
	if (driver == NULL || driver->encoder == nil || uniforms == NULL || instances == NULL || instance_count == 0 || deformation_words == NULL || deformation_word_count == 0 || draws == NULL || draw_count == 0) {
		onix_metal_set_error("invalid Metal mesh draw");
		return 0;
	}
	int dynamic = vertices != NULL || vertex_count != 0 || indices != NULL || index_count != 0;
	if (dynamic && (vertices == NULL || vertex_count == 0 || indices == NULL || index_count == 0)) {
		onix_metal_set_error("incomplete Metal dynamic mesh data");
		return 0;
	}
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeMeshDraw3D *draw = &draws[index];
		if (draw->texture == 0 || draw->texture > driver->texture_slots.count || draw->normal_texture > driver->texture_slots.count || draw->metallic_roughness_texture > driver->texture_slots.count || driver->textures[draw->texture - 1].texture == nil ||
			(draw->normal_texture != 0 && driver->textures[draw->normal_texture - 1].texture == nil) ||
			(draw->metallic_roughness_texture != 0 && driver->textures[draw->metallic_roughness_texture - 1].texture == nil) ||
			draw->index_count == 0 || draw->instance_count == 0 || (uint64_t)draw->first_instance + draw->instance_count > instance_count ||
			(draw->deformation_index != 0 && (draw->mesh == 0 || draw->mesh > driver->mesh_slots.count || driver->meshes[draw->mesh-1].deformation == nil)) ||
			draw->base_vertex != 0 || draw->alpha_mode > 2 || draw->blend_mode >= ONIX_MESH_BLEND_MODE_COUNT ||
			(draw->shader > 0 && (draw->shader > driver->mesh_shader_slots.count || driver->mesh_pipelines[draw->shader - 1].vertex_library == nil))) {
			onix_metal_set_error("invalid Metal mesh draw range");
			return 0;
		}
		if (draw->mesh == 0) {
			if (!dynamic || (uint64_t)draw->first_index + draw->index_count > index_count) {
				onix_metal_set_error("invalid Metal dynamic mesh draw range");
				return 0;
			}
		} else if (draw->mesh > driver->mesh_slots.count || driver->meshes[draw->mesh - 1].vertices == nil ||
			(uint64_t)draw->first_index + draw->index_count > driver->meshes[draw->mesh - 1].index_count) {
			onix_metal_set_error("invalid Metal static mesh draw range");
			return 0;
		}
	}
	if (!onix_metal_require_depth(driver)) return 0;
	OnixNativeUpload vertex_upload = {0}, index_upload = {0}, instance_upload, uniform_upload;
	if (dynamic && (!onix_metal_upload(driver, vertices, (NSUInteger)vertex_count * sizeof(*vertices), 16, 1, &vertex_upload) ||
		!onix_metal_upload(driver, indices, (NSUInteger)index_count * sizeof(*indices), 16, 1, &index_upload))) return 0;
	if (!onix_metal_upload(driver, instances, (NSUInteger)instance_count * sizeof(*instances), 16, 0, &instance_upload) ||
		!onix_metal_upload(driver, uniforms, sizeof(*uniforms), 256, 0, &uniform_upload)) return 0;
	id<MTLBuffer> buffer = (id<MTLBuffer>)(uintptr_t)instance_upload.resource;
	NSUInteger instance_offset = instance_upload.offset;
	NSUInteger vertex_offset = vertex_upload.offset, index_offset = index_upload.offset;
	OnixNativeUpload deformation_upload;
	if (!onix_metal_upload(driver, deformation_words, (NSUInteger)deformation_word_count * sizeof(*deformation_words), 16, 1, &deformation_upload)) return 0;

	onix_metal_apply_dynamic_state(driver);
	[driver->encoder setVertexBuffer:(id<MTLBuffer>)(uintptr_t)uniform_upload.resource offset:uniform_upload.offset atIndex:2];
	[driver->encoder setFragmentBuffer:(id<MTLBuffer>)(uintptr_t)uniform_upload.resource offset:uniform_upload.offset atIndex:2];
	[driver->encoder setVertexBuffer:(id<MTLBuffer>)(uintptr_t)deformation_upload.resource offset:deformation_upload.offset atIndex:3];
	[driver->encoder setFrontFacingWinding:MTLWindingCounterClockwise];
	id<MTLRenderPipelineState> pipeline = nil;
	uint32_t bound_mesh = UINT32_MAX;
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeMeshDraw3D *draw = &draws[index];
		// The built-in shader already rejects these faces. Cull before shading
		// for a single instance; mixed instance orientations stay shader-driven.
		MTLCullMode cull = MTLCullModeNone;
		if (draw->shader == 0 && draw->instance_count == 1 && (int)uniforms->lighting[5] != 7) {
			const OnixMetalInstance3D *instance = &instances[draw->first_instance];
			uint32_t flags = (uint32_t)(instance->material[3] + 0.5f);
			if (instance->material[0] >= 0 && !(flags & (512u | 2048u | 8192u))) {
				int flipped = (uniforms->lighting[7] > 0.5f) != ((flags & 1024u) != 0);
				cull = flipped ? MTLCullModeFront : MTLCullModeBack;
			}
		}
		[driver->encoder setCullMode:cull];
		if (draw->mesh != bound_mesh) {
			id<MTLBuffer> vertex_buffer = draw->mesh == 0 ? (id<MTLBuffer>)(uintptr_t)vertex_upload.resource : driver->meshes[draw->mesh - 1].vertices;
			NSUInteger offset = draw->mesh == 0 ? vertex_offset : 0;
			[driver->encoder setVertexBuffer:vertex_buffer offset:offset atIndex:0];
			bound_mesh = draw->mesh;
		}
		int transparent = draw->alpha_mode == 2;
		id<MTLRenderPipelineState> next_pipeline;
		if (draw->shader == 0) {
			next_pipeline = onix_metal_mesh_pipeline(driver, transparent, draw->blend_mode, 0, onix_mesh_builtin_variant(instances, draw->first_instance, draw->instance_count, (int)uniforms->lighting[5]));
		} else {
			OnixMetalMeshPipeline *custom = &driver->mesh_pipelines[draw->shader - 1];
			id<MTLRenderPipelineState> *variant = transparent ? &custom->transparent[draw->blend_mode] : &custom->opaque;
			if (*variant == nil) *variant = onix_metal_pipeline(driver, custom->vertex_library, custom->fragment_library,
				@"mesh_vertex", @"mesh_fragment", onix_metal_mesh_vertex_descriptor(), transparent ? (int)draw->blend_mode : -1, 1);
			next_pipeline = *variant;
		}
		if (next_pipeline == nil) return 0;
		if (pipeline != next_pipeline) {
			pipeline = next_pipeline;
			[driver->encoder setRenderPipelineState:pipeline];
			[driver->encoder setDepthStencilState:transparent ? driver->mesh_transparent_depth : driver->mesh_opaque_depth];
		}
		[driver->encoder setVertexBuffer:buffer offset:instance_offset + (NSUInteger)draw->first_instance * sizeof(OnixMetalInstance3D) atIndex:1];
		[driver->encoder setVertexBytes:&draw->deformation_index length:sizeof(draw->deformation_index) atIndex:4];
		id<MTLBuffer> static_buffer = draw->deformation_index == 0 ? buffer : driver->meshes[draw->mesh-1].deformation;
		if (static_buffer == nil) { onix_metal_set_error("missing static mesh deformation"); return 0; }
		[driver->encoder setVertexBuffer:static_buffer offset:0 atIndex:5];
		id<MTLTexture> base_texture = driver->textures[draw->texture - 1].texture;
		id<MTLTexture> normal_texture = draw->normal_texture == 0 ? base_texture : driver->textures[draw->normal_texture - 1].texture;
		id<MTLTexture> metallic_roughness_texture = draw->metallic_roughness_texture == 0 ? base_texture : driver->textures[draw->metallic_roughness_texture - 1].texture;
		[driver->encoder setFragmentTexture:base_texture atIndex:0];
		[driver->encoder setFragmentSamplerState:driver->samplers[driver->textures[draw->texture-1].sampler] atIndex:0];
		[driver->encoder setFragmentTexture:normal_texture atIndex:1];
		[driver->encoder setFragmentSamplerState:driver->samplers[driver->textures[(draw->normal_texture ? draw->normal_texture : draw->texture)-1].sampler] atIndex:1];
		[driver->encoder setFragmentTexture:metallic_roughness_texture atIndex:2];
		for (uint32_t slot = 0; slot < 13; slot++) {
			uint32_t texture_handle = draw->extra_textures[slot] ? draw->extra_textures[slot] : draw->texture;
			if (texture_handle > driver->texture_slots.count || driver->textures[texture_handle-1].texture == nil) { onix_metal_set_error("invalid surface texture"); return 0; }
			[driver->encoder setFragmentTexture:driver->textures[texture_handle-1].texture atIndex:3+slot];
			[driver->encoder setFragmentSamplerState:driver->samplers[driver->textures[texture_handle-1].sampler] atIndex:3+slot];
		}
		[driver->encoder setFragmentSamplerState:driver->samplers[driver->textures[(draw->metallic_roughness_texture ? draw->metallic_roughness_texture : draw->texture)-1].sampler] atIndex:2];
		[driver->encoder setVertexTexture:base_texture atIndex:0];
		id<MTLBuffer> index_buffer = draw->mesh == 0 ? (id<MTLBuffer>)(uintptr_t)index_upload.resource : driver->meshes[draw->mesh - 1].indices;
		NSUInteger draw_index_offset = draw->mesh == 0 ? index_offset : 0;
		[driver->encoder drawIndexedPrimitives:MTLPrimitiveTypeTriangle
			indexCount:draw->index_count
			indexType:MTLIndexTypeUInt32
			indexBuffer:index_buffer
			indexBufferOffset:draw_index_offset + (NSUInteger)draw->first_index * sizeof(uint32_t)
			instanceCount:draw->instance_count];
	}
	[driver->encoder setCullMode:MTLCullModeNone];
	return 1;
}

int onix_metal_draw_mesh_picking(
	OnixMetalDriver *driver,
	const OnixNativeMeshUniforms *uniforms,
	const OnixMetalVertex3D *vertices,
	uint32_t vertex_count,
	const uint32_t *indices,
	uint32_t index_count,
	const OnixMetalInstance3D *instances,
	uint32_t instance_count,
	const OnixNativeMeshDraw3D *draws,
	uint32_t draw_count) {
	if (driver == NULL || driver->encoder == nil || driver->active_render_target == 0 || uniforms == NULL ||
		instances == NULL || instance_count == 0 || draws == NULL || draw_count == 0) {
		onix_metal_set_error("invalid Metal mesh picking draw");
		return 0;
	}
	OnixMetalRenderTarget *target = &driver->render_targets[driver->active_render_target - 1];
	if (target->color_format != 1 || target->depth_texture == nil) {
		onix_metal_set_error("Metal mesh picking requires an R32Uint target with depth");
		return 0;
	}
	int dynamic = vertices != NULL || vertex_count != 0 || indices != NULL || index_count != 0;
	if (dynamic && (vertices == NULL || vertex_count == 0 || indices == NULL || index_count == 0)) {
		onix_metal_set_error("incomplete Metal dynamic mesh picking data");
		return 0;
	}
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeMeshDraw3D *draw = &draws[index];
		if (draw->index_count == 0 || draw->instance_count == 0 || draw->base_vertex != 0 ||
			(uint64_t)draw->first_instance + draw->instance_count > instance_count) {
			onix_metal_set_error("invalid Metal mesh picking draw range");
			return 0;
		}
		if (draw->mesh == 0) {
			if (!dynamic || (uint64_t)draw->first_index + draw->index_count > index_count) {
				onix_metal_set_error("invalid Metal dynamic mesh picking draw range");
				return 0;
			}
		} else if (draw->mesh > driver->mesh_slots.count || driver->meshes[draw->mesh - 1].vertices == nil ||
			(uint64_t)draw->first_index + draw->index_count > driver->meshes[draw->mesh - 1].index_count) {
			onix_metal_set_error("invalid Metal static mesh picking draw range");
			return 0;
		}
	}
	if (!onix_metal_require_depth(driver)) return 0;
	OnixNativeUpload vertex_upload = {0}, index_upload = {0}, instance_upload, uniform_upload;
	if (dynamic && (!onix_metal_upload(driver, vertices, (NSUInteger)vertex_count * sizeof(*vertices), 16, 1, &vertex_upload) ||
		!onix_metal_upload(driver, indices, (NSUInteger)index_count * sizeof(*indices), 16, 1, &index_upload))) return 0;
	if (!onix_metal_upload(driver, instances, (NSUInteger)instance_count * sizeof(*instances), 16, 0, &instance_upload) ||
		!onix_metal_upload(driver, uniforms, sizeof(*uniforms), 256, 0, &uniform_upload)) return 0;
	id<MTLBuffer> buffer = (id<MTLBuffer>)(uintptr_t)instance_upload.resource;
	NSUInteger instance_offset = instance_upload.offset;
	NSUInteger vertex_offset = vertex_upload.offset, index_offset = index_upload.offset;

	onix_metal_apply_dynamic_state(driver);
	id<MTLRenderPipelineState> pipeline = onix_metal_mesh_pipeline(driver, 0, 0, 1, ONIX_MESH_FULL);
	if (pipeline == nil) return 0;
	[driver->encoder setRenderPipelineState:pipeline];
	[driver->encoder setDepthStencilState:driver->mesh_opaque_depth];
	[driver->encoder setVertexBuffer:(id<MTLBuffer>)(uintptr_t)uniform_upload.resource offset:uniform_upload.offset atIndex:2];
	uint32_t bound_mesh = UINT32_MAX;
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeMeshDraw3D *draw = &draws[index];
		if (draw->mesh != bound_mesh) {
			id<MTLBuffer> vertex_buffer = draw->mesh == 0 ? (id<MTLBuffer>)(uintptr_t)vertex_upload.resource : driver->meshes[draw->mesh - 1].vertices;
			NSUInteger offset = draw->mesh == 0 ? vertex_offset : 0;
			[driver->encoder setVertexBuffer:vertex_buffer offset:offset atIndex:0];
			bound_mesh = draw->mesh;
		}
		[driver->encoder setVertexBuffer:buffer offset:instance_offset + (NSUInteger)draw->first_instance * sizeof(OnixMetalInstance3D) atIndex:1];
		id<MTLBuffer> index_buffer = draw->mesh == 0 ? (id<MTLBuffer>)(uintptr_t)index_upload.resource : driver->meshes[draw->mesh - 1].indices;
		NSUInteger draw_index_offset = draw->mesh == 0 ? index_offset : 0;
		[driver->encoder drawIndexedPrimitives:MTLPrimitiveTypeTriangle
			indexCount:draw->index_count indexType:MTLIndexTypeUInt32 indexBuffer:index_buffer
			indexBufferOffset:draw_index_offset + (NSUInteger)draw->first_index * sizeof(uint32_t)
			instanceCount:draw->instance_count];
	}
	return 1;
}

_Static_assert(sizeof(OnixMetalInstance) == 132, "OnixMetalInstance ABI must be 132 bytes");
_Static_assert(sizeof(OnixMetalShapeInstance) == 100, "OnixMetalShapeInstance ABI must be 100 bytes");
_Static_assert(sizeof(OnixMetalSpriteUniforms) == 80, "OnixMetalSpriteUniforms ABI must be 80 bytes");
_Static_assert(sizeof(OnixMetalVertex3D) == 48, "OnixMetalVertex3D ABI must be 48 bytes");
_Static_assert(sizeof(OnixMetalInstance3D) == 192, "OnixMetalInstance3D ABI must be 192 bytes");
_Static_assert(sizeof(OnixNativeMeshUniforms) == 3584, "OnixNativeMeshUniforms ABI must be 3584 bytes");
_Static_assert(sizeof(OnixNativeMeshDraw3D) == 104, "OnixNativeMeshDraw3D ABI must be 104 bytes");
