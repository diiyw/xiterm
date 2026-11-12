#include <objc/runtime.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef size_t NSUInteger;
#define NSUIntegerMax SIZE_MAX
#define MTLResourceStorageModeShared 0
#define ONIX_METAL_FRAMES_IN_FLIGHT 3
#define ONIX_METAL_INSTANCE_BUFFER_MINIMUM (64u * 1024u)

static size_t allocations, resources, live_buffers;
static int fail_buffer, fail_reallocation;
static void require(int success, const char *message) {
	if (!success) { fprintf(stderr, "%s\n", message); exit(1); }
}
static void *tracked_calloc(size_t count, size_t size) { allocations++; return calloc(count, size); }
static void *tracked_realloc(void *pointer, size_t size) {
	if (fail_reallocation) { fail_reallocation = 0; return NULL; }
	allocations++;
	return realloc(pointer, size);
}
#define calloc tracked_calloc
#define realloc tracked_realloc
#include "native_buffer.h"
#include "native_upload.h"
#include "native_slots.h"
#include "metal_native.h"
#define ONIX_MESH_BLEND_MODE_COUNT 4

@protocol MTLBuffer
@property(readonly) void *contents;
- (void)release;
@end
@protocol MTLDevice
- (id<MTLBuffer>)newBufferWithLength:(NSUInteger)length options:(NSUInteger)options;
@end

__attribute__((objc_root_class))
@interface OnixTestBuffer <MTLBuffer> {
@public
	Class isa;
	void *bytes;
}
@end
@implementation OnixTestBuffer
- (void *)contents { return bytes; }
- (void)release {
	free(bytes);
	live_buffers--;
	object_dispose(self);
}
@end

__attribute__((objc_root_class))
@interface OnixTestDevice <MTLDevice> { Class isa; }
@end
@implementation OnixTestDevice
- (id<MTLBuffer>)newBufferWithLength:(NSUInteger)length options:(NSUInteger)options {
	(void)options;
	if (fail_buffer) { fail_buffer = 0; return nil; }
	OnixTestBuffer *buffer = class_createInstance(objc_getClass("OnixTestBuffer"), 0);
	buffer->bytes = malloc(length);
	require(buffer->bytes != NULL, "allocate mock GPU memory");
	resources++;
	live_buffers++;
	return buffer;
}
@end

@protocol MTLTexture, MTLRenderPipelineState, MTLDepthStencilState, MTLSamplerState, MTLLibrary, MTLCommandQueue, MTLCommandBuffer, CAMetalDrawable;
@class CAMetalLayer, NSMutableArray, MTLRenderPassDescriptor;
typedef void *CVMetalTextureCacheRef;
typedef void *CVMetalTextureRef;
typedef struct { double x, y, width, height, near, far; } MTLViewport;
typedef struct { NSUInteger x, y, width, height; } MTLScissorRect;
typedef enum { MTLCullModeNone, MTLCullModeFront, MTLCullModeBack } MTLCullMode;
#define MTLWindingCounterClockwise 0
#define MTLPrimitiveTypeTriangle 3
#define MTLIndexTypeUInt32 1
@protocol MTLRenderCommandEncoder
- (void)setVertexBuffer:(id<MTLBuffer>)buffer offset:(NSUInteger)offset atIndex:(NSUInteger)index;
- (void)setFragmentBuffer:(id<MTLBuffer>)buffer offset:(NSUInteger)offset atIndex:(NSUInteger)index;
- (void)setVertexBytes:(const void *)bytes length:(NSUInteger)length atIndex:(NSUInteger)index;
- (void)setFrontFacingWinding:(NSUInteger)winding;
- (void)setCullMode:(MTLCullMode)mode;
- (void)setRenderPipelineState:(id<MTLRenderPipelineState>)pipeline;
- (void)setDepthStencilState:(id<MTLDepthStencilState>)depth;
- (void)setFragmentTexture:(id<MTLTexture>)texture atIndex:(NSUInteger)index;
- (void)setFragmentSamplerState:(id<MTLSamplerState>)sampler atIndex:(NSUInteger)index;
- (void)setVertexTexture:(id<MTLTexture>)texture atIndex:(NSUInteger)index;
- (void)drawIndexedPrimitives:(NSUInteger)primitive indexCount:(NSUInteger)count indexType:(NSUInteger)type indexBuffer:(id<MTLBuffer>)buffer indexBufferOffset:(NSUInteger)offset instanceCount:(NSUInteger)instances;
@end
#include "metal_resource_types.inc"
#include "metal_upload_types.inc"
#include "metal_driver.inc"
static void onix_metal_apply_dynamic_state(OnixMetalDriver *driver) { (void)driver; }
static void onix_metal_set_error(const char *message) { (void)message; }
// GPU pipeline creation is covered by metal_lazy_test.m on Apple hardware.
static int onix_metal_require_depth(OnixMetalDriver *driver) { (void)driver; return 1; }
static id<MTLRenderPipelineState> onix_metal_mesh_pipeline(OnixMetalDriver *driver, int transparent, uint32_t blend, int picking, int variant) {
	(void)transparent; (void)blend; (void)picking; (void)variant;
	return (id<MTLRenderPipelineState>)driver->device;
}
static id onix_metal_mesh_vertex_descriptor(void) { return nil; }
static id<MTLRenderPipelineState> onix_metal_pipeline(OnixMetalDriver *driver, id<MTLLibrary> vertex, id<MTLLibrary> fragment, id vertex_name, id fragment_name, id descriptor, int blend, int depth) {
	(void)vertex; (void)fragment; (void)vertex_name; (void)fragment_name; (void)descriptor; (void)blend; (void)depth;
	return (id<MTLRenderPipelineState>)driver->device;
}
#include "metal_upload_reset.inc"
#include "metal_upload.inc"
#include "metal_mesh_draws.inc"

static const OnixNativeMeshUniforms *expected_uniform;
static const OnixMetalVertex3D *expected_vertices;
static const OnixMetalInstance3D *expected_instances;
static const uint32_t *expected_indices;
static int expected_picking;
static size_t draw_calls;

__attribute__((objc_root_class))
@interface OnixTestEncoder <MTLRenderCommandEncoder> {
	Class isa;
	id<MTLBuffer> vertex_buffers[6], fragment_uniform;
	NSUInteger vertex_offsets[6], fragment_offset;
}
@end
@implementation OnixTestEncoder
- (void)setVertexBuffer:(id<MTLBuffer>)buffer offset:(NSUInteger)offset atIndex:(NSUInteger)index {
	require(index < 6, "vertex buffer binding slot");
	vertex_buffers[index] = buffer;
	vertex_offsets[index] = offset;
}
- (void)setFragmentBuffer:(id<MTLBuffer>)buffer offset:(NSUInteger)offset atIndex:(NSUInteger)index {
	require(index == 2, "fragment uniform slot");
	fragment_uniform = buffer;
	fragment_offset = offset;
}
- (void)setVertexBytes:(const void *)bytes length:(NSUInteger)length atIndex:(NSUInteger)index {
	(void)bytes;
	require(index == 4 && length == 4, "3D uniforms must use the retained buffer rather than transient setBytes copies");
}
- (void)setFrontFacingWinding:(NSUInteger)winding { (void)winding; }
- (void)setCullMode:(MTLCullMode)mode { require(mode <= MTLCullModeBack, "valid face culling state"); }
- (void)setRenderPipelineState:(id<MTLRenderPipelineState>)pipeline { (void)pipeline; }
- (void)setDepthStencilState:(id<MTLDepthStencilState>)depth { (void)depth; }
- (void)setFragmentTexture:(id<MTLTexture>)texture atIndex:(NSUInteger)index { (void)texture; (void)index; }
- (void)setFragmentSamplerState:(id<MTLSamplerState>)sampler atIndex:(NSUInteger)index { (void)sampler; (void)index; }
- (void)setVertexTexture:(id<MTLTexture>)texture atIndex:(NSUInteger)index { (void)texture; (void)index; }
- (void)drawIndexedPrimitives:(NSUInteger)primitive indexCount:(NSUInteger)count indexType:(NSUInteger)type indexBuffer:(id<MTLBuffer>)buffer indexBufferOffset:(NSUInteger)offset instanceCount:(NSUInteger)instances {
	require(primitive == MTLPrimitiveTypeTriangle && type == MTLIndexTypeUInt32 && count == 3 && instances == 1, "indexed mesh draw arguments");
	require(vertex_offsets[2] % 256 == 0 && memcmp((uint8_t *)vertex_buffers[2].contents + vertex_offsets[2], expected_uniform, sizeof(*expected_uniform)) == 0, "vertex uniforms retain the complete aligned snapshot");
	if (!expected_picking) require(vertex_buffers[2] == fragment_uniform && vertex_offsets[2] == fragment_offset, "vertex and fragment stages share exactly one uniform upload");
	require(memcmp((uint8_t *)vertex_buffers[0].contents + vertex_offsets[0], expected_vertices, 3 * sizeof(*expected_vertices)) == 0, "dynamic vertex binding follows its own block and offset");
	require(memcmp((uint8_t *)buffer.contents + offset, expected_indices, 3 * sizeof(*expected_indices)) == 0, "dynamic index binding follows its own block and offset");
	require(memcmp((uint8_t *)vertex_buffers[1].contents + vertex_offsets[1], expected_instances, sizeof(*expected_instances)) == 0, "instance binding follows its own block and offset");
	draw_calls++;
}
@end

static void test_mesh_bindings(OnixMetalDriver *driver, const void *large) {
	OnixMetalVertex3D vertices[3] = {{.position = {1, 2, 3}}, {.position = {4, 5, 6}}, {.position = {7, 8, 9}}};
	uint32_t indices[3] = {2, 1, 0}, deformation[16] = {0};
	OnixMetalInstance3D instance = {.model = {1, 2, 3}};
	OnixNativeMeshUniforms uniforms = {.view_projection = {3, 2, 1}, .occlusion_inverse = {9, 8, 7}};
	OnixNativeMeshDraw3D draw = {.texture = 1, .index_count = 3, .instance_count = 1};
	OnixMetalTexture texture = {.texture = (id<MTLTexture>)driver->device};
	OnixMetalRenderTarget target = {.color_format = 1, .depth_texture = texture.texture};
	driver->encoder = class_createInstance(objc_getClass("OnixTestEncoder"), 0);
	driver->textures = &texture;
	driver->texture_slots.count = 1;
	driver->render_targets = &target;
	driver->mesh_picking_pipeline = (id<MTLRenderPipelineState>)driver->device;
	expected_uniform = &uniforms;
	expected_vertices = vertices;
	expected_indices = indices;
	expected_instances = &instance;
	for (int pass = 0; pass < 3; pass++) {
		driver->frame_index = pass;
		onix_metal_reset_uploads(&driver->uploads[pass]);
		OnixNativeUpload initial;
		require(onix_metal_upload(driver, large, ONIX_METAL_INSTANCE_BUFFER_MINIMUM - 16, 16, 0, &initial), "prime first block before split mesh bindings");
		expected_picking = 0;
		require(onix_metal_draw_meshes(driver, &uniforms, vertices, 3, indices, 3, &instance, 1, deformation, 16, &draw, 1), "submit production 3D draw");
		uint32_t unique = driver->uploads[pass].cache.count;
		require(onix_metal_draw_meshes(driver, &uniforms, vertices, 3, indices, 3, &instance, 1, deformation, 16, &draw, 1), "submit another 3D pass");
		require(driver->uploads[pass].cache.count == unique, "normal passes reuse both geometry and deformation snapshots");
		expected_picking = 1;
		driver->active_render_target = 1;
		require(onix_metal_draw_mesh_picking(driver, &uniforms, vertices, 3, indices, 3, &instance, 1, &draw, 1), "submit production picking draw");
		driver->active_render_target = 0;
	}
	require(draw_calls == 9, "all production draw bindings were checked");
	object_dispose(driver->encoder);
	driver->encoder = nil;
}

static void test_readback_retention(OnixMetalDriver *driver) {
	driver->readback_capacity = 4u * 1024u * 1024u;
	driver->readback_buffer = [driver->device newBufferWithLength:driver->readback_capacity options:0];
	size_t heap = allocations, buffers = resources;
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) {
		driver->readback_used = frame % 7 == 0 ? driver->readback_capacity : 4;
		onix_metal_trim_readback(driver);
		require(driver->readback_buffer != nil, "intermittent large Metal readbacks stay warm");
	}
	// Clear the previous observation window with a full-sized read.
	driver->readback_used = driver->readback_capacity;
	onix_metal_trim_readback(driver);
	for (uint32_t frame = 1; frame < ONIX_BUFFER_TRIM_DELAY; frame++) {
		driver->readback_used = 4;
		onix_metal_trim_readback(driver);
		require(driver->readback_buffer != nil, "short low demand retains Metal readback");
	}
	onix_metal_trim_readback(driver);
	require(driver->readback_buffer == nil && driver->readback_capacity == 0, "Metal readback peak releases after sustained low demand");
	require(allocations == heap && resources == buffers, "Metal readback trimming allocates nothing");
	driver->readback_capacity = 4096;
	driver->readback_buffer = [driver->device newBufferWithLength:4096 options:0];
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) onix_metal_trim_readback(driver);
	require(driver->readback_buffer != nil, "small Metal readback stays warm");
	[driver->readback_buffer release];
	driver->readback_buffer = nil;
	driver->readback_capacity = 0;
}

static void test_upload_table_retention(id<MTLDevice> device) {
	OnixMetalDriver local = {.device = device};
	OnixNativeUpload upload;
	uint32_t value = 29;
	require(onix_metal_upload(&local, &value, sizeof(value), 16, 0, &upload), "create live Metal upload block");
	OnixMetalUploadFrame *frame = &local.uploads[0];
	id<MTLBuffer> first = frame->blocks[0].buffer;
	// Reserve metadata only: the peak GPU buffers have already been retired.
	frame->blocks = realloc(frame->blocks, 1024 * sizeof(*frame->blocks));
	require(frame->blocks != NULL, "reserve Metal upload table peak");
	frame->capacity = 1024;
	size_t buffers = resources;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		if (iteration == ONIX_BUFFER_TRIM_DELAY - 1) fail_reallocation = 1;
		onix_metal_reset_uploads(frame);
		if (iteration == ONIX_BUFFER_TRIM_DELAY - 1) require(frame->capacity == 1024, "failed Metal table shrink preserves capacity");
		require(onix_metal_upload(&local, &value, sizeof(value), 16, 0, &upload) && frame->blocks[0].buffer == first && memcmp(upload.data, &value, sizeof(value)) == 0,
			"Metal metadata shrink preserves live buffer and copied contents");
	}
	require(frame->capacity == 4 && frame->count == 1 && resources == buffers, "Metal upload table releases metadata without recreating buffers");
	size_t heap = allocations;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		onix_metal_reset_uploads(frame);
		require(onix_metal_upload(&local, &value, sizeof(value), 16, 0, &upload), "reuse shrunken Metal table");
	}
	require(allocations == heap && resources == buffers, "shrunken Metal table allocates nothing");
	printf("Metal upload table: %llu -> %llu bytes; failed shrink, buffer ownership and zero warm allocations passed\n",
		(unsigned long long)(1024 * sizeof(*frame->blocks)), (unsigned long long)(frame->capacity * sizeof(*frame->blocks)));
	[first release];
	free(frame->blocks);
}

static void test_upload_cache_retention(id<MTLDevice> device) {
	OnixMetalDriver local = {.device = device};
	uint32_t keys[256];
	for (uint32_t index = 0; index < 256; index++) keys[index] = index;
	size_t heap = 0, buffers = 0;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY + 8; iteration++) {
		OnixMetalUploadFrame *frame = &local.uploads[0];
		onix_metal_reset_uploads(frame);
		if (iteration == ONIX_BUFFER_TRIM_DELAY + 2) { heap = allocations; buffers = resources; }
		for (uint32_t index = 0; index < (iteration == 0 ? 256u : 4u); index++) {
			OnixNativeUpload upload;
			require(onix_metal_upload(&local, &keys[index], 4, 16, 1, &upload), "upload cached Metal geometry after metadata peak");
			require(memcmp(upload.data, &keys[index], 4) == 0, "shrunken Metal cache retains valid native snapshots");
		}
		if (iteration > ONIX_BUFFER_TRIM_DELAY) require(frame->cache.capacity == 16, "Metal frame reset releases peak metadata");
		if (iteration >= ONIX_BUFFER_TRIM_DELAY + 2) require(allocations == heap && resources == buffers, "Metal cache shrink preserves zero warm allocations");
	}
	OnixMetalUploadFrame *frame = &local.uploads[0];
	for (uint32_t index = 0; index < frame->count; index++) [frame->blocks[index].buffer release];
	free(frame->blocks);
	onix_upload_cache_destroy(&frame->cache);
	puts("Metal production upload cache: 256 -> 16 entries, copied geometry and zero warm allocations passed");
}

int main(void) {
	OnixMetalDriver driver = {.device = class_createInstance(objc_getClass("OnixTestDevice"), 0)};
	test_readback_retention(&driver);
	test_upload_cache_retention(driver.device);
	test_upload_table_retention(driver.device);
	uint8_t *large = malloc(ONIX_METAL_INSTANCE_BUFFER_MINIMUM - 16);
	require(large != NULL, "allocate large source");
	memset(large, 3, ONIX_METAL_INSTANCE_BUFFER_MINIMUM - 16);
	uint32_t geometry[16] = {1, 2, 3}, snapshot[16];
	memcpy(snapshot, geometry, sizeof(snapshot));
	uint8_t uniform[3584] = {0};
	size_t heap = 0, buffers = 0;
	for (int iteration = 0; iteration < 15; iteration++) {
		if (iteration == 3) { heap = allocations; buffers = resources; }
		driver.frame_index = iteration % 3;
		OnixMetalUploadFrame *frame = &driver.uploads[driver.frame_index];
		onix_metal_reset_uploads(frame);
		OnixNativeUpload instances, a, b, changed, uniforms;
		require(onix_metal_upload(&driver, large, ONIX_METAL_INSTANCE_BUFFER_MINIMUM - 16, 16, 0, &instances), "upload first block");
		require(frame->blocks[0].capacity == 64u * 1024u, "small Metal upload starts with 64 KiB");
		require(onix_metal_upload(&driver, snapshot, sizeof(snapshot), 16, 1, &a), "upload geometry to next block");
		require(a.offset == 0 && a.resource != instances.resource && frame->count == 2, "new blocks start at zero without empty prefixes");
		require(frame->blocks[1].capacity == 128u * 1024u, "additional Metal blocks grow geometrically");
		NSUInteger used = frame->used;
		memcpy(geometry, snapshot, sizeof(snapshot));
		require(onix_metal_upload(&driver, geometry, sizeof(geometry), 16, 1, &b), "reuse geometry across passes");
		require(a.resource == b.resource && a.offset == b.offset && frame->used == used, "cache hits reserve no GPU memory");
		geometry[0] = 8;
		require(onix_metal_upload(&driver, geometry, sizeof(geometry), 16, 1, &changed), "upload changed scratch source");
		require(changed.offset != a.offset && memcmp(a.data, snapshot, sizeof(snapshot)) == 0, "changing scratch input preserves earlier draws");
		require(onix_metal_upload(&driver, uniform, sizeof(uniform), 256, 0, &uniforms), "upload shared stage uniforms");
		require(uniforms.offset % 256 == 0 && memcmp(uniforms.data, uniform, sizeof(uniform)) == 0, "uniform snapshot is aligned and complete");
		if (iteration >= 3) require(allocations == heap && resources == buffers, "stable Metal frames allocate no heap or GPU blocks");
	}
	test_mesh_bindings(&driver, large);
	// Device allocation failure must not insert a half-created block.
	OnixMetalDriver failure = {.device = driver.device};
	OnixNativeUpload upload;
	fail_buffer = 1;
	require(!onix_metal_upload(&failure, uniform, sizeof(uniform), 256, 0, &upload) && failure.uploads[0].count == 0, "failed buffer creation keeps pool consistent");
	free(failure.uploads[0].blocks);
	for (int index = 0; index < 3; index++) {
		OnixMetalUploadFrame *frame = &driver.uploads[index];
		for (int idle = 0; idle < 31; idle++) onix_metal_reset_uploads(frame);
		require(frame->bytes <= ONIX_METAL_INSTANCE_BUFFER_MINIMUM, "idle frame releases surplus blocks");
		for (uint32_t block = 0; block < frame->count; block++) [frame->blocks[block].buffer release];
		free(frame->blocks);
		onix_upload_cache_destroy(&frame->cache);
	}
	require(live_buffers == 0, "native GPU blocks release exactly once");
	object_dispose(driver.device);
	free(large);
	puts("Metal production allocator: no empty prefixes, shared pass snapshots, mutated sources, aligned uniforms, zero warm allocations, failure recovery, block ownership and production 3D/picking bindings passed (mock device)");
	return 0;
}
