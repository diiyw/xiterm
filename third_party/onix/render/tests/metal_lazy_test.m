// Exercise the production driver on a real GPU without a window or screenshots.
#include "../metal_native.m"

static void require(int success, const char *message) {
	if (!success) {
		fprintf(stderr, "%s: %s\n", message, onix_metal_last_error());
		exit(1);
	}
}

// Only drawable acquisition is replaced. All encoders, pipelines and textures
// belong to the real Metal device. Submission omits presentation to a display.
@interface OnixTestDrawable : NSObject {
@public
	id<MTLTexture> color;
}
@property(readonly) id<MTLTexture> texture;
@end
@implementation OnixTestDrawable
- (id<MTLTexture>)texture { return color; }
- (void)dealloc { [color release]; [super dealloc]; }
@end

@interface OnixTestLayer : CAMetalLayer
@end
@implementation OnixTestLayer
- (id<CAMetalDrawable>)nextDrawable {
	MTLTextureDescriptor *descriptor = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:self.pixelFormat width:self.drawableSize.width height:self.drawableSize.height mipmapped:NO];
	descriptor.usage = MTLTextureUsageRenderTarget;
	descriptor.storageMode = MTLStorageModePrivate;
	OnixTestDrawable *drawable = [[[OnixTestDrawable alloc] init] autorelease];
	drawable->color = [self.device newTextureWithDescriptor:descriptor];
	return (id<CAMetalDrawable>)drawable;
}
@end

static void finish_frame(OnixMetalDriver *driver, uint8_t *pixels) {
	[driver->encoder endEncoding];
	[driver->encoder release];
	driver->encoder = nil;
	id<MTLBuffer> readback = nil;
	if (pixels != NULL) {
		readback = [driver->device newBufferWithLength:32 * 256 options:MTLResourceStorageModeShared];
		id<MTLBlitCommandEncoder> blit = [driver->command_buffer blitCommandEncoder];
		[blit copyFromTexture:driver->drawable.texture sourceSlice:0 sourceLevel:0 sourceOrigin:MTLOriginMake(0, 0, 0) sourceSize:MTLSizeMake(32, 32, 1) toBuffer:readback destinationOffset:0 destinationBytesPerRow:256 destinationBytesPerImage:32 * 256];
		[blit endEncoding];
	}
	[driver->command_buffer commit];
	[driver->command_buffer waitUntilCompleted];
	if (driver->command_buffer.status == MTLCommandBufferStatusError) {
		fprintf(stderr, "%s\n", driver->command_buffer.error.localizedDescription.UTF8String);
		exit(1);
	}
	if (pixels != NULL) memcpy(pixels, readback.contents, 32 * 256);
	[readback release];
	[driver->command_buffer release];
	[driver->drawable release];
	driver->command_buffer = nil;
	driver->drawable = nil;
	driver->frame_index = (driver->frame_index + 1) % ONIX_METAL_FRAMES_IN_FLIGHT;
}

static const float identity[16] = {1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1};
static const float blue[4] = {0, 0, 1, 1};
static uint32_t white_texture;
static OnixMetalSpriteUniforms sprite_uniforms;
static OnixNativeMeshUniforms mesh_uniforms;

static void shapes(OnixMetalDriver *driver) {
	OnixMetalShapeInstance shape = {.model = {2, 0, -1, 0, 2, -1}, .local_model = {1, 0, 0, 0, 1, 0}, .fill = {0, 1, 0, 1}, .params = {32, 32, 0, 0}};
	require(onix_metal_draw_shapes(driver, &sprite_uniforms, &shape, 1), "draw shapes");
}

static void sprites(OnixMetalDriver *driver, uint32_t shader) {
	OnixMetalInstance instance = {.model = {2, 0, -1, 0, 2, -1}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, 1}};
	OnixNativeSpriteDraw draw = {.shader = shader, .texture = white_texture, .instance_count = 1};
	for (int blend = 0; blend < 2; blend++) {
		onix_metal_set_sprite_blend(driver, blend, .25f, .5f, 1, 0);
		require(onix_metal_draw_sprites(driver, &sprite_uniforms, &instance, 1, &draw, 1), "draw sprite attachment/blend variant");
	}
	onix_metal_set_sprite_blend(driver, 0, 0, 0, 0, 0);
}

static int mesh(OnixMetalDriver *driver, int picking, uint32_t id, float z, int transparent, uint32_t blend) {
	OnixMetalVertex3D vertices[3] = {{.position = {-0.6, -0.6, z}, .normal = {0, 0, 1}}, {.position = {0.6, -0.6, z}, .normal = {0, 0, 1}}, {.position = {0, 0.6, z}, .normal = {0, 0, 1}}};
	uint32_t indices[] = {0, 1, 2}, deformation[] = {0};
	OnixMetalInstance3D instance = {.base_color = {1, 0, 0, 1}, .material = {-1, 0, 0, 0}, .uv_transform = {1, 1, 0, 0}};
	if (transparent) { instance.base_color[0] = 0; instance.base_color[1] = 1; }
	memcpy(instance.model, identity, sizeof(identity));
	memcpy(&instance.custom[3][3], &id, sizeof(id));
	OnixNativeMeshDraw3D draw = {.texture = white_texture, .index_count = 3, .instance_count = 1, .alpha_mode = transparent ? 2 : 0, .blend_mode = blend};
	return picking ? onix_metal_draw_mesh_picking(driver, &mesh_uniforms, vertices, 3, indices, 3, &instance, 1, &draw, 1)
		: onix_metal_draw_meshes(driver, &mesh_uniforms, vertices, 3, indices, 3, &instance, 1, deformation, 1, &draw, 1);
}

static void pixel(const uint8_t *pixels, uint32_t x, uint32_t y, int b, int g, int r, const char *message) {
	const uint8_t *value = pixels + y * 256 + x * 4;
	if (value[0] != b || value[1] != g || value[2] != r || value[3] != 255) {
		fprintf(stderr, "%s: BGRA=%u,%u,%u,%u\n", message, value[0], value[1], value[2], value[3]);
		exit(1);
	}
}

int main(void) {
	@autoreleasepool {
		OnixTestLayer *layer = [[OnixTestLayer alloc] init];
		OnixMetalDriver *driver = onix_metal_create(layer, 32, 32);
		require(driver != NULL, "create driver");
		require(driver->depth_texture == nil && driver->mesh_opaque_pipeline == nil && driver->mesh_picking_pipeline == nil && driver->mesh_opaque_depth == nil, "2D startup has no depth or mesh resources");
		for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) require(driver->mesh_transparent_pipelines[blend] == nil, "no transparent pipelines at startup");
		require(onix_metal_resize(driver, 64, 64) && onix_metal_resize(driver, 32, 32) && driver->depth_texture == nil, "2D resize does not allocate depth");
		memcpy(sprite_uniforms.projection, identity, sizeof(identity));
		memcpy(mesh_uniforms.view_projection, identity, sizeof(identity));
		const uint8_t white[] = {255, 255, 255, 255};
		white_texture = onix_metal_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, sizeof(white), 0, 0);
		require(white_texture != 0, "create texture");
		uint32_t material = onix_metal_create_material2d_shader(driver, onix_metal_source, strlen(onix_metal_source));
		require(material != 0, "create custom Material2D");
		uint32_t texture, flat = onix_metal_create_render_target(driver, 32, 32, 0, 0, &texture);
		uint32_t depth = onix_metal_create_render_target(driver, 32, 32, 0, 1, &texture);
		uint32_t picking = onix_metal_create_render_target(driver, 32, 32, 1, 1, &texture);
		require(flat && depth && picking, "create offscreen targets");

		// Exercise both 2D attachment variants and picking before surface 3D.
		require(onix_metal_begin_frame(driver, blue), "begin 2D frame");
		for (int pass = 0; pass < 2; pass++) {
			require(onix_metal_begin_render_target(driver, pass ? depth : flat, blue, 0, 1, 1), "begin 2D target");
			shapes(driver); sprites(driver, 0); sprites(driver, material);
			if (!pass) require(!mesh(driver, 0, 0, 0.6, 0, 0), "reject mesh without target depth");
			require(onix_metal_end_render_target(driver), "restore 2D surface");
		}
		require(onix_metal_begin_render_target(driver, picking, blue, 0, 1, 1), "begin picking target");
		require(mesh(driver, 1, 42, 0.8, 0, 0) && mesh(driver, 1, 7, 0.2, 0, 0), "lazy picking and reversed depth");
		require(onix_metal_end_render_target(driver), "restore after picking");
		require(driver->depth_texture == nil && driver->mesh_opaque_pipeline == nil && driver->mesh_picking_pipeline != nil, "offscreen picking does not allocate surface depth or color mesh pipelines");
		finish_frame(driver, NULL);
		uint32_t ids[32 * 32];
		require(onix_metal_read_render_target(driver, picking, 0, 0, 32, 32, (uint8_t *)ids, sizeof(ids)), "read picking target");
		require(ids[16 * 32 + 16] == 42, "near picking ID survives later far draw");
		OnixBufferMemory retained;
		onix_metal_buffer_memory(driver, &retained);
		require(retained.instances > 0 && retained.readback > 0, "transient buffers allocated before idle");
		id<MTLCommandBuffer> pending = [driver->queue commandBuffer];
		driver->inflight_commands[0] = [pending retain];
		[pending commit];
		require(onix_metal_trim_idle(driver), "trim idle buffers");
		require(driver->inflight_commands[0] == nil, "idle waited for and released in-flight work");
		onix_metal_buffer_memory(driver, &retained);
		require(retained.instances == 0 && retained.staging == 0 && retained.readback == 0 && retained.scratch == 0, "idle released all transient storage");
		require(onix_metal_trim_idle(driver), "repeated idle trim is safe");
		require(onix_metal_read_render_target(driver, picking, 0, 0, 32, 32, (uint8_t *)ids, sizeof(ids)), "recreate readback after idle");
		require(ids[16 * 32 + 16] == 42, "idle preserved persistent render targets");

		uint8_t pixels[32 * 256];
		for (int frame = 0; frame < 4; frame++) {
			require(onix_metal_begin_frame(driver, blue), "begin mixed frame");
			require(!onix_metal_trim_idle(driver), "reject trimming an active frame");
			onix_metal_set_scissor(driver, 1, 0, 0, 4, 4); shapes(driver);
			onix_metal_set_scissor(driver, 0, 0, 0, 0, 0);
			require(mesh(driver, 0, 0, 0.6, 0, 0), "first surface mesh upgrades encoder");
			id<MTLTexture> storage = driver->depth_texture;
			require(storage != nil && driver->mesh_variants[ONIX_MESH_UNLIT-1][0] != nil && driver->mesh_opaque_pipeline == nil, "surface depth and opaque pipeline created");
			if (frame == 0) for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) require(driver->mesh_variants[ONIX_MESH_UNLIT-1][blend+1] == nil, "opaque draw does not build transparent pipelines");
			// A target round trip must preserve surface color and reversed depth.
			require(onix_metal_begin_render_target(driver, depth, blue, 0, 1, 1), "begin mesh target");
			require(mesh(driver, 0, 0, 0.6, 0, 0), "draw offscreen mesh");
			require(onix_metal_end_render_target(driver), "restore mixed surface");
			for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) require(mesh(driver, 0, 0, 0.2, 1, blend), "draw transparent variants behind opaque mesh");
			onix_metal_set_scissor(driver, 1, 28, 28, 4, 4); shapes(driver); sprites(driver, 0); sprites(driver, material);
			onix_metal_set_scissor(driver, 0, 0, 0, 0, 0);
			require(driver->depth_texture == storage, "reuse surface depth");
			finish_frame(driver, pixels);
			pixel(pixels, 2, 2, 0, 255, 0, "preceding 2D pixels survived pass upgrade");
			pixel(pixels, 6, 6, 255, 0, 0, "surface clear survived target switching");
			pixel(pixels, 16, 16, 0, 0, 255, "mesh depth survived target switching");
			pixel(pixels, 30, 30, 255, 255, 255, "2D overlay uses depth attachment variant");
		}
		require(onix_metal_resize(driver, 64, 48) && driver->depth_texture.width == 64 && driver->depth_texture.height == 48, "resize existing surface depth");
		require(onix_metal_begin_frame(driver, blue), "begin resized frame");
		shapes(driver);
		require(mesh(driver, 0, 0, 0.6, 0, 0), "draw after depth resize");
		finish_frame(driver, NULL);
		require(onix_metal_destroy_material2d_shader(driver, material) && !onix_metal_destroy_material2d_shader(driver, material), "retire both material variants once");
		onix_metal_destroy(driver);
		[layer release];
		puts("Metal lazy depth/pipelines: 2D, mixed 3D, targets, LCD, Material2D, reversed-depth picking, resize OK");
	}
	return 0;
}
