#include <GLES3/gl3.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

static GLuint textures[16], array_buffer;
static unsigned int active_slot;
static GLint uniform_values[2];
static size_t texture_binds, uniform_updates, buffer_binds, active_calls, draw_calls;
static uintptr_t instance_base;

static void require(int success, const char *message) {
	if (!success) {
		fprintf(stderr, "%s\n", message);
		exit(1);
	}
}

static void tracked_active_texture(GLenum unit) {
	require(unit >= GL_TEXTURE0 && unit < GL_TEXTURE0 + 16, "invalid texture unit");
	active_slot = unit - GL_TEXTURE0;
	active_calls++;
}

static void tracked_bind_texture(GLenum target, GLuint texture) {
	require(target == GL_TEXTURE_2D, "invalid texture target");
	textures[active_slot] = texture;
	texture_binds++;
}

static void tracked_uniform(GLint location, GLint value) {
	require(location >= 0 && location < 2, "invalid uniform location");
	uniform_values[location] = value;
	uniform_updates++;
}

static void tracked_bind_buffer(GLenum target, GLuint buffer) {
	require(target == GL_ARRAY_BUFFER, "invalid instance buffer target");
	array_buffer = buffer;
	buffer_binds++;
}

static void onix_gl_configure_mesh_vertex_attributes(GLuint vertex, GLuint index, GLsizei stride) {
	require(vertex == 20 && index == 21 && stride == 48, "geometry arguments changed");
}

static void onix_gl_mesh_instance_attributes(GLsizei stride, uintptr_t base, int initialize) {
	require(stride == 192 && initialize == 0 && array_buffer == 22, "invalid instance attributes");
	instance_base = base;
}

static void tracked_draw(GLenum mode, GLsizei count, GLenum type, const void *indices, GLsizei instances);

#define glActiveTexture tracked_active_texture
#define glBindTexture tracked_bind_texture
#define glUniform1i tracked_uniform
#define glBindBuffer tracked_bind_buffer
#define glDrawElementsInstanced tracked_draw
#include "gles_mesh_run.inc"

static const OnixGLESMeshDraw *expected_draws;
static int check_uniforms;

static void tracked_draw(GLenum mode, GLsizei count, GLenum type, const void *indices, GLsizei instances) {
	const OnixGLESMeshDraw *draw = &expected_draws[draw_calls++];
	require(mode == GL_TRIANGLES && type == GL_UNSIGNED_INT && count == draw->index_count &&
		(uintptr_t)indices == draw->index_offset && instances == draw->instances && instance_base == draw->base,
		"draw range or instance offset changed");
	require(active_slot == 0 && array_buffer == 22, "draw leaves wrong active unit or instance buffer");
	require(textures[0] == draw->texture && textures[1] == draw->normal_texture &&
		textures[2] == draw->metallic_roughness_texture, "wrong base, normal or metallic texture");
	for (size_t slot = 0; slot < 13; slot++) require(textures[slot + 3] == draw->extra_textures[slot], "wrong auxiliary texture");
	if (check_uniforms) require(uniform_values[0] == draw->has_normal_texture &&
		uniform_values[1] == draw->has_metallic_roughness_texture, "stale texture presence uniform");
}

static void reset_counters(const OnixGLESMeshDraw *draws, int uniforms) {
	expected_draws = draws;
	check_uniforms = uniforms;
	texture_binds = uniform_updates = buffer_binds = active_calls = draw_calls = 0;
}

static GLuint run(const OnixGLESMeshDraw *draws, size_t count, GLuint bound_texture, int uniforms) {
	return onix_gl_draw_mesh_run(draws, count, 22, 20, 21, uniforms ? 0 : -1, uniforms ? 1 : -1,
		48, 192, bound_texture);
}

int main(void) {
	OnixGLESMeshDraw repeated[4097];
	OnixGLESMeshDraw first = {.texture = 1, .normal_texture = 2, .metallic_roughness_texture = 3,
		.index_count = 6, .instances = 1};
	for (size_t slot = 0; slot < 13; slot++) first.extra_textures[slot] = (GLuint)(slot + 4);
	for (size_t index = 0; index < 4097; index++) {
		repeated[index] = first;
		repeated[index].base = index * 192;
		repeated[index].index_offset = index * 4;
	}
	reset_counters(repeated, 1);
	require(run(repeated, 4097, 0, 1) == 1, "wrong returned base texture");
	require(texture_binds == 16 && uniform_updates == 2 && buffer_binds == 1 && active_calls == 17 && draw_calls == 4097,
		"identical draws repeat texture, uniform or buffer submissions");

	OnixGLESMeshDraw changes[8];
	changes[0] = first;
	for (size_t index = 1; index < 8; index++) {
		changes[index] = changes[index - 1];
		switch (index) {
		case 1: changes[index].has_normal_texture = 1; break;
		case 2: changes[index].normal_texture = 77; break;
		case 3: changes[index].metallic_roughness_texture = 78; changes[index].has_metallic_roughness_texture = 1; break;
		case 4: changes[index].extra_textures[12] = 79; break;
		case 5: changes[index].texture = 80; break;
		case 6:
			changes[index].normal_texture = changes[index].metallic_roughness_texture = changes[index].extra_textures[0] = 0;
			changes[index].has_normal_texture = changes[index].has_metallic_roughness_texture = 0;
			break;
		}
	}
	reset_counters(changes, 1);
	require(run(changes, 8, textures[0], 1) == 80, "wrong changed base texture");
	require(texture_binds == 22 && uniform_updates == 6 && buffer_binds == 1 && draw_calls == 8,
		"changed slots or uniforms are not submitted exactly once");

	// Another program or batch can disturb all auxiliary units and uniforms.
	for (size_t slot = 1; slot < 16; slot++) textures[slot] = 999;
	uniform_values[0] = uniform_values[1] = 99;
	active_slot = 15;
	reset_counters(&first, 1);
	require(run(&first, 1, textures[0], 1) == 1, "new batch returns wrong texture");
	require(texture_binds == 16 && uniform_updates == 2 && draw_calls == 1, "new batch does not restore disturbed state");

	reset_counters(&first, 0);
	run(&first, 1, textures[0], 0);
	require(texture_binds == 15 && uniform_updates == 0 && draw_calls == 1, "missing uniforms cause updates or stale texture bindings");
	puts("GLES binding reuse: 4097 draws bind 16 textures, update 2 uniforms and bind one instance buffer; changed/zero slots, batch boundaries and draw state passed");
	return 0;
}
