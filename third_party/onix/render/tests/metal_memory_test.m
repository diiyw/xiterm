// Exercise production encoders, buffers and retirement on a real Metal device.
#include "../metal_native.m"

static void require(int success, const char *message) {
    if (!success) {
        fprintf(stderr, "%s: %s\n", message, onix_metal_last_error());
        exit(1);
    }
}

// Replace window drawable acquisition with an actual offscreen GPU texture.
@interface OnixMemoryDrawable : NSObject {
@public
    id<MTLTexture> color;
}
@property(readonly) id<MTLTexture> texture;
@end
@implementation OnixMemoryDrawable
- (id<MTLTexture>)texture { return color; }
- (void)dealloc { [color release]; [super dealloc]; }
@end

@interface OnixMemoryLayer : CAMetalLayer
@end
@implementation OnixMemoryLayer
- (id<CAMetalDrawable>)nextDrawable {
    MTLTextureDescriptor *descriptor = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:self.pixelFormat width:self.drawableSize.width height:self.drawableSize.height mipmapped:NO];
    descriptor.usage = MTLTextureUsageRenderTarget;
    descriptor.storageMode = MTLStorageModePrivate;
    OnixMemoryDrawable *drawable = [[[OnixMemoryDrawable alloc] init] autorelease];
    drawable->color = [self.device newTextureWithDescriptor:descriptor];
    return (id<CAMetalDrawable>)drawable;
}
@end

// Submit with the same ownership and slot rotation as present, omitting only
// window presentation. Leave work in flight so the driver must retire it.
static void submit_frame(OnixMetalDriver *driver) {
    [driver->encoder endEncoding];
    [driver->command_buffer commit];
    driver->inflight_commands[driver->frame_index] = driver->command_buffer;
    [driver->encoder release];
    [driver->drawable release];
    driver->encoder = nil;
    driver->command_buffer = nil;
    driver->drawable = nil;
    driver->frame_index = (driver->frame_index + 1) % ONIX_METAL_FRAMES_IN_FLIGHT;
}

int main(void) {
    @autoreleasepool {
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        if (device == nil) {
            puts("SKIP: no Metal device");
            return 77;
        }
        [device release];
        OnixMemoryLayer *layer = [[OnixMemoryLayer alloc] init];
        OnixMetalDriver *driver = onix_metal_create(layer, 64, 64);
        require(driver != NULL, "create driver");
        require(ONIX_METAL_FRAMES_IN_FLIGHT == 2 && layer.maximumDrawableCount == 2, "two frame slots and drawables");
        const float clear[4] = {0, 0, 0, 1};
        OnixMetalSpriteUniforms uniforms = {0};
        uniforms.projection[0] = uniforms.projection[5] = uniforms.projection[10] = uniforms.projection[15] = 1;
        OnixMetalShapeInstance shape = {.model = {2, 0, -1, 0, 2, -1}, .local_model = {1, 0, 0, 0, 1, 0}, .fill = {0, 1, 0, 1}, .params = {64, 64, 0, 0}};
        const uint8_t white[] = {255, 255, 255, 255};
        uint32_t target_texture;
        uint32_t target = onix_metal_create_render_target(driver, 16, 16, 0, 0, &target_texture);
        require(target != 0, "create persistent target");
        // Cover several rotations, resize, idle trim, and allocation after trim.
        for (int cycle = 0; cycle < 3; cycle++) {
            for (int frame = 0; frame < 6; frame++) {
                uint32_t slot = driver->frame_index;
                require(slot == (uint32_t)(frame % 2), "frame slot rotates across both slots");
                require(onix_metal_begin_frame(driver, clear), "begin frame");
                require(!onix_metal_trim_idle(driver), "reject cleanup inside a frame");
                require(onix_metal_draw_shapes(driver, &uniforms, &shape, 1), "draw after buffer recreation");
                require(onix_metal_begin_render_target(driver, target, clear, 0, 1, 1), "begin persistent target");
                require(onix_metal_draw_shapes(driver, &uniforms, &shape, 1), "draw persistent target");
                require(onix_metal_end_render_target(driver), "restore surface");
                uint32_t texture = onix_metal_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, sizeof(white), 0, 0);
                require(texture != 0 && onix_metal_destroy_texture(driver, texture), "retire temporary texture");
                require(driver->frame_resources[slot].count > 0, "retired resource retained by active slot");
                submit_frame(driver);
            }
            uint8_t pixel[4];
            require(onix_metal_read_render_target(driver, target, 8, 8, 1, 1, pixel, sizeof(pixel)), "read persistent target");
            OnixBufferMemory memory;
            onix_metal_buffer_memory(driver, &memory);
            require(memory.instances > 0 && memory.readback > 0, "upload and readback buffers retained before idle");
            require(onix_metal_trim_idle(driver), "trim idle");
            for (uint32_t slot = 0; slot < ONIX_METAL_FRAMES_IN_FLIGHT; slot++) {
                require(driver->inflight_commands[slot] == nil && driver->frame_resources[slot].count == 0, "all commands and retired resources released");
                require(driver->uploads[slot].count == 0 && driver->uploads[slot].blocks == NULL, "upload storage released in every slot");
            }
            onix_metal_buffer_memory(driver, &memory);
            require(memory.instances == 0 && memory.staging == 0 && memory.readback == 0 && memory.scratch == 0, "idle transient storage is zero");
            require(onix_metal_trim_idle(driver), "repeated cleanup is safe");
            require(onix_metal_read_render_target(driver, target, 8, 8, 1, 1, pixel, sizeof(pixel)), "recreate readback after cleanup");
            require(pixel[0] == 0 && pixel[1] == 255 && pixel[2] == 0 && pixel[3] == 255, "idle preserved persistent target contents");
            require(onix_metal_resize(driver, 64 + cycle * 16, 64), "resize after cleanup");
        }
        onix_metal_destroy(driver);
        [layer release];
        puts("Metal memory lifecycle: two frames, resize, retirement, idle trim and recreation OK");
    }
    return 0;
}
