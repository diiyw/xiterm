//go:build cgo && (onix_opengl || (linux && !android && !onix_vulkan)) && !ios && (windows || linux || darwin || android)

package render

/*
// These wrappers consume their Go arguments during the GL call.
#cgo noescape onix_gl_begin_frame
#cgo nocallback onix_gl_begin_frame
#cgo noescape onix_gl_begin_render_target
#cgo nocallback onix_gl_begin_render_target
#cgo noescape onix_gl_draw_sprite_run
#cgo nocallback onix_gl_draw_sprite_run
#cgo noescape onix_gl_draw_mesh_run
#cgo nocallback onix_gl_draw_mesh_run
#cgo noescape onix_gl_draw_mesh_picking_run
#cgo nocallback onix_gl_draw_mesh_picking_run
#cgo noescape onix_gl_set_mesh_uniforms
#cgo nocallback onix_gl_set_mesh_uniforms
#cgo noescape onix_gl_set_projection
#cgo nocallback onix_gl_set_projection
#cgo noescape onix_gl_begin_uploads
#cgo nocallback onix_gl_begin_uploads
#cgo noescape onix_gl_finish_uploads
#cgo nocallback onix_gl_finish_uploads
#cgo noescape onix_gl_destroy_uploads
#cgo nocallback onix_gl_destroy_uploads
#cgo noescape onix_gl_upload_mesh_batch
#cgo nocallback onix_gl_upload_mesh_batch
#cgo noescape onix_gl_upload_buffer
#cgo nocallback onix_gl_upload_buffer
#cgo noescape onix_gl_update_textures
#cgo nocallback onix_gl_update_textures
#include "gl_api.h"
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

static void onix_gl_shader_source(GLuint shader, const char *source, GLint length) {
	const GLchar *sources[] = {source};
	glShaderSource(shader, 1, sources, &length);
}

static void onix_gl_vertex_attrib_pointer(GLuint index, GLint size, GLenum type, GLboolean normalized, GLsizei stride, uintptr_t offset) {
	glVertexAttribPointer(index, size, type, normalized, stride, (const void *)offset);
}

static void onix_gl_sprite_attributes(GLsizei stride, uintptr_t base, int initialize) {
	static const GLint sizes[] = {3, 3, 3, 3, 4, 4, 4, 4, 4, 1};
	static const uintptr_t offsets[] = {0, 12, 24, 36, 48, 64, 80, 96, 112, 128};
	for (GLuint attribute = 0; attribute < 10; attribute++) {
		glVertexAttribPointer(attribute + 2, sizes[attribute], GL_FLOAT, GL_FALSE, stride, (const void *)(base + offsets[attribute] - ((stride == 100 && attribute >= 7) ? 32 : 0)));
		if (initialize) {
			glEnableVertexAttribArray(attribute + 2);
			glVertexAttribDivisor(attribute + 2, 1);
		}
	}
}

static void onix_gl_mesh_instance_attributes(GLsizei stride, uintptr_t base, int initialize) {
	for (GLuint attribute = 0; attribute < 12; attribute++) {
		glVertexAttribPointer(attribute + 3, 4, GL_FLOAT, GL_FALSE, stride, (const void *)(base + attribute * 16));
		if (initialize) {
			glEnableVertexAttribArray(attribute + 3);
			glVertexAttribDivisor(attribute + 3, 1);
		}
	}
}

static void onix_gl_vertex_attrib_i_pointer(GLuint index, GLint size, GLenum type, GLsizei stride, uintptr_t offset) {
	glVertexAttribIPointer(index, size, type, stride, (const void *)offset);
}

#include "gles_upload.h"

static int onix_gl_has_extension(const char *name) {
	if (name == NULL || name[0] == '\0') return 0;
	size_t name_length = strlen(name);
	GLint count = 0;
	glGetIntegerv(GL_NUM_EXTENSIONS, &count);
	for (GLint index = 0; index < count; index++) {
		const char *extension = (const char *)glGetStringi(GL_EXTENSIONS, (GLuint)index);
		if (extension != NULL && strlen(extension) == name_length && strcmp(extension, name) == 0) return 1;
	}
	return 0;
}

static int onix_gl_compressed_block_bytes(GLint internal_format) {
	switch (internal_format) {
	case 0x83F0: // COMPRESSED_RGB_S3TC_DXT1
	case 0x8C4C: // COMPRESSED_SRGB_S3TC_DXT1
	case 0x8DBB: // COMPRESSED_RED_RGTC1
		return 8;
	default:
		return 16;
	}
}

static int onix_gl_compressed_format_supported(GLint internal_format) {
	if (internal_format == 0x93B0 || internal_format == 0x93D0) {
		return onix_gl_has_extension("GL_KHR_texture_compression_astc_ldr");
	}
	if (internal_format == 0x83F0 || internal_format == 0x8C4C ||
		internal_format == 0x83F3 || internal_format == 0x8C4F) {
		return onix_gl_has_extension("GL_EXT_texture_compression_s3tc");
	}
	if (internal_format == 0x8DBB || internal_format == 0x8DBD) {
		return onix_gl_has_extension("GL_EXT_texture_compression_rgtc");
	}
	if (internal_format == 0x8E8C || internal_format == 0x8E8D) {
		return onix_gl_has_extension("GL_EXT_texture_compression_bptc");
	}
	return 0;
}

static GLuint onix_gl_create_texture(GLsizei width, GLsizei height, GLint internal_format, GLenum format, GLenum type,
	GLint min_filter, GLint wrap_u, GLint wrap_v, int swizzle_r8, int mipmap, int compressed_internal_format,
	int levels, size_t length, const void *pixels) {
	GLuint texture = 0;
	glGenTextures(1, &texture);
	if (texture == 0) {
		return 0;
	}
	glActiveTexture(GL_TEXTURE0);
	glBindTexture(GL_TEXTURE_2D, texture);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, min_filter);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, wrap_u);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, wrap_v);
	if (swizzle_r8) {
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_SWIZZLE_R, GL_ONE);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_SWIZZLE_G, GL_ONE);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_SWIZZLE_B, GL_ONE);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_SWIZZLE_A, GL_RED);
	}
	if (compressed_internal_format != 0) {
		if (!onix_gl_compressed_format_supported(compressed_internal_format)) {
			glDeleteTextures(1, &texture);
			return 0;
		}
		int block_bytes = onix_gl_compressed_block_bytes(compressed_internal_format);
		size_t offset = 0;
		for (int level = 0; level < levels; level++) {
			GLsizei level_width = width >> level;
			GLsizei level_height = height >> level;
			if (level_width < 1) level_width = 1;
			if (level_height < 1) level_height = 1;
			size_t level_size = (size_t)((level_width + 3) / 4) * ((level_height + 3) / 4) * (size_t)block_bytes;
			if (level_size > length - offset) {
				glDeleteTextures(1, &texture);
				return 0;
			}
			glCompressedTexImage2D(GL_TEXTURE_2D, level, (GLenum)compressed_internal_format,
				level_width, level_height, 0, (GLsizei)level_size, (const uint8_t *)pixels + offset);
			offset += level_size;
		}
		if (offset != length) {
			glDeleteTextures(1, &texture);
			return 0;
		}
	} else {
		glTexImage2D(GL_TEXTURE_2D, 0, internal_format, width, height, 0, format, type, pixels);
		if (mipmap) {
			glGenerateMipmap(GL_TEXTURE_2D);
		}
	}
	return texture;
}

typedef struct {
	GLuint texture;
	GLint x;
	GLint y;
	GLsizei width;
	GLsizei height;
	GLenum format;
	int mipmap;
	size_t data_offset;
} OnixGLESTextureUpdate;

static GLuint onix_gl_update_textures(const OnixGLESTextureUpdate *updates, size_t count, const uint8_t *pixels, GLuint bound_texture) {
	glActiveTexture(GL_TEXTURE0);
	for (size_t index = 0; index < count; index++) {
		const OnixGLESTextureUpdate *update = &updates[index];
		if (bound_texture != update->texture) {
			glBindTexture(GL_TEXTURE_2D, update->texture);
			bound_texture = update->texture;
		}
		glTexSubImage2D(GL_TEXTURE_2D, 0, update->x, update->y, update->width, update->height,
			update->format, GL_UNSIGNED_BYTE, pixels + update->data_offset);
		if (update->mipmap) {
			glGenerateMipmap(GL_TEXTURE_2D);
		}
	}
	return bound_texture;
}

static void onix_gl_use_program(GLuint program, GLint texture, GLint normal_texture, GLint metallic_roughness_texture,
	const GLint *extra_textures, size_t extra_count, const GLint *additional_textures, size_t additional_count,
	GLint generated_texture_v, GLint generated_texture_f, int set_additional, int generated) {
	glUseProgram(program);
	if (texture >= 0) glUniform1i(texture, 0);
	if (normal_texture >= 0) glUniform1i(normal_texture, 1);
	if (metallic_roughness_texture >= 0) glUniform1i(metallic_roughness_texture, 2);
	for (size_t slot = 0; slot < extra_count; slot++) {
		if (extra_textures[slot] >= 0) glUniform1i(extra_textures[slot], (GLint)(3 + slot));
	}
	if (set_additional) {
		for (size_t slot = 0; slot < additional_count; slot++) {
			if (additional_textures[slot] >= 0) glUniform1i(additional_textures[slot], (GLint)(1 + slot));
		}
	}
	if (generated) {
		if (generated_texture_v >= 0) glUniform1i(generated_texture_v, 0);
		if (generated_texture_f >= 0) glUniform1i(generated_texture_f, 0);
	}
}

static void onix_gl_set_projection(GLint projection, GLint generated_v, GLint generated_f, const GLfloat *value, int generated) {
	if (projection >= 0) glUniformMatrix4fv(projection, 1, GL_FALSE, value);
	if (generated) {
		if (generated_v >= 0) glUniformMatrix4fv(generated_v, 1, GL_FALSE, value);
		if (generated_f >= 0) glUniformMatrix4fv(generated_f, 1, GL_FALSE, value);
	}
}

typedef struct {
	GLuint texture;
	GLuint additional_textures[3];
	int bind_additional;
	uintptr_t base;
	GLsizei instances;
} OnixGLESSpriteDraw;

static GLuint onix_gl_draw_sprite_run(const OnixGLESSpriteDraw *draws, size_t count, GLuint bound_texture, GLsizei stride) {
	glActiveTexture(GL_TEXTURE0);
	for (size_t index = 0; index < count; index++) {
		const OnixGLESSpriteDraw *draw = &draws[index];
		if (bound_texture != draw->texture) {
			glBindTexture(GL_TEXTURE_2D, draw->texture);
			bound_texture = draw->texture;
		}
		if (draw->bind_additional) {
			for (size_t slot = 0; slot < 3; slot++) {
				glActiveTexture(GL_TEXTURE0 + (GLenum)(slot + 1));
				glBindTexture(GL_TEXTURE_2D, draw->additional_textures[slot]);
			}
			glActiveTexture(GL_TEXTURE0);
		}
		onix_gl_sprite_attributes(stride, draw->base, 0);
		glDrawElementsInstanced(GL_TRIANGLES, 6, GL_UNSIGNED_SHORT, NULL, draw->instances);
	}
	return bound_texture;
}

typedef struct {
	GLuint texture;
	GLuint normal_texture;
	GLuint metallic_roughness_texture;
	GLuint extra_textures[13];
	GLint has_normal_texture;
	GLint has_metallic_roughness_texture;
	uintptr_t base;
	GLsizei index_count;
	uintptr_t index_offset;
	GLsizei instances;
} OnixGLESMeshDraw;

static void onix_gl_configure_mesh_vertex_attributes(GLuint vertex_buffer, GLuint index_buffer, GLsizei stride);

static GLuint onix_gl_draw_mesh_run(const OnixGLESMeshDraw *draws, size_t count, GLuint instance_buffer,
	GLuint vertex_buffer, GLuint index_buffer, GLint normal_location, GLint metallic_roughness_location,
	GLsizei vertex_stride, GLsizei instance_stride, GLuint bound_texture) {
	onix_gl_configure_mesh_vertex_attributes(vertex_buffer, index_buffer, vertex_stride);
	glBindBuffer(GL_ARRAY_BUFFER, instance_buffer);
	glActiveTexture(GL_TEXTURE0);
	// Auxiliary bindings and uniforms are established on the first draw of
	// each run, since another program or batch may have changed their state.
	GLuint bound_auxiliary[15] = {0};
	GLint has_normal = 0, has_metallic_roughness = 0;
	for (size_t index = 0; index < count; index++) {
		const OnixGLESMeshDraw *draw = &draws[index];
		if (bound_texture != draw->texture) {
			glBindTexture(GL_TEXTURE_2D, draw->texture);
			bound_texture = draw->texture;
		}
		int auxiliary_changed = 0;
		for (size_t slot = 0; slot < 15; slot++) {
			GLuint texture = slot == 0 ? draw->normal_texture : slot == 1 ? draw->metallic_roughness_texture : draw->extra_textures[slot - 2];
			if (index == 0 || bound_auxiliary[slot] != texture) {
				glActiveTexture(GL_TEXTURE0 + (GLenum)(slot + 1));
				glBindTexture(GL_TEXTURE_2D, texture);
				bound_auxiliary[slot] = texture;
				auxiliary_changed = 1;
			}
		}
		if (auxiliary_changed) glActiveTexture(GL_TEXTURE0);
		if (normal_location >= 0 && (index == 0 || has_normal != draw->has_normal_texture)) {
			glUniform1i(normal_location, draw->has_normal_texture);
			has_normal = draw->has_normal_texture;
		}
		if (metallic_roughness_location >= 0 && (index == 0 || has_metallic_roughness != draw->has_metallic_roughness_texture)) {
			glUniform1i(metallic_roughness_location, draw->has_metallic_roughness_texture);
			has_metallic_roughness = draw->has_metallic_roughness_texture;
		}
		onix_gl_mesh_instance_attributes(instance_stride, draw->base, 0);
		glDrawElementsInstanced(GL_TRIANGLES, draw->index_count, GL_UNSIGNED_INT, (const void *)draw->index_offset, draw->instances);
	}
	return bound_texture;
}

typedef struct {
	uintptr_t base;
	GLsizei index_count;
	uintptr_t index_offset;
	GLsizei instances;
} OnixGLESPickingDraw;

static void onix_gl_draw_mesh_picking_run(const OnixGLESPickingDraw *draws, size_t count, GLuint instance_buffer,
	GLuint vertex_buffer, GLuint index_buffer, GLsizei vertex_stride, GLsizei instance_stride) {
	onix_gl_configure_mesh_vertex_attributes(vertex_buffer, index_buffer, vertex_stride);
	glBindBuffer(GL_ARRAY_BUFFER, instance_buffer);
	for (size_t index = 0; index < count; index++) {
		const OnixGLESPickingDraw *draw = &draws[index];
		onix_gl_mesh_instance_attributes(instance_stride, draw->base, 0);
		glDrawElementsInstanced(GL_TRIANGLES, draw->index_count, GL_UNSIGNED_INT, (const void *)draw->index_offset, draw->instances);
	}
}

static int onix_gl_upload_mesh_batch(OnixGLUploadPool *pool, GLuint *vertex_buffer, GLuint *index_buffer, GLuint *instance_buffer, uintptr_t *instance_offset,
	const void *vertices, GLsizeiptr vertex_bytes, const void *indices, GLsizeiptr index_bytes,
	const void *instances, GLsizeiptr instance_bytes) {
	uintptr_t offset;
	if (vertex_bytes > 0) {
		*vertex_buffer = onix_gl_upload_buffer(pool, GL_ARRAY_BUFFER, vertex_bytes, vertices, 1, &offset);
		*index_buffer = onix_gl_upload_buffer(pool, GL_ELEMENT_ARRAY_BUFFER, index_bytes, indices, 1, &offset);
		if (!*vertex_buffer || !*index_buffer) return 0;
	}
	*instance_buffer = onix_gl_upload_buffer(pool, GL_ARRAY_BUFFER, instance_bytes, instances, 0, instance_offset);
	return *instance_buffer != 0;
}

static void onix_gl_configure_mesh_vertex_attributes(GLuint vertex_buffer, GLuint index_buffer, GLsizei stride) {
	glBindBuffer(GL_ARRAY_BUFFER, vertex_buffer);
	onix_gl_vertex_attrib_pointer(0, 3, GL_FLOAT, GL_FALSE, stride, 0);
	glEnableVertexAttribArray(0);
	onix_gl_vertex_attrib_pointer(1, 3, GL_FLOAT, GL_FALSE, stride, 12);
	glEnableVertexAttribArray(1);
	onix_gl_vertex_attrib_pointer(2, 2, GL_FLOAT, GL_FALSE, stride, 24);
	glEnableVertexAttribArray(2);
	if (index_buffer != 0) {
		glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, index_buffer);
	}
	glEnableVertexAttribArray(15);
	glVertexAttribPointer(15, 4, GL_FLOAT, GL_FALSE, stride, (const void *)32);
	glVertexAttribDivisor(15, 0);
}

// Used only after the first surface mesh draw, never by a pure 2D window.
static void onix_gl_blit_surface(GLuint source, GLuint destination, GLsizei width, GLsizei height, int scissor_enabled) {
	if (scissor_enabled) glDisable(GL_SCISSOR_TEST);
	glBindFramebuffer(GL_READ_FRAMEBUFFER, source);
	glBindFramebuffer(GL_DRAW_FRAMEBUFFER, destination);
	glBlitFramebuffer(0, 0, width, height, 0, 0, width, height, GL_COLOR_BUFFER_BIT, GL_NEAREST);
	glBindFramebuffer(GL_FRAMEBUFFER, destination);
	if (scissor_enabled) glEnable(GL_SCISSOR_TEST);
}

static void onix_gl_begin_frame(GLuint framebuffer, const GLfloat *clear, int set_clear, int restore_scissor, int scissor_set,
	GLint scissor_x, GLint scissor_y, GLsizei scissor_width, GLsizei scissor_height, GLsizei framebuffer_height) {
	glBindFramebuffer(GL_FRAMEBUFFER, framebuffer);
	if (restore_scissor) glDisable(GL_SCISSOR_TEST);
	if (set_clear) glClearColor(clear[0], clear[1], clear[2], clear[3]);
	if (framebuffer != 0) {
		glDepthMask(GL_TRUE);
		glClearDepthf(0);
	}
	glClear(GL_COLOR_BUFFER_BIT | (framebuffer != 0 ? GL_DEPTH_BUFFER_BIT : 0));
	if (restore_scissor && scissor_set) {
		glEnable(GL_SCISSOR_TEST);
		glScissor(scissor_x, framebuffer_height - (scissor_y + scissor_height), scissor_width, scissor_height);
	}
}

static void onix_gl_begin_render_target(GLuint framebuffer, int clear_color, int uint_color, const GLuint *uint_clear,
	const GLfloat *clear, int clear_depth, GLfloat depth, int has_depth, int restore_scissor, int scissor_set,
	GLint scissor_x, GLint scissor_y, GLsizei scissor_width, GLsizei scissor_height, GLsizei framebuffer_height) {
	glBindFramebuffer(GL_FRAMEBUFFER, framebuffer);
	if (restore_scissor) glDisable(GL_SCISSOR_TEST);
	if (clear_color) {
		if (uint_color) glClearBufferuiv(GL_COLOR, 0, uint_clear);
		else {
			glClearColor(clear[0], clear[1], clear[2], clear[3]);
			glClear(GL_COLOR_BUFFER_BIT);
		}
	}
	if (clear_depth && has_depth) {
		glDepthMask(GL_TRUE);
		glClearDepthf(depth);
		glClear(GL_DEPTH_BUFFER_BIT);
	}
	if (restore_scissor && scissor_set) {
		glEnable(GL_SCISSOR_TEST);
		glScissor(scissor_x, framebuffer_height - (scissor_y + scissor_height), scissor_width, scissor_height);
	}
}

static void onix_gl_end_render_target(GLuint framebuffer, int scissor_enabled, int scissor_set, GLint scissor_x, GLint scissor_y,
	GLsizei scissor_width, GLsizei scissor_height, GLsizei framebuffer_height) {
	glBindFramebuffer(GL_FRAMEBUFFER, framebuffer);
	if (scissor_enabled && scissor_set) {
		glScissor(scissor_x, framebuffer_height - (scissor_y + scissor_height), scissor_width, scissor_height);
	}
}

static void onix_gl_read_render_target(GLuint framebuffer, GLint x, GLint y, GLsizei width, GLsizei height,
	GLenum format, GLenum type, void *destination, GLuint restore_framebuffer) {
	glBindFramebuffer(GL_FRAMEBUFFER, framebuffer);
	glReadPixels(x, y, width, height, format, type, destination);
	glBindFramebuffer(GL_FRAMEBUFFER, restore_framebuffer);
}

typedef struct {
	GLint camera_position;
	GLint ambient_light;
	GLint light_direction;
	GLint directional_light;
	GLint shadow_matrices;
	GLint shadow_rects;
	GLint shadow_params;
	GLint light_shadows;
	GLint camera_forward;
	GLint occlusion_inverse;
	GLint environment;
	GLint render_params;
	GLint light_position;
	GLint light_directions;
	GLint light_colors;
	GLint light_right;
	GLint light_up;
	GLint light_params;
	GLint light_count;
	GLint light_meta;
} OnixGLESMeshUniformLocations;

static void onix_gl_set_mesh_uniforms(const OnixGLESMeshUniformLocations *locations, const GLfloat *values, GLsizei max_lights, GLsizei max_shadows, GLint light_count) {
	if (locations->camera_position >= 0) glUniform4fv(locations->camera_position, 1, values + 16);
	if (locations->ambient_light >= 0) glUniform4fv(locations->ambient_light, 1, values + 20);
	if (locations->light_direction >= 0) glUniform4fv(locations->light_direction, 1, values + 24);
	if (locations->directional_light >= 0) glUniform4fv(locations->directional_light, 1, values + 28);
	const GLsizei lighting_offset = 32 + max_lights * 16 + 4;
	const GLsizei shadow_offset = lighting_offset + 12 + max_lights * 8;
	if (locations->occlusion_inverse >= 0) glUniformMatrix4fv(locations->occlusion_inverse, 1, GL_FALSE, values + shadow_offset + max_shadows * 24 + max_lights * 4);
	if (locations->shadow_matrices >= 0) glUniformMatrix4fv(locations->shadow_matrices, max_shadows, GL_FALSE, values + shadow_offset);
	if (locations->shadow_rects >= 0) glUniform4fv(locations->shadow_rects, max_shadows, values + shadow_offset + max_shadows * 16);
	if (locations->shadow_params >= 0) glUniform4fv(locations->shadow_params, max_shadows, values + shadow_offset + max_shadows * 20);
	if (locations->light_shadows >= 0) glUniform4fv(locations->light_shadows, max_lights, values + shadow_offset + max_shadows * 24);
	if (locations->environment >= 0) glUniform4fv(locations->environment, 1, values + lighting_offset);
	if (locations->render_params >= 0) glUniform4fv(locations->render_params, 1, values + lighting_offset + 4);
	if (locations->camera_forward >= 0) glUniform4fv(locations->camera_forward, 1, values + lighting_offset + 8);
	if (locations->light_position >= 0) glUniform4fv(locations->light_position, max_lights, values + 32);
	if (locations->light_directions >= 0) glUniform4fv(locations->light_directions, max_lights, values + 32 + max_lights * 4);
	if (locations->light_colors >= 0) glUniform4fv(locations->light_colors, max_lights, values + 32 + max_lights * 8);
	if (locations->light_right >= 0) glUniform4fv(locations->light_right, max_lights, values + lighting_offset + 12);
	if (locations->light_up >= 0) glUniform4fv(locations->light_up, max_lights, values + lighting_offset + 12 + max_lights * 4);
	if (locations->light_params >= 0) glUniform4fv(locations->light_params, max_lights, values + 32 + max_lights * 12);
	if (locations->light_count >= 0) glUniform1i(locations->light_count, light_count);
	if (locations->light_meta >= 0) glUniform4fv(locations->light_meta, 1, values + 32 + max_lights * 16);
}
*/
import "C"

import (
	_ "embed"
	"errors"
	"fmt"
	"unsafe"
)

//go:embed shaders/gles_sprite.vert
var glesSpriteVertex string

//go:embed shaders/gles_sprite.frag
var glesSpriteFragment string

//go:embed shaders/gles_shape.vert
var glesShapeVertex string

//go:embed shaders/gles_shape.frag
var glesShapeFragment string

//go:embed shaders/gles_mesh.vert
var glesMeshVertex string

//go:embed shaders/gles_mesh.frag
var glesMeshFragment string

//go:embed shaders/gles_mesh_unlit.frag
var glesMeshUnlitFragment string

//go:embed shaders/gles_mesh_depth.frag
var glesMeshDepthFragment string

//go:embed shaders/gles_mesh_basic.frag
var glesMeshBasicFragment string

//go:embed shaders/gles_picking.vert
var glesPickingVertex string

//go:embed shaders/gles_picking.frag
var glesPickingFragment string

const glesFramesInFlight = 3

type glesProgram struct {
	extraTextures    [MeshExtraTextureSlots]C.GLint
	shadowMatrices   C.GLint
	shadowRects      C.GLint
	shadowParams     C.GLint
	lightShadows     C.GLint
	cameraForward    C.GLint
	occlusionInverse C.GLint
	environment      C.GLint
	renderParams     C.GLint
	value            C.GLuint

	projection                  C.GLint
	generatedProjectionV        C.GLint
	generatedProjectionF        C.GLint
	texture                     C.GLint
	generatedTextureV           C.GLint
	generatedTextureF           C.GLint
	normalTexture               C.GLint
	metallicRoughnessTexture    C.GLint
	additionalTextures          [SpriteTextureSlots - 1]C.GLint
	hasNormalTexture            C.GLint
	hasMetallicRoughnessTexture C.GLint
	cameraPosition              C.GLint
	ambientLight                C.GLint
	lightDirection              C.GLint
	directionalLight            C.GLint
	lightPosition               C.GLint
	lightDirections             C.GLint
	lightColors                 C.GLint
	lightRight                  C.GLint
	lightUp                     C.GLint
	lightParams                 C.GLint
	lightCount                  C.GLint
	lightMeta                   C.GLint
	projectionValue             [16]float32
	projectionSet               bool
	generatedWGSL               bool
}

type glesTexture struct {
	value  C.GLuint
	filter TextureFilter
	format TextureFormat
}

type glesRenderTarget struct {
	framebuffer C.GLuint
	depth       C.GLuint
	color       TextureHandle
	descriptor  RenderTargetDescriptor
}

type glesMesh struct {
	vertexBuffer C.GLuint
	indexBuffer  C.GLuint
	indexCount   int
}

type glesGeometry struct {
	vao C.GLuint
}

type glesMeshGeometry struct {
	vao            C.GLuint
	vertexBuffer   C.GLuint
	indexBuffer    C.GLuint
	instanceBuffer C.GLuint
	instanceOffset C.uintptr_t
}

type openGLESDriver struct {
	glContext
	afterPresent      func() error
	surfaceTarget     *glesRenderTarget
	deviceName        string
	spriteDraws       []C.OnixGLESSpriteDraw
	updates           []C.OnixGLESTextureUpdate
	pickingDraws      []C.OnixGLESPickingDraw
	meshDraws         []C.OnixGLESMeshDraw
	uploads           [glesFramesInFlight]C.OnixGLUploadPool
	meshes            GenerationPool[MeshResource, glesMesh]
	material2DShaders GenerationPool[Material2DShaderResource, *glesProgram]
	meshShaders       GenerationPool[MeshShaderResource, *glesProgram]
	textures          GenerationPool[TextureResource, glesTexture]
	targets           GenerationPool[RenderTargetResource, *glesRenderTarget]
	textureUpload     nativeTextureUpload
	driverStatistics
	meshDrawsRetention    scratchRetention
	meshGeometry          glesMeshGeometry
	updateRetention       scratchRetention
	spriteDrawsRetention  scratchRetention
	pickingDrawsRetention scratchRetention
	frameIndex            int
	height                int
	width                 int
	lightUniforms         [meshLightUniformFloatCount]float32
	programs              [3]glesProgram
	meshBuiltins          [meshVariantCount - 1]glesProgram
	pickingProgram        glesProgram
	blend                 BlendState
	clear                 Color
	scissor               Rect
	uintClear             [4]C.GLuint
	viewport              Rect
	shapeGeometry         [glesFramesInFlight]glesGeometry
	spriteGeometry        [glesFramesInFlight]glesGeometry
	activeTarget          RenderTargetHandle
	boundTexture          C.GLuint
	quadIndexBuffer       C.GLuint
	boundBuffer           C.GLuint
	boundVAO              C.GLuint
	currentProgram        C.GLuint
	quadVertexBuffer      C.GLuint
	clearSet              bool
	depthEnabled          bool
	destroyed             bool
	blendEnabled          bool
	scissorEnabled        bool
	scissorSet            bool
	blendSet              bool
}

func initializeGLDriver(driver *openGLESDriver) (Driver, error) {
	version := C.glGetString(C.GL_VERSION)
	if version == nil {
		driver.Destroy()
		return nil, errors.New("render: OpenGL ES 3 context has no version")
	}
	if renderer := C.glGetString(C.GL_RENDERER); renderer != nil {
		driver.deviceName = C.GoString((*C.char)(unsafe.Pointer(renderer)))
	}
	for pipeline, sources := range [2][2]string{
		{spriteMultiVertexGLSL, spriteMultiFragmentGLSL},
		{glesShapeVertex, glesShapeFragment},
	} {
		program, err := createGLESProgram(sources[0], sources[1], false)
		if err != nil {
			driver.Destroy()
			return nil, err
		}
		driver.programs[pipeline] = *program
	}
	if err := driver.createGeometryResources(); err != nil {
		driver.Destroy()
		return nil, err
	}
	C.glEnable(C.GL_BLEND)
	C.glPixelStorei(C.GL_UNPACK_ALIGNMENT, 1)
	C.glPixelStorei(C.GL_PACK_ALIGNMENT, 1)
	driver.blendEnabled = true
	C.glDepthFunc(C.GL_GREATER)
	C.glViewport(0, 0, C.GLsizei(driver.width), C.GLsizei(driver.height))
	if err := driver.SetBlend(PremultipliedAlphaBlend); err != nil {
		driver.Destroy()
		return nil, err
	}
	return driver, nil
}

func (d *openGLESDriver) BufferMemoryStatistics() BufferMemoryStatistics {
	if d.destroyed {
		return BufferMemoryStatistics{}
	}
	var memory BufferMemoryStatistics
	for index := range glesFramesInFlight {
		pool := &d.uploads[index]
		memory.InstanceBytes += uint64(pool.bytes)
		memory.NativeScratchBytes += uint64(pool.snapshot_bytes) + uint64(pool.capacity)*uint64(unsafe.Sizeof(C.OnixGLUploadBuffer{})) +
			uint64(pool.cache.capacity)*(uint64(unsafe.Sizeof(C.OnixNativeUpload{}))+8)
	}
	memory.NativeScratchBytes += uint64(d.textureUpload.capacity)
	memory.ScratchBytes += sliceCapacityBytes(d.updates) +
		sliceCapacityBytes(d.spriteDraws) + sliceCapacityBytes(d.meshDraws) + sliceCapacityBytes(d.pickingDraws)
	return memory
}

func (d *openGLESDriver) surfaceFramebuffer() C.GLuint {
	if d.surfaceTarget != nil {
		return d.surfaceTarget.framebuffer
	}
	return 0
}

// EGL windows request no default depth. Upgrade to an owned color/depth FBO
// only for surface 3D, preserving color already drawn in the current frame.
func (d *openGLESDriver) ensureSurfaceDepth(preserveColor bool) error {
	if target := d.surfaceTarget; target != nil && target.descriptor.Width == d.width && target.descriptor.Height == d.height {
		return nil
	}
	target, err := d.createRenderTarget(RenderTargetDescriptor{Width: d.width, Height: d.height, Depth: true})
	if err != nil {
		return err
	}
	if preserveColor {
		C.onix_gl_blit_surface(d.surfaceFramebuffer(), target.framebuffer, C.GLsizei(d.width), C.GLsizei(d.height), C.int(glesInt(d.scissorEnabled)))
		if d.scissorEnabled {
			C.glDisable(C.GL_SCISSOR_TEST)
		}
		C.glDepthMask(C.GL_TRUE)
		C.glClearDepthf(0)
		C.glClear(C.GL_DEPTH_BUFFER_BIT)
		if d.scissorEnabled {
			C.glEnable(C.GL_SCISSOR_TEST)
		}
	}
	d.deleteRenderTarget(d.surfaceTarget, true)
	d.surfaceTarget = target
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, target.framebuffer)
	return glError("create surface depth")
}

func (d *openGLESDriver) RendererInfo() RendererInfo {
	return RendererInfo{Backend: "OpenGL ES 3", Device: d.deviceName, RenderTargetBottomLeft: true}
}

func (d *openGLESDriver) SurfaceSize() (int, int) { return d.width, d.height }

func (d *openGLESDriver) Activate() { d.makeContextCurrent() }

func (d *openGLESDriver) BeginFrame(clear Color) error {
	d.makeContextCurrent()
	if d.activeTarget.Valid() {
		return errors.New("render: OpenGL ES render target is still active")
	}
	if d.surfaceTarget != nil {
		if err := d.ensureSurfaceDepth(false); err != nil {
			return err
		}
	}
	d.frameIndex = (d.frameIndex + 1) % glesFramesInFlight
	if C.onix_gl_begin_uploads(&d.uploads[d.frameIndex]) == 0 {
		return errors.New("render: OpenGL ES upload fence wait failed")
	}
	d.boundVAO, d.boundBuffer = 0, 0
	restoreScissor := d.scissorEnabled
	setClear := !d.clearSet || d.clear != clear
	if setClear {
		d.clear, d.clearSet = clear, true
	}
	values := [4]C.GLfloat{C.GLfloat(clear.R), C.GLfloat(clear.G), C.GLfloat(clear.B), C.GLfloat(clear.A)}
	C.onix_gl_begin_frame(
		d.surfaceFramebuffer(), &values[0], C.int(glesInt(setClear)), C.int(glesInt(restoreScissor)), C.int(glesInt(d.scissorSet)),
		C.GLint(d.scissor.X), C.GLint(d.scissor.Y), C.GLsizei(d.scissor.Width), C.GLsizei(d.scissor.Height), C.GLsizei(d.height),
	)
	if restoreScissor {
		d.scissorEnabled = d.scissorSet
	}
	d.textureUpload.trim()
	d.updates = (&d.updateRetention).trimScratch(d.updates, 32)
	d.spriteDraws = (&d.spriteDrawsRetention).trimScratch(d.spriteDraws, 64)
	d.meshDraws = (&d.meshDrawsRetention).trimScratch(d.meshDraws, 64)
	d.pickingDraws = (&d.pickingDrawsRetention).trimScratch(d.pickingDraws, 64)
	d.beginFrameStatistics(d.width, d.height)
	return glError("begin frame")
}

func (d *openGLESDriver) SetViewport(viewport Rect) {
	if d.viewport == viewport {
		return
	}
	d.viewport = viewport
	C.glViewport(C.GLint(viewport.X), C.GLint(viewport.Y), C.GLsizei(viewport.Width), C.GLsizei(viewport.Height))
}

func (d *openGLESDriver) Viewport() Rect { return d.viewport }

func (d *openGLESDriver) SetBlend(blend BlendState) error {
	if blend.RGB.Src > BlendOneMinusConstantAlpha || blend.RGB.Dst > BlendOneMinusConstantAlpha ||
		blend.Alpha.Src > BlendOneMinusConstantAlpha || blend.Alpha.Dst > BlendOneMinusConstantAlpha {
		return fmt.Errorf("render: invalid OpenGL ES blend state: %+v", blend)
	}
	if d.blendSet && d.blend == blend {
		return nil
	}
	d.setRawBlend(blend)
	d.blend, d.blendSet = blend, true
	return glError("set blend")
}

func (d *openGLESDriver) SetScissor(scissor *Rect) {
	if scissor == nil {
		d.scissorSet = false
		if d.scissorEnabled {
			C.glDisable(C.GL_SCISSOR_TEST)
			d.scissorEnabled = false
		}
		return
	}
	if d.scissorSet && d.scissor == *scissor && d.scissorEnabled {
		return
	}
	d.scissor, d.scissorSet = *scissor, true
	d.applyScissor(*scissor)
}

func (d *openGLESDriver) applyScissor(scissor Rect) {
	if !d.scissorEnabled {
		C.glEnable(C.GL_SCISSOR_TEST)
		d.scissorEnabled = true
	}
	height := d.height
	if target, ok := d.targets.Get(d.activeTarget); ok && target != nil {
		height = target.descriptor.Height
	}
	C.glScissor(C.GLint(scissor.X), C.GLint(height-int(scissor.Y+scissor.Height)), C.GLsizei(scissor.Width), C.GLsizei(scissor.Height))
}

func (d *openGLESDriver) CreateTexture(descriptor TextureDescriptor, pixels []byte) (TextureHandle, error) {
	if err := descriptor.Validate(pixels); err != nil {
		return TextureHandle{}, err
	}
	minFilter := C.GLint(C.GL_LINEAR)
	if descriptor.Filter == TextureFilterLinearMipmapLinear {
		minFilter = C.GL_LINEAR_MIPMAP_LINEAR
	}
	internalFormat, format := C.GLint(C.GL_RGBA), C.GLenum(C.GL_RGBA)
	if descriptor.Format == TextureFormatSRGBA8 {
		internalFormat = C.GL_SRGB8_ALPHA8
	}
	if descriptor.Format == TextureFormatR8 {
		internalFormat, format = C.GL_R8, C.GL_RED
	}
	compressedInternal, _ := descriptor.Format.GLCompressedInternalFormat()
	levels := descriptor.MipLevels()
	if !descriptor.Format.Compressed() {
		levels = 1
	}
	texture := C.onix_gl_create_texture(
		C.GLsizei(descriptor.Width), C.GLsizei(descriptor.Height), internalFormat, format, C.GL_UNSIGNED_BYTE,
		minFilter, C.GLint(descriptor.WrapU.glValue()), C.GLint(descriptor.WrapV.glValue()),
		C.int(glesInt(descriptor.Format == TextureFormatR8)), C.int(glesInt(descriptor.Filter == TextureFilterLinearMipmapLinear)),
		C.int(compressedInternal), C.int(levels), C.size_t(len(pixels)), sliceData(pixels),
	)
	if texture == 0 {
		return TextureHandle{}, errors.New("render: OpenGL ES could not create texture")
	}
	d.boundTexture = texture
	if err := glError("create texture"); err != nil {
		C.glDeleteTextures(1, &texture)
		return TextureHandle{}, err
	}
	d.recordTextureUploadStatistics(1, uint64(len(pixels)))
	return d.textures.Insert(glesTexture{value: texture, filter: descriptor.Filter, format: descriptor.Format}), nil
}

func (d *openGLESDriver) UpdateTextures(updates []TextureUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	d.updates = (&d.updateRetention).resetScratch(d.updates)
	pixels := updates[0].Pixels
	var uploadBytes uint64
	var length int
	for _, update := range updates {
		texture, ok := d.textures.Get(update.Texture)
		region := update.Region
		texelBytes := texture.format.TexelBytes()
		if !ok || texelBytes == 0 || region.X < 0 || region.Y < 0 || region.Width <= 0 || region.Height <= 0 || len(update.Pixels) != region.Width*region.Height*texelBytes {
			return errors.New("render: invalid OpenGL ES texture update")
		}
		format := C.GLenum(C.GL_RGBA)
		if texture.format == TextureFormatR8 {
			format = C.GL_RED
		}
		offset := length
		if len(update.Pixels) > int(^uint(0)>>1)-length {
			return errors.New("render: texture updates are too large")
		}
		length += len(update.Pixels)
		uploadBytes += uint64(len(update.Pixels))
		d.updates = append(d.updates, C.OnixGLESTextureUpdate{
			texture: texture.value, x: C.GLint(region.X), y: C.GLint(region.Y),
			width: C.GLsizei(region.Width), height: C.GLsizei(region.Height), format: format,
			mipmap: C.int(glesInt(texture.filter == TextureFilterLinearMipmapLinear)), data_offset: C.size_t(offset),
		})
	}
	if len(updates) > 1 {
		var ok bool
		pixels, ok = d.textureUpload.reserve(length)
		if !ok {
			return errors.New("render: OpenGL ES texture upload storage is unavailable")
		}
		for index, update := range updates {
			offset := int(d.updates[index].data_offset)
			copy(pixels[offset:offset+len(update.Pixels)], update.Pixels)
		}
	}
	d.boundTexture = C.onix_gl_update_textures(&d.updates[0], C.size_t(len(d.updates)), (*C.uint8_t)(unsafe.Pointer(&pixels[0])), d.boundTexture)
	if err := glError("update textures"); err != nil {
		return err
	}
	d.recordTextureUploadStatistics(len(updates), uploadBytes)
	return nil
}

func (d *openGLESDriver) DestroyTexture(handle TextureHandle) error {
	texture, ok := d.textures.Get(handle)
	if !ok {
		return fmt.Errorf("render: invalid OpenGL ES texture handle %s", handle)
	}
	C.glDeleteTextures(1, &texture.value)
	if d.boundTexture == texture.value {
		d.boundTexture = 0
	}
	d.textures.Remove(handle)
	return glError("destroy texture")
}

func (d *openGLESDriver) CreateRenderTarget(descriptor RenderTargetDescriptor) (RenderTargetHandle, TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return RenderTargetHandle{}, TextureHandle{}, err
	}
	if d.activeTarget.Valid() {
		return RenderTargetHandle{}, TextureHandle{}, errors.New("render: cannot create an OpenGL ES render target during a target pass")
	}
	target, err := d.createRenderTarget(descriptor)
	if err != nil {
		return RenderTargetHandle{}, TextureHandle{}, err
	}
	return d.targets.Insert(target), target.color, nil
}

func (d *openGLESDriver) ResizeRenderTarget(handle RenderTargetHandle, descriptor RenderTargetDescriptor) (TextureHandle, error) {
	if err := descriptor.Validate(); err != nil {
		return TextureHandle{}, err
	}
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget.Valid() {
		return TextureHandle{}, fmt.Errorf("render: invalid OpenGL ES render target resize %s", handle)
	}
	replacement, err := d.createRenderTarget(descriptor)
	if err != nil {
		return TextureHandle{}, err
	}
	d.deleteRenderTarget(target, true)
	*target = *replacement
	return target.color, nil
}

func (d *openGLESDriver) DestroyRenderTarget(handle RenderTargetHandle) error {
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget == handle {
		return fmt.Errorf("render: invalid OpenGL ES render target destruction %s", handle)
	}
	d.deleteRenderTarget(target, true)
	d.targets.Remove(handle)
	return glError("destroy render target")
}

func (d *openGLESDriver) BeginRenderTarget(handle RenderTargetHandle, clear RenderTargetClear) error {
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget.Valid() {
		return fmt.Errorf("render: invalid OpenGL ES render target begin %s", handle)
	}
	d.activeTarget = handle
	restoreScissor := d.scissorEnabled
	values := [4]C.GLfloat{C.GLfloat(clear.Color.R), C.GLfloat(clear.Color.G), C.GLfloat(clear.Color.B), C.GLfloat(clear.Color.A)}
	uintColor := target.descriptor.ColorFormat == RenderTargetUint32
	if clear.ClearColor {
		if uintColor {
			d.uintClear = [4]C.GLuint{
				C.GLuint(max(clear.Color.R, 0)), C.GLuint(max(clear.Color.G, 0)),
				C.GLuint(max(clear.Color.B, 0)), C.GLuint(max(clear.Color.A, 0)),
			}
		} else {
			d.clear, d.clearSet = clear.Color, true
		}
	}
	C.onix_gl_begin_render_target(
		target.framebuffer, C.int(glesInt(clear.ClearColor)), C.int(glesInt(uintColor)), &d.uintClear[0], &values[0],
		C.int(glesInt(clear.ClearDepth)), C.GLfloat(clear.Depth), C.int(glesInt(target.descriptor.Depth)), C.int(glesInt(restoreScissor)), C.int(glesInt(d.scissorSet)),
		C.GLint(d.scissor.X), C.GLint(d.scissor.Y), C.GLsizei(d.scissor.Width), C.GLsizei(d.scissor.Height), C.GLsizei(target.descriptor.Height),
	)
	if restoreScissor {
		d.scissorEnabled = d.scissorSet
	}
	return glError("begin render target")
}

func (d *openGLESDriver) EndRenderTarget() error {
	if !d.activeTarget.Valid() {
		return errors.New("render: no OpenGL ES render target is active")
	}
	C.onix_gl_end_render_target(d.surfaceFramebuffer(), C.int(glesInt(d.scissorEnabled)), C.int(glesInt(d.scissorSet)),
		C.GLint(d.scissor.X), C.GLint(d.scissor.Y), C.GLsizei(d.scissor.Width), C.GLsizei(d.scissor.Height), C.GLsizei(d.height))
	d.activeTarget = RenderTargetHandle{}
	return glError("end render target")
}

func (d *openGLESDriver) ReadRenderTarget(handle RenderTargetHandle, region RenderTargetRegion, destination []byte) error {
	target, ok := d.targets.Get(handle)
	if !ok || target == nil || d.activeTarget.Valid() {
		return fmt.Errorf("render: invalid OpenGL ES render target readback %s", handle)
	}
	format, pixelType := C.GLenum(C.GL_RGBA), C.GLenum(C.GL_UNSIGNED_BYTE)
	if target.descriptor.ColorFormat == RenderTargetUint32 {
		format, pixelType = C.GL_RED_INTEGER, C.GL_UNSIGNED_INT
	} else if target.descriptor.ColorFormat == RenderTargetR8 {
		format = C.GL_RED
	}
	y := target.descriptor.Height - region.Y - region.Height
	C.onix_gl_read_render_target(target.framebuffer, C.GLint(region.X), C.GLint(y), C.GLsizei(region.Width), C.GLsizei(region.Height), format, pixelType, sliceData(destination), d.surfaceFramebuffer())
	if err := glError("read render target"); err != nil {
		return err
	}
	flipRenderTargetRows(destination, region.Width*target.descriptor.ColorFormat.TexelBytes(), region.Height)
	return nil
}

func (d *openGLESDriver) createRenderTarget(descriptor RenderTargetDescriptor) (*glesRenderTarget, error) {
	defer C.glBindFramebuffer(C.GL_FRAMEBUFFER, d.surfaceFramebuffer())
	var texture, framebuffer, depth C.GLuint
	C.glGenTextures(1, &texture)
	C.glGenFramebuffers(1, &framebuffer)
	if texture == 0 || framebuffer == 0 {
		deleteGLESRenderTargetObjects(framebuffer, texture, depth)
		return nil, errors.New("render: OpenGL ES could not create render target resources")
	}
	d.bindTexture(texture)
	internalFormat, format, pixelType := C.GLint(C.GL_RGBA8), C.GLenum(C.GL_RGBA), C.GLenum(C.GL_UNSIGNED_BYTE)
	filter := TextureFilterLinear
	if descriptor.ColorFormat == RenderTargetUint32 {
		internalFormat, format, pixelType = C.GL_R32UI, C.GL_RED_INTEGER, C.GL_UNSIGNED_INT
		filter = TextureFilter(255)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_NEAREST)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_NEAREST)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	} else {
		if descriptor.ColorFormat == RenderTargetR8 {
			internalFormat, format = C.GL_R8, C.GL_RED
			C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_SWIZZLE_R, C.GL_ONE)
			C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_SWIZZLE_G, C.GL_ONE)
			C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_SWIZZLE_B, C.GL_ONE)
			C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_SWIZZLE_A, C.GL_RED)
		}
		d.setTextureParameters(filter, TextureAddressClamp, TextureAddressClamp)
	}
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, internalFormat, C.GLsizei(descriptor.Width), C.GLsizei(descriptor.Height), 0, format, pixelType, nil)
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, framebuffer)
	C.glFramebufferTexture2D(C.GL_FRAMEBUFFER, C.GL_COLOR_ATTACHMENT0, C.GL_TEXTURE_2D, texture, 0)
	if descriptor.Depth {
		C.glGenRenderbuffers(1, &depth)
		C.glBindRenderbuffer(C.GL_RENDERBUFFER, depth)
		C.glRenderbufferStorage(C.GL_RENDERBUFFER, C.GL_DEPTH_COMPONENT24, C.GLsizei(descriptor.Width), C.GLsizei(descriptor.Height))
		C.glFramebufferRenderbuffer(C.GL_FRAMEBUFFER, C.GL_DEPTH_ATTACHMENT, C.GL_RENDERBUFFER, depth)
	}
	complete := (!descriptor.Depth || depth != 0) && C.glCheckFramebufferStatus(C.GL_FRAMEBUFFER) == C.GL_FRAMEBUFFER_COMPLETE
	C.glBindRenderbuffer(C.GL_RENDERBUFFER, 0)
	C.glBindFramebuffer(C.GL_FRAMEBUFFER, 0)
	if !complete {
		deleteGLESRenderTargetObjects(framebuffer, texture, depth)
		d.boundTexture = 0
		return nil, errors.New("render: OpenGL ES framebuffer is incomplete")
	}
	if err := glError("create render target"); err != nil {
		deleteGLESRenderTargetObjects(framebuffer, texture, depth)
		d.boundTexture = 0
		return nil, err
	}
	textureFormat := TextureFormatRGBA8
	if descriptor.ColorFormat == RenderTargetR8 {
		textureFormat = TextureFormatR8
	}
	color := d.textures.Insert(glesTexture{value: texture, filter: filter, format: textureFormat})
	return &glesRenderTarget{framebuffer: framebuffer, depth: depth, color: color, descriptor: descriptor}, nil
}

func (d *openGLESDriver) deleteRenderTarget(target *glesRenderTarget, removeColor bool) {
	if target == nil {
		return
	}
	if target.framebuffer != 0 {
		C.glDeleteFramebuffers(1, &target.framebuffer)
	}
	if target.depth != 0 {
		C.glDeleteRenderbuffers(1, &target.depth)
	}
	if removeColor {
		if texture, ok := d.textures.Remove(target.color); ok {
			C.glDeleteTextures(1, &texture.value)
			if d.boundTexture == texture.value {
				d.boundTexture = 0
			}
		}
	}
	target.framebuffer, target.depth, target.color = 0, 0, TextureHandle{}
}

func deleteGLESRenderTargetObjects(framebuffer, texture, depth C.GLuint) {
	if framebuffer != 0 {
		C.glDeleteFramebuffers(1, &framebuffer)
	}
	if texture != 0 {
		C.glDeleteTextures(1, &texture)
	}
	if depth != 0 {
		C.glDeleteRenderbuffers(1, &depth)
	}
}

func (d *openGLESDriver) DrawSprites(batch SpriteBatch) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	scratch := spriteUploadBuffers.get()
	defer spriteUploadBuffers.put(scratch)
	scratch.data = scratch.data[:0]
	var sprites uint64
	for _, draw := range batch.Draws {
		sprites += uint64(draw.InstanceCount)
		if draw.InstanceCount == 0 || uint64(draw.FirstInstance)+uint64(draw.InstanceCount) > uint64(len(batch.Instances)) {
			return errors.New("render: invalid OpenGL ES sprite draw range")
		}
		for slot := range SpriteTextureSlots {
			if _, ok := d.textures.Get(draw.TextureAt(slot)); !ok {
				return fmt.Errorf("render: invalid OpenGL ES texture handle %s", draw.TextureAt(slot))
			}
		}
		if draw.Shader.Valid() {
			if _, ok := d.material2DShaders.Get(draw.Shader); !ok {
				return fmt.Errorf("render: invalid OpenGL ES Material2D shader handle %s", draw.Shader)
			}
		}
		scratch.data = appendSpriteDrawInstances(scratch.data, batch.Instances, draw)
	}
	d.setDepthEnabled(false)
	d.setBlendEnabled(true)
	geometry := &d.spriteGeometry[d.frameIndex]
	d.bindVertexArray(geometry.vao)
	base, err := d.uploadGLESInstanceBuffer(geometry, scratch.data)
	if err != nil {
		return err
	}
	packedOffset := uintptr(0)
	for index := 0; index < len(batch.Draws); {
		program := &d.programs[PipelineSprite]
		if batch.Draws[index].Shader.Valid() {
			program, _ = d.material2DShaders.Get(batch.Draws[index].Shader)
		}
		d.useProgram(program)
		d.setProjection(program, batch.Projection)
		d.spriteDraws = (&d.spriteDrawsRetention).resetScratch(d.spriteDraws)
		for index < len(batch.Draws) {
			draw := batch.Draws[index]
			drawProgram := &d.programs[PipelineSprite]
			if draw.Shader.Valid() {
				drawProgram, _ = d.material2DShaders.Get(draw.Shader)
			}
			if drawProgram != program {
				break
			}
			texture, _ := d.textures.Get(draw.Texture)
			packedOffset = uintptr(spriteInstanceOffset(uint64(packedOffset), draw))
			nativeDraw := C.OnixGLESSpriteDraw{
				texture: texture.value, bind_additional: C.int(glesInt(!draw.Shader.Valid())),
				base: C.uintptr_t(base + packedOffset), instances: C.GLsizei(draw.InstanceCount),
			}
			for slot := 1; slot < SpriteTextureSlots; slot++ {
				value, _ := d.textures.Get(draw.TextureAt(slot))
				nativeDraw.additional_textures[slot-1] = value.value
			}
			packedOffset += uintptr(draw.InstanceCount) * uintptr(spriteInstanceStride(draw.Shader.Valid()))
			d.spriteDraws = append(d.spriteDraws, nativeDraw)
			index++
		}
		d.boundTexture = C.onix_gl_draw_sprite_run(&d.spriteDraws[0], C.size_t(len(d.spriteDraws)), d.boundTexture, C.GLsizei(spriteInstanceStride(program != &d.programs[PipelineSprite])))
	}
	if err := glError("draw sprites"); err != nil {
		return err
	}
	d.recordSpriteStatistics(batch, sprites, uint64(len(scratch.data)))
	return nil
}

func (d *openGLESDriver) CreateMaterial2DShader(descriptor ShaderDescriptor) (Material2DShaderHandle, error) {
	return createGLESShader(descriptor, glesSpriteVertex, glesSpriteFragment, "Material2D", &d.material2DShaders)
}

func (d *openGLESDriver) DestroyMaterial2DShader(handle Material2DShaderHandle) error {
	return d.destroyGLESShader(handle, "Material2D", &d.material2DShaders)
}

func (d *openGLESDriver) DrawShapes(batch ShapeBatch) error {
	if len(batch.Instances) == 0 {
		return nil
	}
	d.setDepthEnabled(false)
	d.setBlendEnabled(true)
	program := &d.programs[PipelineShape]
	d.useProgram(program)
	d.setProjection(program, batch.Projection)
	geometry := &d.shapeGeometry[d.frameIndex]
	d.bindVertexArray(geometry.vao)
	if _, err := d.uploadGLESInstanceBuffer(geometry, batch.Instances); err != nil {
		return err
	}
	C.glDrawElementsInstanced(C.GL_TRIANGLES, 6, C.GL_UNSIGNED_SHORT, nil, C.GLsizei(len(batch.Instances)))
	if err := glError("draw shapes"); err != nil {
		return err
	}
	d.recordShapeStatistics(batch)
	return nil
}

func (d *openGLESDriver) CreateMesh(vertices []Vertex3D, indices []uint32) (MeshHandle, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return MeshHandle{}, errors.New("render: OpenGL ES mesh data is empty")
	}
	mesh := glesMesh{indexCount: len(indices)}
	C.glGenBuffers(1, &mesh.vertexBuffer)
	C.glGenBuffers(1, &mesh.indexBuffer)
	if mesh.vertexBuffer == 0 || mesh.indexBuffer == 0 {
		d.deleteMesh(mesh)
		return MeshHandle{}, errors.New("render: OpenGL ES could not create mesh buffers")
	}
	d.bindVertexArray(0)
	d.bindArrayBuffer(mesh.vertexBuffer)
	C.glBufferData(C.GL_ARRAY_BUFFER, C.GLsizeiptr(len(vertices)*int(unsafe.Sizeof(Vertex3D{}))), sliceData(vertices), C.GL_STATIC_DRAW)
	C.glBindBuffer(C.GL_ELEMENT_ARRAY_BUFFER, mesh.indexBuffer)
	C.glBufferData(C.GL_ELEMENT_ARRAY_BUFFER, C.GLsizeiptr(len(indices)*4), sliceData(indices), C.GL_STATIC_DRAW)
	if err := glError("create mesh"); err != nil {
		d.deleteMesh(mesh)
		return MeshHandle{}, err
	}
	return d.meshes.Insert(mesh), nil
}

func (d *openGLESDriver) DestroyMesh(handle MeshHandle) error {
	mesh, ok := d.meshes.Remove(handle)
	if !ok {
		return fmt.Errorf("render: invalid OpenGL ES mesh handle %s", handle)
	}
	d.deleteMesh(mesh)
	return glError("destroy mesh")
}

func (d *openGLESDriver) DrawMeshes(batch MeshBatch3D) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	if err := d.validateMeshBatch(batch, false); err != nil {
		return err
	}
	if !d.activeTarget.Valid() {
		if err := d.ensureSurfaceDepth(true); err != nil {
			return err
		}
	}

	d.setDepthEnabled(true)
	d.setBlendEnabled(false)
	C.glDepthMask(C.GL_TRUE)
	d.bindVertexArray(d.meshGeometry.vao)
	if err := d.uploadMeshBatch(batch); err != nil {
		return err
	}

	transparent := false
	blendMode := MeshBlendMode(^uint32(0))
	var currentProgram *glesProgram
	boundGeometry := false
	boundMesh := MeshHandle{}
	var currentVertexBuffer, currentIndexBuffer C.GLuint
	var counts meshDrawStatistics
	d.meshDraws = (&d.meshDrawsRetention).resetScratch(d.meshDraws)
	flush := func(program *glesProgram, vertexBuffer, indexBuffer C.GLuint) {
		if len(d.meshDraws) == 0 {
			return
		}
		d.boundTexture = C.onix_gl_draw_mesh_run(&d.meshDraws[0], C.size_t(len(d.meshDraws)), d.meshGeometry.instanceBuffer,
			vertexBuffer, indexBuffer, program.hasNormalTexture, program.hasMetallicRoughnessTexture,
			C.GLsizei(unsafe.Sizeof(Vertex3D{})), C.GLsizei(unsafe.Sizeof(Instance3D{})), d.boundTexture)
		d.meshDraws = (&d.meshDrawsRetention).resetScratch(d.meshDraws)
	}
	var currentDrawProgram *glesProgram
	for _, draw := range batch.Draws {
		counts.add(draw)
		if !boundGeometry || draw.Mesh != boundMesh {
			flush(currentDrawProgram, currentVertexBuffer, currentIndexBuffer)
			vertexBuffer, indexBuffer := d.meshGeometry.vertexBuffer, d.meshGeometry.indexBuffer
			if draw.Mesh.Valid() {
				mesh, _ := d.meshes.Get(draw.Mesh)
				vertexBuffer, indexBuffer = mesh.vertexBuffer, mesh.indexBuffer
			}
			currentVertexBuffer, currentIndexBuffer = vertexBuffer, indexBuffer
			boundGeometry, boundMesh = true, draw.Mesh
		}
		program := &d.programs[PipelineMesh]
		if draw.Shader.Valid() {
			program, _ = d.meshShaders.Get(draw.Shader)
		} else {
			variant := meshBuiltinVariant(&batch, &draw)
			if variant != meshVariantFull {
				program = &d.meshBuiltins[variant-1]
			}
			if program.value == 0 {
				source := [meshVariantCount]string{glesMeshFragment, glesMeshUnlitFragment, glesMeshDepthFragment, glesMeshBasicFragment}[variant]
				compiled, err := createGLESProgram(glesMeshVertex, source, false)
				if err != nil {
					return err
				}
				*program = *compiled
			}
		}
		if program != currentProgram {
			flush(currentDrawProgram, currentVertexBuffer, currentIndexBuffer)
			currentProgram = program
			d.useProgram(program)
			d.setProjection(program, batch.ViewProjection)
			d.setMeshUniforms(program, batch)
		}
		isTransparent := draw.AlphaMode == 2
		if isTransparent != transparent {
			flush(currentDrawProgram, currentVertexBuffer, currentIndexBuffer)
			transparent = isTransparent
			d.setBlendEnabled(transparent)
			C.glDepthMask(glesBoolean(!transparent))
		}
		if isTransparent && draw.BlendMode != blendMode {
			flush(currentDrawProgram, currentVertexBuffer, currentIndexBuffer)
			blendMode = draw.BlendMode
			d.setMeshBlendMode(blendMode)
		}
		texture, _ := d.textures.Get(draw.Texture)
		normalTexture, hasNormal := texture, C.GLint(0)
		if draw.NormalTexture.Valid() {
			normalTexture, _ = d.textures.Get(draw.NormalTexture)
			hasNormal = 1
		}
		metallicRoughnessTexture, hasMetallicRoughness := texture, C.GLint(0)
		if draw.MetallicRoughnessTexture.Valid() {
			metallicRoughnessTexture, _ = d.textures.Get(draw.MetallicRoughnessTexture)
			hasMetallicRoughness = 1
		}
		var extraTextures [MeshExtraTextureSlots]C.GLuint
		for slot, handle := range draw.ExtraTextures(&batch.Lighting) {
			extra := texture
			if handle.Valid() {
				var ok bool
				extra, ok = d.textures.Get(handle)
				if !ok {
					return fmt.Errorf("render: invalid surface texture %s", handle)
				}
			}
			extraTextures[slot] = extra.value
		}
		currentDrawProgram = program
		nativeDraw := C.OnixGLESMeshDraw{
			texture: texture.value, normal_texture: normalTexture.value, metallic_roughness_texture: metallicRoughnessTexture.value,
			has_normal_texture: hasNormal, has_metallic_roughness_texture: hasMetallicRoughness,
			base: d.meshGeometry.instanceOffset + C.uintptr_t(uintptr(draw.FirstInstance)*unsafe.Sizeof(Instance3D{})), index_count: C.GLsizei(draw.IndexCount),
			index_offset: C.uintptr_t(draw.FirstIndex * 4), instances: C.GLsizei(draw.InstanceCount),
		}
		copy(nativeDraw.extra_textures[:], extraTextures[:])
		d.meshDraws = append(d.meshDraws, nativeDraw)
	}
	flush(currentDrawProgram, currentVertexBuffer, currentIndexBuffer)
	if blendMode <= MeshBlendScreen && d.blendSet {
		d.setRawBlend(d.blend)
	}
	C.glDepthMask(C.GL_TRUE)
	if err := glError("draw meshes"); err != nil {
		return err
	}
	d.recordMeshStatistics(batch, counts)
	return nil
}

func (d *openGLESDriver) DrawMeshPicking(batch MeshBatch3D) error {
	if len(batch.Instances) == 0 || len(batch.Draws) == 0 {
		return nil
	}
	if err := d.validateMeshBatch(batch, true); err != nil {
		return err
	}
	if d.pickingProgram.value == 0 {
		program, err := createGLESProgram(glesPickingVertex, glesPickingFragment, false)
		if err != nil {
			return err
		}
		d.pickingProgram = *program
	}

	d.setDepthEnabled(true)
	d.setBlendEnabled(false)
	C.glDepthMask(C.GL_TRUE)
	d.bindVertexArray(d.meshGeometry.vao)
	if err := d.uploadMeshBatch(batch); err != nil {
		return err
	}
	d.useProgram(&d.pickingProgram)
	d.setProjection(&d.pickingProgram, batch.ViewProjection)
	boundGeometry := false
	boundMesh := MeshHandle{}
	var currentVertexBuffer, currentIndexBuffer C.GLuint
	d.pickingDraws = (&d.pickingDrawsRetention).resetScratch(d.pickingDraws)
	flush := func() {
		if len(d.pickingDraws) == 0 {
			return
		}
		C.onix_gl_draw_mesh_picking_run(&d.pickingDraws[0], C.size_t(len(d.pickingDraws)), d.meshGeometry.instanceBuffer,
			currentVertexBuffer, currentIndexBuffer, C.GLsizei(unsafe.Sizeof(Vertex3D{})), C.GLsizei(unsafe.Sizeof(Instance3D{})))
		d.pickingDraws = (&d.pickingDrawsRetention).resetScratch(d.pickingDraws)
	}
	for _, draw := range batch.Draws {
		if !boundGeometry || draw.Mesh != boundMesh {
			flush()
			vertexBuffer, indexBuffer := d.meshGeometry.vertexBuffer, d.meshGeometry.indexBuffer
			if draw.Mesh.Valid() {
				mesh, _ := d.meshes.Get(draw.Mesh)
				vertexBuffer, indexBuffer = mesh.vertexBuffer, mesh.indexBuffer
			}
			currentVertexBuffer, currentIndexBuffer = vertexBuffer, indexBuffer
			boundGeometry, boundMesh = true, draw.Mesh
		}
		d.pickingDraws = append(d.pickingDraws, C.OnixGLESPickingDraw{
			base: d.meshGeometry.instanceOffset + C.uintptr_t(uintptr(draw.FirstInstance)*unsafe.Sizeof(Instance3D{})), index_count: C.GLsizei(draw.IndexCount),
			index_offset: C.uintptr_t(draw.FirstIndex * 4), instances: C.GLsizei(draw.InstanceCount),
		})
	}
	flush()
	d.boundBuffer = d.meshGeometry.instanceBuffer
	return glError("draw mesh picking")
}

func (d *openGLESDriver) validateMeshBatch(batch MeshBatch3D, picking bool) error {
	target, targetOK := d.targets.Get(d.activeTarget)
	if picking && (!targetOK || target == nil || target.descriptor.ColorFormat != RenderTargetUint32) {
		return errors.New("render: OpenGL ES mesh picking requires an active R32Uint target")
	}
	dynamic := len(batch.Vertices) != 0 || len(batch.Indices) != 0
	if dynamic && (len(batch.Vertices) == 0 || len(batch.Indices) == 0) {
		return errors.New("render: incomplete OpenGL ES dynamic mesh data")
	}
	for _, draw := range batch.Draws {
		indexCount := len(batch.Indices)
		if draw.Mesh.Valid() {
			mesh, ok := d.meshes.Get(draw.Mesh)
			if !ok {
				return fmt.Errorf("render: invalid OpenGL ES mesh handle %s", draw.Mesh)
			}
			indexCount = mesh.indexCount
		} else if !dynamic {
			return errors.New("render: OpenGL ES mesh draw has no geometry")
		}
		if draw.IndexCount == 0 || uint64(draw.FirstIndex)+uint64(draw.IndexCount) > uint64(indexCount) ||
			draw.BaseVertex != 0 || draw.InstanceCount == 0 || uint64(draw.FirstInstance)+uint64(draw.InstanceCount) > uint64(len(batch.Instances)) {
			return errors.New("render: invalid OpenGL ES mesh draw range")
		}
		if picking {
			continue
		}
		if draw.AlphaMode > 2 || draw.BlendMode > MeshBlendScreen {
			return errors.New("render: invalid OpenGL ES mesh material")
		}
		if _, ok := d.textures.Get(draw.Texture); !ok {
			return fmt.Errorf("render: invalid OpenGL ES texture handle %s", draw.Texture)
		}
		if draw.NormalTexture.Valid() {
			if _, ok := d.textures.Get(draw.NormalTexture); !ok {
				return fmt.Errorf("render: invalid OpenGL ES normal texture handle %s", draw.NormalTexture)
			}
		}
		if draw.MetallicRoughnessTexture.Valid() {
			if _, ok := d.textures.Get(draw.MetallicRoughnessTexture); !ok {
				return fmt.Errorf("render: invalid OpenGL ES metallic roughness texture handle %s", draw.MetallicRoughnessTexture)
			}
		}
		if draw.Shader.Valid() {
			if _, ok := d.meshShaders.Get(draw.Shader); !ok {
				return fmt.Errorf("render: invalid OpenGL ES mesh shader handle %s", draw.Shader)
			}
		}
	}
	return nil
}

func (d *openGLESDriver) uploadMeshBatch(batch MeshBatch3D) error {
	var vertices unsafe.Pointer
	var indices unsafe.Pointer
	vertexBytes, indexBytes := 0, 0
	if len(batch.Vertices) != 0 {
		vertices = sliceData(batch.Vertices)
		indices = sliceData(batch.Indices)
		vertexBytes = len(batch.Vertices) * int(unsafe.Sizeof(Vertex3D{}))
		indexBytes = len(batch.Indices) * 4
	}
	if C.onix_gl_upload_mesh_batch(
		&d.uploads[d.frameIndex], &d.meshGeometry.vertexBuffer, &d.meshGeometry.indexBuffer, &d.meshGeometry.instanceBuffer, &d.meshGeometry.instanceOffset,
		vertices, C.GLsizeiptr(vertexBytes), indices, C.GLsizeiptr(indexBytes),
		sliceData(batch.Instances), C.GLsizeiptr(len(batch.Instances)*int(unsafe.Sizeof(Instance3D{}))),
	) == 0 {
		return errors.New("render: OpenGL ES mesh upload allocation failed")
	}
	d.boundBuffer = d.meshGeometry.instanceBuffer
	return nil
}

func (d *openGLESDriver) setMeshUniforms(program *glesProgram, batch MeshBatch3D) {
	count := writeMeshSceneUniforms(d.lightUniforms[:], &batch)
	locations := C.OnixGLESMeshUniformLocations{
		camera_position: program.cameraPosition, ambient_light: program.ambientLight,
		light_direction: program.lightDirection, directional_light: program.directionalLight,
		shadow_matrices: program.shadowMatrices, shadow_rects: program.shadowRects, shadow_params: program.shadowParams,
		light_shadows: program.lightShadows, camera_forward: program.cameraForward, occlusion_inverse: program.occlusionInverse,
		environment: program.environment, render_params: program.renderParams,
		light_position: program.lightPosition, light_directions: program.lightDirections,
		light_colors: program.lightColors, light_right: program.lightRight, light_up: program.lightUp,
		light_params: program.lightParams, light_count: program.lightCount, light_meta: program.lightMeta,
	}
	C.onix_gl_set_mesh_uniforms(&locations, (*C.GLfloat)(unsafe.Pointer(&d.lightUniforms[0])), MaxMeshLights, MaxShadowMaps, C.GLint(count))
}

func (d *openGLESDriver) CreateMeshShader(descriptor ShaderDescriptor) (MeshShaderHandle, error) {
	return createGLESShader(descriptor, glesMeshVertex, glesMeshFragment, "mesh", &d.meshShaders)
}

func (d *openGLESDriver) DestroyMeshShader(handle MeshShaderHandle) error {
	return d.destroyGLESShader(handle, "mesh", &d.meshShaders)
}

func (d *openGLESDriver) Present() error {
	if d.destroyed {
		return errors.New("render: OpenGL ES driver is destroyed")
	}
	if d.activeTarget.Valid() {
		return errors.New("render: cannot present while an OpenGL ES render target is active")
	}
	if framebuffer := d.surfaceFramebuffer(); framebuffer != 0 {
		C.onix_gl_blit_surface(framebuffer, 0, C.GLsizei(d.width), C.GLsizei(d.height), C.int(glesInt(d.scissorEnabled)))
		C.glBindFramebuffer(C.GL_FRAMEBUFFER, framebuffer)
	}
	if C.onix_gl_finish_uploads(&d.uploads[d.frameIndex]) == 0 {
		return errors.New("render: OpenGL ES upload fence creation failed")
	}
	if err := d.swapContextBuffers(); err != nil {
		return err
	}
	if d.afterPresent != nil {
		return d.afterPresent()
	}
	return nil
}

func (d *openGLESDriver) ResizeSurface(width, height int) error {
	if width <= 0 || height <= 0 || d.width == width && d.height == height {
		return nil
	}
	d.width, d.height = width, height
	d.resizeContext(width, height)
	return nil
}

func (d *openGLESDriver) SetVSync(enabled bool) { d.setContextVSync(enabled) }

func (d *openGLESDriver) Destroy() {
	if d == nil || d.destroyed {
		return
	}
	d.destroyed = true
	d.textureUpload.release()
	d.makeContextCurrent()
	d.deleteRenderTarget(d.surfaceTarget, true)
	d.surfaceTarget = nil
	d.targets.Range(func(target *glesRenderTarget) bool {
		d.deleteRenderTarget(target, false)
		return true
	})
	d.textures.Range(func(texture glesTexture) bool {
		C.glDeleteTextures(1, &texture.value)
		return true
	})
	d.meshShaders.Range(func(program *glesProgram) bool {
		deleteGLESProgram(program)
		return true
	})
	d.material2DShaders.Range(func(program *glesProgram) bool {
		deleteGLESProgram(program)
		return true
	})
	d.meshes.Range(func(mesh glesMesh) bool {
		d.deleteMesh(mesh)
		return true
	})
	for index := range d.meshBuiltins {
		deleteGLESProgram(&d.meshBuiltins[index])
	}
	for index := range d.programs {
		deleteGLESProgram(&d.programs[index])
	}
	deleteGLESProgram(&d.pickingProgram)
	for index := range glesFramesInFlight {
		C.onix_gl_destroy_uploads(&d.uploads[index])
		for _, geometry := range []*glesGeometry{&d.spriteGeometry[index], &d.shapeGeometry[index]} {
			if geometry.vao != 0 {
				C.glDeleteVertexArrays(1, &geometry.vao)
			}
		}
	}
	if d.meshGeometry.vao != 0 {
		C.glDeleteVertexArrays(1, &d.meshGeometry.vao)
	}
	for _, buffer := range []*C.GLuint{
		&d.quadVertexBuffer, &d.quadIndexBuffer,
	} {
		if *buffer != 0 {
			C.glDeleteBuffers(1, buffer)
		}
	}
	d.textures.Clear()
	d.targets.Clear()
	d.meshes.Clear()
	d.meshShaders.Clear()
	d.material2DShaders.Clear()
	d.destroyContext()
}

func (d *openGLESDriver) createGeometryResources() error {
	vertices := [...]float32{
		0, 0, 0, 0, 0,
		1, 0, 0, 1, 0,
		1, 1, 0, 1, 1,
		0, 1, 0, 0, 1,
	}
	indices := [...]uint16{0, 1, 2, 0, 2, 3}
	C.glGenBuffers(1, &d.quadVertexBuffer)
	C.glGenBuffers(1, &d.quadIndexBuffer)
	d.bindArrayBuffer(d.quadVertexBuffer)
	C.glBufferData(C.GL_ARRAY_BUFFER, C.GLsizeiptr(unsafe.Sizeof(vertices)), unsafe.Pointer(&vertices[0]), C.GL_STATIC_DRAW)
	C.glBindBuffer(C.GL_ELEMENT_ARRAY_BUFFER, d.quadIndexBuffer)
	C.glBufferData(C.GL_ELEMENT_ARRAY_BUFFER, C.GLsizeiptr(unsafe.Sizeof(indices)), unsafe.Pointer(&indices[0]), C.GL_STATIC_DRAW)
	for index := range glesFramesInFlight {
		d.spriteGeometry[index] = d.createGeometry(false)
		d.shapeGeometry[index] = d.createGeometry(true)
	}
	C.glGenVertexArrays(1, &d.meshGeometry.vao)

	// Initialize enables/divisors against an existing buffer. Each draw replaces
	// the attribute pointers after binding its transient upload block.
	d.bindVertexArray(d.meshGeometry.vao)
	C.onix_gl_configure_mesh_vertex_attributes(d.quadVertexBuffer, d.quadIndexBuffer, C.GLsizei(unsafe.Sizeof(Vertex3D{})))
	C.onix_gl_mesh_instance_attributes(C.GLsizei(unsafe.Sizeof(Instance3D{})), 0, 1)
	C.glBindVertexArray(0)
	C.glBindBuffer(C.GL_ARRAY_BUFFER, 0)
	d.boundVAO, d.boundBuffer = 0, 0
	if d.quadVertexBuffer == 0 || d.quadIndexBuffer == 0 || d.meshGeometry.vao == 0 {
		return errors.New("render: OpenGL ES could not create geometry resources")
	}
	for index := range glesFramesInFlight {
		if d.spriteGeometry[index].vao == 0 || d.shapeGeometry[index].vao == 0 {
			return errors.New("render: OpenGL ES could not create frame geometry")
		}
		d.uploads[index].vaos = [3]C.GLuint{d.spriteGeometry[index].vao, d.shapeGeometry[index].vao, d.meshGeometry.vao}
		d.uploads[index].quad_vertex = d.quadVertexBuffer
		d.uploads[index].quad_index = d.quadIndexBuffer
	}
	return glError("create geometry resources")
}

func (d *openGLESDriver) createGeometry(shape bool) glesGeometry {
	geometry := glesGeometry{}
	C.glGenVertexArrays(1, &geometry.vao)

	d.bindVertexArray(geometry.vao)
	d.bindArrayBuffer(d.quadVertexBuffer)
	C.onix_gl_vertex_attrib_pointer(0, 3, C.GL_FLOAT, C.GL_FALSE, 20, 0)
	C.glEnableVertexAttribArray(0)
	C.onix_gl_vertex_attrib_pointer(1, 2, C.GL_FLOAT, C.GL_FALSE, 20, 12)
	C.glEnableVertexAttribArray(1)
	C.glBindBuffer(C.GL_ELEMENT_ARRAY_BUFFER, d.quadIndexBuffer)
	if shape {
		d.configureShapeAttributes(0, true)
	} else {
		d.configureSpriteAttributes(0, true)
	}
	return geometry
}

func (d *openGLESDriver) configureSpriteAttributes(base uintptr, initialize bool) {
	C.onix_gl_sprite_attributes(C.GLsizei(unsafe.Sizeof(Instance2D{})), C.uintptr_t(base), C.int(glesBoolean(initialize)))
}

func (d *openGLESDriver) configureShapeAttributes(base uintptr, initialize bool) {
	attributes := [...]struct {
		index  C.GLuint
		size   C.GLint
		offset uintptr
	}{
		{2, 3, 0}, {3, 3, 12}, {4, 3, 24}, {5, 3, 36},
		{6, 4, 48}, {7, 4, 64}, {8, 4, 80},
	}
	for _, attribute := range attributes {
		C.onix_gl_vertex_attrib_pointer(attribute.index, attribute.size, C.GL_FLOAT, C.GL_FALSE, C.GLsizei(unsafe.Sizeof(ShapeInstance2D{})), C.uintptr_t(base+attribute.offset))
		if initialize {
			C.glEnableVertexAttribArray(attribute.index)
			C.glVertexAttribDivisor(attribute.index, 1)
		}
	}
	C.onix_gl_vertex_attrib_i_pointer(9, 1, C.GL_UNSIGNED_INT, C.GLsizei(unsafe.Sizeof(ShapeInstance2D{})), C.uintptr_t(base+96))
	if initialize {
		C.glEnableVertexAttribArray(9)
		C.glVertexAttribDivisor(9, 1)
	}
}

func (d *openGLESDriver) uploadGLESInstanceBuffer[T any](geometry *glesGeometry, values []T) (uintptr, error) {
	var value T
	byteLength := len(values) * int(unsafe.Sizeof(value))
	var offset C.uintptr_t
	buffer := C.onix_gl_upload_buffer(&d.uploads[d.frameIndex], C.GL_ARRAY_BUFFER, C.size_t(byteLength), sliceData(values), 0, &offset)
	if buffer == 0 {
		return 0, errors.New("render: OpenGL ES instance upload allocation failed")
	}
	// Attribute pointers follow both the retained block and this batch's range.
	// Sprite runs apply their individual draw offsets after this upload.
	d.boundBuffer = buffer
	if geometry == &d.shapeGeometry[d.frameIndex] {
		d.configureShapeAttributes(uintptr(offset), false)
	}
	return uintptr(offset), nil
}

func (d *openGLESDriver) useProgram(program *glesProgram) {
	if program == nil || d.currentProgram == program.value {
		return
	}
	d.currentProgram = program.value
	C.onix_gl_use_program(
		program.value, program.texture, program.normalTexture, program.metallicRoughnessTexture,
		&program.extraTextures[0], C.size_t(len(program.extraTextures)), &program.additionalTextures[0], C.size_t(len(program.additionalTextures)),
		program.generatedTextureV, program.generatedTextureF, C.int(glesInt(program == &d.programs[PipelineSprite])), C.int(glesInt(program.generatedWGSL)),
	)
}

func (d *openGLESDriver) bindVertexArray(value C.GLuint) {
	if d.boundVAO == value {
		return
	}
	C.glBindVertexArray(value)
	d.boundVAO = value
	// The element buffer is VAO state and array-buffer bindings may have changed
	// while another VAO was configured.
	d.boundBuffer = 0
}

func (d *openGLESDriver) bindArrayBuffer(value C.GLuint) {
	if d.boundBuffer == value {
		return
	}
	C.glBindBuffer(C.GL_ARRAY_BUFFER, value)
	d.boundBuffer = value
}

func (d *openGLESDriver) bindTexture(value C.GLuint) {
	if d.boundTexture == value {
		return
	}
	C.glBindTexture(C.GL_TEXTURE_2D, value)
	d.boundTexture = value
}

func (d *openGLESDriver) setProjection(program *glesProgram, projection [16]float32) {
	if program.projectionSet && program.projectionValue == projection {
		return
	}
	program.projectionValue = projection
	C.onix_gl_set_projection(program.projection, program.generatedProjectionV, program.generatedProjectionF, (*C.GLfloat)(unsafe.Pointer(&program.projectionValue[0])), C.int(glesInt(program.generatedWGSL)))
	program.projectionSet = true
}

func (d *openGLESDriver) setTextureParameters(filter TextureFilter, wrapU, wrapV TextureAddressMode) {
	minFilter := C.GLint(C.GL_LINEAR)
	if filter == TextureFilterLinearMipmapLinear {
		minFilter = C.GL_LINEAR_MIPMAP_LINEAR
	}
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, minFilter)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GLint(wrapU.glValue()))
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GLint(wrapV.glValue()))
}

func (d *openGLESDriver) setDepthEnabled(enabled bool) {
	if d.depthEnabled == enabled {
		return
	}
	d.depthEnabled = enabled
	if enabled {
		C.glEnable(C.GL_DEPTH_TEST)
	} else {
		C.glDisable(C.GL_DEPTH_TEST)
	}
}

func (d *openGLESDriver) setBlendEnabled(enabled bool) {
	if d.blendEnabled == enabled {
		return
	}
	d.blendEnabled = enabled
	if enabled {
		C.glEnable(C.GL_BLEND)
	} else {
		C.glDisable(C.GL_BLEND)
	}
}

func (d *openGLESDriver) setRawBlend(blend BlendState) {
	C.glBlendColor(C.GLfloat(blend.Constant.R), C.GLfloat(blend.Constant.G), C.GLfloat(blend.Constant.B), C.GLfloat(blend.Constant.A))
	C.glBlendFuncSeparate(
		glesBlendFactor(blend.RGB.Src), glesBlendFactor(blend.RGB.Dst),
		glesBlendFactor(blend.Alpha.Src), glesBlendFactor(blend.Alpha.Dst),
	)
}

func (d *openGLESDriver) setMeshBlendMode(mode MeshBlendMode) {
	source, destination := BlendOne, BlendOneMinusSrcAlpha
	switch mode {
	case MeshBlendAdditive:
		destination = BlendOne
	case MeshBlendMultiply:
		source = BlendDstColor
	}
	d.setRawBlend(BlendState{
		RGB:   Blend{Src: source, Dst: destination},
		Alpha: Blend{Src: source, Dst: destination},
	})
}

func (d *openGLESDriver) deleteMesh(mesh glesMesh) {
	if mesh.vertexBuffer != 0 {
		C.glDeleteBuffers(1, &mesh.vertexBuffer)
	}
	if mesh.indexBuffer != 0 {
		C.glDeleteBuffers(1, &mesh.indexBuffer)
	}
}

func createGLESShader[R any](descriptor ShaderDescriptor, defaultVertex, defaultFragment, kind string, pool *GenerationPool[R, *glesProgram]) (Handle[R], error) {
	if err := descriptor.Validate(); err != nil {
		return Handle[R]{}, err
	}
	if descriptor.WebGL2VertexSource == "" && descriptor.WebGL2FragmentSource == "" {
		return Handle[R]{}, fmt.Errorf("render: %s shader has no GLSL ES source", kind)
	}
	vertex := descriptor.WebGL2VertexSource
	if vertex == "" {
		vertex = defaultVertex
	}
	fragment := descriptor.WebGL2FragmentSource
	if fragment == "" {
		fragment = defaultFragment
	}
	program, err := createGLESProgram(vertex, fragment, descriptor.WebGL2Generated)
	if err != nil {
		return Handle[R]{}, err
	}
	return pool.Insert(program), nil
}

func (driver *openGLESDriver) destroyGLESShader[R any](handle Handle[R], kind string, pool *GenerationPool[R, *glesProgram]) error {
	program, ok := pool.Remove(handle)
	if !ok {
		return fmt.Errorf("render: invalid OpenGL ES %s shader handle %s", kind, handle)
	}
	if driver.currentProgram == program.value {
		driver.currentProgram = 0
	}
	deleteGLESProgram(program)
	return glError("destroy " + kind + " shader")
}

func createGLESProgram(vertexSource, fragmentSource string, generatedWGSL bool) (*glesProgram, error) {
	vertex, err := compileGLESShader(C.GL_VERTEX_SHADER, vertexSource)
	if err != nil {
		return nil, err
	}
	defer C.glDeleteShader(vertex)
	fragment, err := compileGLESShader(C.GL_FRAGMENT_SHADER, fragmentSource)
	if err != nil {
		return nil, err
	}
	defer C.glDeleteShader(fragment)
	value := C.glCreateProgram()
	if value == 0 {
		return nil, errors.New("render: OpenGL ES could not create shader program")
	}
	C.glAttachShader(value, vertex)
	C.glAttachShader(value, fragment)
	C.glLinkProgram(value)
	var linked C.GLint
	C.glGetProgramiv(value, C.GL_LINK_STATUS, &linked)
	if linked == 0 {
		message := glesProgramLog(value)
		C.glDeleteProgram(value)
		return nil, fmt.Errorf("render: OpenGL ES program link failed: %s", message)
	}
	program := &glesProgram{
		extraTextures:               [MeshExtraTextureSlots]C.GLint{glesUniformLocation(value, "occlusionTex"), glesUniformLocation(value, "emissiveTex"), glesUniformLocation(value, "specularTex"), glesUniformLocation(value, "specularColorTex"), glesUniformLocation(value, "environmentTex"), glesUniformLocation(value, "shadowTex"), glesUniformLocation(value, "screenDepthTex"), glesUniformLocation(value, "clearcoatTex"), glesUniformLocation(value, "clearcoatRoughnessTex"), glesUniformLocation(value, "clearcoatNormalTex"), glesUniformLocation(value, "sheenColorTex"), glesUniformLocation(value, "sheenRoughnessTex"), glesUniformLocation(value, "anisotropyTex")},
		shadowMatrices:              glesUniformLocation(value, "shadowMatrices"),
		shadowRects:                 glesUniformLocation(value, "shadowRects"),
		shadowParams:                glesUniformLocation(value, "shadowParams"),
		lightShadows:                glesUniformLocation(value, "lightShadows"),
		cameraForward:               glesUniformLocation(value, "cameraForward"),
		occlusionInverse:            glesUniformLocation(value, "occlusionInverseProjection"),
		environment:                 glesUniformLocation(value, "environment"),
		renderParams:                glesUniformLocation(value, "renderParams"),
		value:                       value,
		projection:                  glesUniformLocation(value, "projection"),
		generatedProjectionV:        glesUniformLocation(value, "_immediates_binding_vs.projection"),
		generatedProjectionF:        glesUniformLocation(value, "_immediates_binding_fs.projection"),
		texture:                     glesUniformLocation(value, "tex"),
		generatedTextureV:           glesUniformLocation(value, "_group_0_binding_1_vs"),
		generatedTextureF:           glesUniformLocation(value, "_group_0_binding_1_fs"),
		normalTexture:               glesUniformLocation(value, "normalTex"),
		metallicRoughnessTexture:    glesUniformLocation(value, "metallicRoughnessTex"),
		additionalTextures:          [SpriteTextureSlots - 1]C.GLint{glesUniformLocation(value, "tex1"), glesUniformLocation(value, "tex2"), glesUniformLocation(value, "tex3")},
		hasNormalTexture:            glesUniformLocation(value, "hasNormalTexture"),
		hasMetallicRoughnessTexture: glesUniformLocation(value, "hasMetallicRoughnessTexture"),
		cameraPosition:              glesUniformLocation(value, "cameraPosition"),
		ambientLight:                glesUniformLocation(value, "ambientLight"),
		lightDirection:              glesUniformLocation(value, "lightDirection"),
		directionalLight:            glesUniformLocation(value, "directionalLight"),
		lightPosition:               glesUniformLocation(value, "lightPosition"),
		lightDirections:             glesUniformLocation(value, "lightDirections"),
		lightColors:                 glesUniformLocation(value, "lightColors"),
		lightParams:                 glesUniformLocation(value, "lightParams"),
		lightRight:                  glesUniformLocation(value, "lightRight"),
		lightUp:                     glesUniformLocation(value, "lightUp"),
		lightCount:                  glesUniformLocation(value, "lightCount"),
		lightMeta:                   glesUniformLocation(value, "lightMeta"),
		generatedWGSL:               generatedWGSL,
	}
	if err := glError("create shader program"); err != nil {
		deleteGLESProgram(program)
		return nil, err
	}
	return program, nil
}

func compileGLESShader(kind C.GLenum, source string) (C.GLuint, error) {
	source = platformGLShaderSource(source)
	shader := C.glCreateShader(kind)
	if shader == 0 {
		return 0, errors.New("render: OpenGL ES could not create shader")
	}
	C.onix_gl_shader_source(shader, (*C.char)(unsafe.Pointer(unsafe.StringData(source))), C.GLint(len(source)))
	C.glCompileShader(shader)
	var compiled C.GLint
	C.glGetShaderiv(shader, C.GL_COMPILE_STATUS, &compiled)
	if compiled == 0 {
		message := glesShaderLog(shader)
		C.glDeleteShader(shader)
		return 0, fmt.Errorf("render: OpenGL ES shader compilation failed: %s", message)
	}
	return shader, nil
}

func deleteGLESProgram(program *glesProgram) {
	if program != nil && program.value != 0 {
		C.glDeleteProgram(program.value)
		program.value = 0
	}
}

func glesUniformLocation(program C.GLuint, name string) C.GLint {
	value := C.CString(name)
	defer C.free(unsafe.Pointer(value))
	return C.glGetUniformLocation(program, value)
}

func glesShaderLog(shader C.GLuint) string {
	var length C.GLint
	C.glGetShaderiv(shader, C.GL_INFO_LOG_LENGTH, &length)
	if length <= 1 {
		return "unknown compilation error"
	}
	buffer := make([]byte, int(length))
	C.glGetShaderInfoLog(shader, C.GLsizei(length), nil, (*C.GLchar)(unsafe.Pointer(&buffer[0])))
	return string(buffer[:len(buffer)-1])
}

func glesProgramLog(program C.GLuint) string {
	var length C.GLint
	C.glGetProgramiv(program, C.GL_INFO_LOG_LENGTH, &length)
	if length <= 1 {
		return "unknown link error"
	}
	buffer := make([]byte, int(length))
	C.glGetProgramInfoLog(program, C.GLsizei(length), nil, (*C.GLchar)(unsafe.Pointer(&buffer[0])))
	return string(buffer[:len(buffer)-1])
}

func glesBlendFactor(factor BlendFactor) C.GLenum {
	return [...]C.GLenum{
		C.GL_ZERO, C.GL_ONE,
		C.GL_SRC_COLOR, C.GL_ONE_MINUS_SRC_COLOR,
		C.GL_SRC_ALPHA, C.GL_ONE_MINUS_SRC_ALPHA,
		C.GL_DST_ALPHA, C.GL_ONE_MINUS_DST_ALPHA,
		C.GL_DST_COLOR, C.GL_ONE_MINUS_DST_COLOR,
		C.GL_SRC_ALPHA_SATURATE,
		C.GL_CONSTANT_COLOR, C.GL_ONE_MINUS_CONSTANT_COLOR,
		C.GL_CONSTANT_ALPHA, C.GL_ONE_MINUS_CONSTANT_ALPHA,
	}[factor]
}

func glesBoolean(value bool) C.GLboolean {
	if value {
		return C.GL_TRUE
	}
	return C.GL_FALSE
}

func glesInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func sliceData[T any](values []T) unsafe.Pointer {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Pointer(&values[0])
}

func glError(operation string) error {
	if code := C.glGetError(); code != C.GL_NO_ERROR {
		return fmt.Errorf("render: OpenGL ES %s failed with GL error %#x", operation, uint32(code))
	}
	return nil
}
