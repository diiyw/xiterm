#include <EGL/egl.h>
#include <GLES3/gl3.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static size_t allocations, storage_calls, upload_calls;
static uintptr_t upload_offset;
static int fail_reallocation;
static void require(int success, const char *message) {
	if (!success) { fprintf(stderr, "%s\n", message); exit(1); }
}
static void *tracked_calloc(size_t count, size_t size) { allocations++; return calloc(count, size); }
static void *tracked_realloc(void *pointer, size_t size) {
	if (fail_reallocation) { fail_reallocation = 0; return NULL; }
	allocations++;
	return realloc(pointer, size);
}
static void tracked_storage(GLenum target, GLsizeiptr size, const void *data, GLenum usage) {
	storage_calls++;
	glBufferData(target, size, data, usage);
}
static void tracked_upload(GLenum target, GLintptr offset, GLsizeiptr size, const void *data) {
	upload_calls++;
	glBufferSubData(target, offset, size, data);
}
static void *tracked_mapping(GLenum target, GLintptr offset, GLsizeiptr size, GLbitfield access) {
	if (access & GL_MAP_WRITE_BIT) {
		require(access == (GL_MAP_WRITE_BIT | GL_MAP_UNSYNCHRONIZED_BIT), "stream writes avoid implicit synchronization");
		upload_calls++;
	}
	return glMapBufferRange(target, offset, size, access);
}
#define glMapBufferRange tracked_mapping
#define calloc tracked_calloc
#define realloc tracked_realloc
#define glBufferData tracked_storage
#define glBufferSubData tracked_upload
#include "gles_draw_wrappers.inc"
#include "native_buddy.h"

static void check_buffer(GLuint buffer, const void *expected, size_t length) {
	glBindBuffer(GL_ARRAY_BUFFER, buffer);
	const void *mapping = glMapBufferRange(GL_ARRAY_BUFFER, 0, (GLsizeiptr)length, GL_MAP_READ_BIT);
	require(mapping && memcmp(mapping, expected, length) == 0, "GPU buffer snapshot changed");
	require(glUnmapBuffer(GL_ARRAY_BUFFER), "unmap GPU snapshot");
}

static void test_buddy(void) {
	OnixNativeBuddy buddy;
	require(onix_buddy_init(&buddy, 1024, 256), "create buddy arena");
	uint64_t offsets[4], nodes[4];
	for (int index = 0; index < 4; index++) {
		nodes[index] = onix_buddy_take(&buddy, 100, &offsets[index]);
		require(nodes[index] && offsets[index] == (uint64_t)index * 256, "aligned disjoint leases");
	}
	uint64_t offset;
	require(!onix_buddy_take(&buddy, 1, &offset), "full arena rejects allocation");
	onix_buddy_release(&buddy, nodes[0]);
	onix_buddy_release(&buddy, nodes[2]);
	require(!onix_buddy_take(&buddy, 512, &offset), "fragmented holes cannot alias a live lease");
	onix_buddy_release(&buddy, nodes[1]);
	uint64_t node = onix_buddy_take(&buddy, 512, &offset);
	require(node && offset == 0, "adjacent holes coalesce");
	onix_buddy_release(&buddy, node);
	onix_buddy_release(&buddy, nodes[3]);
	size_t before = allocations;
	for (int iteration = 0; iteration < 1000; iteration++) {
		node = onix_buddy_take(&buddy, 1024, &offset);
		require(node == 1 && offset == 0, "fully released arena coalesces to root");
		onix_buddy_release(&buddy, node);
	}
	require(allocations == before, "warmed buddy allocations allocate no metadata");
	free(buddy.states);
}

static void test_cache_collisions(void) {
	OnixNativeUploadCache cache = {0};
	const char first[] = "first", second[] = "other";
	OnixNativeUpload a = {.resource = 1, .hash = 42, .size = sizeof(first), .data = first, .offset = 16};
	OnixNativeUpload b = {.resource = 2, .hash = 42, .size = sizeof(second), .data = second, .offset = 256}, found;
	require(onix_upload_cache_insert(&cache, a) && onix_upload_cache_insert(&cache, b), "insert colliding cache keys");
	require(onix_upload_cache_find(&cache, second, sizeof(second), 42, 256, &found) && found.resource == 2, "hash collisions compare native snapshots");
	require(!onix_upload_cache_find(&cache, first, sizeof(first), 42, 256, &found), "cached offsets respect the requested alignment");
	char keys[40][16] = {{0}};
	for (uint32_t index = 0; index < 40; index++) {
		snprintf(keys[index], sizeof(keys[index]), "key-%u", index);
		OnixNativeUpload entry = {.resource = index + 3, .hash = 42, .size = sizeof(keys[index]), .data = keys[index]};
		if (cache.count == cache.capacity) {
			uint32_t count = cache.count;
			fail_reallocation = 1;
			require(!onix_upload_cache_insert(&cache, entry) && cache.count == count, "failed rehash preserves existing cache entries");
		}
		require(onix_upload_cache_insert(&cache, entry), "grow cache with colliding keys");
	}
	for (uint32_t index = 0; index < 40; index++) {
		require(onix_upload_cache_find(&cache, keys[index], sizeof(keys[index]), 42, 1, &found) && found.resource == index + 3,
			"rehashing preserves distinct snapshots through collisions and failed growth");
	}
	onix_upload_cache_reset(&cache);
	require(!onix_upload_cache_find(&cache, second, sizeof(second), 42, 1, &found), "cache expires at frame boundary");
	onix_upload_cache_destroy(&cache);
}

static GLuint test_program(const char *vertex_source) {
	const char *fragment_source = "#version 300 es\nprecision highp float; in vec4 color; out vec4 result; void main(){result=color;}";
	GLuint program = glCreateProgram();
	for (int index = 0; index < 2; index++) {
		const char *source = index ? fragment_source : vertex_source;
		GLuint shader = glCreateShader(index ? GL_FRAGMENT_SHADER : GL_VERTEX_SHADER);
		glShaderSource(shader, 1, &source, NULL);
		glCompileShader(shader);
		GLint success;
		glGetShaderiv(shader, GL_COMPILE_STATUS, &success);
		require(success, "compile upload binding shader");
		glAttachShader(program, shader);
		glDeleteShader(shader);
	}
	glLinkProgram(program);
	GLint success;
	glGetProgramiv(program, GL_LINK_STATUS, &success);
	require(success, "link upload binding shader");
	return program;
}

static void check_draw_color(int blue) {
	uint8_t pixel[4];
	glReadPixels(2, 2, 1, 1, GL_RGBA, GL_UNSIGNED_BYTE, pixel);
	require(pixel[blue ? 2 : 0] == 255 && pixel[blue ? 0 : 2] == 0 && pixel[3] == 255, "production draw wrapper reads the current batch's buffer and attributes");
}

static void test_draw_bindings(void) {
	OnixGLUploadPool pool = {0};
	GLuint vao;
	glGenVertexArrays(1, &vao);
	glBindVertexArray(vao);
	glViewport(0, 0, 4, 4);
	GLuint program = test_program("#version 300 es\nlayout(location=0) in vec3 p; layout(location=3) in vec4 c; out vec4 color; void main(){gl_Position=vec4(p,1);color=c;}");
	glUseProgram(program);
	float vertices[36] = {0};
	vertices[0] = -1; vertices[1] = -1;
	vertices[12] = 3; vertices[13] = -1;
	vertices[24] = -1; vertices[25] = 3;
	uint32_t indices[3] = {0, 1, 2};
	GLuint vertex_buffer = 0, index_buffer = 0, instance_buffer = 0, first_vertex = 0, first_index = 0;
	uintptr_t instance_offset;
	for (int pass = 0; pass < 3; pass++) {
		float instance[48] = {0};
		instance[pass % 2 ? 2 : 0] = instance[3] = 1;
		require(onix_gl_upload_mesh_batch(&pool, &vertex_buffer, &index_buffer, &instance_buffer, &instance_offset,
			vertices, sizeof(vertices), indices, sizeof(indices), instance, sizeof(instance)), "upload production mesh batch");
		if (pass == 0) {
			first_vertex = vertex_buffer; first_index = index_buffer;
			onix_gl_mesh_instance_attributes(192, 0, 1);
		}
		else require(vertex_buffer == first_vertex && index_buffer == first_index, "normal and picking passes share geometry uploads");
		if (pass == 2) {
			OnixGLESPickingDraw draw = {.base = instance_offset, .index_count = 3, .instances = 1};
			onix_gl_draw_mesh_picking_run(&draw, 1, instance_buffer, vertex_buffer, index_buffer, 48, 192);
		} else {
			OnixGLESMeshDraw draw = {.base = instance_offset, .index_count = 3, .instances = 1};
			onix_gl_draw_mesh_run(&draw, 1, instance_buffer, vertex_buffer, index_buffer, -1, -1, 48, 192, 0);
		}
		check_draw_color(pass % 2);
	}
	glDeleteProgram(program);
	program = test_program("#version 300 es\nlayout(location=0) in vec3 p; layout(location=6) in vec4 c; out vec4 color; void main(){gl_Position=vec4(p,1);color=c;}");
	glUseProgram(program);
	for (int pass = 0; pass < 2; pass++) {
		float instance[33] = {0};
		instance[12 + (pass ? 2 : 0)] = instance[15] = 1;
		GLuint buffer = onix_gl_upload_buffer(&pool, GL_ARRAY_BUFFER, sizeof(instance), instance, 0, &upload_offset);
		require(buffer != 0, "upload sprite instance block");
		uintptr_t instance_base = upload_offset;
		onix_gl_sprite_attributes(132, instance_base, 1);
		// Use the already established triangle geometry with the sprite wrapper's
		// six-index convention, repeating the triangle once.
		const uint16_t sprite_indices[6] = {0, 1, 2, 0, 1, 2};
		require(onix_gl_upload_buffer(&pool, GL_ELEMENT_ARRAY_BUFFER, sizeof(sprite_indices), sprite_indices, 1, &upload_offset), "upload sprite indices");
		OnixGLESSpriteDraw draw = {.base = instance_base, .instances = 1};
		onix_gl_draw_sprite_run(&draw, 1, 0, 132);
		check_draw_color(pass);
	}
	require(onix_gl_finish_uploads(&pool) && onix_gl_begin_uploads(&pool), "complete production upload draws");
	glDeleteProgram(program);
	glDeleteVertexArrays(1, &vao);
	onix_gl_destroy_uploads(&pool);
}

static void test_stream_ranges(void) {
	OnixGLUploadPool pools[3] = {0};
	GLuint vao, vertices;
	glGenVertexArrays(1, &vao); glBindVertexArray(vao);
	glGenBuffers(1, &vertices); glBindBuffer(GL_ARRAY_BUFFER, vertices);
	const float triangle[] = {-1, -1, 0, 3, -1, 0, -1, 3, 0};
	glBufferData(GL_ARRAY_BUFFER, sizeof(triangle), triangle, GL_STATIC_DRAW);
	glVertexAttribPointer(0, 3, GL_FLOAT, GL_FALSE, 0, NULL); glEnableVertexAttribArray(0);
	glEnableVertexAttribArray(3); glVertexAttribDivisor(3, 1);
	GLuint program = test_program("#version 300 es\nlayout(location=0) in vec3 p; layout(location=3) in vec4 c; out vec4 color; void main(){gl_Position=vec4(p,1);color=c;}");
	glUseProgram(program); glEnable(GL_SCISSOR_TEST);
	size_t heap = 0, storage = 0;
	for (unsigned frame = 0; frame < 96; frame++) {
		OnixGLUploadPool *pool = &pools[frame % 3];
		require(onix_gl_begin_uploads(pool), "recycle only a completed stream frame");
		glBindVertexArray(vao);
		if (frame == 3) { heap = allocations; storage = storage_calls; }
		GLuint last_buffer = 0;
		uintptr_t last_end = 0;
		unsigned blocks = 0;
		for (unsigned batch = 0; batch < 2600; batch++) {
			float data[32] = {0};
			data[(batch / 163 + frame) % 2 ? 2 : 0] = data[3] = 1;
			data[31] = (float)batch;
			uintptr_t offset;
			GLuint buffer = onix_gl_upload_buffer(pool, GL_ARRAY_BUFFER, sizeof(data), data, 0, &offset);
			require(buffer && offset % 16 == 0, "stream ranges are aligned");
			if (buffer == last_buffer) require(offset >= last_end, "same-frame stream ranges never overlap");
			else blocks++;
			last_buffer = buffer; last_end = offset + sizeof(data);
			if (batch % 163 == 0) {
				unsigned tile = batch / 163;
				glScissor(tile % 4, tile / 4, 1, 1);
				glVertexAttribPointer(3, 4, GL_FLOAT, GL_FALSE, sizeof(data), (void *)offset);
				glDrawArraysInstanced(GL_TRIANGLES, 0, 3, 1);
			}
			memset(data, 0, sizeof(data));
		}
		require(blocks == 3 && pool->count == 3 && pool->bytes == 448u * 1024u,
			"2600 batches share three bounded stream blocks");
		uint8_t pixels[4 * 4 * 4];
		glReadPixels(0, 0, 4, 4, GL_RGBA, GL_UNSIGNED_BYTE, pixels);
		for (unsigned tile = 0; tile < 16; tile++) {
			const uint8_t *pixel = pixels + tile * 4;
			unsigned blue = (tile + frame) % 2;
			if (!(pixel[blue ? 2 : 0] == 255 && pixel[blue ? 0 : 2] == 0 && pixel[3] == 255))
				fprintf(stderr, "stream frame=%u tile=%u rgba=%u,%u,%u,%u GL=0x%x\n", frame, tile, pixel[0], pixel[1], pixel[2], pixel[3], glGetError());
			require(pixel[blue ? 2 : 0] == 255 && pixel[blue ? 0 : 2] == 0 && pixel[3] == 255,
				"interleaved draws retain prior ranges across block growth and three-frame reuse");
		}
		glBindBuffer(GL_ARRAY_BUFFER, last_buffer);
		const float *last = glMapBufferRange(GL_ARRAY_BUFFER, (GLintptr)(last_end - 128), 128, GL_MAP_READ_BIT);
		require(last && last[31] == 2599, "stream uploads own their bytes after source mutation");
		require(glUnmapBuffer(GL_ARRAY_BUFFER), "unmap stream readback");
		require(onix_gl_finish_uploads(pool), "fence stream draws");
		if (frame >= 3) require(allocations == heap && storage_calls == storage,
			"stream slabs stay warm without native allocation or storage recreation beyond the trim window");
	}
	glDisable(GL_SCISSOR_TEST); glDeleteProgram(program);
	glDeleteVertexArrays(1, &vao); glDeleteBuffers(1, &vertices);
	for (unsigned index = 0; index < 3; index++) onix_gl_destroy_uploads(&pools[index]);
	puts("GLES streams: 2600 batches -> 3 buffers, aligned non-overlap, interleaved draw pixels, source mutation, 96 frames with zero warm allocation/storage recreation passed");
}

static void test_cache_retention(void) {
	OnixNativeUploadCache cache = {0};
	uint32_t keys[4096];
	for (uint32_t index = 0; index < 4096; index++) {
		keys[index] = index;
		OnixNativeUpload entry = {.resource = index + 1, .hash = onix_upload_hash(&keys[index], 4), .size = 4, .data = &keys[index]};
		require(onix_upload_cache_insert(&cache, entry), "create upload metadata peak");
	}
	uint64_t peak = onix_upload_cache_bytes(&cache);
	onix_upload_cache_reset(&cache);
	size_t heap = allocations;
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) {
		for (uint32_t index = 0; index < 8; index++) {
			OnixNativeUpload entry = {.resource = index + 1, .hash = 42, .size = 4, .data = &keys[index], .offset = index * 16};
			require(onix_upload_cache_insert(&cache, entry), "insert small upload workload after peak");
			OnixNativeUpload found;
			require(onix_upload_cache_find(&cache, &keys[index], 4, 42, 16, &found) && found.resource == index + 1,
				"cache contents and collision checks survive shrink and failed shrink");
		}
		if (frame == ONIX_BUFFER_TRIM_DELAY - 1) fail_reallocation = 1;
		onix_upload_cache_reset(&cache);
		if (frame < ONIX_BUFFER_TRIM_DELAY - 1) require(cache.capacity == 4096 && allocations == heap, "metadata peak waits for sustained low demand without allocating");
		if (frame == ONIX_BUFFER_TRIM_DELAY - 1) require(cache.capacity == 4096 && cache.count == 0, "failed shrink keeps storage usable and clears previous frame");
	}
	require(cache.capacity == 16 && onix_upload_cache_bytes(&cache) < peak / 100, "upload metadata drops from 4096 entries to 16");
	heap = allocations;
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) {
		OnixNativeUpload entry = {.resource = 7, .hash = 42, .size = 4, .data = &keys[0]};
		require(onix_upload_cache_insert(&cache, entry), "reuse shrunken upload table");
		onix_upload_cache_reset(&cache);
	}
	require(allocations == heap, "shrunken steady cache allocates nothing");
	// A regular burst above the quarter-capacity threshold cancels shrinking.
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) {
		uint32_t count = frame % 7 == 0 ? 129 : 1;
		for (uint32_t index = 0; index < count; index++) {
			OnixNativeUpload entry = {.resource = index + 1, .hash = index, .size = 4, .data = &keys[index]};
			require(onix_upload_cache_insert(&cache, entry), "regrow cache for intermittent demand");
		}
		onix_upload_cache_reset(&cache);
		require(cache.capacity == 256, "intermittent upload demand stays warm");
	}
	printf("Upload metadata retention: %llu -> %llu bytes, failed shrink, collisions, regrowth and zero warm allocations passed\n",
		(unsigned long long)peak, (unsigned long long)(16 * (sizeof(OnixNativeUpload) + 8)));
	onix_upload_cache_destroy(&cache);
}

static void test_buffer_table_retention(void) {
	OnixGLUploadPool pool = {0};
	uint32_t value = 29;
	uint8_t peak_data[64 << 10] = {0};
	require(onix_gl_begin_uploads(&pool), "begin GLES table peak");
	GLuint first = 0;
	for (uint32_t index = 0; index < 256; index++) {
		GLuint buffer = onix_gl_upload_buffer(&pool, GL_ARRAY_BUFFER, sizeof(peak_data), peak_data, 0, &upload_offset);
		require(buffer != 0, "grow GLES buffer table");
		if (index == 0) first = buffer;
	}
	uint32_t peak_capacity = pool.capacity;
	require(onix_gl_finish_uploads(&pool) && peak_capacity > 8, "fence GLES table peak");
	uint64_t peak = (uint64_t)pool.capacity * sizeof(*pool.buffers);
	size_t storage = storage_calls;
	int failed = 0;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 3; iteration++) {
		int inject = !failed && pool.table_retention.frames == ONIX_BUFFER_TRIM_DELAY - 1;
		if (inject) fail_reallocation = 1;
		require(onix_gl_begin_uploads(&pool), "begin small GLES table workload");
		if (inject) { failed = 1; require(pool.capacity == peak_capacity, "failed GLES table shrink preserves storage"); }
		require(onix_gl_upload_buffer(&pool, GL_ARRAY_BUFFER, sizeof(value), &value, 0, &upload_offset) == first, "GLES table compaction keeps first live buffer");
		require(onix_gl_finish_uploads(&pool), "fence small GLES table workload");
	}
	require(failed && pool.capacity == 8 && pool.count == 1 && pool.bytes == 64 << 10 && storage_calls == storage,
		"GLES releases table peak without recreating live GPU storage");
	check_buffer(first, &value, sizeof(value));
	size_t heap = allocations;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		require(onix_gl_begin_uploads(&pool), "reuse shrunken GLES table");
		require(onix_gl_upload_buffer(&pool, GL_ARRAY_BUFFER, sizeof(value), &value, 0, &upload_offset) == first && onix_gl_finish_uploads(&pool), "reuse live GLES storage after table shrink");
	}
	require(allocations == heap && storage_calls == storage, "shrunken GLES table reuses without allocations");
	printf("GLES buffer table: %llu -> %llu bytes; failed shrink, live pixels and zero warm allocations passed\n",
		(unsigned long long)peak, (unsigned long long)((uint64_t)pool.capacity * sizeof(*pool.buffers)));
	onix_gl_destroy_uploads(&pool);
}

static void test_snapshot_retention(void) {
	OnixGLUploadPool pool = {0};
	const size_t size = 128u * 1024u;
	uint8_t *data = malloc(size);
	require(data != NULL, "allocate snapshot workload");
	memset(data, 19, size);
	require(onix_gl_begin_uploads(&pool), "begin snapshot peak");
	GLuint buffer = onix_gl_upload_buffer(&pool, GL_ARRAY_BUFFER, size, data, 1, &upload_offset);
	require(buffer && pool.snapshot_bytes == size && onix_gl_finish_uploads(&pool), "create native snapshot peak");
	size_t heap = allocations, storage = storage_calls;
	for (uint32_t frame = 0; frame <= ONIX_BUFFER_TRIM_DELAY; frame++) {
		require(onix_gl_begin_uploads(&pool), "reuse GPU storage while CPU snapshot becomes idle");
		if (frame < ONIX_BUFFER_TRIM_DELAY) require(pool.snapshot_bytes == size, "snapshot survives short idle periods");
		else require(pool.snapshot_bytes == 0 && pool.buffers[0].snapshot == NULL, "idle CPU snapshot releases independently of GPU buffer");
		require(onix_gl_upload_buffer(&pool, GL_ARRAY_BUFFER, size, data, 0, &upload_offset) == buffer, "uncached upload keeps using the original GPU buffer");
		require(onix_gl_finish_uploads(&pool), "fence uncached snapshot workload");
	}
	require(allocations == heap && storage_calls == storage && pool.bytes == size, "snapshot reclaim does not allocate or recreate GPU storage");
	// Return to cached geometry with a much smaller source.
	require(onix_gl_begin_uploads(&pool), "begin small cached workload");
	require(onix_gl_upload_buffer(&pool, GL_ARRAY_BUFFER, 64, data, 1, &upload_offset) == buffer && pool.snapshot_bytes == 64, "small snapshot rewarms at its actual size");
	check_buffer(buffer, data, 64);
	size_t writes = upload_calls;
	require(onix_gl_upload_buffer(&pool, GL_ELEMENT_ARRAY_BUFFER, 64, data, 1, &upload_offset) == buffer && upload_calls == writes, "rewarmed snapshot still deduplicates across targets");
	require(onix_gl_finish_uploads(&pool), "fence small snapshot");
	onix_gl_destroy_uploads(&pool);
	free(data);
	puts("GLES CPU snapshot: 128 KiB -> 0 while 128 KiB GPU buffer stays active; small cache reuse passed");
}

int main(void) {
	test_buddy();
	test_cache_collisions();
	test_cache_retention();
	EGLDisplay display = eglGetDisplay(EGL_DEFAULT_DISPLAY);
	if (!eglInitialize(display, NULL, NULL)) return 77;
	eglBindAPI(EGL_OPENGL_ES_API);
	const EGLint attributes[] = {EGL_SURFACE_TYPE, EGL_PBUFFER_BIT, EGL_RENDERABLE_TYPE, EGL_OPENGL_ES3_BIT, EGL_NONE};
	EGLConfig config;
	EGLint count;
	if (!eglChooseConfig(display, attributes, &config, 1, &count) || !count) return 77;
	const EGLint surface_attributes[] = {EGL_WIDTH, 4, EGL_HEIGHT, 4, EGL_NONE};
	const EGLint context_attributes[] = {EGL_CONTEXT_CLIENT_VERSION, 3, EGL_NONE};
	EGLSurface surface = eglCreatePbufferSurface(display, config, surface_attributes);
	EGLContext context = eglCreateContext(display, config, EGL_NO_CONTEXT, context_attributes);
	require(eglMakeCurrent(display, surface, surface, context), "activate GLES3 context");
	test_draw_bindings();
	test_stream_ranges();
	test_buffer_table_retention();
	test_snapshot_retention();
	GLuint vao;
	glGenVertexArrays(1, &vao);
	glBindVertexArray(vao);
	OnixGLUploadPool pools[3] = {0};
	uint8_t source[2048], original[2048];
	memset(original, 7, sizeof(original));
	size_t heap = 0, resources = 0;
	for (int frame = 0; frame < 12; frame++) {
		if (frame == 3) { heap = allocations; resources = storage_calls; }
		OnixGLUploadPool *pool = &pools[frame % 3];
		require(onix_gl_begin_uploads(pool), "wait before reusing a frame's buffers");
		memcpy(source, original, sizeof(source));
		GLuint geometry = onix_gl_upload_buffer(pool, GL_ARRAY_BUFFER, sizeof(source), source, 1, &upload_offset);
		require(geometry, "upload geometry");
		size_t writes = upload_calls;
		GLuint shared = onix_gl_upload_buffer(pool, GL_ELEMENT_ARRAY_BUFFER, sizeof(original), original, 1, &upload_offset);
		require(shared == geometry && upload_calls == writes, "another pass reuses the uploaded geometry");
		source[0] = 13;
		GLuint changed = onix_gl_upload_buffer(pool, GL_ARRAY_BUFFER, sizeof(source), source, 1, &upload_offset);
		require(changed && changed != geometry, "mutating the same source pointer creates a new snapshot");
		GLuint instances = onix_gl_upload_buffer(pool, GL_ARRAY_BUFFER, sizeof(source), source, 0, &upload_offset);
		require(instances && instances != geometry && instances != changed, "later batches never overwrite earlier GPU ranges");
		check_buffer(geometry, original, sizeof(original));
		check_buffer(changed, source, sizeof(source));
		require(onix_gl_finish_uploads(pool), "fence uploaded draws");
		if (frame >= 3) require(allocations == heap && storage_calls == resources, "stable frames perform no heap allocation or buffer storage recreation");
	}
	// Exhaust metadata, inject growth failure, then retry without consuming a slot.
	OnixGLUploadPool failure = {0};
	fail_reallocation = 1;
	require(!onix_gl_upload_buffer(&failure, GL_ARRAY_BUFFER, 16, source, 0, &upload_offset) && failure.used == 0, "allocation failure leaves the pool reusable");
	require(onix_gl_upload_buffer(&failure, GL_ARRAY_BUFFER, 16, source, 0, &upload_offset), "retry failed pool growth");
	onix_gl_destroy_uploads(&failure);
	GLuint vaos[3], quad[2];
	glGenVertexArrays(3, vaos);
	glGenBuffers(2, quad);
	for (int index = 0; index < 2; index++) {
		glBindBuffer(GL_ARRAY_BUFFER, quad[index]);
		glBufferData(GL_ARRAY_BUFFER, 128, NULL, GL_STATIC_DRAW);
	}
	for (int index = 0; index < 3; index++) {
		pools[0].vaos[index] = vaos[index];
		glBindVertexArray(vaos[index]);
		glBindBuffer(GL_ARRAY_BUFFER, pools[0].buffers[0].buffer);
		glVertexAttribPointer(3, 4, GL_FLOAT, GL_FALSE, 16, NULL);
		glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, pools[0].buffers[0].buffer);
	}
	pools[0].quad_vertex = quad[0];
	pools[0].quad_index = quad[1];
	for (int frame = 0; frame < 31; frame++) require(onix_gl_begin_uploads(&pools[0]), "idle trim waits for its frame");
	require(pools[0].bytes == 0 && pools[0].snapshot_bytes == 0, "idle pool releases oversized buffers and snapshots");
	for (int index = 0; index < 3; index++) {
		GLint attribute_buffer, element_buffer;
		glBindVertexArray(vaos[index]);
		glGetVertexAttribiv(3, GL_VERTEX_ATTRIB_ARRAY_BUFFER_BINDING, &attribute_buffer);
		glGetIntegerv(GL_ELEMENT_ARRAY_BUFFER_BINDING, &element_buffer);
		require((GLuint)attribute_buffer == quad[0] && (GLuint)element_buffer == quad[1], "idle VAOs release references to retired upload storage");
	}
	glDeleteVertexArrays(3, vaos);
	glDeleteBuffers(2, quad);
	for (int index = 0; index < 3; index++) onix_gl_destroy_uploads(&pools[index]);
	require(glGetError() == GL_NO_ERROR, "GLES upload paths remain valid");
	glDeleteVertexArrays(1, &vao);
	eglMakeCurrent(display, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
	eglDestroyContext(display, context);
	eglDestroySurface(display, surface);
	eglTerminate(display);
	puts("GLES uploads: production mesh/picking/sprite pixels, zero warm allocations/storage recreation, fence reuse, shared passes, mutated sources, failures, idle VAO trimming; buddy coalescing and cache collisions passed");
	return 0;
}
