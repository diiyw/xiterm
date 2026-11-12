#define VK_USE_PLATFORM_XLIB_KHR
#define VK_USE_PLATFORM_WAYLAND_KHR
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <float.h>
#include "display_reference.h"
#include <time.h>
#if defined(__GLIBC__)
#include <malloc.h>
#endif
#include <X11/Xlib.h>
#include "../vulkan/vulkan.h"

static size_t host_allocations;
static size_t resource_allocations;
static size_t live_shader_modules, live_pipelines, live_descriptor_pools;
static uint32_t last_pool_sets;
static int fail_next_descriptor_pool, fail_next_descriptor_set;
static size_t surface_descriptor_updates;
static size_t mesh_descriptor_updates;
static size_t descriptor_bind_calls;
static size_t indexed_draw_calls;
static int fail_next_reallocation;
#if defined(__GLIBC__)
static size_t host_heap_trim_calls;

static int tracked_malloc_trim(size_t padding) {
	host_heap_trim_calls++;
	return malloc_trim(padding);
}
#endif

static void *tracked_malloc(size_t size) {
	host_allocations++;
	return malloc(size);
}

static void *tracked_calloc(size_t count, size_t size) {
	host_allocations++;
	return calloc(count, size);
}

static void *tracked_realloc(void *pointer, size_t size) {
	if (fail_next_reallocation) { fail_next_reallocation = 0; return NULL; }
	host_allocations++;
	return realloc(pointer, size);
}

#define TRACK_CREATE(name, info, handle) \
static VkResult tracked_##name(VkDevice device, const info *descriptor, const VkAllocationCallbacks *allocator, handle *result) { \
	resource_allocations++; \
	return name(device, descriptor, allocator, result); \
}

TRACK_CREATE(vkCreateBuffer, VkBufferCreateInfo, VkBuffer)
TRACK_CREATE(vkAllocateMemory, VkMemoryAllocateInfo, VkDeviceMemory)
TRACK_CREATE(vkCreateFence, VkFenceCreateInfo, VkFence)
TRACK_CREATE(vkCreateImageView, VkImageViewCreateInfo, VkImageView)
TRACK_CREATE(vkCreateFramebuffer, VkFramebufferCreateInfo, VkFramebuffer)
static VkResult tracked_vkCreateDescriptorPool(VkDevice device, const VkDescriptorPoolCreateInfo *info, const VkAllocationCallbacks *allocator, VkDescriptorPool *result) {
	resource_allocations++;
	if (fail_next_descriptor_pool) { fail_next_descriptor_pool = 0; *result = VK_NULL_HANDLE; return VK_ERROR_OUT_OF_DEVICE_MEMORY; }
	VkResult status = vkCreateDescriptorPool(device, info, allocator, result);
	if (status == VK_SUCCESS) { live_descriptor_pools++; last_pool_sets = info->maxSets; }
	return status;
}

static void tracked_vkDestroyDescriptorPool(VkDevice device, VkDescriptorPool pool, const VkAllocationCallbacks *allocator) {
	if (pool) live_descriptor_pools--;
	vkDestroyDescriptorPool(device, pool, allocator);
}
TRACK_CREATE(vkCreateCommandPool, VkCommandPoolCreateInfo, VkCommandPool)
static VkResult tracked_vkCreateShaderModule(VkDevice device, const VkShaderModuleCreateInfo *info, const VkAllocationCallbacks *allocator, VkShaderModule *result) {
	resource_allocations++;
	VkResult status = vkCreateShaderModule(device, info, allocator, result);
	if (status == VK_SUCCESS) live_shader_modules++;
	return status;
}

static void tracked_vkDestroyShaderModule(VkDevice device, VkShaderModule shader, const VkAllocationCallbacks *allocator) {
	if (shader) live_shader_modules--;
	vkDestroyShaderModule(device, shader, allocator);
}

static VkResult tracked_vkCreateGraphicsPipelines(VkDevice device, VkPipelineCache cache, uint32_t count, const VkGraphicsPipelineCreateInfo *info, const VkAllocationCallbacks *allocator, VkPipeline *result) {
	resource_allocations++;
	VkResult status = vkCreateGraphicsPipelines(device, cache, count, info, allocator, result);
	for (uint32_t index = 0; index < count; index++) if (result[index]) live_pipelines++;
	return status;
}

static void tracked_vkDestroyPipeline(VkDevice device, VkPipeline pipeline, const VkAllocationCallbacks *allocator) {
	if (pipeline) live_pipelines--;
	vkDestroyPipeline(device, pipeline, allocator);
}

static VkResult tracked_vkCreateImage(VkDevice device, const VkImageCreateInfo *descriptor, const VkAllocationCallbacks *allocator, VkImage *result) {
	resource_allocations++;
	VkImageCreateInfo image = *descriptor;
	// The test reads surface depth to verify its lazy clear and occlusion.
	if (image.usage & VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT) image.usage |= VK_IMAGE_USAGE_TRANSFER_SRC_BIT;
	return vkCreateImage(device, &image, allocator, result);
}

#define TRACK_ALLOCATE(name, info, handle) \
static VkResult tracked_##name(VkDevice device, const info *descriptor, handle *result) { \
	resource_allocations++; \
	return name(device, descriptor, result); \
}

TRACK_ALLOCATE(vkAllocateCommandBuffers, VkCommandBufferAllocateInfo, VkCommandBuffer)
static VkResult tracked_vkAllocateDescriptorSets(VkDevice device, const VkDescriptorSetAllocateInfo *info, VkDescriptorSet *result) {
	resource_allocations++;
	if (fail_next_descriptor_set) { fail_next_descriptor_set = 0; *result = VK_NULL_HANDLE; return VK_ERROR_OUT_OF_POOL_MEMORY; }
	return vkAllocateDescriptorSets(device, info, result);
}

static void tracked_vkUpdateDescriptorSets(VkDevice device, uint32_t count, const VkWriteDescriptorSet *writes, uint32_t copy_count, const VkCopyDescriptorSet *copies) {
	if (count == 15 && writes[0].descriptorType == VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER) surface_descriptor_updates++;
	if (count != 0 && (writes[0].descriptorType == VK_DESCRIPTOR_TYPE_STORAGE_BUFFER ||
		writes[0].descriptorType == VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER)) mesh_descriptor_updates++;
	vkUpdateDescriptorSets(device, count, writes, copy_count, copies);
}

static void tracked_vkCmdBindDescriptorSets(VkCommandBuffer command, VkPipelineBindPoint point, VkPipelineLayout layout, uint32_t first, uint32_t count, const VkDescriptorSet *sets, uint32_t offset_count, const uint32_t *offsets) {
	descriptor_bind_calls++;
	vkCmdBindDescriptorSets(command, point, layout, first, count, sets, offset_count, offsets);
}

static void tracked_vkCmdDrawIndexed(VkCommandBuffer command, uint32_t index_count, uint32_t instance_count, uint32_t first_index, int32_t vertex_offset, uint32_t first_instance) {
	indexed_draw_calls++;
	vkCmdDrawIndexed(command, index_count, instance_count, first_index, vertex_offset, first_instance);
}

static void tracked_vkCmdBindPipeline(VkCommandBuffer command, VkPipelineBindPoint point, VkPipeline pipeline) {
	if (pipeline == VK_NULL_HANDLE) {
		fputs("cannot bind a null Vulkan pipeline\n", stderr);
		exit(1);
	}
	vkCmdBindPipeline(command, point, pipeline);
}

#define malloc tracked_malloc
#define calloc tracked_calloc
#define realloc tracked_realloc
#if defined(__GLIBC__)
#define malloc_trim tracked_malloc_trim
#endif
#define vkCreateBuffer tracked_vkCreateBuffer
#define vkAllocateMemory tracked_vkAllocateMemory
#define vkCreateFence tracked_vkCreateFence
#define vkCreateImage tracked_vkCreateImage
#define vkCreateImageView tracked_vkCreateImageView
#define vkCreateFramebuffer tracked_vkCreateFramebuffer
#define vkCreateDescriptorPool tracked_vkCreateDescriptorPool
#define vkDestroyDescriptorPool tracked_vkDestroyDescriptorPool
#define vkCreateCommandPool tracked_vkCreateCommandPool
#define vkCreateShaderModule tracked_vkCreateShaderModule
#define vkDestroyShaderModule tracked_vkDestroyShaderModule
#define vkCreateGraphicsPipelines tracked_vkCreateGraphicsPipelines
#define vkDestroyPipeline tracked_vkDestroyPipeline
#define vkAllocateCommandBuffers tracked_vkAllocateCommandBuffers
#define vkAllocateDescriptorSets tracked_vkAllocateDescriptorSets
#define vkUpdateDescriptorSets tracked_vkUpdateDescriptorSets
#define vkCmdBindDescriptorSets tracked_vkCmdBindDescriptorSets
#define vkCmdDrawIndexed tracked_vkCmdDrawIndexed
#define vkCmdBindPipeline tracked_vkCmdBindPipeline
#include "../vulkan_native.c"

static void require(int success, const char *operation) {
	if (!success) {
		fprintf(stderr, "%s: %s\n", operation, onix_gpu_last_error());
		exit(1);
	}
}

static OnixGPUShader load_shader(const char *path) {
	FILE *file = fopen(path, "rb");
	require(file != NULL, path);
	require(fseek(file, 0, SEEK_END) == 0, "seek shader");
	long length = ftell(file);
	require(length > 0, "shader length");
	rewind(file);
	uint32_t *code = malloc((size_t)length);
	require(code != NULL, "allocate shader");
	require(fread(code, 1, (size_t)length, file) == (size_t)length, "read shader");
	fclose(file);
	return (OnixGPUShader){.code = code, .size = (size_t)length};
}

static void fill_pixels(uint8_t *pixels, size_t length, uint8_t red, uint8_t green) {
	for (size_t offset = 0; offset < length; offset += 4) {
		pixels[offset] = red;
		pixels[offset + 1] = green;
		pixels[offset + 2] = 0;
		pixels[offset + 3] = 255;
	}
}

static void draw_target(OnixGPUDriver *driver, uint32_t target, uint32_t texture) {
	const float clear[] = {0, 0, 0, 1};
	require(onix_gpu_begin_render_target(driver, target, clear, 0, 1, 1), "begin target");
	onix_gpu_set_viewport(driver, 0, 0, 16, 16);
	OnixGPUSpritePush push = {.projection = {[0] = 0.125f, [5] = 0.125f, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
	OnixGPUInstance instance = {.model = {16, 0, 0, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, 1}};
	OnixNativeSpriteDraw draw = {.texture = texture, .instance_count = 1};
	require(onix_gpu_draw_sprites(driver, &push, &instance, 1, &draw, 1), "draw sprite");
	require(onix_gpu_end_render_target(driver), "end target");
}

static void test_target_upload_resume(OnixGPUDriver *driver, int depth) {
	uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, depth, &color);
	const uint8_t red[] = {255, 0, 0, 255}, green[] = {0, 255, 0, 255};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, red, sizeof(red), 0, 0);
	require(target && texture, "create upload resume resources");
	const float clear[] = {0, 0, 0, 1};
	OnixGPUSpritePush push = {.projection = {[0] = .125f, [5] = .125f, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
	OnixGPUInstance instance = {.model = {8, 0, 0, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, 1}};
	OnixNativeSpriteDraw draw = {.texture = texture, .instance_count = 1};
	require(onix_gpu_begin_frame(driver, clear), "begin upload resume frame");
	require(onix_gpu_begin_render_target(driver, target, clear, 0, 1, 1), "begin upload resume target");
	onix_gpu_set_viewport(driver, 0, 0, 16, 16);
	require(onix_gpu_draw_sprites(driver, &push, &instance, 1, &draw, 1), "draw before target upload");
	uint32_t uploaded = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, green, sizeof(green), 0, 0);
	require(uploaded != 0, "create texture during target pass");
	draw.texture = uploaded;
	instance.model[2] = 8;
	require(onix_gpu_draw_sprites(driver, &push, &instance, 1, &draw, 1), "draw after target upload");
	require(!driver->frame_cleared, "target upload does not mark window cleared");
	require(onix_gpu_end_render_target(driver), "end upload resume target");
	require(onix_gpu_present(driver), "present upload resume frame");
	uint8_t pixel[4];
	for (int side = 0; side < 2; side++) {
		require(onix_gpu_read_render_target(driver, target, 4 + side * 8, 8, 1, 1, pixel, sizeof(pixel)), "read resumed target");
		if (driver->swapchain_format == VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format == VK_FORMAT_B8G8R8A8_SRGB) {
			uint8_t blue = pixel[0]; pixel[0] = pixel[2]; pixel[2] = blue;
		}
		const uint8_t *expected = side == 0 ? red : green;
		if (memcmp(pixel, expected, sizeof(pixel)) != 0) {
			fprintf(stderr, "resumed target side %d: pixel=(%d,%d,%d,%d)\n", side, pixel[0], pixel[1], pixel[2], pixel[3]);
			exit(1);
		}
	}
	require(onix_gpu_destroy_texture(driver, texture), "destroy upload resume source");
	require(onix_gpu_destroy_texture(driver, uploaded), "destroy uploaded texture");
	require(onix_gpu_destroy_render_target(driver, target), "destroy upload resume target");
}

static void test_mesh_lighting(OnixGPUDriver *driver) {
	uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
	const uint8_t gray[] = {64, 64, 64, 64};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_SRGBA8, 0, gray, sizeof(gray), 0, 0);
	const uint8_t radiance[] = {64, 64, 64, 255};
	uint32_t environment = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, radiance, sizeof(radiance), 0, 0);
	uint8_t rgbe[16*64*4];
    for(int y=0;y<64;y++) for(int x=0;x<16;x++) {
        int i=(y*16+x)*4;
        rgbe[i]=rgbe[i+1]=rgbe[i+2]=255;rgbe[i+3]=x<8?133:126;
    }
    uint32_t hdr=onix_gpu_create_texture_with_format(driver,16,64,1,ONIX_TEXTURE_FORMAT_RGBA8,0,rgbe,sizeof(rgbe),0,0);
    uint8_t panorama[64*32*4];
    for(int y=0;y<32;y++) for(int x=0;x<64;x++) {
        int i=(y*64+x)*4;panorama[i]=x%2?255:0;panorama[i+1]=0;panorama[i+2]=x%2?0:255;panorama[i+3]=128;
    }
    uint32_t sky=onix_gpu_create_texture_with_format(driver,64,32,1,ONIX_TEXTURE_FORMAT_RGBA8,0,panorama,sizeof(panorama),0,0);
    require(target && texture && environment && hdr && sky, "create lighting test resources");
	OnixGPUMeshPush push = {.view_projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
	OnixGPUVertex3D vertices[] = {
		{.position = {-1, -1, .5f}, .normal = {0, 0, 1}},
		{.position = {1, -1, .5f}, .normal = {0, 0, 1}},
		{.position = {0, 1, .5f}, .normal = {0, 0, 1}},
	};
	uint32_t indices[] = {0, 1, 2}, deformation[16] = {0};
	OnixGPUInstance3D instance = {.model = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}, .base_color = {1, 1, 1, 1}, .material = {.8f, 1.5f, .5f, 256}, .uv_transform = {1, 1, 0, 0}};
	OnixNativeMeshDraw3D draw = {.texture = texture, .extra_textures = {[4] = environment}, .index_count = 3, .instance_count = 1};
	float lighting[ONIX_GPU_MESH_LIGHTING_BYTES / sizeof(float)] = {[18] = 2, [19] = 1, [292] = 1, [296] = 1};
	const float clear[] = {0, 0, 0, 1};
	for (int test = 0; test < 8; test++) {
		// CCW unlit, rejected back face, double-sided back face, inward-facing sky.
		if (test == 1) { indices[1] = 2; indices[2] = 1; }
		if (test == 2) instance.material[3] = 256 + 512;
		if (test == 3) instance.material[3] = 2048 + 512;
        if (test == 4) {
            draw.extra_textures[4]=hdr;lighting[851]=1;lighting[294]=1.0f/16;lighting[295]=1.0f/8;lighting[296]=.0625f;
            instance.custom[3][0]=1;instance.custom[3][3]=1;
        }
        if(test>=5) {
            instance.material[3]=2048+512+8388608;draw.texture=sky;lighting[296]=1;lighting[293]=test==6?3.14159265359f/32:0;
            float angle=-3.14159265359f/64;
            instance.custom[3][0]=test==7?-1:cosf(angle);instance.custom[3][1]=0;instance.custom[3][2]=test==7?0:sinf(angle);instance.custom[3][3]=1;
        }
		require(onix_gpu_begin_frame(driver, clear), "begin lighting frame");
		require(onix_gpu_begin_render_target(driver, target, clear, 0, 1, 1), "begin lighting target");
		onix_gpu_set_viewport(driver, 0, 0, 16, 16);
		require(onix_gpu_draw_meshes(driver, &push, vertices, 3, indices, 3, &instance, 1, deformation, 16, &draw, 1, lighting), "draw lighting test");
		require(onix_gpu_end_render_target(driver), "end lighting target");
		require(onix_gpu_present(driver), "present lighting frame");
		uint8_t pixel[4];
		require(onix_gpu_read_render_target(driver, target, 8, 8, 1, 1, pixel, sizeof(pixel)), "read lighting pixel");
		int expected = test == 1 ? 0 : test == 3 ? 231 : test == 4 || test == 5 ? 188 : test == 6 ? 0 : test == 7 ? 156 : 64;
		if (abs((int)pixel[driver->swapchain_format==VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format==VK_FORMAT_B8G8R8A8_SRGB?2:0] - expected) > 1 || pixel[3] != 255) {
			fprintf(stderr, "lighting case %d: pixel=(%d,%d,%d,%d), expected red=%d\n", test, pixel[0], pixel[1], pixel[2], pixel[3], expected);
			exit(1);
		}
	}
    require(onix_gpu_destroy_texture(driver,sky),"destroy original panorama");
	require(onix_gpu_destroy_render_target(driver, target), "destroy lighting target");
	require(onix_gpu_destroy_texture(driver, texture), "destroy lighting texture");
	require(onix_gpu_destroy_texture(driver, environment), "destroy environment texture");
	require(onix_gpu_destroy_texture(driver,hdr), "destroy RGBE texture");
	printf("Vulkan original panorama detail/rotation/seam, RGBE interpolation, sRGB, opaque alpha, back-face coverage and sky pixels passed\n");
}

static void test_mesh_pipeline_laziness(OnixGPUDriver *driver) {
	const uint8_t white[] = {255, 255, 255, 255};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, sizeof(white), 0, 0);
	uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
	require(texture && target, "create guide resources");
	OnixGPUMeshPush push = {.view_projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
	OnixGPUVertex3D vertices[] = {{.position = {-1, -1, .5f}}, {.position = {1, -1, .5f}}, {.position = {0, 1, .5f}}};
	uint32_t indices[] = {0, 1, 2}, deformation[16] = {0};
	OnixGPUInstance3D instance = {.model = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}, .base_color = {.25f, .5f, 1, 1}, .material = {-1}, .uv_transform = {1, 1}};
	OnixNativeMeshDraw3D draw = {.texture = texture, .index_count = 3, .instance_count = 1, .alpha_mode = 2};
	const float clear[] = {0, 0, 0, 1};
	float lighting[ONIX_GPU_MESH_LIGHTING_BYTES / sizeof(float)] = {0};
	struct timespec start, end;
	timespec_get(&start, TIME_UTC);
	require(onix_gpu_begin_frame(driver, clear), "begin first guide frame");
	require(onix_gpu_begin_render_target(driver, target, clear, 0, 1, 1), "begin guide target");
	onix_gpu_set_viewport(driver, 0, 0, 16, 16);
	require(onix_gpu_draw_meshes(driver, &push, vertices, 3, indices, 3, &instance, 1, deformation, 16, &draw, 1, lighting), "draw first guide");
	timespec_get(&end, TIME_UTC);
	require(onix_gpu_end_render_target(driver), "end guide target");
	require(onix_gpu_present(driver), "present guide");
	for (uint32_t index=0;index<driver->image_count;index++) require(driver->depth_images[index]==VK_NULL_HANDLE, "offscreen mesh leaves surface depth unallocated");
	require(driver->mesh_builtin[1].transparent[0] != VK_NULL_HANDLE, "guide uses unlit pipeline");
	for (uint32_t variant = 0; variant < ONIX_MESH_VARIANT_COUNT; variant++) {
		require(driver->mesh_builtin[variant].opaque == VK_NULL_HANDLE && driver->mesh_builtin[variant].opaque_surface == VK_NULL_HANDLE, "guide avoids unused opaque pipelines");
		for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) {
			if (variant != 1 || blend != 0) require(driver->mesh_builtin[variant].transparent[blend] == VK_NULL_HANDLE, "guide avoids unused lighting/blend pipelines");
		}
	}
	uint8_t pixel[4];
	require(onix_gpu_read_render_target(driver, target, 8, 8, 1, 1, pixel, sizeof(pixel)), "read unlit pixel");
	int red = (driver->swapchain_format == VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format == VK_FORMAT_B8G8R8A8_SRGB) ? 2 : 0;
	require(abs(pixel[red] - 64) <= 1 && abs(pixel[1] - 128) <= 1 && pixel[2 - red] == 255, "unlit pipeline preserves color");
	require(onix_gpu_destroy_render_target(driver, target), "destroy guide target");
	require(onix_gpu_destroy_texture(driver, texture), "destroy guide texture");
	printf("First unlit Vulkan draw: %.2f ms; only one pipeline compiled\n", (end.tv_sec - start.tv_sec)*1000.0 + (end.tv_nsec - start.tv_nsec)/1000000.0);
}

static void test_material_fresnel(OnixGPUDriver *driver) {
	uint32_t color,target=onix_gpu_create_render_target(driver,16,16,0,1,&color);
	const uint8_t white[]={255,255,255,255};
	uint32_t texture=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,white,4,0,0);
	uint8_t atlas[8*4];
	for(int i=0;i<8;i++){atlas[i*4]=atlas[i*4+1]=atlas[i*4+2]=128;atlas[i*4+3]=129;}
	atlas[28]=255;atlas[29]=atlas[30]=0;atlas[31]=255;
	uint32_t environment=onix_gpu_create_texture_with_format(driver,1,8,1,ONIX_TEXTURE_FORMAT_RGBA8,0,atlas,32,0,0);
	require(target && texture && environment,"create Fresnel resources");
	OnixGPUMeshPush push={.view_projection={[0]=1,[5]=1,[10]=1,[15]=1}};
	OnixGPUVertex3D vertices[]={
		{.position={-1,-1,.5f},.normal={0,0,1}},
		{.position={1,-1,.5f},.normal={0,0,1}},
		{.position={0,1,.5f},.normal={0,0,1}},
	};
	uint32_t indices[]={0,1,2},deformation[16]={0};
	OnixGPUInstance3D instance={.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={0,0,0,1},.material={1,1.5f,.5f,4096},.uv_transform={1,1,0,0},.custom={{1,1,1,0},{1,1,1,0},{0,0,0,0},{0,1,0,1}}};
	OnixNativeMeshDraw3D draw={.texture=texture,.index_count=3,.instance_count=1};
	const float clear[]={0,0,0,1},iors[]={1,1.33f,1.5f,2.42f,32,FLT_MAX,0};
	int values[7];
	for(int test=0;test<18;test++) {
		float lighting[ONIX_GPU_MESH_LIGHTING_BYTES/sizeof(float)]={[16]=.0625f,[17]=-.0625f,[18]=1.5f,[19]=1,[26]=-1,[289]=1,[296]=1};
		instance.material[1]=iors[test%7];
		if(test<7 || test>=14) lighting[28]=lighting[29]=lighting[30]=4*3.14159265359f;
		if(test>=7 && test<14) {lighting[851]=1;lighting[292]=255.0f/256;lighting[294]=lighting[295]=.5f;draw.extra_textures[4]=environment;}
		else draw.extra_textures[4]=0;
		if(test==14) {instance.material[1]=4;instance.custom[0][2]=.25f;instance.custom[1][0]=instance.custom[1][1]=instance.custom[1][2]=4;}
		if(test>=15) {
			instance.material[1]=1.5f;instance.custom[0][2]=0;
			instance.custom[1][0]=instance.custom[1][1]=instance.custom[1][2]=1;
			instance.base_color[0]=instance.base_color[1]=instance.base_color[2]=1;
			instance.emissive_metallic[3]=.5f;
		}
		if(test==15) {lighting[28]=lighting[29]=lighting[30]=0;lighting[20]=lighting[21]=lighting[22]=1;}
		if(test==17) {
			// CPU-packed emissive factor (.25,.5,1) * strength 64.
			instance.emissive_metallic[0]=16;instance.emissive_metallic[1]=32;instance.emissive_metallic[2]=64;
			lighting[28]=lighting[29]=lighting[30]=0;lighting[297]=3;lighting[296]=.001f;
		}
		require(onix_gpu_begin_frame(driver,clear),"begin Fresnel frame");
		require(onix_gpu_begin_render_target(driver,target,clear,0,1,1),"begin Fresnel target");onix_gpu_set_viewport(driver,0,0,16,16);
		require(onix_gpu_draw_meshes(driver,&push,vertices,3,indices,3,&instance,1,deformation,16,&draw,1,lighting),"draw Fresnel mesh");
		require(onix_gpu_end_render_target(driver),"end Fresnel target");require(onix_gpu_present(driver),"present Fresnel frame");
		uint8_t pixel[4];require(onix_gpu_read_render_target(driver,target,8,8,1,1,pixel,4),"read Fresnel pixel");
		int redIndex=(driver->swapchain_format==VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format==VK_FORMAT_B8G8R8A8_SRGB)?2:0;
		if(test==17) {require(abs(pixel[redIndex]-64)<=1 && abs(pixel[1]-128)<=1 && pixel[2-redIndex]==255 && pixel[3]==134,"preserve HDR emission in RGBE capture");continue;}
		int value=pixel[redIndex];
		if(test<7) values[test]=value;
		if(test>=7 && test<14) require(abs(value-values[test-7])<=1,"IBL agrees with direct IOR");
		float srgb=value/255.0f,linear=srgb<=.04045f?srgb/12.92f:powf((srgb+.055f)/1.055f,2.4f),radiance=linear/(1-linear);
		float ratio=instance.material[1]==0?1:1-2/(instance.material[1]+1),expected=ratio*ratio;
		if(test==14) expected=.25f;
		if(test==15) expected=.5f;
		if(test==16) expected=2.5f;
		if(fabsf(radiance-expected)>(test==16?.06f:.018f)) {fprintf(stderr,"Vulkan Fresnel case %d: %g expected %g\n",test,radiance,expected);exit(1);}
	}
	require(onix_gpu_destroy_render_target(driver,target),"destroy Fresnel target");
	require(onix_gpu_destroy_texture(driver,texture),"destroy Fresnel texture");require(onix_gpu_destroy_texture(driver,environment),"destroy Fresnel environment");
	printf("Vulkan IOR direct/IBL: air=%d water=%d default=%d diamond=%d large=%d max=%d infinite=%d; tinted F0, fractional metalness and HDR emission passed\n",values[0],values[1],values[2],values[3],values[4],values[5],values[6]);
}

static void test_material_layers(OnixGPUDriver *driver) {
	uint32_t color, target=onix_gpu_create_render_target(driver,16,16,0,1,&color);
	const uint8_t white[]={255,255,255,255}, zero_coat[]={0,255,255,255}, zero_anisotropy[]={255,128,0,255};
	uint32_t texture=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,white,4,0,0);
	uint32_t coat_map=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,zero_coat,4,0,0);
	uint32_t anisotropy_map=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,zero_anisotropy,4,0,0);
	require(target && texture && coat_map && anisotropy_map,"create material layer resources");
	OnixGPUMeshPush push={.view_projection={[0]=1,[5]=1,[10]=1,[15]=1}};
	OnixGPUVertex3D vertices[]={
		{.position={-1,-1,.5f},.normal={0,0,1},.uv={.5f,.5f},.tangent={1,0,0,1}},
		{.position={1,-1,.5f},.normal={0,0,1},.uv={.5f,.5f},.tangent={1,0,0,1}},
		{.position={0,1,.5f},.normal={0,0,1},.uv={.5f,.5f},.tangent={1,0,0,1}},
	};
	uint32_t indices[]={0,1,2},deformation[96]={0};
	OnixGPUInstance3D instance={.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={1,1,1,1},.emissive_metallic={0,0,0,1},.material={.45f,1.5f,.5f,4096},.uv_transform={1,1,0,0},.custom={{1,1,1,0},{1,1,1,.6f},{0,0,0,0},{0,1,0,1}}};
	OnixNativeMeshDraw3D draw={.texture=texture,.extra_textures={[7]=coat_map,[12]=anisotropy_map},.index_count=3,.instance_count=1};
	float lighting[ONIX_GPU_MESH_LIGHTING_BYTES/sizeof(float)]={[16]=.6625f,[17]=-.0625f,[18]=1.3f,[19]=1,[26]=-1,[28]=1,[29]=1,[30]=1,[296]=1};
	const float clear[]={0,0,0,1};
	// Two tangent morph cases and a joint rotation exercise the actual storage
	// buffer ABI, rather than supplying already deformed vertex directions.
	deformation[1]=8;deformation[3]=75;deformation[4]=3;deformation[5]=1;
	float one=1,minus_one=-1;
	memcpy(&deformation[75],&one,4);
	for(int i=0;i<3;i++) {memcpy(&deformation[8+i*9+6],&minus_one,4);memcpy(&deformation[8+i*9+7],&one,4);}
	int values[10];
	for(int test=0;test<10;test++) {
		instance.custom[3][0]=test==0 || test>=7 ? 0 : .9f;
		instance.custom[3][1]=test==2 ? 0 : 1;instance.custom[3][2]=test==2 ? 1 : 0;
		for(int i=0;i<3;i++){vertices[i].tangent[0]=test==3 ? 0 : 1;vertices[i].tangent[1]=test==3 ? 1 : 0;}
		instance.material[3]=4096+(test==6 ? 2097152 : 0)+(test==9 ? 65536 : 0);
		draw.deformation_index=test==4 || test==5 ? 1 : 0;
		if(test==5) {
			// A 90-degree joint rotation should match the authored Y tangent.
			instance.material[3]=4096;deformation[0]=35;deformation[1]=0;deformation[2]=59;deformation[5]=0;deformation[6]=1;
			float matrix[16]={[1]=1,[4]=-1,[10]=1,[15]=1};memcpy(deformation+59,matrix,sizeof(matrix));
			for(int i=0;i<3;i++) memcpy(&deformation[35+i*8+4],&one,4);
		}
		if(test>=7) {
			lighting[28]=lighting[29]=lighting[30]=0;
			instance.emissive_metallic[0]=instance.emissive_metallic[1]=instance.emissive_metallic[2]=.5f;
			instance.custom[0][3]=test==7 ? 0 : 1;
		}
		draw.mesh = 0;
		if (draw.deformation_index != 0) {
			draw.mesh = onix_gpu_create_mesh(driver,vertices,3,indices,3);
			require(draw.mesh != 0,"create deformation geometry");
			require(onix_gpu_set_mesh_deformation(driver,draw.mesh,deformation,96,3),"upload static deformation");
		}
		require(onix_gpu_begin_frame(driver,clear),"begin material frame");
		require(onix_gpu_begin_render_target(driver,target,clear,0,1,1),"begin material target");
		onix_gpu_set_viewport(driver,0,0,16,16);
		require(onix_gpu_draw_meshes(driver,&push,vertices,3,indices,3,&instance,1,deformation,96,&draw,1,lighting),"draw material layers");
		require(onix_gpu_end_render_target(driver),"end material target");require(onix_gpu_present(driver),"present material frame");
		uint8_t pixel[4];require(onix_gpu_read_render_target(driver,target,8,8,1,1,pixel,4),"read material layers");values[test]=pixel[0];
		if(draw.mesh != 0) require(onix_gpu_destroy_mesh(driver,draw.mesh),"destroy deformation mesh");
	}
	if(abs(values[1]-values[2])<10 || abs(values[2]-values[3])>1 || abs(values[3]-values[4])>1 || abs(values[3]-values[5])>1 || abs(values[6]-values[0])>1 || values[8]>=values[7] || abs(values[9]-values[7])>1) {
		fprintf(stderr,"Vulkan material layers: iso=%d X=%d rotation=%d Y=%d morph=%d skin=%d B=%d emission=%d coat=%d R=%d\n",values[0],values[1],values[2],values[3],values[4],values[5],values[6],values[7],values[8],values[9]);exit(1);
	}
	require(onix_gpu_destroy_render_target(driver,target),"destroy material target");
	require(onix_gpu_destroy_texture(driver,texture),"destroy material texture");
	require(onix_gpu_destroy_texture(driver,coat_map),"destroy coat map");
	require(onix_gpu_destroy_texture(driver,anisotropy_map),"destroy anisotropy map");
	printf("Vulkan anisotropic highlights, authored tangent rotation, GPU tangent morph/skin and clearcoat emission/texture passed\n");
}

static void test_mesh_occlusion(OnixGPUDriver *driver) {
	uint32_t depth_color, color;
	uint32_t depth_target = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &depth_color);
	uint32_t target = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
	const uint8_t white[] = {255, 255, 255, 255};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, sizeof(white), 0, 0);
	require(depth_target && target && texture, "create SSAO resources");
	OnixGPUMeshPush push = {.view_projection = {[0]=1,[5]=1,[10]=1,[15]=1}};
	OnixGPUVertex3D vertices[] = {
		{.position={-1,-1,.75f},.normal={0,0,1}}, {.position={1,-1,.75f},.normal={0,0,1}},
		{.position={1,1,.75f},.normal={0,0,1}}, {.position={-1,1,.75f},.normal={0,0,1}},
	};
	uint32_t indices[] = {0,1,2,0,2,3}, deformation[16] = {0};
	OnixGPUInstance3D instance = {.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={1,1,1,1},.material={.8f,1.0f,.5f,0},.uv_transform={1,1,0,0},.custom={{1,1,0,0}}};
	OnixNativeMeshDraw3D draw = {.texture=texture,.index_count=6,.instance_count=1};
	float lighting[ONIX_GPU_MESH_LIGHTING_BYTES/sizeof(float)] = {[18]=2,[19]=1,[20]=1,[21]=1,[22]=1,[290]=1,[291]=.025f,[296]=1,[303]=16,[880]=1,[885]=1,[890]=1,[895]=1};
	const float clear[] = {0,0,0,0};
	int values[5], bottom=0;
	for (int test=0;test<5;test++) {
		for (int i=0;i<4;i++) vertices[i].position[2] = test==3?.5f:.75f;
		vertices[0].position[1]=vertices[1].position[1]=test==4?0:-1;
		lighting[290]=test==4?.5f:1;
		instance.material[3]=0;
		draw.extra_textures[6]=0;
		lighting[297]=2;lighting[298]=0;
		require(onix_gpu_begin_frame(driver,clear), "begin SSAO frame");
		require(onix_gpu_begin_render_target(driver,depth_target,clear,0,1,1), "begin SSAO depth");
		onix_gpu_set_viewport(driver,0,0,16,16);
		require(onix_gpu_draw_meshes(driver,&push,vertices,4,indices,6,&instance,1,deformation,16,&draw,1,lighting), "draw SSAO depth");
		require(onix_gpu_end_render_target(driver), "end SSAO depth");
		for (int i=0;i<4;i++) vertices[i].position[2]=.5f;
		vertices[0].position[1]=vertices[1].position[1]=-1;
		draw.extra_textures[6]=depth_color;
		instance.material[3]=test==2?32768:0;
		lighting[297]=0;lighting[298]=test==0?0:1;
		require(onix_gpu_begin_render_target(driver,target,clear,0,1,1), "begin SSAO receiver");
		onix_gpu_set_viewport(driver,0,0,16,16);
		require(onix_gpu_draw_meshes(driver,&push,vertices,4,indices,6,&instance,1,deformation,16,&draw,1,lighting), "draw SSAO receiver");
		require(onix_gpu_end_render_target(driver), "end SSAO receiver");
		require(onix_gpu_present(driver), "present SSAO frame");
		uint8_t pixel[4];
		require(onix_gpu_read_render_target(driver,target,8,test==4?4:8,1,1,pixel,sizeof(pixel)), "read SSAO receiver");
		values[test]=pixel[0];
		if(draw.mesh != 0) require(onix_gpu_destroy_mesh(driver,draw.mesh),"destroy deformation mesh");
		if (test==4) {
			require(onix_gpu_read_render_target(driver,target,8,12,1,1,pixel,sizeof(pixel)), "read unoccluded bottom");
			bottom=pixel[0];
		}
	}
	if (values[0]<185 || values[1]>=values[0]-15 || abs(values[2]-values[0])>1 || abs(values[3]-values[0])>1) {
		fprintf(stderr,"Vulkan SSAO: reference=%d contact=%d excluded=%d flat=%d\n",values[0],values[1],values[2],values[3]);exit(1);
	}
	if (values[4]>=values[0]-10 || abs(bottom-values[0])>1) {
		fprintf(stderr,"Vulkan SSAO screen direction: reference=%d top=%d bottom=%d\n",values[0],values[4],bottom);exit(1);
	}
	require(onix_gpu_destroy_render_target(driver,depth_target), "destroy SSAO depth");
	require(onix_gpu_destroy_render_target(driver,target), "destroy SSAO receiver");
	require(onix_gpu_destroy_texture(driver,texture), "destroy SSAO texture");
	printf("Vulkan SSAO depth prepass, contact occlusion, exclusions, flat surface and screen direction passed\n");
}

static void test_screen_reflections(OnixGPUDriver *driver) {
	uint32_t depth_color,scene_color,out_color;
	uint32_t depth=onix_gpu_create_render_target(driver,64,64,0,1,&depth_color);
	uint32_t scene=onix_gpu_create_render_target(driver,128,64,0,1,&scene_color);
	uint32_t target=onix_gpu_create_render_target(driver,128,96,0,1,&out_color);
	const uint8_t white[]={255,255,255,255},black[]={0,0,0,255};
	uint32_t texture=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,white,4,0,0);
	uint32_t ao_map=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,black,4,0,0);
	require(depth && scene && target && texture && ao_map,"create SSR resources");
	OnixGPUVertex3D vertices[]={
		{.position={-1,-1,-2},.normal={.8f,0,.6f}}, {.position={.4f,-1,-2},.normal={.8f,0,.6f}},
		{.position={.4f,1,-2},.normal={.8f,0,.6f}}, {.position={-1,1,-2},.normal={.8f,0,.6f}},
		{.position={.5f,-.5f,-2.2f},.normal={0,0,1}}, {.position={1.5f,-.5f,-2.2f},.normal={0,0,1}},
		{.position={1.5f,.5f,-2.2f},.normal={0,0,1}}, {.position={.5f,.5f,-2.2f},.normal={0,0,1}},
	};
	uint32_t indices[]={0,1,2,0,2,3,4,5,6,4,6,7},deformation[16]={0};
	OnixGPUInstance3D instances[]={
		{.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={1,1,1,1},.emissive_metallic={.05f,.05f,.05f,1},.material={.045f,1.5f,.5f,0},.uv_transform={1,1,0,0},.custom={{1,1,1,0},{1,1,1,.045f},{0,0,0,0},{0,1,0,1}}},
		{.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={1,1,1,1},.emissive_metallic={4,0,0,0},.material={.8f,1.5f,.5f,0},.uv_transform={1,1,0,0},.custom={{1,1,1,0},{1,1,1,0},{0,0,0,0},{0,1,0,1}}},
	};
	OnixNativeMeshDraw3D draws[]={
		{.texture=texture,.index_count=6,.instance_count=1},
		{.texture=texture,.first_index=6,.first_instance=1,.index_count=6,.instance_count=1},
	};
	const float perspective[16]={[0]=1,[5]=1,[11]=-1,[14]=.1f};
	const float inverse[16]={[0]=1,[5]=1,[11]=10,[14]=-1};
	const float orthographic[16]={[0]=.5f,[5]=.5f,[10]=.2f,[14]=1,[15]=1};
	const float inverse_ortho[16]={[0]=2,[5]=2,[10]=5,[14]=-5,[15]=1};
	const float clear[]={0,0,0,0};
	int values[13]={0};
	const float iors[]={1,1.33f,1.5f,2.42f,0};
	for(int test=0;test<25;test++) {
		int scenario=test<8?test:1;
		OnixGPUMeshPush push={0};memcpy(push.view_projection,scenario==6?orthographic:perspective,64);
		float lighting[ONIX_GPU_MESH_LIGHTING_BYTES/sizeof(float)]={[19]=1,[26]=-1,[296]=1,[302]=-1};
		memcpy(lighting+432,push.view_projection,64);memcpy(lighting+880,scenario==6?inverse_ortho:inverse,64);
		lighting[688]=64;lighting[689]=.1f;lighting[690]=1;
		lighting[752]=4;lighting[753]=.1f;lighting[754]=scenario==0?0:1;lighting[755]=.6f;
		instances[0].material[3]=scenario==2?4194304:scenario==4?16:0;
		instances[0].emissive_metallic[3]=scenario==3?0:1;
		instances[0].custom[0][2]=scenario==3?0:1;instances[0].custom[0][3]=scenario==3?1:0;
		instances[0].material[1]=1.5f;
		if(test>=8 && test<13) {
			instances[0].material[1]=iors[test-8];instances[0].emissive_metallic[3]=0;
		}
		for(int i=0;i<2;i++) {
			memset(instances[i].model,0,64);instances[i].model[0]=instances[i].model[5]=instances[i].model[10]=instances[i].model[15]=1;
			draws[i].extra_textures[0]=scenario==4?ao_map:0;draws[i].extra_textures[5]=draws[i].extra_textures[6]=0;
		}
		if(scenario==5) {
			// Rotate scene and camera together by 90 degrees around Y.
			for(int i=0;i<2;i++) {instances[i].model[0]=instances[i].model[10]=0;instances[i].model[2]=-1;instances[i].model[8]=1;}
			memset(push.view_projection,0,64);push.view_projection[8]=-1;push.view_projection[5]=1;push.view_projection[3]=-1;push.view_projection[14]=.1f;
			memcpy(lighting+432,push.view_projection,64);
			memset(lighting+880,0,64);lighting[882]=-1;lighting[885]=1;lighting[891]=10;lighting[892]=-1;
			lighting[300]=-1;lighting[302]=0;
		}
		if(scenario==6) lighting[19]=-1;
		require(onix_gpu_begin_frame(driver,clear),"begin SSR frame");
		lighting[297]=5;
		require(onix_gpu_begin_render_target(driver,depth,clear,0,1,1),"begin SSR depth");onix_gpu_set_viewport(driver,0,0,64,64);
		require(onix_gpu_draw_meshes(driver,&push,vertices,8,indices,12,instances,2,deformation,16,draws,2,lighting),"draw SSR depth");require(onix_gpu_end_render_target(driver),"end SSR depth");
		lighting[297]=3;
		require(onix_gpu_begin_render_target(driver,scene,clear,0,1,1),"begin SSR capture");onix_gpu_set_viewport(driver,0,0,64,64);
		require(onix_gpu_draw_meshes(driver,&push,vertices,8,indices,12,instances,2,deformation,16,draws,2,lighting),"draw SSR capture");require(onix_gpu_end_render_target(driver),"end SSR capture");
		lighting[297]=6;
		require(onix_gpu_begin_render_target(driver,scene,clear,0,0,1),"begin SSR AO tile");onix_gpu_set_viewport(driver,64,0,64,64);
		require(onix_gpu_draw_meshes(driver,&push,vertices,8,indices,12,instances,2,deformation,16,draws,2,lighting),"draw SSR AO tile");require(onix_gpu_end_render_target(driver),"end SSR AO tile");
		lighting[297]=4;
		for(int i=0;i<2;i++) {draws[i].extra_textures[5]=scene_color;draws[i].extra_textures[6]=depth_color;}
		int offset_x=scenario==7?16:0,offset_y=scenario==7?8:0;
		require(onix_gpu_begin_render_target(driver,target,clear,0,1,1),"begin SSR resolve");onix_gpu_set_viewport(driver,offset_x,offset_y,64,64);
		require(onix_gpu_draw_meshes(driver,&push,vertices,8,indices,12,instances,2,deformation,16,draws,2,lighting),"draw SSR resolve");require(onix_gpu_end_render_target(driver),"end SSR resolve");
		require(onix_gpu_present(driver),"present SSR frame");
		uint8_t pixel[4];require(onix_gpu_read_render_target(driver,target,32+offset_x,32+offset_y,1,1,pixel,4),"read SSR pixel");
		int red=(driver->swapchain_format==VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format==VK_FORMAT_B8G8R8A8_SRGB) ? pixel[2] : pixel[0];
		if(test<13) values[test]=red;
		// Warm every in-flight frame before measuring all four passes.
		if(test==20) host_allocations=resource_allocations=0;
	}
	if(values[0]>80 || values[1]<220 || abs(values[2]-values[0])>1 || values[3]<=values[0]+10 || abs(values[4]-values[0])>1 || abs(values[5]-values[1])>1 || values[6]<220 || abs(values[7]-values[1])>1 || host_allocations || resource_allocations) {
		fprintf(stderr,"Vulkan SSR: base=%d hit=%d excluded=%d coat=%d AO=%d rotated=%d ortho=%d viewport=%d; heap=%zu resources=%zu\n",values[0],values[1],values[2],values[3],values[4],values[5],values[6],values[7],host_allocations,resource_allocations);exit(1);
	}
	for(int i=8;i<12;i++) require(values[i]<values[i+1],"SSR responds to dielectric IOR");
	require(abs(values[12]-values[1])<=1,"infinite-IOR SSR matches white metal");
	require(onix_gpu_destroy_render_target(driver,depth),"destroy SSR depth");require(onix_gpu_destroy_render_target(driver,scene),"destroy SSR scene");require(onix_gpu_destroy_render_target(driver,target),"destroy SSR output");
	require(onix_gpu_destroy_texture(driver,texture),"destroy SSR texture");require(onix_gpu_destroy_texture(driver,ao_map),"destroy SSR AO map");
	printf("Vulkan SSR: base=%d hit=%d coat=%d AO=%d rotated=%d ortho=%d; capture/AO preservation and 4 stable frames heap=0 resources=0 passed\n",values[0],values[1],values[3],values[4],values[5],values[6]);
	printf("Vulkan SSR IOR: air=%d water=%d default=%d diamond=%d infinite=%d\n",values[8],values[9],values[10],values[11],values[12]);
}

static void draw_mesh(OnixGPUDriver *driver, uint32_t texture, uint32_t picking_target) {
	OnixGPUMeshPush push = {.view_projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
	OnixGPUVertex3D vertices[] = {
		{.position = {-1, -1, 0.5f}, .normal = {0, 0, 1}},
		{.position = {1, -1, 0.5f}, .normal = {0, 0, 1}},
		{.position = {0, 1, 0.5f}, .normal = {0, 0, 1}},
	};
	uint32_t indices[] = {0, 1, 2};
	OnixGPUInstance3D instance = {.model = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}, .base_color = {1, 1, 1, 1}};
	uint32_t deformation[16] = {0};
	OnixNativeMeshDraw3D draw = {.texture = texture, .index_count = 3, .instance_count = 1};
	if (picking_target == 0) {
		require(onix_gpu_draw_meshes(driver, &push, vertices, 3, indices, 3, &instance, 1, deformation, 16, &draw, 1, NULL), "draw mesh");
	} else {
		const float clear[] = {0, 0, 0, 0};
		uint32_t identifier = 1234567;
		memcpy(&instance.custom[3][3], &identifier, sizeof(identifier));
		require(onix_gpu_begin_render_target(driver, picking_target, clear, 0, 1, 1), "begin picking");
		onix_gpu_set_viewport(driver, 0, 0, 16, 16);
		require(onix_gpu_draw_mesh_picking(driver, &push, vertices, 3, indices, 3, &instance, 1, &draw, 1), "draw picking");
		require(onix_gpu_end_render_target(driver), "end picking");
	}
}

static float read_surface_depth(OnixGPUDriver *driver) {
	require(vkDeviceWaitIdle(driver->device) == VK_SUCCESS, "wait for surface depth");
	VkBuffer buffer;
	VkDeviceMemory memory;
	VkMemoryPropertyFlags host = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
	require(onix_vk_create_buffer(driver, sizeof(float), VK_BUFFER_USAGE_TRANSFER_DST_BIT, host, &buffer, &memory), "create depth readback");
	VkCommandBuffer command;
	VkCommandBufferAllocateInfo allocate = {
		.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
		.commandPool = driver->command_pool,
		.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY,
		.commandBufferCount = 1,
	};
	require(vkAllocateCommandBuffers(driver->device, &allocate, &command) == VK_SUCCESS, "allocate depth readback command");
	VkCommandBufferBeginInfo begin = {.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO};
	require(vkBeginCommandBuffer(command, &begin) == VK_SUCCESS, "begin depth readback");
	VkImageMemoryBarrier barrier = {
		.sType = VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,
		.srcAccessMask = VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT,
		.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT,
		.oldLayout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
		.newLayout = VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,
		.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED,
		.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED,
		.image = driver->depth_images[driver->image_index],
		.subresourceRange = {VK_IMAGE_ASPECT_DEPTH_BIT, 0, 1, 0, 1},
	};
	vkCmdPipelineBarrier(command, VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT | VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, 0, 0, NULL, 0, NULL, 1, &barrier);
	VkBufferImageCopy copy = {
		.imageSubresource = {VK_IMAGE_ASPECT_DEPTH_BIT, 0, 0, 1},
		.imageOffset = {(int32_t)driver->extent.width / 2, (int32_t)driver->extent.height / 2, 0},
		.imageExtent = {1, 1, 1},
	};
	vkCmdCopyImageToBuffer(command, barrier.image, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, buffer, 1, &copy);
	barrier.srcAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
	barrier.dstAccessMask = VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT;
	barrier.oldLayout = VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL;
	barrier.newLayout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL;
	vkCmdPipelineBarrier(command, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT | VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT, 0, 0, NULL, 0, NULL, 1, &barrier);
	require(vkEndCommandBuffer(command) == VK_SUCCESS, "end depth readback");
	VkSubmitInfo submit = {.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO, .commandBufferCount = 1, .pCommandBuffers = &command};
	require(vkQueueSubmit(driver->queue, 1, &submit, VK_NULL_HANDLE) == VK_SUCCESS && vkQueueWaitIdle(driver->queue) == VK_SUCCESS, "submit depth readback");
	void *mapped;
	require(vkMapMemory(driver->device, memory, 0, sizeof(float), 0, &mapped) == VK_SUCCESS, "map depth readback");
	float depth;
	memcpy(&depth, mapped, sizeof(depth));
	vkUnmapMemory(driver->device, memory);
	vkFreeCommandBuffers(driver->device, driver->command_pool, 1, &command);
	onix_vk_destroy_buffer(driver, &buffer, &memory);
	return depth;
}

static void test_surface_2d_laziness(OnixGPUDriver *driver) {
	const uint8_t white[] = {255, 255, 255, 255};
	const float clear[] = {0, 0, 0, 1};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, sizeof(white), 0, 0);
	OnixGPUSpritePush push = {.projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
	OnixGPUInstance instance = {.model = {2, 0, -1, 0, 2, -1}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, 1}};
	OnixNativeSpriteDraw draw = {.texture = texture, .instance_count = 1};
	onix_gpu_set_corner_radius(driver, 8);
	for (int frame = 0; frame < 6; frame++) {
		require(onix_gpu_begin_frame(driver, clear) && onix_gpu_draw_sprites(driver, &push, &instance, 1, &draw, 1) && onix_gpu_present(driver), "pure 2D surface frame");
	}
	require(driver->color_only_pipelines[3] != VK_NULL_HANDLE && driver->corner_mask_pipeline == VK_NULL_HANDLE,
		"rounded 2D surface uses the color-only corner mask pipeline");
	onix_gpu_set_corner_radius(driver, 0);
	require(onix_gpu_resize(driver, 64, 64), "resize pure 2D surface");
	for (uint32_t index = 0; index < driver->image_count; index++) {
		require(!driver->depth_images[index] && !driver->depth_views[index] && !driver->depth_memories[index] && !driver->depth_framebuffers[index], "2D frames and resize allocate no surface depth");
	}
	require(!driver->mesh_picking_pipeline && !driver->mesh_builtin[0].opaque_surface && !driver->mesh_builtin[1].opaque_surface, "2D creates no mesh pipelines");
	require(onix_gpu_destroy_texture(driver, texture), "destroy pure 2D texture");
	puts("Pure 2D startup/frames/resize: no swapchain depth allocation or mesh pipeline");
}

static void test_surface_depth(OnixGPUDriver *driver) {
	const uint8_t white[] = {255, 255, 255, 255};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, sizeof(white), 0, 0);
	uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, 0, &color);
	require(texture && target, "create surface depth resources");
	const float clear[] = {0, 0, 0, 1};
	for (int frame = 0; frame < 6; frame++) {
		float expected = frame % 2 == 0 ? .5f : .25f;
		require(onix_gpu_begin_frame(driver, clear), "begin surface depth frame");
		onix_gpu_set_viewport(driver, 0, 0, (int32_t)driver->extent.width, (int32_t)driver->extent.height);
		OnixGPUSpritePush sprite_push = {.projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
		OnixGPUInstance sprite = {.model = {2, 0, -1, 0, 2, -1}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {0, 1, 0, 1}};
		OnixNativeSpriteDraw sprite_draw = {.texture = texture, .instance_count = 1};
		require(onix_gpu_draw_sprites(driver, &sprite_push, &sprite, 1, &sprite_draw, 1), "draw 2D before upgrading surface depth");
		OnixGPUMeshPush push = {.view_projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
		OnixGPUVertex3D vertices[] = {
			{.position = {-1, -1, expected}, .normal = {0, 0, 1}},
			{.position = {1, -1, expected}, .normal = {0, 0, 1}},
			{.position = {0, 1, expected}, .normal = {0, 0, 1}},
		};
		uint32_t indices[] = {0, 1, 2}, deformation[16] = {0};
		OnixGPUInstance3D instance = {.model = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}, .base_color = {1, 1, 1, 1}, .material = {[3] = 256}};
		OnixNativeMeshDraw3D draw = {.texture = texture, .index_count = 3, .instance_count = 1};
		require(onix_gpu_draw_meshes(driver, &push, vertices, 3, indices, 3, &instance, 1, deformation, 16, &draw, 1, NULL), "draw near surface mesh");
		// Interrupt the surface pass, then draw behind the first mesh.
		draw_target(driver, target, texture);
		onix_gpu_set_viewport(driver, 0, 0, (int32_t)driver->extent.width, (int32_t)driver->extent.height);
		for (uint32_t index = 0; index < 3; index++) vertices[index].position[2] = expected / 2;
		require(onix_gpu_draw_meshes(driver, &push, vertices, 3, indices, 3, &instance, 1, deformation, 16, &draw, 1, NULL), "draw far surface mesh");
		require(onix_gpu_present(driver), "present surface depth frame");
		float depth = read_surface_depth(driver);
		if (depth != expected) {
			fprintf(stderr, "surface depth frame %d: got %g, expected %g\n", frame, depth, expected);
			exit(1);
		}
	}
	require(onix_gpu_destroy_render_target(driver, target), "destroy surface depth target");
	require(onix_gpu_destroy_texture(driver, texture), "destroy surface depth texture");
	printf("Surface depth clears on each 3D frame and survives offscreen passes\n");
}

static void draw_multi_texture(OnixGPUDriver *driver, uint32_t target, const uint32_t textures[4], uint32_t shader, int frame) {
	const float clear[] = {0, 0, 0, 0};
	require(onix_gpu_begin_render_target(driver, target, clear, 0, 1, 1), "begin multi-texture target");
	onix_gpu_set_viewport(driver, 0, 0, 16, 16);
	OnixGPUSpritePush push = {.projection = {[0] = 0.125f, [5] = 0.125f, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
	OnixGPUInstance instances[8] = {0};
	OnixNativeSpriteDraw draws[3] = {{.instance_count = 6}, {.shader = shader, .texture = textures[2], .first_instance = 6, .instance_count = 1}, {.texture = textures[0], .first_instance = 7, .instance_count = 1}};
	// Change the binding order every frame, including after the allocation reset.
	uint32_t rotation = (uint32_t)frame % 4;
	draws[0].texture = textures[rotation];
	for (uint32_t slot = 1; slot < 4; slot++) draws[0].additional_textures[slot - 1] = textures[(slot + rotation) % 4];
	for (uint32_t index = 0; index < 8; index++) {
		instances[index] = (OnixGPUInstance){.model = {4, 0, (float)(index % 4) * 4, 0, 8, index < 4 ? 0 : 8}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, 1}};
		if (index < 6) instances[index].texture_slot = (float)((index % 4 + 4 - rotation) % 4);
	}
	instances[3].color[1] = 0;
	instances[5].model[2] = 0; // Half-alpha green over red tests painter order.
	require(onix_gpu_draw_sprites(driver, &push, instances, 8, draws, 3), "draw multi-texture and custom sprites");
	require(onix_gpu_end_render_target(driver), "end multi-texture target");
}

static void check_multi_texture(OnixGPUDriver *driver, uint32_t target) {
	const uint8_t expected[8][4] = {{255, 0, 0, 255}, {0, 128, 0, 128}, {0, 0, 255, 255}, {128, 0, 128, 128}, {127, 128, 0, 255}, {0, 0, 0, 0}, {0, 0, 255, 255}, {255, 0, 0, 255}};
	for (int index = 0; index < 8; index++) {
		uint8_t pixel[4];
		// Vulkan's built-in vertex shader flips Y; target coordinates are top-down.
		require(onix_gpu_read_render_target(driver, target, (uint32_t)(index % 4) * 4 + 2, index < 4 ? 12 : 4, 1, 1, pixel, sizeof(pixel)), "read multi-texture pixel");
		if (driver->swapchain_format == VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format == VK_FORMAT_B8G8R8A8_SRGB) {
			uint8_t blue = pixel[0]; pixel[0] = pixel[2]; pixel[2] = blue;
		}
		for (int channel = 0; channel < 4; channel++) {
			if (abs((int)pixel[channel] - expected[index][channel]) > 1) {
				fprintf(stderr, "multi-texture pixel %d = %u,%u,%u,%u, expected %u,%u,%u,%u\n", index, pixel[0], pixel[1], pixel[2], pixel[3], expected[index][0], expected[index][1], expected[index][2], expected[index][3]);
				exit(1);
			}
		}
	}
}

static void check_pixel(OnixGPUDriver *driver, uint32_t target, uint8_t red, uint8_t green, int frame) {
	uint8_t pixel[4];
	require(onix_gpu_read_render_target(driver, target, 8, 8, 1, 1, pixel, sizeof(pixel)), "read target");
	if (driver->swapchain_format == VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format == VK_FORMAT_B8G8R8A8_SRGB) {
		uint8_t blue = pixel[0];
		pixel[0] = pixel[2];
		pixel[2] = blue;
	}
	if (pixel[0] != red || pixel[1] != green || pixel[2] != 0 || pixel[3] != 255) {
		fprintf(stderr, "frame %d pixel = %u,%u,%u,%u, expected %u,%u,0,255\n", frame, pixel[0], pixel[1], pixel[2], pixel[3], red, green);
		exit(1);
	}
}

static void test_external_queue(OnixGPUDriver *driver, uint32_t texture) {
	OnixVkTexture *value = &driver->textures[texture - 1];
	int external = value->external;
	value->external = 1;
	value->external_ready = 1;
	value->external_initialized = 1;
	driver->pending_external[driver->pending_external_count++] = texture - 1;
	driver->textures[texture - 1].pending_external_index = driver->pending_external_count;
	onix_vk_initialize_external_textures(driver);
	require(driver->pending_external_count == 0, "drain initialized external queue");
	value->external = external;
	value->external_ready = 0;
	value->external_initialized = 0;
}

static void test_texture_addressing(OnixGPUDriver *driver) {
 const uint8_t pixels[]={255,0,0,255,0,255,0,255};
 const float clear[]={0,0,0,1};
 uint32_t color,target=onix_gpu_create_render_target(driver,16,16,0,0,&color);
 require(target!=0,"create sampler target");
 for(uint32_t mode=0;mode<3;mode++) {
  uint32_t texture=onix_gpu_create_texture_with_format(driver,2,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,pixels,sizeof(pixels),mode,0);
  require(texture!=0,"create addressed texture");
  for(int point=0;point<2;point++) {
   require(onix_gpu_begin_frame(driver,clear),"begin sampler frame");
   require(onix_gpu_begin_render_target(driver,target,clear,0,1,1),"begin sampler target");
   onix_gpu_set_viewport(driver,0,0,16,16);
   OnixGPUSpritePush push={.projection={[0]=.125f,[5]=.125f,[10]=1,[12]=-1,[13]=-1,[15]=1}};
   OnixGPUInstance instance={.model={16,0,0,0,16,0},.texture_model={0,0,1.25f+point*.5f,0,0,.5f},.color={1,1,1,1}};
   OnixNativeSpriteDraw draw={.texture=texture,.instance_count=1};
   require(onix_gpu_draw_sprites(driver,&push,&instance,1,&draw,1),"draw addressed texture");
   require(onix_gpu_end_render_target(driver),"end sampler target");
   require(onix_gpu_present(driver),"present sampler frame");
   int red=(mode==1 && point==0)||(mode==2 && point==1);
   check_pixel(driver,target,red?255:0,red?0:255,point);
  }
  require(onix_gpu_destroy_texture(driver,texture),"destroy addressed texture");
 }
 require(onix_gpu_destroy_render_target(driver,target),"destroy sampler target");
}

static void test_texture_update_batch(OnixGPUDriver *driver) {
	uint8_t rgba[12], mask[3] = {255, 255, 255};
	fill_pixels(rgba, sizeof(rgba), 255, 0);
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 3, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, rgba, sizeof(rgba), 0, 0);
	uint32_t r8 = onix_gpu_create_texture_with_format(driver, 3, 1, 1, ONIX_TEXTURE_FORMAT_R8, 0, mask, sizeof(mask), 0, 0);
	uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, 0, &color);
	uint32_t mask_target = onix_gpu_create_render_target(driver, 16, 16, 0, 0, &color);
	require(texture && r8 && target && mask_target, "create batch update resources");
	OnixGPUTextureUpdate updates[] = {
		{.texture = r8, .width = 3, .height = 1, .data_length = 3},
		{.texture = texture, .width = 3, .height = 1, .data_offset = 4, .data_length = 12},
	};
	const float clear[] = {0, 0, 0, 1};
	size_t warm_host = 0, warm_resources = 0;
	for (int frame = 0; frame < 8; frame++) {
		if (frame == 4) {
			warm_host = host_allocations;
			warm_resources = resource_allocations;
		}
		if (frame % 2 != 0) require(onix_gpu_begin_frame(driver, clear), "begin batch frame before upload");
		uint32_t pending = driver->upload_count;
		VkDeviceSize reserved = onix_vk_staging_bytes_reserved(&driver->frames[driver->frame_index]);
		updates[1].width = 4;
		require(onix_gpu_prepare_texture_updates(driver, updates, 2, 16) == NULL, "reject invalid batch region");
		require(driver->upload_count == pending && onix_vk_staging_bytes_reserved(&driver->frames[driver->frame_index]) == reserved, "invalid batch leaves uploads unchanged");
		updates[1].width = 3;
		updates[1].data_offset = 3;
		require(onix_gpu_prepare_texture_updates(driver, updates, 2, 16) == NULL, "reject unaligned batch offset");
		require(driver->upload_count == pending, "unaligned batch leaves uploads unchanged");
		updates[1].data_offset = 4;
		uint8_t red = frame % 2 == 0 ? 255 : 0, green = frame % 2 == 0 ? 0 : 255;
		fill_pixels(rgba, sizeof(rgba), red, green);
		uint8_t *staging = onix_gpu_prepare_texture_updates(driver, updates, 2, 16);
		require(staging != NULL, "prepare mapped batch update");
		memcpy(staging, mask, sizeof(mask));
		memcpy(staging + 4, rgba, sizeof(rgba));
		require(onix_gpu_commit_texture_updates(driver), "commit mapped batch update");
		memset(rgba, 0, sizeof(rgba));
		if (frame % 2 == 0) require(onix_gpu_begin_frame(driver, clear), "begin batch frame after upload");
		draw_target(driver, target, texture);
		draw_target(driver, mask_target, r8);
		require(onix_gpu_present(driver), "present batch update");
		check_pixel(driver, target, red, green, frame);
		uint8_t pixel[4];
		require(onix_gpu_read_render_target(driver, mask_target, 8, 8, 1, 1, pixel, sizeof(pixel)), "read batch R8 pixel");
		require(pixel[0] == 255 && pixel[1] == 255 && pixel[2] == 255 && pixel[3] == 255, "batch R8 alignment preserves pixels");
	}
	require(host_allocations == warm_host && resource_allocations == warm_resources, "mapped batches allocate nothing after warmup");
	printf("Mapped RGBA8/R8 batches: aligned pixels, invalid-batch isolation and zero steady-state allocations passed\n");
	require(onix_gpu_destroy_render_target(driver, target), "destroy batch target");
	require(onix_gpu_destroy_render_target(driver, mask_target), "destroy batch mask target");
	require(onix_gpu_destroy_texture(driver, texture), "destroy batch texture");
	require(onix_gpu_destroy_texture(driver, r8), "destroy batch mask");
}


static void test_fxaa(OnixGPUDriver *driver) {
 uint32_t source_color,result_color;
 uint32_t source=onix_gpu_create_render_target(driver,32,32,0,1,&source_color);
 uint32_t result=onix_gpu_create_render_target(driver,48,48,0,1,&result_color);
 const uint8_t white[]={255,255,255,255};
 uint32_t texture=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,white,4,0,0);
 require(source && result && texture,"create FXAA resources");
 OnixGPUVertex3D vertices[]={
  {.position={-1,-1,1},.normal={0,0,1},.uv={0,0}}, {.position={1,-1,1},.normal={0,0,1},.uv={1,0}},
  {.position={1,1,1},.normal={0,0,1},.uv={1,1}}, {.position={-1,1,1},.normal={0,0,1},.uv={0,1}},
  {.position={-1,0,1},.normal={0,0,1}}, {.position={1,0,1},.normal={0,0,1}},
 };
 uint32_t indices[]={0,1,2,0,2,3,0,1,5,0,5,4,4,5,2,4,2,3},deformation[16]={0};
 OnixGPUInstance3D instances[2]={
  {.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={1,1,1,1},.material={.8f,1.5f,.5f,768},.uv_transform={1,1,0,0}},
  {.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={0,1,0,.5f},.material={.8f,1.5f,.5f,770},.uv_transform={1,1,0,0}},
 };
 OnixGPUInstance3D composite={.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={1,1,1,1},.material={0,1.5f,0,512},.uv_transform={1,1,0,0}};
 OnixGPUMeshPush push={.view_projection={[0]=1,[5]=1,[10]=1,[15]=1}};
 OnixNativeMeshDraw3D draws[2]={{.texture=texture,.index_count=6,.instance_count=1},{.texture=texture,.first_index=12,.index_count=6,.first_instance=1,.instance_count=1,.alpha_mode=2}};
 OnixNativeMeshDraw3D filter={.texture=source_color,.index_count=6,.instance_count=1,.alpha_mode=2};
 float lighting[ONIX_GPU_MESH_LIGHTING_BYTES/sizeof(float)]={0};
 uint8_t rgba[48*48*4];
 int red=driver->swapchain_format==VK_FORMAT_B8G8R8A8_UNORM?2:0,smoothed=0,alpha_smoothed=0;
 for(int test=0;test<4;test++) {
  float clear[]={0,0,0,test<2?1:0};
  instances[0].base_color[0]=test==0?.25f:test==2?0:1;
  instances[0].base_color[1]=test==0?.5f:test==2 || test==3?0:1;
  instances[0].base_color[2]=test==0?.75f:test==2 || test==3?0:1;
  instances[0].base_color[3]=test==3?.5f:1;
  instances[0].material[3]=test==3?770:768;
  draws[0].first_index=test==3?6:0;draws[0].index_count=test==1 || test==2?3:6;draws[0].alpha_mode=test==3?2:0;
  for(int frame=0;frame<12;frame++) {
   require(onix_gpu_begin_frame(driver,clear),"begin FXAA frame");
   require(onix_gpu_begin_render_target(driver,source,clear,0,1,1),"begin FXAA source");onix_gpu_set_viewport(driver,0,0,32,32);
   lighting[296]=1;lighting[297]=0;
   require(onix_gpu_draw_meshes(driver,&push,vertices,6,indices,18,instances,2,deformation,16,draws,test==3?2:1,lighting),"draw FXAA source geometry");require(onix_gpu_end_render_target(driver),"end FXAA source");
   const float zero[]={0,0,0,0};
   require(onix_gpu_begin_render_target(driver,result,zero,0,1,1),"begin FXAA composite");onix_gpu_set_viewport(driver,8,8,32,32);
   lighting[296]=999;lighting[297]=7;
   require(onix_gpu_draw_meshes(driver,&push,vertices,6,indices,18,&composite,1,deformation,16,&filter,1,lighting),"draw FXAA composite");require(onix_gpu_end_render_target(driver),"end FXAA composite");
   require(onix_gpu_present(driver),"present FXAA frame");
   require(onix_gpu_read_render_target(driver,result,0,0,48,48,rgba,sizeof(rgba)),"read FXAA result");
   if(test==0 && frame==7) host_allocations=resource_allocations=0;
   if(test==0 && frame>=8) require(host_allocations==0 && resource_allocations==0,"FXAA steady frames allocated");
  }
  for(int y=8;y<40;y++) for(int x=8;x<40;x++) {
   int i=(y*48+x)*4;
   if(test==0 && !(abs(rgba[i+red]-137)<=1 && abs(rgba[i+1]-188)<=1 && abs(rgba[i+2-red]-225)<=1 && rgba[i+3]==255)) { fprintf(stderr,"FXAA flat (%d,%d): %d/%d/%d/%d redIndex=%d\n",x,y,rgba[i],rgba[i+1],rgba[i+2],rgba[i+3],red);exit(1); }
   if(test==1 && rgba[i+red]>5 && rgba[i+red]<250) smoothed++;
   if(test==2) {
    require(rgba[i]==0 && rgba[i+1]==0 && rgba[i+2]==0,"FXAA transparency gained color");
    if(rgba[i+3]>5 && rgba[i+3]<250) alpha_smoothed++;
   }
  }
  require(rgba[3]==0 && rgba[(47*48+47)*4+3]==0,"FXAA leaked outside its viewport");
  if(test==3) {
   int top=(12*48+12)*4,bottom=(36*48+12)*4;
   require(rgba[top+red]==0 && abs(rgba[top+1]-128)<=1 && abs(rgba[top+3]-128)<=1 && abs(rgba[bottom+red]-128)<=1 && rgba[bottom+1]==0,"FXAA orientation or premultiplied alpha");
  }
 }
 require(smoothed>0 && alpha_smoothed>0,"FXAA did not smooth diagonal or transparent edges");
 require(onix_gpu_destroy_render_target(driver,source),"destroy FXAA source");require(onix_gpu_destroy_render_target(driver,result),"destroy FXAA result");require(onix_gpu_destroy_texture(driver,texture),"destroy FXAA texture");
 printf("Vulkan FXAA geometry edges (%d pixels), transparency (%d pixels), flat color, orientation, viewport and zero steady C/resource allocations passed\n",smoothed,alpha_smoothed);
}


static int camera_display_byte(double radiance,int curve) {
 double value=curve?fmin(1.0,fmax(0.0,((radiance/0.6)*(radiance/0.6+0.0245786)-0.000090537)/((radiance/0.6)*(0.983729*radiance/0.6+0.4329510)+0.238081))):radiance/(radiance+1);
 return (int)lround((value<=.0031308?12.92*value:1.055*pow(value,1.0/2.4)-.055)*255);
}
static void test_camera_color(OnixGPUDriver *driver) {
 uint32_t color,target=onix_gpu_create_render_target(driver,16,16,0,1,&color);
 const uint8_t white[]={255,255,255,255};
 uint32_t texture=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,white,4,0,0);
 require(target && texture,"create camera color resources");
 OnixGPUVertex3D vertices[]={
  {.position={-1,-1,-4},.normal={0,0,1}}, {.position={1,-1,-4},.normal={0,0,1}},
  {.position={1,1,-4},.normal={0,0,1}}, {.position={-1,1,-4},.normal={0,0,1}},
 };
 uint32_t indices[]={0,1,2,0,2,3},deformation[16]={0};
 OnixGPUInstance3D instance={.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={0,0,0,1},.material={.8f,1.5f,.5f,512+4096},.uv_transform={1,1,0,0},.custom={{1,1,0,0},{1,1,1,0},{0,0,0,0},{0,1,0,1}}};
 OnixNativeMeshDraw3D draw={.texture=texture,.index_count=6,.instance_count=1};
 OnixGPUMeshPush push={.view_projection={[0]=1,[5]=1,[14]=.5f,[15]=1}};
 const float clear[]={0,0,0,0},radiance[]={0,.001f,.18f,1,4,100,FLT_MAX},distances[]={1,2,4,6,10};
 int red=(driver->swapchain_format==VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format==VK_FORMAT_B8G8R8A8_SRGB)?2:0;
 for(int test=0;test<32;test++) {
  float lighting[ONIX_GPU_MESH_LIGHTING_BYTES/sizeof(float)]={[16]=.0625f,[17]=-.0625f,[19]=1,[296]=1,[302]=-1};
  double expected_radiance=0;
  int curve=test<14?test/7:0,expected_alpha=255;
  instance.material[3]=512+4096;instance.base_color[3]=1;draw.alpha_mode=0;
  for(int i=0;i<3;i++) instance.emissive_metallic[i]=test<14?radiance[test%7]:i==2?1:0;
  if(test<14) expected_radiance=radiance[test%7];
  else {
   int mode=test<19?1:2,index=test<19?test-14:test<24?test-19:2;
   float distance=test<24?distances[index]:4;
   for(int i=0;i<4;i++) vertices[i].position[2]=-distance;
   lighting[819]=1;lighting[831]=2;lighting[835]=6;lighting[839]=.25f;lighting[843]=(float)mode;
   double delta=fmax(distance-2,0);
   expected_radiance=mode==1?fmin(delta/4,1):1-exp(-delta*delta*.0625);
   if(test==24) {lighting[843]=1;lighting[19]=-1;instance.model[12]=100;expected_radiance=.5;}
   if(test==25 || test==26) {
    lighting[843]=1;for(int i=0;i<4;i++) vertices[i].position[2]=-10;
    expected_radiance=1;instance.base_color[3]=.5f;instance.material[3]=(test==25?256:4096)+512+2;draw.alpha_mode=2;expected_alpha=128;
   }
   if(test==27) {lighting[297]=3;lighting[296]=999;curve=1;expected_radiance=0;expected_alpha=128;}
   if(test==28) {
    lighting[843]=1;lighting[847]=1;curve=1;expected_radiance=0;
    instance.material[3]=512+2048+16384;
    for(int i=0;i<3;i++) {instance.custom[i][0]=instance.custom[i][1]=0;instance.custom[i][2]=1;}
   }
   if(test>=29) {lighting[847]=1;curve=1;}
  }
  lighting[847]=(float)curve;
  // Keep the shifted orthographic sample visible while preserving world X.
  push.view_projection[12]=test==24?-100:0;
  require(onix_gpu_begin_frame(driver,clear),"begin camera color frame");
  require(onix_gpu_begin_render_target(driver,target,clear,0,1,1),"begin camera color target");onix_gpu_set_viewport(driver,0,0,16,16);
  require(onix_gpu_draw_meshes(driver,&push,vertices,4,indices,6,&instance,1,deformation,16,&draw,1,lighting),"draw fog/curve");require(onix_gpu_end_render_target(driver),"end camera color target");require(onix_gpu_present(driver),"present camera color frame");
  uint8_t pixel[4];require(onix_gpu_read_render_target(driver,target,8,8,1,1,pixel,4),"read fog/curve pixel");
  int expected=camera_display_byte(expected_radiance,curve);
  if(test>=28) {
   const float fogged[]={(float)expected_radiance,0,1-(float)expected_radiance};
   unsigned char expected_rgb[3];aces_display_reference(fogged,expected_rgb);
   expected=expected_rgb[0];
   require(abs(pixel[1]-expected_rgb[1])<=1 && abs(pixel[2-red]-expected_rgb[2])<=1,"ACES fog/sky color differs from CPU reference");
  }
  if(test==25 || test==26) expected=(int)lround(expected*.5);
  if(abs(pixel[red]-expected)>1 || abs(pixel[3]-expected_alpha)>1) {fprintf(stderr,"camera color %d: %d/%d expected %d/%d\n",test,pixel[red],pixel[3],expected,expected_alpha);exit(1);}
  if(test==27) require(pixel[2-red]==255,"fog/curve contaminated radiance capture");
  instance.model[12]=0;
  instance.custom[0][0]=instance.custom[0][1]=1;instance.custom[0][2]=0;
  instance.custom[1][0]=instance.custom[1][1]=instance.custom[1][2]=1;
  instance.custom[2][0]=instance.custom[2][1]=instance.custom[2][2]=0;
  if(test==28) host_allocations=resource_allocations=0;
  if(test>=29) require(host_allocations==0 && resource_allocations==0,"camera color steady frames allocated");
 }
 const float colors[][3]={{1,0,0},{0,1,0},{0,0,1},{.03f,.2f,3},{4,1,.05f}};
 for(int i=0;i<5;i++) {
  float lighting[ONIX_GPU_MESH_LIGHTING_BYTES/sizeof(float)]={[19]=1,[296]=1,[847]=1};
  for(int c=0;c<3;c++) instance.emissive_metallic[c]=colors[i][c];
  instance.material[3]=512+4096;instance.base_color[3]=1;draw.alpha_mode=0;
  unsigned char expected[3],pixel[4];aces_display_reference(colors[i],expected);
  require(onix_gpu_begin_frame(driver,clear),"begin ACES color frame");
  require(onix_gpu_begin_render_target(driver,target,clear,0,1,1),"begin ACES color target");onix_gpu_set_viewport(driver,0,0,16,16);
  require(onix_gpu_draw_meshes(driver,&push,vertices,4,indices,6,&instance,1,deformation,16,&draw,1,lighting),"draw ACES color");
  require(onix_gpu_end_render_target(driver),"end ACES color target");require(onix_gpu_present(driver),"present ACES color frame");
  require(onix_gpu_read_render_target(driver,target,8,8,1,1,pixel,4),"read ACES color");
  require(abs(pixel[red]-expected[0])<=1 && abs(pixel[1]-expected[1])<=1 && abs(pixel[2-red]-expected[2])<=1,"ACES color transform differs from CPU reference");
 }
 require(onix_gpu_destroy_render_target(driver,target),"destroy camera color target");require(onix_gpu_destroy_texture(driver,texture),"destroy camera color texture");
 printf("Vulkan Reinhard/ACES fit, huge HDR values, linear/exp2 fog, orthographic, unlit/transparent fog, sky/capture isolation and zero steady C/resource allocations passed\n");
}

static void test_surface_cache_retention(OnixGPUDriver *driver, OnixVkFrame *frame, const uint32_t textures[2]) {
	uint64_t peak_bytes = (uint64_t)frame->surface_capacity * (sizeof(*frame->surface_sets) + 8);
	VkDescriptorSet first = frame->surface_sets[0].descriptor;
	VkDescriptorPool first_pool = frame->surface_pools[0], last_pool = frame->surface_pools[ONIX_GPU_SURFACE_SETS / ONIX_GPU_SURFACE_POOL_SETS - 1];
	onix_vk_reset_surface_cache(driver, frame);
	size_t heap = host_allocations, resources = resource_allocations;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		for (uint32_t combination = 0; combination < 8; combination++) {
			OnixNativeMeshDraw3D draw = {.texture = textures[0]};
			for (uint32_t slot = 0; slot < 3; slot++) draw.extra_textures[slot] = combination & (1u << slot) ? textures[1] : 0;
			require(onix_vk_surface_descriptor(driver, frame, &draw) == frame->surface_sets[combination].descriptor,
				"material keys remain usable after shrink or failed shrink");
		}
		if (iteration == ONIX_BUFFER_TRIM_DELAY - 1) fail_next_reallocation = 1;
		onix_vk_reset_surface_cache(driver, frame);
		if (iteration < ONIX_BUFFER_TRIM_DELAY - 1) require(frame->surface_capacity == 4096 && host_allocations == heap && resource_allocations == resources,
			"short material workload retains descriptor cache without allocating");
		if (iteration == ONIX_BUFFER_TRIM_DELAY - 1) require(frame->surface_capacity == 4096 && frame->surface_pools[ONIX_GPU_SURFACE_SETS / ONIX_GPU_SURFACE_POOL_SETS - 1] == last_pool,
			"failed table shrink preserves every owning descriptor pool");
	}
	require(frame->surface_capacity == 64 && frame->surface_set_count == 64 && frame->surface_pools[0] == first_pool && frame->surface_pools[1] == VK_NULL_HANDLE,
		"material shrink retains the first descriptor block and releases the tail");
	require(frame->surface_sets[0].descriptor == first, "shrinking preserves retained descriptor handles");
	heap = host_allocations; resources = resource_allocations;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		OnixNativeMeshDraw3D draw = {.texture = textures[0]};
		require(onix_vk_surface_descriptor(driver, frame, &draw) == first, "retained material descriptor stays reusable");
		onix_vk_reset_surface_cache(driver, frame);
	}
	require(host_allocations == heap && resource_allocations == resources, "shrunken material cache allocates no heap or Vulkan resources");
	printf("Surface metadata retention: %llu -> %llu bytes; 64 descriptor pools -> 1, handles and zero warm allocations preserved\n",
		(unsigned long long)peak_bytes, (unsigned long long)((uint64_t)frame->surface_capacity * (sizeof(*frame->surface_sets) + 8)));
	for (uint32_t combination = 0; combination <= ONIX_GPU_SURFACE_POOL_SETS; combination++) {
		OnixNativeMeshDraw3D draw = {.texture = textures[0]};
		for (uint32_t slot = 0; slot < 7; slot++) draw.extra_textures[slot] = combination & (1u << slot) ? textures[1] : 0;
		require(onix_vk_surface_descriptor(driver, frame, &draw) != VK_NULL_HANDLE, "material cache regrows past retained pool boundary");
	}
	require(frame->surface_capacity == 128 && frame->surface_set_count == 65 && frame->surface_pools[1] && frame->surface_sets[0].descriptor == first,
		"regrowth allocates a new tail pool without disturbing retained handles");
}

static void test_surface_descriptor_cache(OnixGPUDriver *driver) {
	const uint8_t pixel[] = {255, 255, 255, 255};
	uint32_t textures[2];
	for (uint32_t index = 0; index < 2; index++) {
		textures[index] = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, pixel, sizeof(pixel), index, 0);
		require(textures[index] != 0, "create surface cache texture");
	}
	// These descriptors are never submitted, so a separate cache can exercise
	// every capacity boundary without growing the driver's steady-state storage.
	OnixVkFrame frame = {0};
	OnixNativeMeshDraw3D draw = {.texture = textures[0]};
	VkDescriptorSet sets[ONIX_GPU_SURFACE_SETS];
	size_t updates = surface_descriptor_updates;
	for (uint32_t combination = 0; combination < ONIX_GPU_SURFACE_SETS; combination++) {
		for (uint32_t slot = 0; slot < 13; slot++) draw.extra_textures[slot] = combination & (1u << slot) ? textures[1] : 0;
		sets[combination] = onix_vk_surface_descriptor(driver, &frame, &draw);
		require(sets[combination] != VK_NULL_HANDLE, "allocate unique surface combination");
		if (combination == 0) require(frame.surface_pools[1] == VK_NULL_HANDLE && frame.surface_capacity == 16, "small material count keeps surface storage small");
		if (combination == ONIX_GPU_SURFACE_POOL_SETS) require(frame.surface_pools[1] != VK_NULL_HANDLE, "surface pool grows at block boundary");
	}
	require(surface_descriptor_updates - updates == ONIX_GPU_SURFACE_SETS, "write each unique combination once");
	size_t heap = host_allocations, resources = resource_allocations;
	for (uint32_t combination = ONIX_GPU_SURFACE_SETS; combination-- > 0;) {
		for (uint32_t slot = 0; slot < 13; slot++) draw.extra_textures[slot] = combination & (1u << slot) ? textures[1] : textures[0];
		draw.metallic_roughness_texture = textures[0];
		require(onix_vk_surface_descriptor(driver, &frame, &draw) == sets[combination], "reuse normalized keys after collisions and growth");
	}
	require(surface_descriptor_updates - updates == ONIX_GPU_SURFACE_SETS && host_allocations == heap && resource_allocations == resources, "cache hits at capacity perform no writes or allocations");
	draw.extra_textures[12] = textures[1];
	require(onix_vk_surface_descriptor(driver, &frame, &draw) == VK_NULL_HANDLE, "reject additional unique combination at capacity");
	draw.extra_textures[12] = textures[0];
	require(onix_vk_surface_descriptor(driver, &frame, &draw) == sets[0], "capacity failure preserves existing entries");
	draw.metallic_roughness_texture = textures[1];
	require(onix_vk_surface_descriptor(driver, &frame, &draw) == VK_NULL_HANDLE, "metallic roughness binding participates in key");
	test_surface_cache_retention(driver, &frame, textures);
	require(onix_gpu_destroy_texture(driver, textures[0]), "retire cached surface texture");
	draw.metallic_roughness_texture = textures[0];
	require(onix_vk_surface_descriptor(driver, &frame, &draw) == VK_NULL_HANDLE, "cache hit still rejects retired texture");
	require(onix_gpu_destroy_texture(driver, textures[1]), "retire second surface texture");
	for (uint32_t pool = 0; pool < ONIX_GPU_SURFACE_SETS / ONIX_GPU_SURFACE_POOL_SETS; pool++) {
		if (frame.surface_pools[pool] != VK_NULL_HANDLE) vkDestroyDescriptorPool(driver->device, frame.surface_pools[pool], NULL);
	}
	free(frame.surface_sets);
	free(frame.surface_buckets);
	printf("Surface cache: 4096 unique combinations, normalized keys, collisions, growth, capacity hits and retirement passed\n");
}

static void test_draw_descriptor_reuse(OnixGPUDriver *driver, const uint32_t textures[4], uint32_t shader) {
	uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
	OnixGPUVertex3D vertices[] = {
		{.position = {-1, -1, .5f}, .normal = {0, 0, 1}},
		{.position = {1, -1, .5f}, .normal = {0, 0, 1}},
		{.position = {0, 1, .5f}, .normal = {0, 0, 1}},
	};
	uint32_t indices[] = {0, 1, 2};
	uint32_t mesh = onix_gpu_create_mesh(driver, vertices, 3, indices, 3);
	OnixNativeMeshDraw3D *draws = calloc(4097, sizeof(*draws));
	require(target && mesh && draws, "create descriptor reuse resources");
	for (uint32_t index = 0; index < 4097; index++) {
		draws[index] = (OnixNativeMeshDraw3D){.mesh = mesh, .texture = textures[0], .index_count = 3, .instance_count = 1};
		if (index % 2) {
			draws[index].metallic_roughness_texture = textures[0];
			for (uint32_t slot = 0; slot < 13; slot++) draws[index].extra_textures[slot] = textures[0];
		}
	}
	OnixGPUMeshPush push = {.view_projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
	OnixGPUInstance3D instance = {.model = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}, .base_color = {1, 1, 1, 1}, .material = {[3] = 512}};
	uint32_t deformation[16] = {0};
	const float clear[] = {0, 0, 0, 1};
	VkDescriptorSet retained_deformation[ONIX_VK_FRAMES_IN_FLIGHT] = {0};
	// Rotate each frame's material key so a stale hash table would skip writes.
	for (uint32_t iteration = 0; iteration < ONIX_VK_FRAMES_IN_FLIGHT * (ONIX_BUFFER_TRIM_DELAY + 4); iteration++) {
		draws[0].extra_textures[0] = textures[iteration / ONIX_VK_FRAMES_IN_FLIGHT % 2];
		require(onix_gpu_begin_frame(driver, clear), "begin descriptor reuse frame");
		require(onix_gpu_begin_render_target(driver, target, clear, 0, 1, 1), "begin descriptor reuse target");
		onix_gpu_set_viewport(driver, 0, 0, 16, 16);
		OnixVkFrame *frame = &driver->frames[driver->frame_index];
		require(frame->surface_set_index == 0, "surface cache resets on frame fence");
		size_t updates = surface_descriptor_updates;
		size_t mesh_updates = mesh_descriptor_updates;
		size_t binds = descriptor_bind_calls;
		uint32_t count = iteration == 0 ? 4097 : 1;
		size_t calls = indexed_draw_calls;
		require(onix_gpu_draw_meshes(driver, &push, NULL, 0, NULL, 0, &instance, 1, deformation, 16, draws, count, NULL), "draw repeated mesh material");
		require(indexed_draw_calls - calls == count && surface_descriptor_updates - updates == 1 && frame->surface_set_index == 1, "4097 draws consume one surface set");
		require(descriptor_bind_calls == binds + 1 && mesh_descriptor_updates == mesh_updates + 1, "mesh batch binds once and updates both buffer descriptors together");
		updates = surface_descriptor_updates;
		binds = descriptor_bind_calls;
		require(onix_gpu_draw_meshes(driver, &push, NULL, 0, NULL, 0, &instance, 1, deformation, 16, draws, 1, NULL), "reuse surface in another batch");
		require(surface_descriptor_updates == updates && frame->surface_set_index == 1, "surface cache spans batches");
		require(descriptor_bind_calls == binds + 1, "first mesh in each batch binds descriptors");
		if (iteration == 0) {
			OnixNativeMeshDraw3D changes[8];
			for (uint32_t index = 0; index < 8; index++) changes[index] = draws[0];
			changes[1].normal_texture = textures[0]; // Same normalized binding.
			changes[2].normal_texture = textures[1];
			changes[3].texture = changes[4].texture = textures[1];
			changes[4].extra_textures[12] = changes[5].extra_textures[12] = textures[2];
			changes[6].alpha_mode = 2; // Pipeline change keeps the same layout.
			binds = descriptor_bind_calls;
			calls = indexed_draw_calls;
			require(onix_gpu_draw_meshes(driver, &push, NULL, 0, NULL, 0, &instance, 1, deformation, 16, changes, 8, NULL), "draw changed mesh texture bindings and pipelines");
			require(descriptor_bind_calls == binds + 6 && indexed_draw_calls == calls + 8, "mesh binds only when base, normal or surface descriptors change");
			OnixNativeMeshDraw3D invalid[2] = {draws[0], draws[0]};
			invalid[1].first_instance = UINT32_MAX;
			calls = indexed_draw_calls;
			require(!onix_gpu_draw_meshes(driver, &push, NULL, 0, NULL, 0, &instance, 1, deformation, 16, invalid, 2, NULL), "reject mesh instance overflow in draw loop");
			require(indexed_draw_calls == calls + 1, "invalid mesh is never drawn");
			invalid[0].first_index = UINT32_MAX;
			require(!onix_gpu_draw_meshes(driver, &push, NULL, 0, NULL, 0, &instance, 1, deformation, 16, invalid, 1, NULL), "reject mesh index overflow");
			invalid[0] = draws[0]; invalid[0].normal_texture = driver->texture_slots.count + 1;
			require(!onix_gpu_draw_meshes(driver, &push, NULL, 0, NULL, 0, &instance, 1, deformation, 16, invalid, 1, NULL), "reject invalid normal texture");
			invalid[0] = draws[0]; invalid[0].extra_textures[12] = driver->texture_slots.count + 1;
			require(!onix_gpu_draw_meshes(driver, &push, NULL, 0, NULL, 0, &instance, 1, deformation, 16, invalid, 1, NULL), "reject invalid surface texture");
			require(indexed_draw_calls == calls + 1, "invalid mesh ranges and textures record no draws");
		}
		if (iteration < ONIX_VK_FRAMES_IN_FLIGHT) {
			while (frame->deformation_set_index <= ONIX_VK_DEFORMATION_POOL_SETS) {
				require(onix_gpu_draw_meshes(driver, &push, NULL, 0, NULL, 0, &instance, 1, deformation, 16, draws, 1, NULL), "draw deformation descriptor peak in each frame slot");
			}
			retained_deformation[driver->frame_index] = frame->deformation_sets[0];
			require(frame->deformation_pools[1] != VK_NULL_HANDLE, "GPU workload grows a second deformation pool");
		}
		if (iteration >= ONIX_VK_FRAMES_IN_FLIGHT * (ONIX_BUFFER_TRIM_DELAY + 2)) {
			require(frame->deformation_pools[1] == VK_NULL_HANDLE && frame->deformation_sets[0] == retained_deformation[driver->frame_index],
				"frame fence permits tail pool reclaim while preserving live draw handles");
		}
		OnixGPUSpritePush sprite_push = {.projection = {[0] = .125f, [5] = .125f, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
		OnixGPUInstance sprite = {.model = {16, 0, 0, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, 1}};
		OnixNativeSpriteDraw sprites[6] = {
			{.texture = textures[0], .instance_count = 1},
			{.texture = textures[0], .additional_textures = {textures[0], textures[0], textures[0]}, .instance_count = 1},
			{.shader = shader, .texture = textures[0], .instance_count = 1},
			{.texture = textures[0], .additional_textures = {[2] = textures[1]}, .instance_count = 1},
			{.shader = shader, .texture = textures[0], .additional_textures = {[2] = textures[1]}, .instance_count = 1},
			{.texture = textures[0], .instance_count = 1},
		};
		binds = descriptor_bind_calls;
		calls = indexed_draw_calls;
		require(onix_gpu_draw_sprites(driver, &sprite_push, &sprite, 1, sprites, 6), "draw repeated sprite bindings");
		require(descriptor_bind_calls - binds == 3 && indexed_draw_calls - calls == 6, "sprite binds only on changes, including shader switches");
		binds = descriptor_bind_calls;
		require(onix_gpu_draw_sprites(driver, &sprite_push, &sprite, 1, sprites, 1), "draw sprite in another batch");
		require(descriptor_bind_calls == binds + 1, "first sprite in each batch binds descriptors");
		sprites[1].first_instance = UINT32_MAX;
		calls = indexed_draw_calls;
		require(!onix_gpu_draw_sprites(driver, &sprite_push, &sprite, 1, sprites, 2), "reject sprite instance overflow in draw loop");
		require(indexed_draw_calls == calls + 1, "invalid sprite is never drawn");
		sprites[0].additional_textures[2] = driver->texture_slots.count + 1;
		require(!onix_gpu_draw_sprites(driver, &sprite_push, &sprite, 1, sprites, 1), "reject invalid sprite texture");
		require(indexed_draw_calls == calls + 1, "invalid sprite texture records no draw");
		require(onix_gpu_end_render_target(driver), "end descriptor reuse target");
		require(onix_gpu_present(driver), "present descriptor reuse frame");
		uint8_t pixel[4];
		require(onix_gpu_read_render_target(driver, target, 8, 8, 1, 1, pixel, sizeof(pixel)), "read descriptor reuse target");
		uint32_t red = driver->swapchain_format == VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format == VK_FORMAT_B8G8R8A8_SRGB ? 2 : 0;
		require(pixel[red] == 255 && pixel[1] == 0 && pixel[2 - red] == 0 && pixel[3] == 255, "descriptor reuse preserves pixels");
	}
	free(draws);
	require(onix_gpu_destroy_mesh(driver, mesh), "destroy descriptor reuse mesh");
	require(onix_gpu_destroy_render_target(driver, target), "destroy descriptor reuse target");
	printf("Descriptor reuse: 4097 mesh draws bind once, shared materials, texture/pipeline changes, range checks; deformation pool reclaim across both frame fences and pixels passed\n");
}

static void test_static_deformation_residency(OnixGPUDriver *driver) {
 const uint8_t red[]={255,0,0,255};
 uint32_t texture=onix_gpu_create_texture_with_format(driver,1,1,1,ONIX_TEXTURE_FORMAT_RGBA8,0,red,4,0,0),color;
 uint32_t target=onix_gpu_create_render_target(driver,16,16,0,1,&color);
 OnixGPUVertex3D vertices[]={ {.position={-.25f,-.25f,.5f},.normal={0,0,1}}, {.position={.25f,-.25f,.5f},.normal={0,0,1}}, {.position={0,.25f,.5f},.normal={0,0,1}} };
 uint32_t indices[]={0,1,2},meshes[2];
 for (uint32_t mesh_index=0;mesh_index<2;mesh_index++) {
  uint32_t stream[52]={0};float one=1,delta=mesh_index==0?.2f:-.2f;
  for(uint32_t vertex=0;vertex<3;vertex++) { memcpy(stream+1+vertex*8+4,&one,4);memcpy(stream+25+vertex*9,&delta,4); }
  meshes[mesh_index]=onix_gpu_create_mesh(driver,vertices,3,indices,3);
  require(meshes[mesh_index]!=0 && onix_gpu_set_mesh_deformation(driver,meshes[mesh_index],stream,52,3),"upload resident skin/morph");
  memset(stream,0,sizeof(stream)); // The driver owns a copy after upload.
 }
 OnixGPUMeshPush push={.view_projection={[0]=1,[5]=1,[10]=1,[15]=1}};
 OnixGPUInstance3D instance={.model={[0]=1,[5]=1,[10]=1,[15]=1},.base_color={1,1,1,1},.material={-1,1.5f,.5f},.uv_transform={1,1}};
 OnixNativeMeshDraw3D draws[2]={{.mesh=meshes[0],.texture=texture,.index_count=3,.instance_count=1,.deformation_index=1},{.mesh=meshes[1],.texture=texture,.index_count=3,.instance_count=1,.deformation_index=1}};
 uint32_t animated[25]={1,25,8,24,3,1,1,1};
 const float clear[]={0,0,0,0};float matrix[16]={[0]=1,[5]=1,[10]=1,[15]=1},weight=.5f;
 for(int frame=0;frame<12;frame++) {
  if(frame==8) host_allocations=resource_allocations=0;
  matrix[12]=frame%2?.45f:-.45f;memcpy(animated+8,matrix,sizeof(matrix));memcpy(animated+24,&weight,4);
  require(onix_gpu_begin_frame(driver,clear),"begin resident deformation frame");
  require(onix_gpu_begin_render_target(driver,target,clear,0,1,1),"begin resident deformation target");onix_gpu_set_viewport(driver,0,0,16,16);
  require(onix_gpu_draw_meshes(driver,&push,NULL,0,NULL,0,&instance,1,animated,25,draws,2,NULL),"draw resident deformation batch");
  require(onix_gpu_end_render_target(driver),"end resident deformation target");require(onix_gpu_present(driver),"present resident deformation frame");
  unsigned char pixel[4];int x=(int)((matrix[12]+1.0f)*8.0f);
  require(onix_gpu_read_render_target(driver,target,x,8,1,1,pixel,4),"read animated resident stream");
  int red_channel=driver->swapchain_format==VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format==VK_FORMAT_B8G8R8A8_SRGB ? 2 : 0;
  require(pixel[red_channel]==255 && pixel[3]==255,"resident morph and joint update pixels");
 }
 require(host_allocations==0 && resource_allocations==0,"resident deformation stable frames allocate nothing");
 uint32_t retire=(driver->frame_index+ONIX_VK_FRAMES_IN_FLIGHT-1)%ONIX_VK_FRAMES_IN_FLIGHT;
 require(onix_gpu_destroy_mesh(driver,meshes[0]) && onix_gpu_destroy_mesh(driver,meshes[1]),"retire resident meshes");
 require(driver->frames[retire].retired_meshes==meshes[1] && driver->meshes[meshes[1]-1].retire_next==meshes[0],"resident meshes use indexed retirement list");
 require(!onix_gpu_destroy_mesh(driver,meshes[1]),"reject duplicate retirement");
 require(onix_gpu_destroy_texture(driver,texture) && onix_gpu_destroy_render_target(driver,target),"retire resident material/target");
 for(uint32_t frame=0;frame<ONIX_VK_FRAMES_IN_FLIGHT;frame++) { require(onix_gpu_begin_frame(driver,clear),"begin retirement drain");require(onix_gpu_present(driver),"present retirement drain"); }
 require(driver->meshes[meshes[0]-1].deformation_buffer==VK_NULL_HANDLE && driver->meshes[meshes[1]-1].deformation_buffer==VK_NULL_HANDLE && driver->frames[retire].retired_meshes==0,"resident buffers release after their frame fence");
 printf("Resident static skin/morph: copied source, animated poses, shared material across meshes, zero stable allocations and indexed retirement passed\n");
}

static void test_native_slot_lifecycle(void) {
	OnixNativeSlots slots = {0};
	OnixVkTexture *entries = NULL;
	uint32_t index;
	fail_next_reallocation = 1;
	require(!onix_vk_textures_acquire(&entries, &slots, &index) && entries == NULL && slots.count == 0, "failed allocation keeps the slot pool empty");
	for (uint32_t value = 0; value < ONIX_VK_MAX_TEXTURES; value++) {
		require(onix_vk_textures_acquire(&entries, &slots, &index) && index == value, "new slots append without scanning occupied entries");
		onix_vk_textures_insert(entries, &slots, index, (OnixVkTexture){.width = value + 1});
	}
	require(!onix_vk_textures_acquire(&entries, &slots, &index), "texture slot capacity enforced");
	for (uint32_t value = 0; value < ONIX_VK_MAX_TEXTURES; value += 2) onix_vk_textures_recycle(entries, &slots, value);
	size_t allocations = host_allocations;
	for (uint32_t value = ONIX_VK_MAX_TEXTURES; value != 0; value -= 2) {
		require(onix_vk_textures_acquire(&entries, &slots, &index) && index == value - 2, "recycled slots follow the free chain");
		uint32_t same;
		require(onix_vk_textures_acquire(&entries, &slots, &same) && same == index, "failed resource creation does not consume a prepared slot");
		onix_vk_textures_insert(entries, &slots, index, (OnixVkTexture){.width = value});
	}
	require(host_allocations == allocations && slots.count == ONIX_VK_MAX_TEXTURES, "warm slot reuse never allocates or grows the high water mark");
	for (uint32_t value = 1; value < ONIX_VK_MAX_TEXTURES; value += 2) require(entries[value].width == value + 1, "slot reuse preserves live neighbours");
	free(entries);
	printf("Native slots: allocation failure, capacity, abandoned acquisition and 2048 zero-allocation reuses passed\n");
}

static void test_interleaved_upload_cancellation(OnixGPUDriver *driver) {
	const uint8_t red[] = {255, 0, 0, 255}, green[] = {0, 255, 0, 255}, blue[] = {0, 0, 255, 255};
	const float clear[] = {0, 0, 0, 1};
	for (int burst = 0; burst < 2; burst++) {
		uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
		uint32_t textures[3];
		for (uint32_t index = 0; index < 3; index++) {
			textures[index] = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, red, sizeof(red), 0, 0);
			require(textures[index] != 0, "create interleaved upload source");
			driver->pending_external[driver->pending_external_count++] = textures[index] - 1;
			driver->textures[textures[index] - 1].pending_external_index = driver->pending_external_count;
		}
		OnixGPUTextureUpdate updates[] = {
			{.texture = textures[0], .width = 1, .height = 1, .data_length = 4},
			{.texture = textures[1], .width = 1, .height = 1, .data_offset = 4, .data_length = 4},
			{.texture = textures[2], .width = 1, .height = 1, .data_offset = 8, .data_length = 4},
			{.texture = textures[1], .width = 1, .height = 1, .data_offset = 12, .data_length = 4},
		};
		uint8_t pixels[16];
		memcpy(pixels, green, 4); memcpy(pixels + 4, red, 4); memcpy(pixels + 8, blue, 4); memcpy(pixels + 12, green, 4);
		for (int repeat = 0; repeat < 40; repeat++) require(onix_gpu_update_textures(driver, updates, 4, pixels, sizeof(pixels)), "queue interleaved uploads across reallocations");
		size_t allocations = host_allocations;
		require(onix_gpu_destroy_texture(driver, textures[0]) && onix_gpu_destroy_texture(driver, textures[2]), "cancel first and swapped external queue entries");
		require(host_allocations == allocations && driver->upload_live_count == 81 && driver->pending_external_count == 1 && driver->pending_external[0] == textures[1] - 1, "cancellation only marks the matching uploads and fixes reverse indices");
		uint32_t fresh = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, blue, sizeof(blue), 0, 0);
		require(fresh && fresh != textures[0] && fresh != textures[2], "pending retirement slots cannot be reused");
		if (burst) require(onix_vk_submit_staging_burst(driver), "burst submission skips cancelled uploads");
		require(onix_gpu_begin_frame(driver, clear), "begin cancellation frame");
		draw_target(driver, target, textures[1]);
		require(onix_gpu_present(driver), "present surviving interleaved uploads");
		check_pixel(driver, target, 0, 255, -1);
		OnixGPUTextureUpdate fresh_update = {.texture = fresh, .width = 1, .height = 1, .data_length = 4};
		require(onix_gpu_update_textures(driver, &fresh_update, 1, blue, sizeof(blue)), "queue next upload epoch");
		require(onix_gpu_destroy_texture(driver, textures[1]) && driver->upload_live_count == 1, "stale upload heads cannot cancel a new queue epoch");
		require(onix_gpu_begin_frame(driver, clear), "begin next epoch frame");
		draw_target(driver, target, fresh);
		require(onix_gpu_present(driver), "present next epoch frame");
		uint8_t pixel[4];
		require(onix_gpu_read_render_target(driver, target, 8, 8, 1, 1, pixel, sizeof(pixel)), "read surviving new epoch source");
		int blue_channel = driver->swapchain_format == VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format == VK_FORMAT_B8G8R8A8_SRGB ? 0 : 2;
		require(pixel[2 - blue_channel] == 0 && pixel[1] == 0 && pixel[blue_channel] == 255, "new epoch uploads survive stale cancellation");
		require(onix_gpu_destroy_texture(driver, fresh) && onix_gpu_destroy_render_target(driver, target), "retire cancellation resources");
	}
	printf("Interleaved upload cancellation: queue growth, order, burst/frame submission, epochs and deferred reuse passed\n");
}

static void test_buffer_retention(OnixGPUDriver *driver) {
	OnixBufferRetention retention = {0};
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) {
		uint64_t used = frame % 7 == 0 ? 3u * 1024u * 1024u : 65536u;
		require(onix_buffer_trim_capacity(&retention, 8u * 1024u * 1024u, used, 1024u * 1024u) == 0,
			"intermittent high demand resets the shrink window");
	}
	retention = (OnixBufferRetention){0};
	for (uint32_t frame = 1; frame < ONIX_BUFFER_TRIM_DELAY; frame++) {
		require(onix_buffer_trim_capacity(&retention, 8u * 1024u * 1024u, 1024u * 1024u, 1024u * 1024u) == 0,
			"native retention waits for sustained underuse");
	}
	require(onix_buffer_trim_capacity(&retention, 8u * 1024u * 1024u, 0, 1024u * 1024u) == 2u * 1024u * 1024u,
		"native retention keeps twice the recent peak on an idle frame");

	// These pools have never been submitted to the GPU, so they can be trimmed
	// directly. Test the former single-large-block escape from the trim path.
	OnixVkFrame frame = {0};
	require(onix_vk_append_instance_buffer(driver, &frame, 8u * ONIX_VK_INSTANCE_BUFFER_BLOCK_SIZE) != NULL, "allocate large instance block");
	frame.instance_bytes_used = 65536;
	for (uint32_t index = 1; index < ONIX_BUFFER_TRIM_DELAY; index++) {
		onix_vk_trim_instance_buffers(driver, &frame);
		require(frame.instance_buffer_count == 1, "instance block survives short low load");
	}
	onix_vk_trim_instance_buffers(driver, &frame);
	require(frame.instance_buffer_count == 0 && frame.instance_capacity == 0, "single oversized instance block releases");
	require(onix_vk_append_instance_buffer(driver, &frame, 65536) != NULL, "rewarm small instance pool");
	size_t heap = host_allocations, resources = resource_allocations;
	for (uint32_t index = 0; index < ONIX_BUFFER_TRIM_DELAY * 2; index++) onix_vk_trim_instance_buffers(driver, &frame);
	require(frame.instance_capacity == ONIX_VK_INSTANCE_BUFFER_BLOCK_SIZE && host_allocations == heap && resource_allocations == resources,
		"small instance pool remains warm without allocations");
	require(frame.instance_capacity == 64u * 1024u, "small Vulkan instance upload reserves 64 KiB");
	onix_vk_destroy_instance_buffers(driver, &frame);

	require(onix_vk_append_staging_buffer(driver, &frame, 4u * ONIX_VK_STAGING_BUFFER_BLOCK_SIZE) != NULL, "allocate large staging block");
	frame.staging_reserved = 65536;
	for (uint32_t index = 0; index < ONIX_BUFFER_TRIM_DELAY; index++) onix_vk_trim_staging_buffers(driver, &frame);
	require(frame.staging_buffer_count == 0 && frame.staging_capacity == 0, "single oversized staging block releases");
	require(onix_vk_append_staging_buffer(driver, &frame, 65536) != NULL, "allocate warm staging block");
	require(onix_vk_append_staging_buffer(driver, &frame, 4u * ONIX_VK_STAGING_BUFFER_BLOCK_SIZE) != NULL, "allocate staging spike");
	frame.staging_reserved = 0;
	heap = host_allocations; resources = resource_allocations;
	for (uint32_t index = 0; index < ONIX_BUFFER_TRIM_DELAY; index++) onix_vk_trim_staging_buffers(driver, &frame);
	require(frame.staging_buffer_count == 1 && frame.staging_capacity == ONIX_VK_STAGING_BUFFER_BLOCK_SIZE && host_allocations == heap && resource_allocations == resources,
		"idle staging retains the small warm block without allocating");
	require(frame.staging_capacity == 256u * 1024u, "small Vulkan texture upload reserves 256 KiB");
	onix_vk_destroy_staging_buffers(driver, &frame);
	require(onix_vk_append_staging_buffer(driver, &frame, 9u * 1024u * 1024u) != NULL, "allocate staging payload above bulk size");
	require(frame.staging_capacity == 12u * 1024u * 1024u, "large staging requests round by bulk size rather than doubling the payload");
	onix_vk_destroy_staging_buffers(driver, &frame);
	// Exercise the frame hooks as well as the pool functions: each slot must
	// observe its own completed demand before its offsets are reset.
	for (uint32_t slot = 0; slot < ONIX_VK_FRAMES_IN_FLIGHT; slot++) {
		require(onix_vk_append_instance_buffer(driver, &driver->frames[slot], 8u * ONIX_VK_INSTANCE_BUFFER_BLOCK_SIZE) != NULL, "allocate frame instance spike");
		require(onix_vk_append_staging_buffer(driver, &driver->frames[slot], 4u * ONIX_VK_STAGING_BUFFER_BLOCK_SIZE) != NULL, "allocate frame staging spike");
	}
	const float clear[] = {0, 0, 0, 1};
	for (uint32_t index = 0; index < ONIX_BUFFER_TRIM_DELAY * ONIX_VK_FRAMES_IN_FLIGHT; index++) {
		require(onix_gpu_begin_frame(driver, clear), "begin retention frame");
		require(onix_gpu_present(driver), "present retention frame");
	}
	OnixBufferMemory memory;
	onix_gpu_buffer_memory(driver, &memory);
	require(memory.instances == 0 && memory.staging == 0, "completed idle frame slots release oversized pools");
	printf("Buffer retention: single-block peaks release, intermittent demand stays warm, pool compaction allocates nothing\n");
}

static void test_mesh_memory_reuse(OnixGPUDriver *driver) {
	OnixGPUVertex3D vertices[3] = {{.position = {-.5f, -.5f, .5f}}, {.position = {.5f, -.5f, .5f}}, {.position = {0, .5f, .5f}}};
	uint32_t indices[] = {0, 1, 2}, stream[64] = {1, 2, 3};
	uint32_t first = onix_gpu_create_mesh(driver, vertices, 3, indices, 3);
	uint32_t second = onix_gpu_create_mesh(driver, vertices, 3, indices, 3);
	require(first && second && onix_gpu_set_mesh_deformation(driver, second, stream, 64, 3), "create pooled mesh geometry and deformation");
	OnixVkMesh a = driver->meshes[first - 1], b = driver->meshes[second - 1];
	require(a.vertex_buffer == b.vertex_buffer && a.vertex_buffer == a.index_buffer && b.deformation_buffer == a.vertex_buffer,
		"small mesh geometry and deformation share one GPU buffer");
	require(a.vertex_offset != b.vertex_offset && b.deformation_offset != b.vertex_offset && b.deformation_offset != a.vertex_offset,
		"resident mesh leases never overlap");
	OnixVkMeshBlock *block = &driver->mesh_blocks[a.geometry.block];
	require(memcmp((uint8_t *)block->mapping + b.vertex_offset, vertices, sizeof(vertices)) == 0 &&
		memcmp((uint8_t *)block->mapping + b.index_offset, indices, sizeof(indices)) == 0 &&
		memcmp((uint8_t *)block->mapping + b.deformation_offset, stream, sizeof(stream)) == 0,
		"all resident streams honor their arena offsets");
	const float clear[4] = {0};
	require(onix_gpu_begin_frame(driver, clear), "begin arena retirement test");
	require(onix_gpu_destroy_mesh(driver, first), "retire arena lease");
	uint32_t third = onix_gpu_create_mesh(driver, vertices, 3, indices, 3);
	require(third && driver->meshes[third - 1].vertex_offset != a.vertex_offset, "pending GPU leases stay occupied before their fence");
	require(onix_gpu_destroy_mesh(driver, third) && onix_gpu_destroy_mesh(driver, second), "retire remaining arena leases");
	require(onix_gpu_present(driver), "submit arena retirement");
	for (uint32_t index = 0; index < ONIX_VK_FRAMES_IN_FLIGHT; index++) {
		require(onix_gpu_begin_frame(driver, clear) && onix_gpu_present(driver), "drain arena retirement fences");
	}
	size_t heap = host_allocations, resources = resource_allocations;
	for (int iteration = 0; iteration < 100; iteration++) {
		uint32_t mesh = onix_gpu_create_mesh(driver, vertices, 3, indices, 3);
		require(mesh && onix_gpu_set_mesh_deformation(driver, mesh, stream, 64, 3), "reuse coalesced resident ranges");
		// This mesh has never been submitted. Release directly to isolate arena
		// reuse from command-buffer allocation and frame scheduling.
		onix_vk_release_mesh(driver, &driver->meshes[mesh - 1]);
		onix_vk_meshes_recycle(driver->meshes, &driver->mesh_slots, mesh - 1);
	}
	require(host_allocations == heap && resource_allocations == resources, "100 resident mesh recreations allocate no C heap or Vulkan resources");
	for (int iteration = 0; iteration < 6; iteration++) {
		require(onix_gpu_begin_frame(driver, clear), "begin shared-upload frame");
		OnixVkFrame *frame = &driver->frames[driver->frame_index];
		OnixNativeUpload original, same, changed, aligned;
		require(onix_vk_mesh_upload(driver, vertices, sizeof(vertices), 16, 1, &original), "upload frame geometry snapshot");
		VkDeviceSize before = frame->instance_bytes_used;
		OnixGPUVertex3D copy[3];
		memcpy(copy, vertices, sizeof(copy));
		require(onix_vk_mesh_upload(driver, copy, sizeof(copy), 16, 1, &same), "reuse geometry in another pass");
		require(original.resource == same.resource && original.offset == same.offset && frame->instance_bytes_used == before,
			"identical pass geometry consumes no additional GPU upload range");
		copy[0].position[0] += 1;
		require(onix_vk_mesh_upload(driver, copy, sizeof(copy), 16, 1, &changed), "upload mutated scratch geometry");
		require((original.resource != changed.resource || original.offset != changed.offset) && memcmp(original.data, vertices, sizeof(vertices)) == 0,
			"scratch mutations preserve the earlier uploaded snapshot");
		require(onix_vk_mesh_upload(driver, stream, sizeof(stream), 256, 1, &aligned) && aligned.offset % 256 == 0, "shared uploads preserve storage alignment");
		require(onix_gpu_present(driver), "submit shared-upload frame");
	}
	// Release a large surplus block after its leases become idle, retaining the
	// small warm arena for the next scene instead of retaining historical peaks.
	OnixVkMeshLease large;
	VkBuffer buffer;
	VkDeviceSize offset;
	void *mapping;
	require(onix_vk_mesh_storage(driver, ONIX_VK_MESH_BLOCK_SIZE * 2, &large, &buffer, &offset, &mapping), "create a temporary large arena");
	uint32_t large_index = large.block;
	onix_buddy_release(&driver->mesh_blocks[large_index].buddy, large.node);
	for (uint32_t index = 0; index < ONIX_BUFFER_TRIM_DELAY; index++) onix_vk_trim_mesh_storage(driver);
	require(driver->mesh_blocks[large_index].buffer == VK_NULL_HANDLE, "idle large arenas release their GPU allocation");
	require(onix_vk_mesh_storage(driver, ONIX_VK_MESH_BLOCK_SIZE * 2, &large, &buffer, &offset, &mapping) && large.block == large_index,
		"trimmed arena slots can be reused without growing the table");
	onix_buddy_release(&driver->mesh_blocks[large.block].buddy, large.node);
	puts("Mesh memory reuse: shared resident arena offsets, fence retirement, 100 zero-allocation recreations, shared-pass snapshots, mutated scratch and idle arena trimming passed");
}

static void test_readback_retention(OnixGPUDriver *driver) {
	require(onix_vk_prepare_readback(driver, 32u * 1024u * 1024u), "allocate readback peak");
	VkBuffer original = driver->readback.buffer;
	size_t heap = host_allocations, resources = resource_allocations;
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) {
		driver->readback_used = frame % 7 == 0 ? driver->readback.capacity : 4;
		onix_vk_trim_readback(driver);
		require(driver->readback.buffer == original, "intermittent large Vulkan readbacks stay warm");
	}
	driver->readback_used = driver->readback.capacity;
	onix_vk_trim_readback(driver);
	// Pending submissions must not lose their mapping or age the shrink window.
	driver->readback_pending = 1;
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) onix_vk_trim_readback(driver);
	require(driver->readback.buffer == original && driver->readback_retention.frames == 0, "pending readback storage stays owned by GPU");
	driver->readback_pending = 0;
	for (uint32_t frame = 1; frame < ONIX_BUFFER_TRIM_DELAY; frame++) {
		driver->readback_used = 4;
		onix_vk_trim_readback(driver);
		require(driver->readback.buffer == original, "readback peak survives short low demand");
	}
	onix_vk_trim_readback(driver);
	OnixBufferMemory memory;
	onix_gpu_buffer_memory(driver, &memory);
	require(memory.readback == 0 && driver->readback.mapping == NULL, "readback capacity and mapping release together");
	require(host_allocations == heap && resource_allocations == resources, "readback trimming allocates nothing");
	require(onix_vk_prepare_readback(driver, 4) && driver->readback.capacity == 4096, "small readback rewarms to one page");
	heap = host_allocations; resources = resource_allocations;
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) onix_vk_trim_readback(driver);
	require(driver->readback.capacity == 4096 && host_allocations == heap && resource_allocations == resources, "small readback stays warm without allocations");
	puts("Readback retention: 32 MiB peak released, 4 KiB warm buffer, intermittent demand and pending submission protection passed");
}

static void test_lcd_text_blend(OnixGPUDriver *driver) {
	const uint8_t mask[] = {64, 128, 192, 255};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, mask, sizeof(mask), 0, 0);
	require(texture != 0, "create LCD coverage texture");
	const float background[] = {.2f, .4f, .6f, 1};
	const float foreground[2][3] = {{.8f, .1f, .5f}, {.1f, .9f, .2f}};
	OnixGPUSpritePush push = {.projection = {[0] = .125f, [5] = .125f, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
	OnixGPUInstance glyphs[2] = {
		{.model = {16, 0, 0, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, 1}},
		{.model = {16, 0, 0, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, .5f}},
	};
	OnixGPUInstance next = {.model = {8, 0, 8, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {1, 1, 1, .5f}};
	for (uint32_t depth = 0; depth < 2; depth++) {
		uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, depth, &color);
		require(target != 0, "create LCD target");
		require(onix_gpu_begin_frame(driver, background) && onix_gpu_begin_render_target(driver, target, background, 0, 1, depth), "begin LCD frame");
		onix_gpu_set_viewport(driver, 0, 0, 16, 16);
		OnixNativeSpriteDraw draw = {.texture = texture, .instance_count = 2};
		size_t calls = indexed_draw_calls;
		onix_gpu_set_sprite_blend(driver, 1, foreground[0][0], foreground[0][1], foreground[0][2], 0);
		require(onix_gpu_draw_sprites(driver, &push, glyphs, 2, &draw, 1), "draw overlapping LCD glyphs in one batch");
		draw.instance_count = 1;
		onix_gpu_set_sprite_blend(driver, 1, foreground[1][0], foreground[1][1], foreground[1][2], 0);
		require(onix_gpu_draw_sprites(driver, &push, &next, 1, &draw, 1), "change LCD foreground constant");
		require(indexed_draw_calls - calls == 2, "LCD draws scale with color runs");
		require(onix_gpu_end_render_target(driver) && onix_gpu_present(driver), "present LCD frame");
		for (uint32_t sample = 0; sample < 2; sample++) {
			uint8_t pixel[4];
			require(onix_gpu_read_render_target(driver, target, sample ? 12 : 4, 8, 1, 1, pixel, sizeof(pixel)), "read LCD composite");
			if (driver->swapchain_format == VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format == VK_FORMAT_B8G8R8A8_SRGB) {
				uint8_t blue = pixel[0]; pixel[0] = pixel[2]; pixel[2] = blue;
			}
			for (uint32_t channel = 0; channel < 3; channel++) {
				float coverage = mask[channel] / 255.0f;
				float expected = background[channel];
				for (uint32_t glyph = 0; glyph < 2 + sample; glyph++) {
					float opacity = glyph == 0 ? 1 : .5f;
					expected = roundf((foreground[glyph == 2][channel] * coverage * opacity + expected * (1 - coverage * opacity)) * 255) / 255;
				}
				require(abs(pixel[channel] - (int)roundf(expected * 255)) <= 2, "LCD preserves RGB coverage, opacity and overlapping glyph order");
			}
			require(pixel[3] == 255, "LCD keeps opaque target alpha");
		}
		onix_gpu_destroy_render_target(driver, target);
	}
	onix_gpu_set_sprite_blend(driver, 0, 0, 0, 0, 0);
	onix_gpu_destroy_texture(driver, texture);
	puts("LCD text: single-pass RGB coverage, overlapping glyphs, opacity, foreground changes and depth/color-only targets passed");
}

static void test_color_only_targets(OnixGPUDriver *driver) {
	uint32_t color, target = onix_gpu_create_render_target(driver, 16, 16, 0, 0, &color);
	require(target != 0, "create color-only target");
	OnixVkRenderTarget *entry = &driver->render_targets[target - 1];
	require(!entry->depth_image && !entry->depth_view && !entry->depth_memory, "Depth=false owns no depth resources");
	const uint8_t white[] = {255, 255, 255, 255};
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1, 1, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, white, 4, 0, 0);
	uint32_t sprite_shader = onix_gpu_create_material2d_shader(driver, driver->sprite_vertex_shader, driver->sprite_fragment_shader);
	uint32_t mesh_shader = onix_gpu_create_mesh_shader(driver, driver->mesh_vertex_shader, driver->mesh_unlit_fragment_shader);
	require(texture && sprite_shader && mesh_shader, "create color-only draw resources");
	require(driver->mesh_pipelines[mesh_shader - 1].opaque == VK_NULL_HANDLE &&
		driver->material2d_pipelines[sprite_shader - 1].pipeline == VK_NULL_HANDLE, "custom shaders compile only consumed variants");
	const float clear[] = {0, 0, 0, 1};
	OnixGPUSpritePush push = {.projection = {[0] = .125f, [5] = .125f, [10] = 1, [12] = -1, [13] = -1, [15] = 1}};
	OnixGPUInstance sprite = {.model = {16, 0, 0, 0, 16, 0}, .texture_model = {1, 0, 0, 0, 1, 0}, .color = {.25f, .5f, 1, 1}};
	OnixNativeSpriteDraw sprite_draw = {.texture = texture, .instance_count = 1};
	OnixGPUShapeInstance shapes[2] = {
		{.model = {16, 0, 0, 0, 16, 0}, .local_model = {1, 0, 0, 0, 1, 0}, .fill = {1, 1, 1, 1}, .params = {16, 16, 0, 0}},
		{.model = {16, 0, 0, 0, 16, 0}, .local_model = {1, 0, 0, 0, 1, 0}, .fill = {1, 1, 1, 1}, .params = {16, 16, 0, 0}, .kind = 3},
	};
	OnixGPUMeshPush mesh_push = {.view_projection = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}};
	OnixGPUVertex3D vertices[] = {{.position = {-1, -1, .5f}}, {.position = {1, -1, .5f}}, {.position = {0, 1, .5f}}};
	uint32_t indices[] = {0, 1, 2}, deformation[16] = {0};
	OnixGPUInstance3D instance = {.model = {[0] = 1, [5] = 1, [10] = 1, [15] = 1}, .base_color = {.25f, .5f, 1, 1}, .material = {-1}, .uv_transform = {1, 1}};
	OnixNativeMeshDraw3D mesh_draw = {.texture = texture, .index_count = 3, .instance_count = 1};
	float lighting[ONIX_GPU_MESH_LIGHTING_BYTES / sizeof(float)] = {0};
	size_t heap = 0, resources = 0;
	for (uint32_t frame = 0; frame < 8; frame++) {
		if (frame == 4) { heap = host_allocations; resources = resource_allocations; }
		require(onix_gpu_begin_frame(driver, clear), "begin color-only frame");
		require(onix_gpu_begin_render_target(driver, target, clear, 0, 1, 1), "begin color-only pass");
		onix_gpu_set_viewport(driver, 0, 0, 16, 16);
		require(onix_gpu_draw_shapes(driver, &push, shapes, 2), "draw shapes and corner masks without depth");
		for (uint32_t blend = 0; blend < 2; blend++) {
			onix_gpu_set_sprite_blend(driver, blend, .25f, .5f, 1, 0);
			sprite_draw.shader = 0;
			require(onix_gpu_draw_sprites(driver, &push, &sprite, 1, &sprite_draw, 1), "draw color-only sprite blend variant");
		}
		onix_gpu_set_sprite_blend(driver, 0, 0, 0, 0, 0);
		sprite_draw.shader = sprite_shader;
		require(onix_gpu_draw_sprites(driver, &push, &sprite, 1, &sprite_draw, 1), "draw custom color-only sprite");
		for (uint32_t custom = 0; custom < 2; custom++) {
			mesh_draw.shader = custom ? mesh_shader : 0;
			for (uint32_t variant = 0; variant < 1 + ONIX_MESH_BLEND_MODE_COUNT; variant++) {
				mesh_draw.alpha_mode = variant ? 2 : 0;
				mesh_draw.blend_mode = variant ? variant - 1 : 0;
				require(onix_gpu_draw_meshes(driver, &mesh_push, vertices, 3, indices, 3, &instance, 1,
					deformation, 16, &mesh_draw, 1, lighting), "draw mesh without depth");
			}
		}
		// Finish with a deterministic opaque pixel after all blending variants.
		mesh_draw.alpha_mode = 0;
		require(onix_gpu_draw_meshes(driver, &mesh_push, vertices, 3, indices, 3, &instance, 1,
			deformation, 16, &mesh_draw, 1, lighting), "write final color-only pixel");
		require(onix_gpu_end_render_target(driver), "end color-only pass");
		// A load pass must ignore a depth-only clear, preserving the color.
		require(onix_gpu_begin_render_target(driver, target, clear, 0, 0, 1), "load color-only pass with depth clear requested");
		require(onix_gpu_end_render_target(driver), "end color-only load pass");
		require(onix_gpu_present(driver), "present color-only frame");
		uint8_t pixel[4];
		require(onix_gpu_read_render_target(driver, target, 8, 8, 1, 1, pixel, 4), "read color-only pixel");
		int red = driver->swapchain_format == VK_FORMAT_B8G8R8A8_UNORM || driver->swapchain_format == VK_FORMAT_B8G8R8A8_SRGB ? 2 : 0;
		require(abs(pixel[red] - 64) <= 1 && abs(pixel[1] - 128) <= 1 && pixel[2 - red] == 255, "color-only pipelines preserve draw colors");
		if (frame >= 4) require(host_allocations == heap && resource_allocations == resources, "color-only steady frames allocate nothing");
	}
	require(driver->mesh_pipelines[mesh_shader - 1].opaque == VK_NULL_HANDLE &&
		driver->material2d_pipelines[sprite_shader - 1].pipeline == VK_NULL_HANDLE, "color-only custom shaders never compile depth variants");
	// Resize can change depth attachment requirements in either direction.
	require(onix_gpu_resize_render_target(driver, target, 32, 32, 0, 1, &color), "resize to depth-enabled target");
	require(driver->render_targets[target - 1].depth_memory != VK_NULL_HANDLE, "Depth=true restores depth storage");
	require(onix_gpu_begin_frame(driver, clear) && onix_gpu_begin_render_target(driver, target, clear, 0, 1, 1), "begin resized depth target");
	onix_gpu_set_viewport(driver, 0, 0, 16, 16);
	require(onix_gpu_draw_sprites(driver, &push, &sprite, 1, &sprite_draw, 1), "reuse custom sprite shader with depth attachment");
	require(onix_gpu_draw_meshes(driver, &mesh_push, vertices, 3, indices, 3, &instance, 1,
		deformation, 16, &mesh_draw, 1, lighting), "reuse custom mesh shader with depth attachment");
	require(onix_gpu_end_render_target(driver) && onix_gpu_present(driver), "finish resized depth target");
	require(driver->mesh_pipelines[mesh_shader - 1].opaque != VK_NULL_HANDLE &&
		driver->material2d_pipelines[sprite_shader - 1].pipeline != VK_NULL_HANDLE, "custom depth variants compile on first use");
	require(onix_gpu_resize_render_target(driver, target, 16, 16, 0, 0, &color), "resize back to color-only target");
	require(driver->render_targets[target - 1].depth_memory == VK_NULL_HANDLE, "resizing back removes depth storage");
	// Integer targets can also omit depth; picking requires depth explicitly.
	uint32_t integer_target = onix_gpu_create_render_target(driver, 16, 16, 1, 0, &color);
	require(integer_target && !driver->render_targets[integer_target - 1].depth_image, "integer color-only target owns no depth image");
	require(onix_gpu_begin_frame(driver, clear), "begin resized color-only frame");
	draw_target(driver, target, texture);
	require(onix_gpu_begin_render_target(driver, integer_target, clear, 0, 1, 1), "begin integer color-only target");
	require(!onix_gpu_draw_mesh_picking(driver, &mesh_push, vertices, 3, indices, 3, &instance, 1, &mesh_draw, 1), "picking rejects target without depth");
	require(onix_gpu_end_render_target(driver) && onix_gpu_present(driver), "finish resized color-only frame");
	uint32_t identifier = 1;
	require(onix_gpu_read_render_target(driver, integer_target, 0, 0, 1, 1, (uint8_t *)&identifier, 4) && identifier == 0, "integer color-only clear and readback");
	require(onix_gpu_destroy_render_target(driver, target) && onix_gpu_destroy_render_target(driver, integer_target) &&
		onix_gpu_destroy_texture(driver, texture) && onix_gpu_destroy_mesh_shader(driver, mesh_shader) &&
		onix_gpu_destroy_material2d_shader(driver, sprite_shader), "retire color-only resources and custom modules");
	puts("Color-only targets: no depth allocations, builtin/custom sprite/shape/mesh variants, load passes, resize, integer clear/readback and zero steady allocations passed");
}

#if defined(__GLIBC__)
static void test_host_heap_retention(void) {
	OnixGPUDriver local = {.physical_device_properties = {.deviceType = VK_PHYSICAL_DEVICE_TYPE_CPU}};
	size_t calls = host_heap_trim_calls, heap = host_allocations, resources = resource_allocations;
	for (int index = 0; index < 3; index++) onix_vk_retire_host_memory(&local, ONIX_VK_HOST_HEAP_RELEASE_BYTES / 4);
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) onix_vk_release_idle_host_memory(&local);
	require(host_heap_trim_calls == calls, "small retirements never trigger process heap trimming");
	onix_vk_retire_host_memory(&local, ONIX_VK_HOST_HEAP_RELEASE_BYTES / 4);
	for (uint32_t frame = 1; frame < ONIX_BUFFER_TRIM_DELAY; frame++) onix_vk_release_idle_host_memory(&local);
	require(host_heap_trim_calls == calls, "large retirement waits for a quiet window");
	onix_vk_note_host_activity(&local);
	for (uint32_t frame = 1; frame < ONIX_BUFFER_TRIM_DELAY; frame++) onix_vk_release_idle_host_memory(&local);
	require(host_heap_trim_calls == calls, "renewed allocation pressure postpones heap trimming");
	onix_vk_release_idle_host_memory(&local);
	require(host_heap_trim_calls == calls + 1 && local.retired_host_bytes == 0, "quiet retirement trims once and consumes its request");
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 4; frame++) onix_vk_release_idle_host_memory(&local);
	require(host_heap_trim_calls == calls + 1, "ordinary idle frames never repeatedly trim the process heap");
	local.physical_device_properties.deviceType = VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU;
	onix_vk_retire_host_memory(&local, UINT64_MAX);
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 2; frame++) onix_vk_release_idle_host_memory(&local);
	require(host_heap_trim_calls == calls + 1, "hardware Vulkan does not enter software-driver heap policy");
	local.physical_device_properties.deviceType = VK_PHYSICAL_DEVICE_TYPE_CPU;
	OnixVkFrame completed = {.command_count = 32768};
	onix_vk_observe_commands(&local, &completed);
	require(completed.command_count == 0 && completed.command_peak == 32768, "completed command peak is retained without trimming");
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 3; frame++) {
		completed.command_count = 16;
		onix_vk_observe_commands(&local, &completed);
		onix_vk_release_idle_host_memory(&local);
	}
	require(host_heap_trim_calls == calls + 2 && completed.command_peak == ONIX_VK_COMMANDS_WARM,
		"completed command peak schedules one trim after sustained low load");
	completed = (OnixVkFrame){.command_count = 32768};
	onix_vk_observe_commands(&local, &completed);
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY * 3; frame++) {
		completed.command_count = frame % 7 == 0 ? 16384 : 16;
		onix_vk_observe_commands(&local, &completed);
		onix_vk_release_idle_host_memory(&local);
	}
	require(host_heap_trim_calls == calls + 2 && host_allocations == heap && resource_allocations == resources,
		"intermittent command bursts prevent trimming; maintenance allocates nothing");
	puts("Host heap release: software CPU driver only, 4 MiB retirement threshold, quiet window, activity postponement, command peaks, once per burst and zero allocations passed");
}
#endif

static void test_deformation_descriptor_pools(OnixGPUDriver *driver) {
	// Mesh resources are lazy; this isolated allocator test owns its layout.
	require(driver->deformation_layout == VK_NULL_HANDLE, "deformation layout starts lazy");
	VkDescriptorSetLayoutBinding bindings[] = {
		{.binding = 0, .descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER, .descriptorCount = 1, .stageFlags = VK_SHADER_STAGE_VERTEX_BIT},
		{.binding = 1, .descriptorType = VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, .descriptorCount = 1, .stageFlags = VK_SHADER_STAGE_FRAGMENT_BIT},
	};
	VkDescriptorSetLayoutCreateInfo layout = {.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO, .bindingCount = 2, .pBindings = bindings};
	require(vkCreateDescriptorSetLayout(driver->device, &layout, NULL, &driver->deformation_layout) == VK_SUCCESS, "create test deformation layout");
	OnixVkFrame frame = {0};
	size_t live = live_descriptor_pools;
	fail_next_descriptor_pool = 1;
	require(!onix_vk_deformation_descriptor(driver, &frame) && !frame.deformation_pools[0], "deformation pool failure commits no owner");
	fail_next_descriptor_set = 1;
	require(!onix_vk_deformation_descriptor(driver, &frame) && !frame.deformation_pools[0] && live_descriptor_pools == live,
		"failed first deformation set releases its new pool");
	VkDescriptorSet first = onix_vk_deformation_descriptor(driver, &frame);
	require(first && last_pool_sets == 16 && live_descriptor_pools == live + 1, "first mesh descriptor reserves 16 sets");
	fail_next_descriptor_set = 1;
	require(!onix_vk_deformation_descriptor(driver, &frame) && frame.deformation_set_count == 1 && frame.deformation_set_index == 1,
		"failed allocation in existing deformation pool preserves its cursor");
	for (uint32_t index = 1; index < ONIX_VK_DEFORMATION_SETS; index++) require(onix_vk_deformation_descriptor(driver, &frame) != VK_NULL_HANDLE, "grow deformation pools to limit");
	size_t heap = host_allocations, resources = resource_allocations;
	require(!onix_vk_deformation_descriptor(driver, &frame) && host_allocations == heap && resource_allocations == resources,
		"deformation descriptor limit rejects without allocating");
	require(live_descriptor_pools == live + 16, "full deformation workload uses 16 small pools");
	onix_vk_reset_deformation_descriptors(driver, &frame);
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY; iteration++) {
		require(onix_vk_deformation_descriptor(driver, &frame) == first, "deformation first handle stays stable");
		onix_vk_reset_deformation_descriptors(driver, &frame);
		if (iteration + 1 < ONIX_BUFFER_TRIM_DELAY) require(live_descriptor_pools == live + 16, "deformation peak survives short low demand");
	}
	require(live_descriptor_pools == live + 1 && frame.deformation_set_count == 16 && frame.deformation_pools[1] == VK_NULL_HANDLE,
		"deformation peak releases unused tail pools after sustained low demand");
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		require(onix_vk_deformation_descriptor(driver, &frame) == first, "shrunken deformation pool stays reusable");
		onix_vk_reset_deformation_descriptors(driver, &frame);
	}
	require(host_allocations == heap && resource_allocations == resources, "deformation trimming and warm reuse allocate nothing");
	for (uint32_t index = 0; index < ONIX_VK_DEFORMATION_POOL_SETS; index++) require(onix_vk_deformation_descriptor(driver, &frame) != VK_NULL_HANDLE, "reuse retained deformation pool");
	fail_next_descriptor_set = 1;
	require(!onix_vk_deformation_descriptor(driver, &frame) && frame.deformation_set_count == 16 && !frame.deformation_pools[1] && live_descriptor_pools == live + 1,
		"failed regrowth releases its uncommitted tail pool");
	require(onix_vk_deformation_descriptor(driver, &frame) && frame.deformation_sets[0] == first && frame.deformation_set_count == 17,
		"deformation pools regrow without disturbing retained handles");
	onix_vk_reset_deformation_descriptors(driver, &frame);
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		uint32_t demand = iteration % 7 == 0 ? 9 : 1;
		for (uint32_t index = 0; index < demand; index++) require(onix_vk_deformation_descriptor(driver, &frame) != VK_NULL_HANDLE, "reuse intermittent deformation workload");
		onix_vk_reset_deformation_descriptors(driver, &frame);
		require(frame.deformation_pools[1] != VK_NULL_HANDLE, "regular deformation bursts keep second pool warm");
	}
	for (uint32_t index = 0; index < ONIX_VK_DEFORMATION_SETS / ONIX_VK_DEFORMATION_POOL_SETS; index++) {
		if (frame.deformation_pools[index]) vkDestroyDescriptorPool(driver->device, frame.deformation_pools[index], NULL);
	}
	require(live_descriptor_pools == live, "deformation descriptor pools release exactly once");
	vkDestroyDescriptorSetLayout(driver->device, driver->deformation_layout, NULL);
	driver->deformation_layout = VK_NULL_HANDLE;
	puts("Deformation descriptor pools: 256 -> 16 reserved sets, failures, capacity, retained handles, regrowth and zero warm allocations passed");
}

static void test_upload_queue_retention(OnixGPUDriver *driver) {
	OnixGPUDriver local = {0};
	// Only exercise CPU queue ownership here; no commands use this image.
	OnixVkTexture textures[2] = {{.image = driver->images[0]}, {.image = driver->images[0]}};
	require(onix_vk_grow_uploads(&local, 4096), "reserve upload queue peak");
	for (uint32_t index = 0; index < 4096; index++) onix_vk_append_texture_upload(&local, &textures[index % 2], (OnixVkTextureUpload){.image = textures[index % 2].image, .x = index});
	uint64_t peak = (uint64_t)local.upload_capacity * sizeof(*local.uploads);
	onix_vk_reset_uploads(&local);
	onix_vk_trim_uploads(&local);
	require(local.upload_capacity == 4096 && local.upload_retention.frames == 0, "flushed queue peak still resets retention window");
	for (uint32_t index = 0; index < 4; index++) onix_vk_append_texture_upload(&local, &textures[index % 2], (OnixVkTextureUpload){.image = textures[index % 2].image, .x = index});
	onix_vk_cancel_texture_uploads(&local, &textures[0]);
	OnixVkTextureUpload expected[4];
	memcpy(expected, local.uploads, sizeof(expected));
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		if (iteration == ONIX_BUFFER_TRIM_DELAY - 1) fail_next_reallocation = 1;
		onix_vk_trim_uploads(&local);
		require(local.upload_count == 4 && local.upload_live_count == 2 && memcmp(expected, local.uploads, sizeof(expected)) == 0,
			"queue shrink preserves pending order, cancellation holes and indices");
		if (iteration < ONIX_BUFFER_TRIM_DELAY * 2 - 1) require(local.upload_capacity == 4096, "failed queue shrink keeps allocation until next window");
	}
	require(local.upload_capacity == 32, "upload queue metadata releases historical peak");
	onix_vk_cancel_texture_uploads(&local, &textures[1]);
	require(local.upload_count == 0 && local.upload_live_count == 0, "cancel heads remain valid after queue shrink");
	onix_vk_cancel_texture_uploads(&local, &textures[0]);
	size_t heap = host_allocations;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		require(onix_vk_grow_uploads(&local, 1), "reuse shrunken upload queue");
		onix_vk_append_texture_upload(&local, &textures[0], (OnixVkTextureUpload){.image = textures[0].image});
		onix_vk_reset_uploads(&local);
		onix_vk_trim_uploads(&local);
	}
	require(host_allocations == heap, "warm upload queue allocates nothing");
	printf("Upload queue metadata: %llu -> %llu bytes; pending records, cancellation, failed shrink and zero warm allocations passed\n",
		(unsigned long long)peak, (unsigned long long)((uint64_t)local.upload_capacity * sizeof(*local.uploads)));
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		uint32_t demand = iteration % 7 == 0 ? 129 : 1;
		require(onix_vk_grow_uploads(&local, demand), "regrow upload queue for intermittent demand");
		for (uint32_t index = 0; index < demand; index++) onix_vk_append_texture_upload(&local, &textures[0], (OnixVkTextureUpload){.image = textures[0].image, .x = index});
		onix_vk_reset_uploads(&local);
		onix_vk_trim_uploads(&local);
		require(local.upload_capacity == 256, "already-flushed intermittent upload bursts stay warm");
	}
	free(local.uploads);
}

static void test_native_table_retention(OnixGPUDriver *driver) {
	OnixGPUDriver local = {.device = driver->device, .physical_device = driver->physical_device};
	OnixVkFrame *frame = &local.frames[0];
	require(onix_vk_append_instance_buffer(&local, frame, 64) && onix_vk_append_staging_buffer(&local, frame, 64), "create live buffers for table retention");
	VkBuffer instance = frame->instance_buffers[0].buffer, staging = frame->staging_buffers[0].buffer;
	memset(frame->instance_buffers[0].mapping, 23, 64);
	memset(frame->staging_buffers[0].mapping, 29, 64);
	// Simulate tables left after GPU peak buffers have already been retired.
	frame->instance_buffers = realloc(frame->instance_buffers, 1024 * sizeof(*frame->instance_buffers));
	frame->staging_buffers = realloc(frame->staging_buffers, 1024 * sizeof(*frame->staging_buffers));
	frame->retired_targets = calloc(1024, sizeof(*frame->retired_targets));
	require(frame->instance_buffers && frame->staging_buffers && frame->retired_targets, "reserve peak native tables");
	frame->instance_buffer_capacity = frame->staging_buffer_capacity = frame->retired_target_capacity = 1024;
	OnixBufferMemory peak, small;
	onix_gpu_buffer_memory(&local, &peak);
	size_t resources = resource_allocations;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		if (iteration == ONIX_BUFFER_TRIM_DELAY - 1) fail_next_reallocation = 1;
		onix_vk_trim_instance_buffers(&local, frame);
		onix_vk_trim_staging_buffers(&local, frame);
		onix_vk_collect_resources(&local, 0);
		if (iteration == ONIX_BUFFER_TRIM_DELAY - 1) require(frame->instance_buffer_capacity == 1024, "failed native table shrink preserves capacity");
	}
	require(frame->instance_buffer_capacity == 4 && frame->staging_buffer_capacity == 2 && frame->retired_target_capacity == 4, "native tables release peak metadata");
	require(frame->instance_buffers[0].buffer == instance && frame->staging_buffers[0].buffer == staging &&
		((uint8_t *)frame->instance_buffers[0].mapping)[0] == 23 && ((uint8_t *)frame->staging_buffers[0].mapping)[0] == 29,
		"metadata shrink preserves live GPU handles and mappings");
	onix_gpu_buffer_memory(&local, &small);
	size_t heap = host_allocations;
	for (uint32_t iteration = 0; iteration < ONIX_BUFFER_TRIM_DELAY * 2; iteration++) {
		onix_vk_trim_instance_buffers(&local, frame);
		onix_vk_trim_staging_buffers(&local, frame);
		onix_vk_collect_resources(&local, 0);
	}
	require(host_allocations == heap && resource_allocations == resources && small.instances == peak.instances && small.staging == peak.staging,
		"warm table reuse allocates nothing and keeps GPU capacity");
	printf("Vulkan buffer/retirement tables: %llu -> %llu bytes; failed shrink, live mappings and zero warm allocations passed\n",
		(unsigned long long)peak.scratch, (unsigned long long)small.scratch);
	onix_vk_destroy_instance_buffers(&local, frame);
	onix_vk_destroy_staging_buffers(&local, frame);
	free(frame->retired_targets);
}

static void test_texture_descriptor_pools(OnixGPUDriver *driver) {
	OnixGPUDriver local = {.device = driver->device, .texture_layout = driver->texture_layout};
	OnixVkTexture *textures = calloc(ONIX_VK_MAX_TEXTURES + 1, sizeof(*textures));
	require(textures != NULL, "allocate descriptor ownership workload");
	size_t live = live_descriptor_pools;
	fail_next_descriptor_pool = 1;
	require(!onix_vk_allocate_texture_descriptor(&local, &textures[0]) && local.texture_descriptor_pool_count == 0, "pool creation failure commits no owner");
	fail_next_descriptor_set = 1;
	require(!onix_vk_allocate_texture_descriptor(&local, &textures[0]) && local.texture_descriptor_pool_count == 0 && live_descriptor_pools == live,
		"failed first set allocation releases its new pool");
	require(onix_vk_allocate_texture_descriptor(&local, &textures[0]) && last_pool_sets == 64 && local.texture_descriptor_pool_count == 1,
		"first texture reserves 64 descriptor sets rather than 4096");
	fail_next_descriptor_set = 1;
	require(!onix_vk_allocate_texture_descriptor(&local, &textures[1]) && local.texture_descriptor_pools[0].count == 1 && !textures[1].descriptor,
		"failed allocation in an existing pool preserves its live lease");
	for (uint32_t index = 1; index < ONIX_VK_MAX_TEXTURES; index++) {
		require(onix_vk_allocate_texture_descriptor(&local, &textures[index]), "grow descriptor blocks to texture limit");
		require(textures[index].descriptor_pool == index / ONIX_VK_TEXTURE_DESCRIPTOR_SETS + 1, "descriptor records its owning pool");
	}
	require(local.texture_descriptor_pool_count == ONIX_VK_TEXTURE_DESCRIPTOR_POOLS && live_descriptor_pools == live + ONIX_VK_TEXTURE_DESCRIPTOR_POOLS,
		"descriptor blocks grow to the full texture capacity");
	size_t heap = host_allocations, resources = resource_allocations;
	require(!onix_vk_allocate_texture_descriptor(&local, &textures[ONIX_VK_MAX_TEXTURES]) && host_allocations == heap && resource_allocations == resources,
		"full descriptor capacity rejects without changing GPU resources");
	onix_vk_release_texture_descriptor(&local, &textures[0]);
	require(onix_vk_allocate_texture_descriptor(&local, &textures[ONIX_VK_MAX_TEXTURES]) && textures[ONIX_VK_MAX_TEXTURES].descriptor_pool == 1,
		"freed descriptor capacity reuses its existing pool");
	onix_vk_release_texture_descriptor(&local, &textures[ONIX_VK_MAX_TEXTURES]);
	for (uint32_t index = 1; index < ONIX_VK_MAX_TEXTURES - 1; index++) onix_vk_release_texture_descriptor(&local, &textures[index]);
	for (uint32_t frame = 1; frame < ONIX_BUFFER_TRIM_DELAY; frame++) {
		onix_vk_trim_texture_descriptors(&local);
		require(live_descriptor_pools == live + ONIX_VK_TEXTURE_DESCRIPTOR_POOLS, "empty blocks survive short idle periods");
	}
	heap = host_allocations; resources = resource_allocations;
	onix_vk_trim_texture_descriptors(&local);
	require(live_descriptor_pools == live + 2 && local.texture_descriptor_pools[0].pool &&
		local.texture_descriptor_pools[ONIX_VK_TEXTURE_DESCRIPTOR_POOLS - 1].count == 1, "idle blocks release while a live tail descriptor and first warm block survive");
	require(host_allocations == heap && resource_allocations == resources, "descriptor block trimming allocates nothing");
	onix_vk_release_texture_descriptor(&local, &textures[ONIX_VK_MAX_TEXTURES - 1]);
	for (uint32_t frame = 0; frame < ONIX_BUFFER_TRIM_DELAY; frame++) onix_vk_trim_texture_descriptors(&local);
	require(live_descriptor_pools == live + 1, "one empty texture descriptor block stays warm");
	for (uint32_t index = 0; index <= ONIX_VK_TEXTURE_DESCRIPTOR_SETS; index++) require(onix_vk_allocate_texture_descriptor(&local, &textures[index]), "regrow using holes in pool table");
	require(textures[ONIX_VK_TEXTURE_DESCRIPTOR_SETS].descriptor_pool == 2 && host_allocations == heap, "pool table holes reuse without C metadata allocation");
	for (uint32_t index = 0; index <= ONIX_VK_TEXTURE_DESCRIPTOR_SETS; index++) onix_vk_release_texture_descriptor(&local, &textures[index]);
	for (uint32_t index = 0; index < local.texture_descriptor_pool_count; index++) {
		if (local.texture_descriptor_pools[index].pool) vkDestroyDescriptorPool(local.device, local.texture_descriptor_pools[index].pool, NULL);
	}
	require(live_descriptor_pools == live, "all local descriptor pool leases release exactly once");
	free(textures);
	puts("Texture descriptor blocks: 64-set startup, 4096-set limit, allocation failures, pool ownership, idle reclaim and hole reuse passed");
}

int main(void) {
	test_native_slot_lifecycle();
#if defined(__GLIBC__)
	test_host_heap_retention();
#endif
	Display *display = XOpenDisplay(NULL);
	require(display != NULL, "open X display");
	Window window = XCreateSimpleWindow(display, DefaultRootWindow(display), 0, 0, 64, 64, 0, 0, 0);
	XMapWindow(display, window);
	XSync(display, False);
	OnixGPUShader shaders[] = {
		load_shader("shaders/vulkan_sprite_multi.vert.spv"), load_shader("shaders/vulkan_sprite_multi.frag.spv"),
		load_shader("shaders/vulkan_shape.vert.spv"), load_shader("shaders/vulkan_shape.frag.spv"),
		load_shader("shaders/vulkan_mesh.vert.spv"), load_shader("shaders/vulkan_mesh.frag.spv"),
		load_shader("shaders/vulkan_picking.vert.spv"), load_shader("shaders/vulkan_picking.frag.spv"),
	};
	OnixGPUSurface surface = {.kind = ONIX_GPU_SURFACE_XLIB, .display = display, .window = window, .width = 64, .height = 64};
	OnixGPUShader unlit_shader = load_shader("shaders/vulkan_mesh_unlit.frag.spv");
	OnixGPUShader depth_shader = load_shader("shaders/vulkan_mesh_depth.frag.spv"), basic_shader = load_shader("shaders/vulkan_mesh_basic.frag.spv");
	OnixGPUDriver *driver = onix_gpu_create(surface, shaders[0], shaders[1], shaders[2], shaders[3], shaders[4], shaders[5], unlit_shader, depth_shader, basic_shader, shaders[6], shaders[7]);
	require(driver != NULL, "create Vulkan driver");
	require(driver->texture_descriptor_pool_count == 0, "startup reserves no texture descriptor pool before first texture");
	test_texture_descriptor_pools(driver);
	test_deformation_descriptor_pools(driver);
	test_upload_queue_retention(driver);
	test_native_table_retention(driver);
	test_buffer_retention(driver);
	test_surface_2d_laziness(driver);
	test_mesh_memory_reuse(driver);
	test_texture_addressing(driver);
	test_texture_update_batch(driver);
	test_interleaved_upload_cancellation(driver);
	require(driver->mesh_picking_pipeline == VK_NULL_HANDLE, "2D avoids picking pipeline");
	for (uint32_t index = 0; index < ONIX_VK_FRAMES_IN_FLIGHT; index++) {
		require(driver->frames[index].deformation_pools[0] == VK_NULL_HANDLE, "2D avoids mesh descriptor pools");
	}
	test_mesh_pipeline_laziness(driver);
	test_surface_depth(driver);
	test_target_upload_resume(driver, 1);
	test_target_upload_resume(driver, 0);
	test_mesh_lighting(driver);
	test_screen_reflections(driver);
	test_mesh_occlusion(driver);
	test_material_layers(driver);
	test_static_deformation_residency(driver);
	test_material_fresnel(driver);
	test_fxaa(driver);
	test_camera_color(driver);
	test_readback_retention(driver);
	test_lcd_text_blend(driver);
	test_color_only_targets(driver);
	OnixGPUShader custom_vertex = load_shader("shaders/vulkan_sprite.vert.spv"), custom_fragment = load_shader("shaders/vulkan_sprite.frag.spv");
	uint32_t custom = onix_gpu_create_material2d_shader(driver, custom_vertex, custom_fragment);
	free((void *)custom_vertex.code);
	free((void *)custom_fragment.code);
	require(custom != 0, "create single-texture custom pipeline");
	const uint8_t multi_pixels[4][4] = {{255, 0, 0, 255}, {0, 255, 0, 128}, {0, 0, 255, 255}, {128}};
	uint32_t multi_textures[4];
	for (uint32_t index = 0; index < 4; index++) {
		uint8_t data[4 * 4 * 4];
		uint32_t bpp = index == 3 ? 1 : 4, width = index + 1;
		for (uint32_t pixel = 0; pixel < width * width; pixel++) memcpy(data + pixel * bpp, multi_pixels[index], bpp);
		multi_textures[index] = onix_gpu_create_texture_with_format(driver, width, width, index % 2 ? onix_gpu_texture_mip_level_count(width, width) : 1, index == 3 ? ONIX_TEXTURE_FORMAT_R8 : ONIX_TEXTURE_FORMAT_RGBA8, index % 2, data, width * width * bpp, 0, 0);
		require(multi_textures[index] != 0, "create multi-texture source");
	}
	test_surface_descriptor_cache(driver);
	test_draw_descriptor_reuse(driver, multi_textures, custom);
	const size_t length = 1024 * 1024 * 4;
	uint8_t *pixels = malloc(length);
	require(pixels != NULL, "allocate source pixels");
	fill_pixels(pixels, length, 255, 0);
	uint32_t texture = onix_gpu_create_texture_with_format(driver, 1024, 1024, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, pixels, length, 0, 0);
	require(texture != 0, "create texture");
	uint32_t color;
	uint32_t first = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
	uint32_t second = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
	uint32_t picking = onix_gpu_create_render_target(driver, 16, 16, 1, 1, &color);
	uint32_t multi_target = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
	require(multi_target != 0, "create multi-texture target");
	require(first != 0 && second != 0 && picking != 0, "create targets");
	OnixGPUTextureUpdate update = {.texture = texture, .width = 1024, .height = 1024, .data_length = length};
	const float clear[] = {0, 0, 0, 1};
	for (int frame = 0; frame < 12; frame++) {
		if (frame == 8) host_allocations = resource_allocations = 0;
		uint8_t red = (uint8_t)(frame + 100);
		uint8_t green = (uint8_t)(frame + 150);
		if (frame >= 4 && frame % 2 == 0) {
			require(onix_gpu_begin_frame(driver, clear), "begin idle frame");
			require(onix_gpu_present(driver), "present idle frame");
		}
		fill_pixels(pixels, length, red, 0);
		require(onix_gpu_update_textures(driver, &update, 1, pixels, length), "upload before begin");
		fill_pixels(pixels, length, 0, green);
		require(onix_gpu_begin_frame(driver, clear), "begin frame");
		test_external_queue(driver, texture);
		draw_target(driver, first, texture);
		require(onix_gpu_update_textures(driver, &update, 1, pixels, length), "upload after begin");
		memset(pixels, 0, length);
		draw_target(driver, second, texture);
		draw_mesh(driver, texture, 0);
		draw_mesh(driver, texture, 0);
		draw_mesh(driver, texture, picking);
		draw_multi_texture(driver, multi_target, multi_textures, custom, frame);
		require(onix_gpu_present(driver), "present");
		check_pixel(driver, first, red, 0, frame);
		check_pixel(driver, second, 0, green, frame);
		check_multi_texture(driver, multi_target);
		uint32_t identifier = 0;
		require(onix_gpu_read_render_target(driver, picking, 8, 8, 1, 1, (uint8_t *)&identifier, sizeof(identifier)), "read picking");
		require(identifier == 1234567, "picking identifier");
		uint8_t region[16 * 16 * 4];
		require(onix_gpu_read_render_target(driver, second, 0, 0, 16, 16, region, sizeof(region)), "read larger region within capacity");
	}
	if (host_allocations != 0 || resource_allocations != 0) {
		fprintf(stderr, "steady-state C heap allocations: %zu; Vulkan resource calls: %zu\n", host_allocations, resource_allocations);
		exit(1);
	}
	OnixBufferMemory memory;
	onix_gpu_buffer_memory(driver, &memory);
	VkDeviceSize instances = 0, staging = 0;
	uint64_t scratch = (uint64_t)driver->upload_capacity * sizeof(*driver->uploads);
	for (uint32_t index = 0; index < ONIX_VK_FRAMES_IN_FLIGHT; index++) {
		OnixVkFrame *slot = &driver->frames[index];
		for (uint32_t buffer = 0; buffer < slot->instance_buffer_count; buffer++) instances += slot->instance_buffers[buffer].capacity;
		for (uint32_t buffer = 0; buffer < slot->staging_buffer_count; buffer++) staging += slot->staging_buffers[buffer].capacity;
		scratch += onix_upload_cache_bytes(&slot->mesh_uploads) + (uint64_t)slot->surface_capacity * (sizeof(*slot->surface_sets) + 8) +
			(uint64_t)slot->instance_buffer_capacity * sizeof(*slot->instance_buffers) +
			(uint64_t)slot->staging_buffer_capacity * sizeof(*slot->staging_buffers) +
			(uint64_t)slot->retired_target_capacity * sizeof(*slot->retired_targets);
	}
	require(memory.instances == instances && memory.staging == staging && memory.readback == driver->readback.capacity && memory.scratch == scratch,
		"reported native capacities match live buffer pools");
	printf("4 steady-state upload frames plus 4 idle frames: C heap=%zu, Vulkan resource calls=%zu; 8 MiB uploads/frame, multi-texture RGBA/R8 sprites, custom shaders, meshes, picking and readback passed\n", host_allocations, resource_allocations);
	fill_pixels(pixels, length, 255, 0);
	uint32_t cancelled = onix_gpu_create_texture_with_format(driver, 1024, 1024, 1, ONIX_TEXTURE_FORMAT_RGBA8, 0, pixels, length, 0, 0);
	require(cancelled != 0, "create cancelled upload");
	driver->pending_external[driver->pending_external_count++] = cancelled - 1;
	driver->textures[cancelled - 1].pending_external_index = driver->pending_external_count;
	require(onix_gpu_destroy_texture(driver, cancelled), "cancel texture");
	require(driver->upload_count == 0 && driver->pending_external_count == 0, "cancel pending queues");
	require(onix_gpu_begin_frame(driver, clear), "begin after cancellation");
	require(onix_gpu_present(driver), "present after cancellation");
	uint32_t uninitialized = onix_gpu_create_render_target(driver, 16, 16, 0, 1, &color);
	uint8_t pixel[4];
	require(uninitialized != 0, "create unreadable target");
	require(!onix_gpu_read_render_target(driver, uninitialized, 0, 0, 1, 1, pixel, sizeof(pixel)), "reject uninitialized target");
	require(onix_gpu_destroy_render_target(driver, first), "destroy target");
	require(!onix_gpu_read_render_target(driver, first, 0, 0, 1, 1, pixel, sizeof(pixel)), "reject destroyed target");
	onix_gpu_destroy(driver);
	require(live_shader_modules == 0 && live_pipelines == 0 && live_descriptor_pools == 0, "all shader modules, pipeline variants and descriptor pools are released");
	free((void *)unlit_shader.code);
	free((void *)depth_shader.code);
	free((void *)basic_shader.code);
	free(pixels);
	for (size_t index = 0; index < sizeof(shaders) / sizeof(shaders[0]); index++) free((void *)shaders[index].code);
	XDestroyWindow(display, window);
	XCloseDisplay(display);
	return 0;
}
