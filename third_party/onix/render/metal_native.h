#ifndef ONIX_ENGINE_RENDER_METAL_NATIVE_H
#define ONIX_ENGINE_RENDER_METAL_NATIVE_H

#include <stddef.h>
#include <stdint.h>
#include "native_buffer.h"
#include "native_draw.h"

typedef struct OnixMetalDriver OnixMetalDriver;
int onix_metal_trim_idle(OnixMetalDriver *driver);
void onix_metal_buffer_memory(OnixMetalDriver *driver, OnixBufferMemory *memory);

typedef struct {
	float model[6];
	float texture_model[6];
	float color[4];
	float custom[4][4];
	float texture_slot;
} OnixMetalInstance;

typedef struct {
	float model[6];
	float local_model[6];
	float fill[4];
	float border[4];
	float params[4];
	uint32_t kind;
} OnixMetalShapeInstance;

typedef struct {
	uint32_t texture;
	int32_t x;
	int32_t y;
	uint32_t width;
	uint32_t height;
	uint64_t data_offset;
	uint64_t data_length;
} OnixMetalTextureUpdate;

/* Keep these values in lockstep with render.TextureFormat and the Vulkan
 * native header.  Metal is built independently, so it cannot include the
 * Vulkan header just to obtain the shared wire format. */
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
	float texture_coverage[4];
} OnixMetalSpriteUniforms;

typedef struct {
	float position[3];
	float normal[3];
	float uv[2];
	float tangent[4];
} OnixMetalVertex3D;

typedef OnixNativeInstance3D OnixMetalInstance3D;

const char *onix_metal_last_error(void);
OnixMetalDriver *onix_metal_create(void *layer, uint32_t width, uint32_t height);
const char *onix_metal_device_name(OnixMetalDriver *driver);
void onix_metal_destroy(OnixMetalDriver *driver);
int onix_metal_resize(OnixMetalDriver *driver, uint32_t width, uint32_t height);
void onix_metal_set_vsync(OnixMetalDriver *driver, int enabled);

int onix_metal_begin_frame(OnixMetalDriver *driver, const float clear_color[4]);
void onix_metal_set_sprite_blend(OnixMetalDriver *driver, int mode, float red, float green, float blue, float alpha);
void onix_metal_set_viewport(OnixMetalDriver *driver, int32_t x, int32_t y, int32_t width, int32_t height);
void onix_metal_set_scissor(OnixMetalDriver *driver, int enabled, int32_t x, int32_t y, int32_t width, int32_t height);
int onix_metal_present(OnixMetalDriver *driver);

uint32_t onix_metal_create_texture_with_format(OnixMetalDriver *driver, uint32_t width, uint32_t height, uint32_t levels, uint32_t format, int mipmap, const uint8_t *pixels, size_t length, uint32_t wrap_u, uint32_t wrap_v);
uint32_t onix_metal_create_external_cv_texture(OnixMetalDriver *driver, uintptr_t pixel_buffer, uint32_t width, uint32_t height);
int onix_metal_refresh_external_texture(OnixMetalDriver *driver, uint32_t texture);
int onix_metal_update_textures(OnixMetalDriver *driver, const OnixMetalTextureUpdate *updates, uint32_t update_count, const uint8_t *pixels, size_t length);
int onix_metal_destroy_texture(OnixMetalDriver *driver, uint32_t texture);

uint32_t onix_metal_create_render_target(OnixMetalDriver *driver, uint32_t width, uint32_t height, uint32_t color_format, int depth, uint32_t *color_texture);
int onix_metal_resize_render_target(OnixMetalDriver *driver, uint32_t target, uint32_t width, uint32_t height, uint32_t color_format, int depth, uint32_t *color_texture);
int onix_metal_destroy_render_target(OnixMetalDriver *driver, uint32_t target);
int onix_metal_begin_render_target(OnixMetalDriver *driver, uint32_t target, const float clear_color[4], double clear_depth, int clear_color_enabled, int clear_depth_enabled);
int onix_metal_end_render_target(OnixMetalDriver *driver);
int onix_metal_read_render_target(OnixMetalDriver *driver, uint32_t target, int32_t x, int32_t y, uint32_t width, uint32_t height, uint8_t *destination, size_t length);

uint32_t onix_metal_create_mesh(OnixMetalDriver *driver, const OnixMetalVertex3D *vertices, uint32_t vertex_count, const uint32_t *indices, uint32_t index_count);
int onix_metal_set_mesh_deformation(OnixMetalDriver *driver, uint32_t mesh, const uint32_t *words, uint32_t word_count, uint32_t vertex_count);
int onix_metal_destroy_mesh(OnixMetalDriver *driver, uint32_t mesh);

uint32_t onix_metal_create_mesh_shader(OnixMetalDriver *driver, const char *source, size_t length);
int onix_metal_destroy_mesh_shader(OnixMetalDriver *driver, uint32_t shader);
uint32_t onix_metal_create_material2d_shader(OnixMetalDriver *driver, const char *source, size_t length);
int onix_metal_destroy_material2d_shader(OnixMetalDriver *driver, uint32_t shader);

int onix_metal_draw_sprites(
	OnixMetalDriver *driver,
	const OnixMetalSpriteUniforms *uniforms,
	const OnixMetalInstance *instances,
	uint32_t instance_count,
	const OnixNativeSpriteDraw *draws,
	uint32_t draw_count);
int onix_metal_draw_shapes(
	OnixMetalDriver *driver,
	const OnixMetalSpriteUniforms *uniforms,
	const OnixMetalShapeInstance *instances,
	uint32_t instance_count);
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
	uint32_t draw_count);

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
	uint32_t draw_count);

#endif
