// Independent Linux/glibc software-Vulkan process for external RSS sampling.
// Compile with -I pointing to the render directory under measurement.
#define _GNU_SOURCE
#include <malloc.h>
#include <stdint.h>
#include <stdio.h>
#include <time.h>

static uint32_t heap_trims;
static uint64_t heap_trim_ns;

static uint64_t clock_ns(void) {
	struct timespec now;
	clock_gettime(CLOCK_MONOTONIC, &now);
	return (uint64_t)now.tv_sec * 1000000000 + now.tv_nsec;
}

static int probe_malloc_trim(size_t padding) {
	uint64_t start = clock_ns();
	int result = malloc_trim(padding);
	heap_trim_ns += clock_ns() - start;
	heap_trims++;
	return result;
}

#define malloc_trim probe_malloc_trim
#include "vulkan_native.c"

static void check(int success, const char *operation) {
	if (!success) { fprintf(stderr, "%s: %s\n", operation, onix_gpu_last_error()); exit(1); }
}

static OnixGPUShader load_shader(const char *path) {
	FILE *file = fopen(path, "rb");
	check(file != NULL, path);
	check(fseek(file, 0, SEEK_END) == 0, "seek shader");
	long length = ftell(file);
	check(length > 0, "shader length");
	rewind(file);
	uint32_t *code = malloc((size_t)length);
	check(code && fread(code, 1, (size_t)length, file) == (size_t)length, "read shader");
	fclose(file);
	return (OnixGPUShader){.code = code, .size = (size_t)length};
}

static void sample(const char *stage, OnixGPUDriver *driver) {
	check(vkDeviceWaitIdle(driver->device) == VK_SUCCESS, "complete sampled GPU work");
	OnixBufferMemory memory;
	onix_gpu_buffer_memory(driver, &memory);
	struct mallinfo2 heap = mallinfo2();
	printf("sample %s %llu %zu %zu %zu %u %llu\n", stage,
		(unsigned long long)(memory.instances + memory.staging + memory.readback + memory.scratch),
		heap.uordblks, heap.fordblks, heap.hblkhd, heap_trims, (unsigned long long)heap_trim_ns);
	fflush(stdout);
	// Parent samples /proc while this process is paused, without allocating a
	// profiler or forcing reclamation in the process being measured.
	check(getchar() == '\n', "sample handshake");
}

static void draw(OnixGPUDriver *driver, const OnixNativeSpriteDraw *draws, uint32_t count) {
	const float clear[] = {0, 0, 0, 1};
	OnixGPUSpritePush push = {.projection = {[0] = 1.0f / 480, [5] = 1.0f / 270, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
	OnixGPUInstance instance = {.model = {1, 0, 0, 0, 1, 0}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, 1}};
	check(onix_gpu_begin_frame(driver, clear), "begin probe frame");
	check(onix_gpu_draw_sprites(driver, &push, &instance, 1, draws, count), "draw probe sprites");
	check(onix_gpu_present(driver), "present probe frame");
}

int main(int argc, char **argv) {
	uint32_t count = argc > 1 ? (uint32_t)strtoul(argv[1], NULL, 10) : 32768;
	check(count > 0 && count <= 1048576, "draw count must be 1..1048576");
	Display *display = XOpenDisplay(NULL);
	check(display != NULL, "open probe display");
	Window window = XCreateSimpleWindow(display, DefaultRootWindow(display), 0, 0, 960, 540, 0, 0, 0);
	XMapWindow(display, window);
	XSync(display, False);
	OnixGPUShader shaders[] = {
		load_shader("shaders/vulkan_sprite_multi.vert.spv"), load_shader("shaders/vulkan_sprite_multi.frag.spv"),
		load_shader("shaders/vulkan_shape.vert.spv"), load_shader("shaders/vulkan_shape.frag.spv"),
		load_shader("shaders/vulkan_mesh.vert.spv"), load_shader("shaders/vulkan_mesh.frag.spv"),
		load_shader("shaders/vulkan_mesh_unlit.frag.spv"),
        load_shader("shaders/vulkan_mesh_depth.frag.spv"), load_shader("shaders/vulkan_mesh_basic.frag.spv"),
		load_shader("shaders/vulkan_picking.vert.spv"), load_shader("shaders/vulkan_picking.frag.spv"),
	};
	OnixGPUSurface surface = {.kind = ONIX_GPU_SURFACE_XLIB, .display = display, .window = window, .width = 960, .height = 540};
	OnixGPUDriver *driver = onix_gpu_create(surface, shaders[0], shaders[1], shaders[2], shaders[3], shaders[4], shaders[5], shaders[6], shaders[7], shaders[8], shaders[9], shaders[10]);
	check(driver != NULL, "create probe driver");
	check(driver->physical_device_properties.deviceType == VK_PHYSICAL_DEVICE_TYPE_CPU, "probe requires a CPU Vulkan ICD");
	fprintf(stderr, "device: %s\n", onix_gpu_device_name(driver));
	sample("startup", driver);
	const uint8_t white[] = {255, 255, 255, 255};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, sizeof(white), 0, 0);
	check(texture != 0, "create probe texture");
	OnixNativeSpriteDraw *draws = calloc(count, sizeof(*draws));
	check(draws != NULL, "allocate probe draws");
	for (uint32_t index = 0; index < count; index++) draws[index] = (OnixNativeSpriteDraw){.texture = texture, .instance_count = 1};
	for (uint32_t frame = 0; frame < 8; frame++) draw(driver, draws, 1);
	sample("warm", driver);
	for (uint32_t frame = 0; frame < ONIX_VK_FRAMES_IN_FLIGHT; frame++) draw(driver, draws, count);
	sample("peak", driver);
	for (uint32_t frame = 0; frame < ONIX_VK_FRAMES_IN_FLIGHT * 65; frame++) draw(driver, draws, 1);
	sample("low", driver);
	for (uint32_t frame = 0; frame < ONIX_VK_FRAMES_IN_FLIGHT * 65; frame++) draw(driver, draws, 1);
	sample("steady", driver);
	onix_gpu_destroy(driver);
	free(draws);
	for (uint32_t index = 0; index < sizeof(shaders) / sizeof(shaders[0]); index++) free((void *)shaders[index].code);
	XDestroyWindow(display, window);
	XCloseDisplay(display);
	return 0;
}
