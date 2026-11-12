#ifndef ONIX_ENGINE_RENDER_NATIVE_GPU_H
#define ONIX_ENGINE_RENDER_NATIVE_GPU_H

#define ONIX_GPU_MESH_LIGHTING_BYTES 3584

#include <stddef.h>
#include <stdint.h>
#include "native_buffer.h"
#include "native_draw.h"

enum {
    ONIX_GPU_SURFACE_XLIB = 1,
    ONIX_GPU_SURFACE_WAYLAND = 2,
    ONIX_GPU_SURFACE_ANDROID = 3,
    ONIX_GPU_SURFACE_WIN32 = 4,
    ONIX_GPU_SURFACE_DRM = 5,
    ONIX_GPU_SURFACE_METAL = 6,
};

typedef struct OnixGPUDriver OnixGPUDriver;
void onix_gpu_buffer_memory(OnixGPUDriver *driver, OnixBufferMemory *memory);

typedef struct {
    int kind;
    void *display;
    uintptr_t window;
    uint32_t width;
    uint32_t height;
    int transparent;
    int32_t drm_fd;
    uint32_t drm_connector_id;
    uint32_t drm_refresh_rate;
} OnixGPUSurface;

typedef struct {
    const uint32_t *code;
    size_t size;
} OnixGPUShader;

typedef struct {
	float model[6];
	float texture_model[6];
	float color[4];
	float custom[4][4];
	float texture_slot;
} OnixGPUInstance;

typedef struct {
	float model[6];
	float local_model[6];
	float fill[4];
	float border[4];
	float params[4];
	uint32_t kind;
} OnixGPUShapeInstance;

typedef struct {
	uint32_t texture;
	int32_t x;
	int32_t y;
	uint32_t width;
	uint32_t height;
	uint64_t data_offset;
	uint64_t data_length;
} OnixGPUTextureUpdate;

/* These values mirror render.TextureFormat so the driver can pass the Go enum
 * straight through. Block-compressed entries start at BC1 and are contiguous,
 * which is what onix_vk_texture_format_is_compressed relies on. */
#define ONIX_TEXTURE_FORMAT_RGBA8 0
#define ONIX_TEXTURE_FORMAT_R8 1
#define ONIX_TEXTURE_FORMAT_SRGBA8 2
#define ONIX_TEXTURE_FORMAT_BC1 3
#define ONIX_TEXTURE_FORMAT_BC1_SRGB 4
#define ONIX_TEXTURE_FORMAT_BC3 5
#define ONIX_TEXTURE_FORMAT_BC3_SRGB 6
#define ONIX_TEXTURE_FORMAT_BC4 7
#define ONIX_TEXTURE_FORMAT_BC5 8
#define ONIX_TEXTURE_FORMAT_BC7 9
#define ONIX_TEXTURE_FORMAT_BC7_SRGB 10
#define ONIX_TEXTURE_FORMAT_ASTC_4X4 11
#define ONIX_TEXTURE_FORMAT_ASTC_4X4_SRGB 12
#define ONIX_TEXTURE_FORMAT_COUNT 13

typedef struct {
    float projection[16];
	float shader_padding[8];
} OnixGPUSpritePush;

typedef struct {
	float position[3];
	float normal[3];
	float uv[2];
	float tangent[4];
} OnixGPUVertex3D;

typedef OnixNativeInstance3D OnixGPUInstance3D;

typedef struct {
	float view_projection[16];
	float camera_position[4];
	float ambient_light[4];
	float light_direction[4];
	float directional_light[4];
} OnixGPUMeshPush;

const char *onix_gpu_last_error(void);
const char *onix_gpu_device_name(OnixGPUDriver *driver);

OnixGPUDriver *onix_gpu_create(
    OnixGPUSurface surface,
    OnixGPUShader sprite_vertex,
	OnixGPUShader sprite_fragment,
	OnixGPUShader shape_vertex,
	OnixGPUShader shape_fragment,
	OnixGPUShader mesh_vertex,
	OnixGPUShader mesh_fragment,
	OnixGPUShader mesh_unlit_fragment,
	OnixGPUShader mesh_depth_fragment,
	OnixGPUShader mesh_basic_fragment,
	OnixGPUShader picking_vertex,
	OnixGPUShader picking_fragment);
void onix_gpu_destroy(OnixGPUDriver *driver);
int onix_gpu_resize(OnixGPUDriver *driver, uint32_t width, uint32_t height);

int onix_gpu_begin_frame(OnixGPUDriver *driver, const float clear_color[4]);
void onix_gpu_set_sprite_blend(OnixGPUDriver *driver, int mode, float red, float green, float blue, float alpha);
void onix_gpu_set_viewport(OnixGPUDriver *driver, int32_t x, int32_t y, int32_t width, int32_t height);
void onix_gpu_set_scissor(OnixGPUDriver *driver, int enabled, int32_t x, int32_t y, int32_t width, int32_t height);
void onix_gpu_set_vsync(OnixGPUDriver *driver, int enabled);
void onix_gpu_set_corner_radius(OnixGPUDriver *driver, int radius);
int onix_gpu_present(OnixGPUDriver *driver);

/* levels is the depth of the mip chain the caller wants. An uncompressed
 * payload supplies the base level only and the driver blits the rest; a
 * block-compressed payload supplies every level, because the device cannot
 * generate mips for block data. */
uint32_t onix_gpu_create_texture_with_format(OnixGPUDriver *driver, uint32_t width, uint32_t height, uint32_t levels, uint32_t format, int mipmap, const uint8_t *pixels, size_t length, uint32_t wrap_u, uint32_t wrap_v);
/* Encoded size of a mip chain, exposed so the Go side can check its own
 * layout arithmetic against this one. */
uint64_t onix_gpu_texture_chain_size(uint32_t format, uint32_t width, uint32_t height, uint32_t levels);
uint32_t onix_gpu_texture_mip_level_count(uint32_t width, uint32_t height);
uint32_t onix_gpu_create_external_d3d11_texture(OnixGPUDriver *driver, uintptr_t texture, uint32_t width, uint32_t height);
uint32_t onix_gpu_create_external_dmabuf_texture(OnixGPUDriver *driver, int fd, uint32_t width, uint32_t height, uint32_t stride, uint32_t offset, uint64_t modifier);
int onix_gpu_refresh_external_texture(OnixGPUDriver *driver, uint32_t texture);
int onix_gpu_update_textures(OnixGPUDriver *driver, const OnixGPUTextureUpdate *updates, uint32_t update_count, const uint8_t *pixels, size_t length);
/* Prepare validates the entire batch and returns native mapped storage. Fill
 * every data range before calling commit or any other driver entry point.
 * data_offset values must be multiples of four. */
uint8_t *onix_gpu_prepare_texture_updates(OnixGPUDriver *driver, const OnixGPUTextureUpdate *updates, uint32_t update_count, size_t length);
int onix_gpu_commit_texture_updates(OnixGPUDriver *driver);
int onix_gpu_destroy_texture(OnixGPUDriver *driver, uint32_t texture);

uint32_t onix_gpu_create_render_target(OnixGPUDriver *driver, uint32_t width, uint32_t height, uint32_t color_format, int depth, uint32_t *color_texture);
int onix_gpu_resize_render_target(OnixGPUDriver *driver, uint32_t target, uint32_t width, uint32_t height, uint32_t color_format, int depth, uint32_t *color_texture);
int onix_gpu_destroy_render_target(OnixGPUDriver *driver, uint32_t target);
int onix_gpu_begin_render_target(OnixGPUDriver *driver, uint32_t target, const float clear_color[4], float clear_depth, int clear_color_enabled, int clear_depth_enabled);
int onix_gpu_end_render_target(OnixGPUDriver *driver);
int onix_gpu_read_render_target(OnixGPUDriver *driver, uint32_t target, int32_t x, int32_t y, uint32_t width, uint32_t height, uint8_t *destination, size_t length);

uint32_t onix_gpu_create_mesh(OnixGPUDriver *driver, const OnixGPUVertex3D *vertices, uint32_t vertex_count, const uint32_t *indices, uint32_t index_count);
int onix_gpu_set_mesh_deformation(OnixGPUDriver *driver, uint32_t mesh, const uint32_t *words, uint32_t word_count, uint32_t vertex_count);
int onix_gpu_destroy_mesh(OnixGPUDriver *driver, uint32_t mesh);

uint32_t onix_gpu_create_mesh_shader(OnixGPUDriver *driver, OnixGPUShader vertex, OnixGPUShader fragment);
const uint8_t *onix_gpu_pipeline_cache_uuid(OnixGPUDriver *driver);
void onix_gpu_load_pipeline_cache(OnixGPUDriver *driver, const void *data, size_t size);
size_t onix_gpu_pipeline_cache_data(OnixGPUDriver *driver, void *data, size_t size);
int onix_gpu_destroy_mesh_shader(OnixGPUDriver *driver, uint32_t shader);
uint32_t onix_gpu_create_material2d_shader(OnixGPUDriver *driver, OnixGPUShader vertex, OnixGPUShader fragment);
int onix_gpu_destroy_material2d_shader(OnixGPUDriver *driver, uint32_t shader);

/* Sprite and mesh batches validate each draw while recording commands. On
 * failure, earlier draws in the batch may already have been recorded. */
int onix_gpu_draw_sprites(
	OnixGPUDriver *driver,
	const OnixGPUSpritePush *push,
	const OnixGPUInstance *instances,
	uint32_t instance_count,
	const OnixNativeSpriteDraw *draws,
	uint32_t draw_count);

int onix_gpu_draw_shapes(
	OnixGPUDriver *driver,
	const OnixGPUSpritePush *push,
	const OnixGPUShapeInstance *instances,
	uint32_t instance_count);

int onix_gpu_draw_meshes(
	OnixGPUDriver *driver,
	const OnixGPUMeshPush *push,
	const OnixGPUVertex3D *vertices,
	uint32_t vertex_count,
	const uint32_t *indices,
	uint32_t index_count,
	const OnixGPUInstance3D *instances,
	uint32_t instance_count,
	const uint32_t *deformation_words,
	uint32_t deformation_word_count,
	const OnixNativeMeshDraw3D *draws,
	uint32_t draw_count, const float *lighting);

int onix_gpu_draw_mesh_picking(
	OnixGPUDriver *driver,
	const OnixGPUMeshPush *push,
	const OnixGPUVertex3D *vertices,
	uint32_t vertex_count,
	const uint32_t *indices,
	uint32_t index_count,
	const OnixGPUInstance3D *instances,
	uint32_t instance_count,
	const OnixNativeMeshDraw3D *draws,
	uint32_t draw_count);
#endif
