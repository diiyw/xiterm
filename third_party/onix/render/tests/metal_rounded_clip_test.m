#define main onix_metal_lazy_test_main
#include "metal_lazy_test.m"
#undef main

int main(void) {
	@autoreleasepool {
		OnixTestLayer *layer = [[OnixTestLayer alloc] init];
		OnixMetalDriver *driver = onix_metal_create(layer, 32, 32);
		require(driver != NULL, "create rounded clip driver");
		memcpy(sprite_uniforms.projection, identity, sizeof(identity));
		sprite_uniforms.projection[0] = 1.0f / 16;
		sprite_uniforms.projection[5] = -1.0f / 16;
		sprite_uniforms.projection[12] = -1;
		sprite_uniforms.projection[13] = 1;
		const uint8_t white[] = {255, 255, 255, 255};
		white_texture = onix_metal_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, sizeof(white), 0, 0);
		require(white_texture != 0, "create rounded clip texture");
		OnixMetalInstance instance = {
			.model = {24, 0, 4, 0, 24, 4},
			.texture_model = {1, 0, 0, 0, 1, 0},
			.color = {0, 1, 0, 1},
			.custom = {{0}, {0}, {4, 4, 28, 28}, {8, 1, 0, 0}},
		};
		OnixNativeSpriteDraw draw = {.texture = white_texture, .instance_count = 1};
		uint8_t pixels[32 * 256];
		for (int clipped = 0; clipped < 2; clipped++) {
			instance.custom[3][1] = clipped;
			require(onix_metal_begin_frame(driver, blue), "begin rounded clip frame");
			require(onix_metal_draw_sprites(driver, &sprite_uniforms, &instance, 1, &draw, 1), "draw rounded clip sprite");
			finish_frame(driver, pixels);
			for (int y = 4; y <= 27; y += 23) for (int x = 4; x <= 27; x += 23) {
				pixel(pixels, x, y, clipped ? 255 : 0, clipped ? 0 : 255, 0, "rounded corners preserve the parent background");
			}
			pixel(pixels, 16, 16, 0, 255, 0, "rounded clip preserves the interior");
			pixel(pixels, 4, 16, 0, 255, 0, "rounded clip preserves straight edges");
			pixel(pixels, 3, 16, 255, 0, 0, "sprite bounds remain unchanged");
			if (clipped) {
				const uint8_t *arc = pixels + 7 * 256 + 5 * 4;
				require(arc[0] > 0 && arc[0] < 255 && arc[1] > 0 && arc[1] < 255, "rounded arc has antialiased coverage");
			}
		}
		instance.color[3] = .5f;
		require(onix_metal_begin_frame(driver, blue), "begin translucent clipped frame");
		require(onix_metal_draw_sprites(driver, &sprite_uniforms, &instance, 1, &draw, 1), "draw translucent clipped sprite");
		finish_frame(driver, pixels);
		pixel(pixels, 4, 4, 255, 0, 0, "translucent clipped corner preserves parent");
		const uint8_t *center = pixels + 16 * 256 + 16 * 4;
		require(abs(center[0] - 128) <= 1 && abs(center[1] - 128) <= 1 && center[2] == 0 && center[3] == 255, "clip preserves premultiplied opacity");
		onix_metal_destroy(driver);
		[layer release];
		puts("Metal rounded clip: translated bounds, four corners, straight edges, antialiasing, disabled clip and opacity OK");
	}
	return 0;
}
