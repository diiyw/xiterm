#ifndef ONIX_NATIVE_DRAW_H
#define ONIX_NATIVE_DRAW_H

#include <stdint.h>
#include <stddef.h>
#include <string.h>

enum { ONIX_SPRITE_COMPACT_BYTES = 100, ONIX_SPRITE_CUSTOM_BYTES = 132 };

static size_t onix_sprite_stride(uint32_t shader) {
	return shader ? ONIX_SPRITE_CUSTOM_BYTES : ONIX_SPRITE_COMPACT_BYTES;
}

// Pack directly into mapped upload storage: no additional CPU staging buffer.
static void onix_sprite_copy(void *destination, const void *source, uint32_t count, uint32_t shader) {
	if (shader) {
		memcpy(destination, source, (size_t)count * ONIX_SPRITE_CUSTOM_BYTES);
		return;
	}
	uint8_t *dst = destination;
	const uint8_t *src = source;
	for (uint32_t index = 0; index < count; index++, dst += ONIX_SPRITE_COMPACT_BYTES, src += ONIX_SPRITE_CUSTOM_BYTES) {
		memcpy(dst, src, 64);
		memcpy(dst + 64, src + 96, 36);
	}
}

typedef struct {
	float model[16];
	float base_color[4];
	float emissive_metallic[4];
	float material[4];
	float uv_transform[4];
	float custom[4][4];
} OnixNativeInstance3D;

enum { ONIX_MESH_FULL, ONIX_MESH_UNLIT, ONIX_MESH_DEPTH, ONIX_MESH_BASIC, ONIX_MESH_VARIANT_COUNT };

static inline int onix_mesh_builtin_variant(const OnixNativeInstance3D *instances, uint32_t first, uint32_t count, int pass) {
	if (pass == 7) return ONIX_MESH_UNLIT;
	if (pass == 4) return ONIX_MESH_FULL;
	int depth_pass = pass == 1 || pass == 2 || pass == 5;
	int unlit = 1, basic = 1, depth = 1;
	for (uint32_t i = first; i < first + count; i++) {
		const OnixNativeInstance3D *v = &instances[i];
		uint32_t flags = (uint32_t)(v->material[3] + 0.5f);
		int special = v->material[0] < 0 || (flags & (256u | 2048u | 8192u));
		unlit &= special;
		depth &= v->material[0] >= 0 && !(flags & (8192u | 2048u));
		basic &= !special && v->custom[0][3] <= 0 && v->custom[2][0] <= 0 && v->custom[2][1] <= 0 && v->custom[2][2] <= 0 && v->custom[3][0] <= 0;
		if (!unlit && !basic && (!depth_pass || !depth)) return ONIX_MESH_FULL;
	}
	if (depth && depth_pass) return ONIX_MESH_DEPTH;
	if (unlit) return ONIX_MESH_UNLIT;
	return basic ? ONIX_MESH_BASIC : ONIX_MESH_FULL;
}

// Shared CPU draw records; both backends consume these arrays directly.
typedef struct {
	uint32_t shader;
	uint32_t texture;
	uint32_t additional_textures[3];
	uint32_t first_instance;
	uint32_t instance_count;
} OnixNativeSpriteDraw;

typedef struct {
	uint32_t mesh;
	uint32_t shader;
	uint32_t texture;
	uint32_t normal_texture;
	uint32_t metallic_roughness_texture;
	uint32_t extra_textures[13];
	uint32_t first_index;
	uint32_t index_count;
	int32_t base_vertex;
	uint32_t first_instance;
	uint32_t instance_count;
	uint32_t alpha_mode;
	uint32_t blend_mode;
	uint32_t deformation_index;
} OnixNativeMeshDraw3D;

typedef struct {
	float view_projection[16];
	float camera_position[4];
	float ambient_light[4];
	float light_direction[4];
	float directional_light[4];
	float light_position[64];
	float light_directions[64];
	float light_colors[64];
	float light_params[64];
	float light_meta[4];
	float lighting[12];
	float light_right[64];
	float light_up[64];
	float shadow_matrices[256];
	float shadow_rects[64];
	float shadow_params[64];
	float light_shadows[64];
	float occlusion_inverse[16];
} OnixNativeMeshUniforms;

#endif
