//go:build cgo && (onix_vulkan || android) && (windows || linux || android || (darwin && !ios)) && !onix_directx && !onix_metal && !onix_opengl && !onix_webgpu && !onix_webgl


#if defined(_WIN32)
#define COBJMACROS
#endif

#if defined(__ANDROID__)
#define VK_USE_PLATFORM_ANDROID_KHR
#elif defined(_WIN32)
#define VK_USE_PLATFORM_WIN32_KHR
#define VK_NO_PROTOTYPES
#elif !defined(__APPLE__) && !defined(ONIX_VK_DRM)
#define VK_USE_PLATFORM_XLIB_KHR
#define VK_USE_PLATFORM_WAYLAND_KHR
#endif

#include "native_gpu.h"
#include "native_slots.h"
#include "native_buffer.h"
#include "native_upload.h"
#include "native_buddy.h"

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#if defined(__GLIBC__)
#include <malloc.h>
#endif
#include "vulkan/vulkan.h"

#if defined(__ANDROID__)
#include <android/native_window.h>
#elif defined(_WIN32)
#include <windows.h>
#include <d3d11.h>
#include <dxgi1_2.h>
#else
#if !defined(__APPLE__) && !defined(ONIX_VK_DRM)
#include <X11/Xlib.h>
#include <wayland-client.h>
#endif
#include <unistd.h>
#endif

#define ONIX_VK_MAX_TEXTURES 4096
#define ONIX_VK_TEXTURE_DESCRIPTOR_SETS 64
#define ONIX_VK_TEXTURE_DESCRIPTOR_POOLS (ONIX_VK_MAX_TEXTURES / ONIX_VK_TEXTURE_DESCRIPTOR_SETS)
#define ONIX_VK_MAX_EXTERNAL_TEXTURES 16
#define ONIX_VK_INSTANCE_BUFFER_BLOCK_SIZE (64u * 1024u)
#define ONIX_VK_MESH_BLOCK_SIZE (1u * 1024u * 1024u)
#define ONIX_VK_STAGING_BUFFER_BLOCK_SIZE (256u * 1024u)
#define ONIX_VK_STAGING_BURST_LIMIT (64u * 1024u * 1024u)
#define ONIX_VK_FRAMES_IN_FLIGHT 2
#define ONIX_VK_DEFORMATION_SETS 256
#define ONIX_VK_DEFORMATION_POOL_SETS 16
#define ONIX_GPU_SURFACE_TEXTURES 14
#define ONIX_GPU_SURFACE_SETS 4096
#define ONIX_GPU_SURFACE_POOL_SETS 64
#define ONIX_MESH_BLEND_MODE_COUNT 4
#define ONIX_VK_PIPELINE_NO_DEPTH 16
#define ONIX_VK_PIPELINE_COMPACT_SPRITE 32
#define ONIX_VK_PIPELINE_R8 64
#define ONIX_VK_HOST_HEAP_RELEASE_BYTES (4u * 1024u * 1024u)
#define ONIX_VK_COMMANDS_WARM 1024u

#if defined(_WIN32)
#define ONIX_VK_WINDOWS_FUNCTIONS(X) \
    X(vkAcquireNextImageKHR) \
    X(vkAllocateCommandBuffers) \
    X(vkAllocateDescriptorSets) \
    X(vkAllocateMemory) \
    X(vkBeginCommandBuffer) \
    X(vkBindBufferMemory) \
    X(vkBindImageMemory) \
    X(vkCmdBeginRenderPass) \
    X(vkCmdBindDescriptorSets) \
    X(vkCmdBindIndexBuffer) \
    X(vkCmdBindPipeline) \
	X(vkCmdBindVertexBuffers) \
	X(vkCmdBlitImage) \
	X(vkCmdClearAttachments) \
	X(vkCmdCopyBufferToImage) \
	X(vkCmdCopyImageToBuffer) \
    X(vkCmdDrawIndexed) \
    X(vkCmdEndRenderPass) \
    X(vkCmdPipelineBarrier) \
    X(vkCmdPushConstants) \
    X(vkCmdSetBlendConstants) \
    X(vkCmdSetScissor) \
    X(vkCmdSetViewport) \
    X(vkCreateBuffer) \
    X(vkCreateCommandPool) \
    X(vkCreateDescriptorPool) \
    X(vkCreateDescriptorSetLayout) \
    X(vkCreateDevice) \
    X(vkCreateFence) \
    X(vkCreateFramebuffer) \
    X(vkCreateGraphicsPipelines) \
    X(vkCreatePipelineCache) \
    X(vkCreateImage) \
    X(vkCreateImageView) \
    X(vkCreateInstance) \
    X(vkCreatePipelineLayout) \
    X(vkCreateRenderPass) \
    X(vkCreateSampler) \
    X(vkCreateSemaphore) \
    X(vkCreateShaderModule) \
    X(vkCreateSwapchainKHR) \
    X(vkCreateWin32SurfaceKHR) \
    X(vkDestroyBuffer) \
    X(vkDestroyCommandPool) \
    X(vkDestroyDescriptorPool) \
    X(vkDestroyDescriptorSetLayout) \
    X(vkDestroyDevice) \
    X(vkDestroyFence) \
    X(vkDestroyFramebuffer) \
    X(vkDestroyImage) \
    X(vkDestroyImageView) \
    X(vkDestroyInstance) \
    X(vkDestroyPipeline) \
    X(vkDestroyPipelineCache) \
    X(vkDestroyPipelineLayout) \
    X(vkDestroyRenderPass) \
    X(vkDestroySampler) \
    X(vkDestroySemaphore) \
    X(vkDestroyShaderModule) \
    X(vkDestroySurfaceKHR) \
    X(vkDestroySwapchainKHR) \
    X(vkDeviceWaitIdle) \
    X(vkEndCommandBuffer) \
    X(vkEnumerateDeviceExtensionProperties) \
    X(vkEnumeratePhysicalDevices) \
    X(vkFreeCommandBuffers) \
    X(vkFreeDescriptorSets) \
    X(vkFreeMemory) \
    X(vkGetBufferMemoryRequirements) \
    X(vkGetDeviceQueue) \
    X(vkGetImageMemoryRequirements) \
    X(vkGetPhysicalDeviceFormatProperties) \
    X(vkGetPhysicalDeviceMemoryProperties) \
    X(vkGetPhysicalDeviceProperties) \
    X(vkGetPipelineCacheData) \
    X(vkGetPhysicalDeviceQueueFamilyProperties) \
    X(vkGetPhysicalDeviceSurfaceCapabilitiesKHR) \
    X(vkGetPhysicalDeviceSurfaceFormatsKHR) \
    X(vkGetPhysicalDeviceSurfacePresentModesKHR) \
    X(vkGetPhysicalDeviceSurfaceSupportKHR) \
    X(vkGetSwapchainImagesKHR) \
    X(vkMapMemory) \
    X(vkMergePipelineCaches) \
    X(vkQueuePresentKHR) \
    X(vkQueueSubmit) \
	X(vkQueueWaitIdle) \
	X(vkResetCommandBuffer) \
	X(vkResetDescriptorPool) \
    X(vkResetFences) \
    X(vkUnmapMemory) \
    X(vkUpdateDescriptorSets) \
    X(vkWaitForFences)

#define ONIX_VK_DECLARE(name) static PFN_##name name;
ONIX_VK_WINDOWS_FUNCTIONS(ONIX_VK_DECLARE)
#undef ONIX_VK_DECLARE
#endif

typedef struct {
	VkDeviceSize offset;
	VkDeviceSize size;
	int occupied;
} OnixVkMemoryRange;

// Small textures share device-local blocks: the driver charges each dedicated
// allocation separately, so one atlas alone can cost a 4 MB host page.
#define ONIX_VK_TEXTURE_POOL_BLOCK_SIZE (4u * 1024u * 1024u)
#define ONIX_VK_TEXTURE_POOL_MAX_IMAGE (4u * 1024u * 1024u)
#define ONIX_VK_TEXTURE_POOL_BLOCKS 8
#define ONIX_VK_TEXTURE_POOL_RANGES 48

typedef struct {
	VkDeviceMemory memory;
	uint32_t memory_type;
	uint32_t occupied_count;
	OnixVkMemoryRange ranges[ONIX_VK_TEXTURE_POOL_RANGES];
} OnixVkTexturePool;

typedef struct {
    VkImage image;
    VkDeviceMemory memory;
    VkImageView view;
    VkDescriptorSet descriptor;
	uint32_t descriptor_pool;
    uint32_t width;
	uint32_t height;
	uint32_t mip_levels;
	uint32_t format;
	uint32_t sampler_index;
	uint64_t upload_epoch;
	uint32_t upload_head;
	uint32_t pending_external_index;
	uint32_t pool;  // pool lease
	int external;
	int external_ready;
	int external_initialized;
	int destroy_pending;
	uint32_t retire_frame;
	uint32_t retire_next;
	uint32_t free_next;
} OnixVkTexture;

typedef struct {
	VkDescriptorPool pool;
	uint32_t count, idle_frames;
} OnixVkTextureDescriptorPool;

typedef struct {
    VkBuffer buffer;
    VkDeviceMemory memory;
    void *mapping;
    VkDeviceSize capacity;
    VkDeviceSize offset;
} OnixVkInstanceBuffer;

typedef OnixVkInstanceBuffer OnixVkStagingBuffer;

typedef struct {
	VkImage image;
	int32_t x;
	int32_t y;
	uint32_t width;
	uint32_t height;
	uint32_t image_width;
	uint32_t image_height;
	uint32_t mip_levels;
	/* mip_level is the level this upload writes. generate_mips asks the driver
	 * to blit the rest of the chain from it, which is only possible for
	 * uncompressed data. */
	uint32_t mip_level;
	int generate_mips;
	int initialize;
	VkBuffer staging;
	VkDeviceSize staging_offset;
	uint32_t next;
} OnixVkTextureUpload;

typedef struct {
	VkFramebuffer framebuffer;
	VkImage depth_image;
	VkImageView depth_view;
	VkImageView color_view; // identity view for R8 attachment; sampling uses the coverage swizzle
	VkDeviceMemory depth_memory;
} OnixVkRetiredRenderTarget;

typedef struct {
	VkDescriptorSet descriptor;
	uint64_t hash;
	uint32_t textures[ONIX_GPU_SURFACE_TEXTURES];
	uint32_t static_mesh;
} OnixVkSurfaceDescriptor;

typedef struct {
	VkCommandBuffer command_buffer;
	VkSemaphore image_available;
	VkSemaphore render_finished;
	VkFence fence;
	uint64_t command_count, command_peak;
	OnixBufferRetention command_retention;
	VkDescriptorPool deformation_pools[ONIX_VK_DEFORMATION_SETS / ONIX_VK_DEFORMATION_POOL_SETS];
	VkDescriptorSet deformation_sets[ONIX_VK_DEFORMATION_SETS];
	uint32_t deformation_set_count;
	uint32_t deformation_set_index;
	OnixBufferRetention deformation_retention;
	VkDescriptorPool surface_pools[ONIX_GPU_SURFACE_SETS / ONIX_GPU_SURFACE_POOL_SETS];
	OnixVkSurfaceDescriptor *surface_sets;
	uint32_t *surface_buckets;
	uint32_t surface_capacity;
	uint32_t surface_set_count;
	uint32_t surface_set_index;
	OnixBufferRetention surface_retention;
	OnixNativeUploadCache mesh_uploads;
	OnixVkInstanceBuffer *instance_buffers;
	uint32_t instance_buffer_count;
	uint32_t instance_buffer_capacity;
	uint32_t instance_buffer_index;
	VkDeviceSize instance_capacity;
	VkDeviceSize instance_bytes_used;
	OnixBufferRetention instance_retention, instance_table_retention;
	OnixVkStagingBuffer *staging_buffers;
	uint32_t staging_buffer_count;
	uint32_t staging_buffer_capacity;
	uint32_t staging_buffer_index;
	VkDeviceSize staging_capacity;
	OnixBufferRetention staging_retention, staging_table_retention;
	int staging_prepared;
	VkDeviceSize staging_reserved;
	OnixVkRetiredRenderTarget *retired_targets;
	uint32_t retired_target_count;
	uint32_t retired_target_capacity;
	OnixBufferRetention retired_target_retention;
	uint32_t retired_textures, retired_meshes, retired_mesh_pipelines, retired_material2d_pipelines, retired_render_targets;
} OnixVkFrame;

typedef struct {
	VkPipeline opaque;
	VkPipeline opaque_surface;
	VkPipeline transparent[ONIX_MESH_BLEND_MODE_COUNT];
	VkPipeline color_only[2][1 + ONIX_MESH_BLEND_MODE_COUNT];
	VkShaderModule vertex, fragment;
	int destroy_pending;
	uint32_t retire_frame;
	uint32_t retire_next;
	uint32_t free_next;
} OnixVkMeshPipeline;

typedef struct {
	VkPipeline pipeline, color_only[2];
	VkShaderModule vertex, fragment;
	int destroy_pending;
	uint32_t retire_frame;
	uint32_t retire_next;
	uint32_t free_next;
} OnixVkMaterial2DPipeline;

typedef struct {
	uint32_t color_texture;
	uint32_t color_format;
	VkFramebuffer framebuffer;
	VkImage depth_image;
	VkImageView depth_view;
	VkImageView color_view; // identity view for R8 attachment; sampling uses the coverage swizzle
	VkDeviceMemory depth_memory;
	uint32_t width;
	uint32_t height;
	int initialized;
	int destroy_pending;
	uint32_t retire_frame;
	uint32_t retire_next;
	uint32_t free_next;
} OnixVkRenderTarget;

typedef struct {
	uint32_t block;
	uint64_t node;
} OnixVkMeshLease;

typedef struct {
	VkBuffer buffer;
	VkDeviceMemory memory;
	void *mapping;
	OnixNativeBuddy buddy;
	uint32_t idle_frames;
} OnixVkMeshBlock;

typedef struct {
	VkBuffer vertex_buffer, index_buffer, deformation_buffer;
	VkDeviceSize vertex_offset, index_offset, deformation_offset, deformation_size;
	OnixVkMeshLease geometry, deformation;
	uint32_t vertex_count, index_count;
	int destroy_pending;
	uint32_t retire_frame, retire_next, free_next;
} OnixVkMesh;

ONIX_NATIVE_SLOTS(onix_vk_textures, OnixVkTexture, 64, ONIX_VK_MAX_TEXTURES)
ONIX_NATIVE_SLOTS(onix_vk_targets, OnixVkRenderTarget, 8, UINT32_MAX)
ONIX_NATIVE_SLOTS(onix_vk_meshes, OnixVkMesh, 16, UINT32_MAX)
ONIX_NATIVE_SLOTS(onix_vk_mesh_shaders, OnixVkMeshPipeline, 8, UINT32_MAX)
ONIX_NATIVE_SLOTS(onix_vk_material2d_shaders, OnixVkMaterial2DPipeline, 8, UINT32_MAX)

struct OnixGPUDriver {
    OnixGPUSurface native_surface;
    VkInstance instance;
    VkSurfaceKHR surface;
    VkPhysicalDevice physical_device;
    VkPhysicalDeviceProperties physical_device_properties;
#if defined(ONIX_VK_DRM)
    VkDisplayKHR drm_display;
    PFN_vkReleaseDisplayEXT release_display;
    int display_acquired;
#endif
    VkDevice device;
	VkPipelineCache pipeline_cache;
    uint32_t queue_family;
	VkQueue queue;
	int external_d3d11_supported;
	int external_dmabuf_supported;

    VkSwapchainKHR swapchain;
    VkFormat swapchain_format;
    VkExtent2D extent;
    uint32_t image_count;
    VkImage *images;
    VkImageView *image_views;
	VkImage *depth_images;
	VkImageView *depth_views;
	VkDeviceMemory *depth_memories;
    VkFramebuffer *framebuffers;
	VkFramebuffer *depth_framebuffers;

	VkRenderPass render_pass;
	VkRenderPass load_render_pass;
	VkRenderPass color_only_surface_passes[2];
	VkRenderPass target_render_passes[4];
	VkRenderPass picking_target_render_passes[4];
	VkRenderPass color_only_target_passes[3][2];
	VkDescriptorSetLayout texture_layout;
	VkDescriptorSetLayout deformation_layout;
	VkBuffer empty_deformation_buffer;
	VkDeviceMemory empty_deformation_memory;
	VkDescriptorSetLayout surface_layout;
	OnixVkTextureDescriptorPool texture_descriptor_pools[ONIX_VK_TEXTURE_DESCRIPTOR_POOLS];
	uint32_t texture_descriptor_pool_count;
    VkSampler texture_samplers[18];
	VkPipelineLayout sprite_layout;
	VkPipelineLayout shape_layout;
	VkPipelineLayout mesh_layout;
	VkPipeline sprite_pipeline;
	VkPipeline shape_pipeline;
	VkPipeline corner_mask_pipeline;
	OnixVkMeshPipeline mesh_builtin[ONIX_MESH_VARIANT_COUNT];
	VkPipeline mesh_picking_pipeline;
	VkPipeline lcd_pipeline;
	VkPipeline color_only_pipelines[2][11];
	OnixVkMeshPipeline *mesh_pipelines;
	OnixNativeSlots mesh_shader_slots;
	OnixVkMaterial2DPipeline *material2d_pipelines;
	OnixNativeSlots material2d_shader_slots;
	OnixVkMesh *meshes;
	OnixNativeSlots mesh_slots;

	VkCommandPool command_pool;
	OnixVkFrame frames[ONIX_VK_FRAMES_IN_FLIGHT];
	uint32_t frame_index;
	uint32_t image_index;
	int frame_active;
	int render_pass_active;
	uint32_t active_render_target;
	int frame_cleared;
	int surface_depth_cleared;
	float clear_color[4];
    int vsync_enabled;
	int swapchain_dirty;
	int corner_radius;
	int sprite_blend;
	float blend_constant[4];

	VkBuffer quad_vertex_buffer;
	VkDeviceMemory quad_vertex_memory;
	VkBuffer quad_index_buffer;
	VkDeviceMemory quad_index_memory;

    VkRect2D viewport_rect;
    VkRect2D scissor_rect;
    int scissor_enabled;

	OnixVkTexture *textures;
	OnixNativeSlots texture_slots;
	OnixVkTexturePool texture_pools[ONIX_VK_TEXTURE_POOL_BLOCKS];
	OnixVkMeshBlock *mesh_blocks;
	uint32_t mesh_block_count, mesh_block_capacity;
	OnixVkRenderTarget *render_targets;
	OnixNativeSlots target_slots;
	OnixVkTextureUpload *uploads;
	uint32_t upload_count;
	uint32_t upload_capacity;
	uint32_t upload_peak;
	OnixBufferRetention upload_retention;
	uint32_t upload_live_count;
	uint64_t upload_epoch;
	uint32_t pending_external[ONIX_VK_MAX_TEXTURES];
	uint32_t pending_external_count;
#if defined(_WIN32)
	uint32_t external_textures[ONIX_VK_MAX_EXTERNAL_TEXTURES];
	uint32_t external_texture_count;
#endif
	OnixVkInstanceBuffer readback;
	OnixBufferRetention readback_retention;
	VkDeviceSize readback_used;
	VkCommandBuffer readback_command;
	VkFence readback_fence;
	int readback_pending;
	uint64_t retired_host_bytes;
	uint32_t host_heap_quiet_frames;

	OnixGPUShader sprite_vertex_shader;
	OnixGPUShader sprite_fragment_shader;
	OnixGPUShader shape_vertex_shader;
	OnixGPUShader shape_fragment_shader;
	OnixGPUShader mesh_vertex_shader;
	OnixGPUShader mesh_fragment_shader;
	OnixGPUShader mesh_unlit_fragment_shader;
	OnixGPUShader mesh_depth_fragment_shader, mesh_basic_fragment_shader;
	OnixGPUShader picking_vertex_shader;
	OnixGPUShader picking_fragment_shader;
};

static int onix_vk_prepare_draw(OnixGPUDriver *driver);

void onix_gpu_buffer_memory(OnixGPUDriver *driver, OnixBufferMemory *memory) {
	*memory = (OnixBufferMemory){0};
	if (driver == NULL) return;
	for (uint32_t index = 0; index < ONIX_VK_FRAMES_IN_FLIGHT; index++) {
		memory->instances += driver->frames[index].instance_capacity;
		memory->staging += driver->frames[index].staging_capacity;
		const OnixVkFrame *frame = &driver->frames[index];
		memory->scratch += onix_upload_cache_bytes(&frame->mesh_uploads) +
			(uint64_t)frame->surface_capacity * (sizeof(*frame->surface_sets) + 2 * sizeof(*frame->surface_buckets)) +
			(uint64_t)frame->instance_buffer_capacity * sizeof(*frame->instance_buffers) +
			(uint64_t)frame->staging_buffer_capacity * sizeof(*frame->staging_buffers) +
			(uint64_t)frame->retired_target_capacity * sizeof(*frame->retired_targets);
	}
	memory->scratch += (uint64_t)driver->upload_capacity * sizeof(*driver->uploads);
	memory->readback = driver->readback.capacity;
}

static _Thread_local char onix_vk_error[1024];

static void onix_vk_set_error(const char *format, ...) {
    va_list arguments;
    va_start(arguments, format);
    vsnprintf(onix_vk_error, sizeof(onix_vk_error), format, arguments);
    va_end(arguments);
}

const char *onix_gpu_last_error(void) {
    return onix_vk_error;
}

#if defined(_WIN32)
static int onix_vk_load_library(void) {
    static HMODULE library;
    if (library != NULL) {
        return 1;
    }
    library = LoadLibraryW(L"vulkan-1.dll");
    if (library == NULL) {
        onix_vk_set_error("Vulkan loader vulkan-1.dll is unavailable");
        return 0;
    }
#define ONIX_VK_LOAD(name) \
    name = (PFN_##name)GetProcAddress(library, #name); \
    if (name == NULL) { \
        onix_vk_set_error("Vulkan loader function %s is unavailable", #name); \
        return 0; \
    }
    ONIX_VK_WINDOWS_FUNCTIONS(ONIX_VK_LOAD)
#undef ONIX_VK_LOAD
    return 1;
}
#endif

static int onix_vk_result(VkResult result, const char *operation) {
    if (result == VK_SUCCESS) {
        return 1;
    }
    onix_vk_set_error("%s failed with VkResult %d", operation, (int)result);
    return 0;
}

// Software Vulkan uses the process heap for command and resource storage. Only
// request page reclamation after a substantial retirement; never trim per draw.
static void onix_vk_retire_host_memory(OnixGPUDriver *driver, uint64_t bytes) {
#if defined(__GLIBC__)
	if (driver->physical_device_properties.deviceType != VK_PHYSICAL_DEVICE_TYPE_CPU) return;
	uint64_t remaining = ONIX_VK_HOST_HEAP_RELEASE_BYTES - driver->retired_host_bytes;
	driver->retired_host_bytes += bytes < remaining ? bytes : remaining;
	if (driver->retired_host_bytes == ONIX_VK_HOST_HEAP_RELEASE_BYTES) driver->host_heap_quiet_frames = ONIX_BUFFER_TRIM_DELAY;
#else
	(void)driver; (void)bytes;
#endif
}

static void onix_vk_note_host_activity(OnixGPUDriver *driver) {
	if (driver->retired_host_bytes == ONIX_VK_HOST_HEAP_RELEASE_BYTES) driver->host_heap_quiet_frames = ONIX_BUFFER_TRIM_DELAY;
}

// Observe the completed slot, including commands its driver has already freed
// during normal command-buffer resets. Regular bursts keep the window warm.
static void onix_vk_observe_commands(OnixGPUDriver *driver, OnixVkFrame *frame) {
	if (frame->command_count > frame->command_peak) frame->command_peak = frame->command_count;
	if (frame->command_count > ONIX_VK_COMMANDS_WARM) onix_vk_note_host_activity(driver);
	uint64_t target = onix_buffer_trim_capacity(&frame->command_retention,
		frame->command_peak, frame->command_count, ONIX_VK_COMMANDS_WARM);
	if (target) {
		frame->command_peak = target;
		onix_vk_retire_host_memory(driver, ONIX_VK_HOST_HEAP_RELEASE_BYTES);
	}
	frame->command_count = 0;
}

static void onix_vk_release_idle_host_memory(OnixGPUDriver *driver) {
#if defined(__GLIBC__)
	if (driver->retired_host_bytes < ONIX_VK_HOST_HEAP_RELEASE_BYTES || --driver->host_heap_quiet_frames != 0) return;
	// glibc may retain free arena pages after Vulkan releases its objects. This
	// affects only free pages, and runs once after a quiet window, after fences.
	malloc_trim(0);
	driver->retired_host_bytes = 0;
#else
	(void)driver;
#endif
}

static int onix_vk_allocate_texture_descriptor(OnixGPUDriver *driver, OnixVkTexture *texture) {
	uint32_t index = driver->texture_descriptor_pool_count, available = index;
	for (uint32_t next = 0; next < driver->texture_descriptor_pool_count; next++) {
		OnixVkTextureDescriptorPool *pool = &driver->texture_descriptor_pools[next];
		if (pool->pool == VK_NULL_HANDLE) {
			if (available == driver->texture_descriptor_pool_count) available = next;
		} else if (pool->count < ONIX_VK_TEXTURE_DESCRIPTOR_SETS) {
			index = next;
			break;
		}
	}
	if (index == driver->texture_descriptor_pool_count) index = available;
	if (index == ONIX_VK_TEXTURE_DESCRIPTOR_POOLS) {
		onix_vk_set_error("texture descriptor capacity exceeded");
		return 0;
	}
	OnixVkTextureDescriptorPool *pool = &driver->texture_descriptor_pools[index];
	VkDescriptorPool handle = pool->pool;
	if (handle == VK_NULL_HANDLE) {
		VkDescriptorPoolSize sizes[] = {
			{VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, ONIX_VK_TEXTURE_DESCRIPTOR_SETS},
			{VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE, ONIX_VK_TEXTURE_DESCRIPTOR_SETS},
			{VK_DESCRIPTOR_TYPE_SAMPLER, ONIX_VK_TEXTURE_DESCRIPTOR_SETS},
		};
		VkDescriptorPoolCreateInfo create = {
			.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO,
			.flags = VK_DESCRIPTOR_POOL_CREATE_FREE_DESCRIPTOR_SET_BIT,
			.maxSets = ONIX_VK_TEXTURE_DESCRIPTOR_SETS,
			.poolSizeCount = sizeof(sizes) / sizeof(sizes[0]),
			.pPoolSizes = sizes,
		};
		if (!onix_vk_result(vkCreateDescriptorPool(driver->device, &create, NULL, &handle), "vkCreateDescriptorPool(texture)")) return 0;
	}
	VkDescriptorSetAllocateInfo allocate = {
		.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO,
		.descriptorPool = handle,
		.descriptorSetCount = 1,
		.pSetLayouts = &driver->texture_layout,
	};
	VkDescriptorSet descriptor = VK_NULL_HANDLE;
	if (!onix_vk_result(vkAllocateDescriptorSets(driver->device, &allocate, &descriptor), "vkAllocateDescriptorSets(texture)")) {
		if (pool->pool == VK_NULL_HANDLE) vkDestroyDescriptorPool(driver->device, handle, NULL);
		return 0;
	}
	pool->pool = handle;
	pool->count++;
	pool->idle_frames = 0;
	if (index == driver->texture_descriptor_pool_count) driver->texture_descriptor_pool_count++;
	texture->descriptor = descriptor;
	texture->descriptor_pool = index + 1;
	return 1;
}

// Texture retirement has already waited for all draws referencing this set.
static void onix_vk_release_texture_descriptor(OnixGPUDriver *driver, OnixVkTexture *texture) {
	if (texture->descriptor == VK_NULL_HANDLE) return;
	OnixVkTextureDescriptorPool *pool = &driver->texture_descriptor_pools[texture->descriptor_pool - 1];
	vkFreeDescriptorSets(driver->device, pool->pool, 1, &texture->descriptor);
	pool->count--;
	texture->descriptor = VK_NULL_HANDLE;
	texture->descriptor_pool = 0;
}

static void onix_vk_trim_texture_descriptors(OnixGPUDriver *driver) {
	for (uint32_t index = 0; index < driver->texture_descriptor_pool_count; index++) {
		OnixVkTextureDescriptorPool *pool = &driver->texture_descriptor_pools[index];
		if (pool->pool == VK_NULL_HANDLE) continue;
		if (pool->count || index == 0) { pool->idle_frames = 0; continue; }
		if (++pool->idle_frames < ONIX_BUFFER_TRIM_DELAY) continue;
		vkDestroyDescriptorPool(driver->device, pool->pool, NULL);
		*pool = (OnixVkTextureDescriptorPool){0};
	}
}

static uint32_t onix_vk_find_memory_type(
    OnixGPUDriver *driver,
    uint32_t bits,
    VkMemoryPropertyFlags properties) {
    VkPhysicalDeviceMemoryProperties memory_properties;
    vkGetPhysicalDeviceMemoryProperties(driver->physical_device, &memory_properties);
    for (uint32_t index = 0; index < memory_properties.memoryTypeCount; index++) {
        if ((bits & (1u << index)) != 0 &&
            (memory_properties.memoryTypes[index].propertyFlags & properties) == properties) {
            return index;
        }
    }
    return UINT32_MAX;
}

static int onix_vk_create_buffer(
    OnixGPUDriver *driver,
    VkDeviceSize size,
    VkBufferUsageFlags usage,
    VkMemoryPropertyFlags properties,
    VkBuffer *buffer,
    VkDeviceMemory *memory) {
	if (size >= 1024u * 1024u) onix_vk_note_host_activity(driver);
    VkBufferCreateInfo buffer_info = {
        .sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO,
        .size = size,
        .usage = usage,
        .sharingMode = VK_SHARING_MODE_EXCLUSIVE,
    };
    if (!onix_vk_result(vkCreateBuffer(driver->device, &buffer_info, NULL, buffer), "vkCreateBuffer")) {
        return 0;
    }
    VkMemoryRequirements requirements;
    vkGetBufferMemoryRequirements(driver->device, *buffer, &requirements);
    uint32_t memory_type = onix_vk_find_memory_type(driver, requirements.memoryTypeBits, properties);
    if (memory_type == UINT32_MAX) {
        onix_vk_set_error("Vulkan buffer has no compatible memory type");
        vkDestroyBuffer(driver->device, *buffer, NULL);
        *buffer = VK_NULL_HANDLE;
        return 0;
    }
    VkMemoryAllocateInfo allocation = {
        .sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
        .allocationSize = requirements.size,
        .memoryTypeIndex = memory_type,
    };
    if (!onix_vk_result(vkAllocateMemory(driver->device, &allocation, NULL, memory), "vkAllocateMemory(buffer)")) {
        vkDestroyBuffer(driver->device, *buffer, NULL);
        *buffer = VK_NULL_HANDLE;
        return 0;
    }
    if (!onix_vk_result(vkBindBufferMemory(driver->device, *buffer, *memory, 0), "vkBindBufferMemory")) {
        vkFreeMemory(driver->device, *memory, NULL);
        vkDestroyBuffer(driver->device, *buffer, NULL);
        *memory = VK_NULL_HANDLE;
        *buffer = VK_NULL_HANDLE;
        return 0;
    }
    return 1;
}

static int onix_vk_upload_memory(OnixGPUDriver *driver, VkDeviceMemory memory, const void *data, size_t size) {
    void *mapping = NULL;
    if (!onix_vk_result(vkMapMemory(driver->device, memory, 0, size, 0, &mapping), "vkMapMemory")) {
        return 0;
    }
    memcpy(mapping, data, size);
    vkUnmapMemory(driver->device, memory);
    return 1;
}

static void onix_vk_image_barrier(
	VkCommandBuffer command,
	VkImage image,
	VkImageLayout old_layout,
	VkImageLayout new_layout,
	uint32_t base_mip_level,
	uint32_t level_count) {
    VkImageMemoryBarrier barrier = {
        .sType = VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,
        .oldLayout = old_layout,
        .newLayout = new_layout,
        .srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED,
        .dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED,
        .image = image,
        .subresourceRange = {
            .aspectMask = VK_IMAGE_ASPECT_COLOR_BIT,
			.baseMipLevel = base_mip_level,
			.levelCount = level_count,
            .baseArrayLayer = 0,
            .layerCount = 1,
        },
    };
    VkPipelineStageFlags source_stage;
    VkPipelineStageFlags destination_stage;
    if (old_layout == VK_IMAGE_LAYOUT_UNDEFINED) {
        barrier.srcAccessMask = 0;
        source_stage = VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT;
	} else if (old_layout == VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL) {
		barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
		source_stage = VK_PIPELINE_STAGE_TRANSFER_BIT;
	} else if (old_layout == VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL) {
		barrier.srcAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
		source_stage = VK_PIPELINE_STAGE_TRANSFER_BIT;
    } else {
        barrier.srcAccessMask = VK_ACCESS_SHADER_READ_BIT;
        source_stage = VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT;
    }
	if (new_layout == VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL) {
		barrier.dstAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
		destination_stage = VK_PIPELINE_STAGE_TRANSFER_BIT;
	} else if (new_layout == VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL) {
		barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
		destination_stage = VK_PIPELINE_STAGE_TRANSFER_BIT;
    } else {
        barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
        destination_stage = VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT;
    }
    vkCmdPipelineBarrier(
        command,
        source_stage,
        destination_stage,
        0,
        0,
        NULL,
        0,
        NULL,
        1,
        &barrier);
}

_Static_assert(sizeof(OnixGPUInstance) == 132, "OnixGPUInstance ABI must be 132 bytes");
_Static_assert(sizeof(OnixGPUShapeInstance) == 100, "OnixGPUShapeInstance ABI must be 100 bytes");
_Static_assert(sizeof(OnixGPUSpritePush) == 96, "OnixGPUSpritePush ABI must be 96 bytes");
_Static_assert(sizeof(OnixGPUVertex3D) == 48, "OnixGPUVertex3D ABI must be 48 bytes");
_Static_assert(sizeof(OnixGPUInstance3D) == 192, "OnixGPUInstance3D ABI must be 192 bytes");
_Static_assert(sizeof(OnixGPUMeshPush) == 128, "OnixGPUMeshPush ABI must be 128 bytes");
_Static_assert(sizeof(OnixNativeMeshDraw3D) == 104, "OnixNativeMeshDraw3D ABI must be 104 bytes");

#if defined(ONIX_VK_DRM)
#include "vulkan_display.h"
#endif

static int onix_vk_create_surface(OnixGPUDriver *driver) {
    VkResult result = VK_ERROR_EXTENSION_NOT_PRESENT;
#if defined(__ANDROID__)
    if (driver->native_surface.kind != ONIX_GPU_SURFACE_ANDROID) {
        onix_vk_set_error("Android Vulkan driver requires an ANativeWindow surface");
        return 0;
    }
    VkAndroidSurfaceCreateInfoKHR info = {
        .sType = VK_STRUCTURE_TYPE_ANDROID_SURFACE_CREATE_INFO_KHR,
        .window = (ANativeWindow *)driver->native_surface.window,
    };
    result = vkCreateAndroidSurfaceKHR(driver->instance, &info, NULL, &driver->surface);
#elif defined(_WIN32)
    if (driver->native_surface.kind != ONIX_GPU_SURFACE_WIN32) {
        onix_vk_set_error("Windows Vulkan driver requires a Win32 window surface");
        return 0;
    }
    VkWin32SurfaceCreateInfoKHR info = {
        .sType = VK_STRUCTURE_TYPE_WIN32_SURFACE_CREATE_INFO_KHR,
        .hinstance = GetModuleHandleW(NULL),
        .hwnd = (HWND)driver->native_surface.window,
    };
    result = vkCreateWin32SurfaceKHR(driver->instance, &info, NULL, &driver->surface);
#elif defined(__APPLE__)
    typedef struct { VkStructureType sType; const void *pNext; VkFlags flags; const void *pLayer; } OnixMetalSurfaceInfo;
    typedef VkResult (VKAPI_PTR *OnixCreateMetalSurface)(VkInstance,const OnixMetalSurfaceInfo *,const VkAllocationCallbacks *,VkSurfaceKHR *);
    OnixCreateMetalSurface create=(OnixCreateMetalSurface)vkGetInstanceProcAddr(driver->instance,"vkCreateMetalSurfaceEXT");
    if (!create || driver->native_surface.kind != ONIX_GPU_SURFACE_METAL) { onix_vk_set_error("Vulkan requires VK_EXT_metal_surface and a CAMetalLayer"); return 0; }
    OnixMetalSurfaceInfo info={.sType=VK_STRUCTURE_TYPE_METAL_SURFACE_CREATE_INFO_EXT,.pLayer=(const void *)driver->native_surface.window};
    result=create(driver->instance,&info,NULL,&driver->surface);
#elif defined(ONIX_VK_DRM)
    return onix_vk_create_drm_surface(driver);
#else
    if (driver->native_surface.kind == ONIX_GPU_SURFACE_XLIB) {
        VkXlibSurfaceCreateInfoKHR info = {
            .sType = VK_STRUCTURE_TYPE_XLIB_SURFACE_CREATE_INFO_KHR,
            .dpy = (Display *)driver->native_surface.display,
            .window = (Window)driver->native_surface.window,
        };
        result = vkCreateXlibSurfaceKHR(driver->instance, &info, NULL, &driver->surface);
    } else if (driver->native_surface.kind == ONIX_GPU_SURFACE_WAYLAND) {
        VkWaylandSurfaceCreateInfoKHR info = {
            .sType = VK_STRUCTURE_TYPE_WAYLAND_SURFACE_CREATE_INFO_KHR,
            .display = (struct wl_display *)driver->native_surface.display,
            .surface = (struct wl_surface *)driver->native_surface.window,
        };
        result = vkCreateWaylandSurfaceKHR(driver->instance, &info, NULL, &driver->surface);
    } else {
        onix_vk_set_error("unsupported Linux Vulkan surface kind %d", driver->native_surface.kind);
        return 0;
    }
#endif
    return onix_vk_result(result, "create Vulkan surface");
}

static int onix_vk_create_instance(OnixGPUDriver *driver) {
#if defined(__ANDROID__)
    const char *surface_extension = VK_KHR_ANDROID_SURFACE_EXTENSION_NAME;
#elif defined(_WIN32)
    const char *surface_extension = VK_KHR_WIN32_SURFACE_EXTENSION_NAME;
#elif defined(__APPLE__)
    const char *surface_extension = "VK_EXT_metal_surface";
#elif defined(ONIX_VK_DRM)
    const char *surface_extension = VK_KHR_DISPLAY_EXTENSION_NAME;
#else
    const char *surface_extension = driver->native_surface.kind == ONIX_GPU_SURFACE_WAYLAND
        ? VK_KHR_WAYLAND_SURFACE_EXTENSION_NAME
        : VK_KHR_XLIB_SURFACE_EXTENSION_NAME;
#endif
    const char *extensions[] = {
        VK_KHR_SURFACE_EXTENSION_NAME,
        surface_extension,
#if defined(__APPLE__)
        VK_KHR_PORTABILITY_ENUMERATION_EXTENSION_NAME,
#endif
#if defined(ONIX_VK_DRM)
        VK_EXT_DIRECT_MODE_DISPLAY_EXTENSION_NAME,
        VK_EXT_ACQUIRE_DRM_DISPLAY_EXTENSION_NAME,
#endif
    };
    VkApplicationInfo application = {
        .sType = VK_STRUCTURE_TYPE_APPLICATION_INFO,
        .pApplicationName = "Onix",
        .applicationVersion = VK_MAKE_VERSION(1, 0, 0),
        .pEngineName = "Onix",
        .engineVersion = VK_MAKE_VERSION(1, 0, 0),
        .apiVersion = VK_API_VERSION_1_0,
    };
    VkInstanceCreateInfo create_info = {
        .sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,
        .pApplicationInfo = &application,
#if defined(__APPLE__)
        .flags = VK_INSTANCE_CREATE_ENUMERATE_PORTABILITY_BIT_KHR,
#endif
        .enabledExtensionCount = sizeof(extensions) / sizeof(extensions[0]),
        .ppEnabledExtensionNames = extensions,
    };
    return onix_vk_result(vkCreateInstance(&create_info, NULL, &driver->instance),
#if defined(ONIX_VK_DRM)
        "vkCreateInstance (DRM requires VK_KHR_display, VK_EXT_direct_mode_display and VK_EXT_acquire_drm_display)"
#else
        "vkCreateInstance"
#endif
    );
}

static int onix_vk_select_device(OnixGPUDriver *driver) {
    uint32_t device_count = 0;
    if (!onix_vk_result(vkEnumeratePhysicalDevices(driver->instance, &device_count, NULL), "vkEnumeratePhysicalDevices(count)") || device_count == 0) {
        onix_vk_set_error("no Vulkan physical device is available");
        return 0;
    }
    VkPhysicalDevice *devices = calloc(device_count, sizeof(VkPhysicalDevice));
    if (devices == NULL) {
        onix_vk_set_error("failed to allocate Vulkan physical device list");
        return 0;
    }
    if (!onix_vk_result(vkEnumeratePhysicalDevices(driver->instance, &device_count, devices), "vkEnumeratePhysicalDevices")) {
        free(devices);
        return 0;
    }
    int best_score = -1;
    for (uint32_t device_index = 0; device_index < device_count; device_index++) {
#if defined(ONIX_VK_DRM)
        if (devices[device_index] != driver->physical_device) continue;
#endif
        VkPhysicalDeviceProperties properties;
        vkGetPhysicalDeviceProperties(devices[device_index], &properties);
        int score;
        switch (properties.deviceType) {
            case VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU: score = 500; break;
            case VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU: score = 400; break;
            case VK_PHYSICAL_DEVICE_TYPE_VIRTUAL_GPU: score = 300; break;
            case VK_PHYSICAL_DEVICE_TYPE_OTHER: score = 200; break;
            case VK_PHYSICAL_DEVICE_TYPE_CPU: score = 100; break;
            default: score = 0; break;
        }
        uint32_t queue_count = 0;
        vkGetPhysicalDeviceQueueFamilyProperties(devices[device_index], &queue_count, NULL);
        VkQueueFamilyProperties *queues = calloc(queue_count, sizeof(VkQueueFamilyProperties));
        if (queues == NULL) {
            continue;
        }
        vkGetPhysicalDeviceQueueFamilyProperties(devices[device_index], &queue_count, queues);
        for (uint32_t queue_index = 0; queue_index < queue_count; queue_index++) {
            VkBool32 present = VK_FALSE;
            vkGetPhysicalDeviceSurfaceSupportKHR(devices[device_index], queue_index, driver->surface, &present);
            if ((queues[queue_index].queueFlags & VK_QUEUE_GRAPHICS_BIT) != 0 && present == VK_TRUE && score > best_score) {
                driver->physical_device = devices[device_index];
                driver->physical_device_properties = properties;
                driver->queue_family = queue_index;
                best_score = score;
                break;
            }
        }
        free(queues);
    }
    free(devices);
    if (best_score < 0) {
        onix_vk_set_error("no Vulkan graphics queue can present to this surface");
        return 0;
    }
    return 1;
}

const char *onix_gpu_device_name(OnixGPUDriver *driver) {
    return driver == NULL ? "" : driver->physical_device_properties.deviceName;
}

static int onix_vk_has_device_extension(OnixGPUDriver *driver, const char *name) {
    uint32_t count = 0;
    if (vkEnumerateDeviceExtensionProperties(driver->physical_device, NULL, &count, NULL) != VK_SUCCESS || count == 0) {
        return 0;
    }
    VkExtensionProperties *properties = calloc(count, sizeof(VkExtensionProperties));
    if (properties == NULL) return 0;
    VkResult result = vkEnumerateDeviceExtensionProperties(driver->physical_device, NULL, &count, properties);
    int found = 0;
    if (result == VK_SUCCESS) {
        for (uint32_t index = 0; index < count; index++) {
            if (strcmp(properties[index].extensionName, name) == 0) {
                found = 1;
                break;
            }
        }
    }
    free(properties);
    return found;
}

static int onix_vk_create_device(OnixGPUDriver *driver) {
    float priority = 1.0f;
    VkDeviceQueueCreateInfo queue = {
        .sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO,
        .queueFamilyIndex = driver->queue_family,
        .queueCount = 1,
        .pQueuePriorities = &priority,
    };
	const char *extensions[6] = {VK_KHR_SWAPCHAIN_EXTENSION_NAME};
	uint32_t extension_count = 1;
#if defined(__APPLE__)
    if (onix_vk_has_device_extension(driver,"VK_KHR_portability_subset")) extensions[extension_count++]="VK_KHR_portability_subset";
#endif
#if defined(_WIN32)
	const char *external_extensions[] = {
		VK_KHR_EXTERNAL_MEMORY_EXTENSION_NAME,
		VK_KHR_EXTERNAL_MEMORY_WIN32_EXTENSION_NAME,
		VK_KHR_WIN32_KEYED_MUTEX_EXTENSION_NAME,
		VK_KHR_DEDICATED_ALLOCATION_EXTENSION_NAME,
		VK_KHR_GET_MEMORY_REQUIREMENTS_2_EXTENSION_NAME,
	};
	driver->external_d3d11_supported = 1;
	for (uint32_t index = 0; index < sizeof(external_extensions) / sizeof(external_extensions[0]); index++) {
		if (!onix_vk_has_device_extension(driver, external_extensions[index])) {
			driver->external_d3d11_supported = 0;
			break;
		}
	}
	if (driver->external_d3d11_supported) {
		for (uint32_t index = 0; index < sizeof(external_extensions) / sizeof(external_extensions[0]); index++) {
			extensions[extension_count++] = external_extensions[index];
		}
	}
#elif defined(__linux__) && !defined(__ANDROID__)
	const char *external_extensions[] = {
		VK_KHR_EXTERNAL_MEMORY_EXTENSION_NAME,
		VK_KHR_EXTERNAL_MEMORY_FD_EXTENSION_NAME,
		VK_EXT_EXTERNAL_MEMORY_DMA_BUF_EXTENSION_NAME,
		VK_EXT_IMAGE_DRM_FORMAT_MODIFIER_EXTENSION_NAME,
	};
	driver->external_dmabuf_supported = 1;
	for (uint32_t index = 0; index < sizeof(external_extensions) / sizeof(external_extensions[0]); index++) {
		if (!onix_vk_has_device_extension(driver, external_extensions[index])) {
			driver->external_dmabuf_supported = 0;
			break;
		}
	}
	if (driver->external_dmabuf_supported) {
		for (uint32_t index = 0; index < sizeof(external_extensions) / sizeof(external_extensions[0]); index++) {
			extensions[extension_count++] = external_extensions[index];
		}
	}
#endif
    VkDeviceCreateInfo device = {
        .sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO,
        .queueCreateInfoCount = 1,
        .pQueueCreateInfos = &queue,
		.enabledExtensionCount = extension_count,
        .ppEnabledExtensionNames = extensions,
    };
    if (!onix_vk_result(vkCreateDevice(driver->physical_device, &device, NULL, &driver->device), "vkCreateDevice")) {
        return 0;
    }
    vkGetDeviceQueue(driver->device, driver->queue_family, 0, &driver->queue);
    return 1;
}

static void onix_vk_destroy_swapchain(OnixGPUDriver *driver) {
    if (driver->device == VK_NULL_HANDLE) {
        return;
    }
    for (uint32_t index = 0; index < driver->image_count; index++) {
        if (driver->framebuffers != NULL && driver->framebuffers[index] != VK_NULL_HANDLE) {
            vkDestroyFramebuffer(driver->device, driver->framebuffers[index], NULL);
        }
		if (driver->depth_framebuffers != NULL && driver->depth_framebuffers[index] != VK_NULL_HANDLE) {
			vkDestroyFramebuffer(driver->device, driver->depth_framebuffers[index], NULL);
		}
        if (driver->image_views != NULL && driver->image_views[index] != VK_NULL_HANDLE) {
            vkDestroyImageView(driver->device, driver->image_views[index], NULL);
        }
		if (driver->depth_views != NULL && driver->depth_views[index] != VK_NULL_HANDLE) {
			vkDestroyImageView(driver->device, driver->depth_views[index], NULL);
		}
		if (driver->depth_images != NULL && driver->depth_images[index] != VK_NULL_HANDLE) {
			vkDestroyImage(driver->device, driver->depth_images[index], NULL);
		}
		if (driver->depth_memories != NULL && driver->depth_memories[index] != VK_NULL_HANDLE) {
			vkFreeMemory(driver->device, driver->depth_memories[index], NULL);
		}
    }
    free(driver->framebuffers);
	free(driver->depth_framebuffers);
    free(driver->image_views);
    free(driver->images);
	free(driver->depth_images);
	free(driver->depth_views);
	free(driver->depth_memories);
    driver->framebuffers = NULL;
	driver->depth_framebuffers = NULL;
    driver->image_views = NULL;
    driver->images = NULL;
	driver->depth_images = NULL;
	driver->depth_views = NULL;
	driver->depth_memories = NULL;
    driver->image_count = 0;
    if (driver->swapchain != VK_NULL_HANDLE) {
        vkDestroySwapchainKHR(driver->device, driver->swapchain, NULL);
        driver->swapchain = VK_NULL_HANDLE;
    }
}

static VkCompositeAlphaFlagBitsKHR onix_vk_composite_alpha(VkCompositeAlphaFlagsKHR supported, int transparent) {
    const VkCompositeAlphaFlagBitsKHR choices[] = {
        VK_COMPOSITE_ALPHA_PRE_MULTIPLIED_BIT_KHR,
        VK_COMPOSITE_ALPHA_POST_MULTIPLIED_BIT_KHR,
        VK_COMPOSITE_ALPHA_INHERIT_BIT_KHR,
        VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR,
    };
    if (!transparent && (supported & VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR) != 0) {
        return VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR;
    }
    for (size_t index = 0; index < sizeof(choices) / sizeof(choices[0]); index++) {
        if ((supported & choices[index]) != 0) {
            return choices[index];
        }
    }
    return VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR;
}

static VkPresentModeKHR onix_vk_present_mode(OnixGPUDriver *driver) {
    if (driver->vsync_enabled) {
        return VK_PRESENT_MODE_FIFO_KHR;
    }
    uint32_t count = 0;
    if (vkGetPhysicalDeviceSurfacePresentModesKHR(driver->physical_device, driver->surface, &count, NULL) != VK_SUCCESS || count == 0) {
        return VK_PRESENT_MODE_FIFO_KHR;
    }
    VkPresentModeKHR *modes = calloc(count, sizeof(VkPresentModeKHR));
    if (modes == NULL || vkGetPhysicalDeviceSurfacePresentModesKHR(driver->physical_device, driver->surface, &count, modes) != VK_SUCCESS) {
        free(modes);
        return VK_PRESENT_MODE_FIFO_KHR;
    }
    VkPresentModeKHR selected = VK_PRESENT_MODE_FIFO_KHR;
    for (uint32_t index = 0; index < count; index++) {
        if (modes[index] == VK_PRESENT_MODE_MAILBOX_KHR) {
            selected = VK_PRESENT_MODE_MAILBOX_KHR;
            break;
        }
        if (modes[index] == VK_PRESENT_MODE_IMMEDIATE_KHR) {
            selected = VK_PRESENT_MODE_IMMEDIATE_KHR;
        }
    }
    free(modes);
    return selected;
}

static int onix_vk_create_swapchain(OnixGPUDriver *driver) {
    VkSurfaceCapabilitiesKHR capabilities;
    if (!onix_vk_result(vkGetPhysicalDeviceSurfaceCapabilitiesKHR(driver->physical_device, driver->surface, &capabilities), "vkGetPhysicalDeviceSurfaceCapabilitiesKHR")) {
        return 0;
    }
    uint32_t format_count = 0;
    vkGetPhysicalDeviceSurfaceFormatsKHR(driver->physical_device, driver->surface, &format_count, NULL);
    if (format_count == 0) {
        onix_vk_set_error("Vulkan surface has no formats");
        return 0;
    }
    VkSurfaceFormatKHR *formats = calloc(format_count, sizeof(VkSurfaceFormatKHR));
    vkGetPhysicalDeviceSurfaceFormatsKHR(driver->physical_device, driver->surface, &format_count, formats);
    VkSurfaceFormatKHR selected = formats[0];
    for (uint32_t index = 0; index < format_count; index++) {
        if (formats[index].format == VK_FORMAT_B8G8R8A8_UNORM && formats[index].colorSpace == VK_COLOR_SPACE_SRGB_NONLINEAR_KHR) {
            selected = formats[index];
            break;
        }
    }
    free(formats);

    VkExtent2D extent = capabilities.currentExtent;
    if (extent.width == UINT32_MAX) {
        extent.width = driver->native_surface.width;
        extent.height = driver->native_surface.height;
        if (extent.width < capabilities.minImageExtent.width) extent.width = capabilities.minImageExtent.width;
        if (extent.width > capabilities.maxImageExtent.width) extent.width = capabilities.maxImageExtent.width;
        if (extent.height < capabilities.minImageExtent.height) extent.height = capabilities.minImageExtent.height;
        if (extent.height > capabilities.maxImageExtent.height) extent.height = capabilities.maxImageExtent.height;
    }
    if (extent.width == 0 || extent.height == 0) {
        onix_vk_set_error("Vulkan surface extent is zero");
        return 0;
    }
    VkSwapchainCreateInfoKHR create_info = {
        .sType = VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR,
        .surface = driver->surface,
        .minImageCount = capabilities.minImageCount,
        .imageFormat = selected.format,
        .imageColorSpace = selected.colorSpace,
        .imageExtent = extent,
        .imageArrayLayers = 1,
        .imageUsage = VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT,
        .imageSharingMode = VK_SHARING_MODE_EXCLUSIVE,
        .preTransform = capabilities.currentTransform,
        .compositeAlpha = onix_vk_composite_alpha(capabilities.supportedCompositeAlpha, driver->native_surface.transparent),
        .presentMode = onix_vk_present_mode(driver),
        .clipped = VK_TRUE,
    };
    if (!onix_vk_result(vkCreateSwapchainKHR(driver->device, &create_info, NULL, &driver->swapchain), "vkCreateSwapchainKHR")) {
        return 0;
    }
    driver->swapchain_format = selected.format;
    driver->extent = extent;
    driver->swapchain_dirty = 0;
    vkGetSwapchainImagesKHR(driver->device, driver->swapchain, &driver->image_count, NULL);
    driver->images = calloc(driver->image_count, sizeof(VkImage));
    driver->image_views = calloc(driver->image_count, sizeof(VkImageView));
    driver->framebuffers = calloc(driver->image_count, sizeof(VkFramebuffer));
	driver->depth_framebuffers = calloc(driver->image_count, sizeof(VkFramebuffer));
	driver->depth_images = calloc(driver->image_count, sizeof(VkImage));
	driver->depth_views = calloc(driver->image_count, sizeof(VkImageView));
	driver->depth_memories = calloc(driver->image_count, sizeof(VkDeviceMemory));
    if (driver->images == NULL || driver->image_views == NULL || driver->framebuffers == NULL || driver->depth_framebuffers == NULL ||
		driver->depth_images == NULL || driver->depth_views == NULL || driver->depth_memories == NULL) {
        onix_vk_set_error("failed to allocate Vulkan swapchain resources");
        return 0;
    }
    vkGetSwapchainImagesKHR(driver->device, driver->swapchain, &driver->image_count, driver->images);
    for (uint32_t index = 0; index < driver->image_count; index++) {
        VkImageViewCreateInfo view = {
            .sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO,
            .image = driver->images[index],
            .viewType = VK_IMAGE_VIEW_TYPE_2D,
            .format = driver->swapchain_format,
            .components = {
                VK_COMPONENT_SWIZZLE_IDENTITY,
                VK_COMPONENT_SWIZZLE_IDENTITY,
                VK_COMPONENT_SWIZZLE_IDENTITY,
                VK_COMPONENT_SWIZZLE_IDENTITY,
            },
            .subresourceRange = {
                .aspectMask = VK_IMAGE_ASPECT_COLOR_BIT,
                .baseMipLevel = 0,
                .levelCount = 1,
                .baseArrayLayer = 0,
                .layerCount = 1,
            },
        };
        if (!onix_vk_result(vkCreateImageView(driver->device, &view, NULL, &driver->image_views[index]), "vkCreateImageView(swapchain)")) {
            return 0;
        }
    }
    driver->viewport_rect = (VkRect2D){{0, 0}, extent};
    return 1;
}

static int onix_vk_create_render_pass_with_load(
	OnixGPUDriver *driver,
	VkAttachmentLoadOp load_op,
	VkImageLayout initial_layout,
	int depth,
	VkRenderPass *render_pass) {
	VkAttachmentDescription attachments[] = {{
        .format = driver->swapchain_format,
        .samples = VK_SAMPLE_COUNT_1_BIT,
		.loadOp = load_op,
        .storeOp = VK_ATTACHMENT_STORE_OP_STORE,
        .stencilLoadOp = VK_ATTACHMENT_LOAD_OP_DONT_CARE,
        .stencilStoreOp = VK_ATTACHMENT_STORE_OP_DONT_CARE,
		.initialLayout = initial_layout,
        .finalLayout = VK_IMAGE_LAYOUT_PRESENT_SRC_KHR,
	}, {
		.format = VK_FORMAT_D32_SFLOAT,
		.samples = VK_SAMPLE_COUNT_1_BIT,
		// Only mesh draws use surface depth; 2D frames leave its pages untouched.
		.loadOp = load_op == VK_ATTACHMENT_LOAD_OP_CLEAR ? VK_ATTACHMENT_LOAD_OP_DONT_CARE : load_op,
		.storeOp = VK_ATTACHMENT_STORE_OP_STORE,
		.stencilLoadOp = VK_ATTACHMENT_LOAD_OP_DONT_CARE,
		.stencilStoreOp = VK_ATTACHMENT_STORE_OP_DONT_CARE,
		.initialLayout = load_op == VK_ATTACHMENT_LOAD_OP_CLEAR ? VK_IMAGE_LAYOUT_UNDEFINED : VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
		.finalLayout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
	}};
    VkAttachmentReference color_reference = {
        .attachment = 0,
        .layout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
    };
	VkAttachmentReference depth_reference = {
		.attachment = 1,
		.layout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
	};
    VkSubpassDescription subpass = {
        .pipelineBindPoint = VK_PIPELINE_BIND_POINT_GRAPHICS,
        .colorAttachmentCount = 1,
		.pColorAttachments = &color_reference,
		.pDepthStencilAttachment = depth ? &depth_reference : NULL,
    };
    VkSubpassDependency dependency = {
        .srcSubpass = VK_SUBPASS_EXTERNAL,
        .dstSubpass = 0,
		.srcStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT | VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT | VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT,
		.dstStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT | VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT | VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT,
		.srcAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT,
		.dstAccessMask = VK_ACCESS_COLOR_ATTACHMENT_READ_BIT | VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT,
    };
    VkRenderPassCreateInfo info = {
        .sType = VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO,
		.attachmentCount = depth ? 2 : 1,
		.pAttachments = attachments,
        .subpassCount = 1,
        .pSubpasses = &subpass,
        .dependencyCount = 1,
        .pDependencies = &dependency,
    };
	return onix_vk_result(vkCreateRenderPass(driver->device, &info, NULL, render_pass), "vkCreateRenderPass");
}

static int onix_vk_create_target_render_pass(OnixGPUDriver *driver, uint32_t clear_flags, VkFormat color_format, int depth, VkRenderPass *render_pass) {
	int clear_color = (clear_flags & 1u) != 0;
	int clear_depth = (clear_flags & 2u) != 0;
	VkAttachmentDescription attachments[] = {{
		.format = color_format,
		.samples = VK_SAMPLE_COUNT_1_BIT,
		.loadOp = clear_color ? VK_ATTACHMENT_LOAD_OP_CLEAR : VK_ATTACHMENT_LOAD_OP_LOAD,
		.storeOp = VK_ATTACHMENT_STORE_OP_STORE,
		.stencilLoadOp = VK_ATTACHMENT_LOAD_OP_DONT_CARE,
		.stencilStoreOp = VK_ATTACHMENT_STORE_OP_DONT_CARE,
		.initialLayout = clear_color ? VK_IMAGE_LAYOUT_UNDEFINED : VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
		.finalLayout = VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
	}, {
		.format = VK_FORMAT_D32_SFLOAT,
		.samples = VK_SAMPLE_COUNT_1_BIT,
		.loadOp = clear_depth ? VK_ATTACHMENT_LOAD_OP_CLEAR : VK_ATTACHMENT_LOAD_OP_LOAD,
		.storeOp = VK_ATTACHMENT_STORE_OP_STORE,
		.stencilLoadOp = VK_ATTACHMENT_LOAD_OP_DONT_CARE,
		.stencilStoreOp = VK_ATTACHMENT_STORE_OP_DONT_CARE,
		.initialLayout = clear_depth ? VK_IMAGE_LAYOUT_UNDEFINED : VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
		.finalLayout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
	}};
	VkAttachmentReference color_reference = {.attachment = 0, .layout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL};
	VkAttachmentReference depth_reference = {.attachment = 1, .layout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL};
	VkSubpassDescription subpass = {
		.pipelineBindPoint = VK_PIPELINE_BIND_POINT_GRAPHICS,
		.colorAttachmentCount = 1,
		.pColorAttachments = &color_reference,
		.pDepthStencilAttachment = depth ? &depth_reference : NULL,
	};
	VkSubpassDependency dependencies[] = {{
		.srcSubpass = VK_SUBPASS_EXTERNAL,
		.dstSubpass = 0,
		.srcStageMask = VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT | VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT,
		.dstStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT | VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT,
		.srcAccessMask = VK_ACCESS_SHADER_READ_BIT,
		.dstAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT,
	}, {
		.srcSubpass = 0,
		.dstSubpass = VK_SUBPASS_EXTERNAL,
		.srcStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT,
		.dstStageMask = VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT,
		.srcAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT,
		.dstAccessMask = VK_ACCESS_SHADER_READ_BIT,
	}};
	VkRenderPassCreateInfo info = {
		.sType = VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO,
		.attachmentCount = depth ? 2 : 1,
		.pAttachments = attachments,
		.subpassCount = 1,
		.pSubpasses = &subpass,
		.dependencyCount = sizeof(dependencies) / sizeof(dependencies[0]),
		.pDependencies = dependencies,
	};
	return onix_vk_result(vkCreateRenderPass(driver->device, &info, NULL, render_pass), "vkCreateRenderPass(target)");
}

static int onix_vk_color_only(const OnixGPUDriver *driver) {
	return driver->active_render_target == 0 ? driver->depth_framebuffers[driver->image_index] == VK_NULL_HANDLE
		: driver->render_targets[driver->active_render_target - 1].depth_image == VK_NULL_HANDLE;
}

static int onix_vk_r8_target(const OnixGPUDriver *driver) {
	return driver->active_render_target != 0 && driver->render_targets[driver->active_render_target - 1].color_format == 2;
}

static VkRenderPass onix_vk_target_pass(OnixGPUDriver *driver, const OnixVkRenderTarget *target, uint32_t clear_flags) {
	if (target->depth_image == VK_NULL_HANDLE) return driver->color_only_target_passes[target->color_format][clear_flags & 1u];
	return target->color_format == 1 ? driver->picking_target_render_passes[clear_flags] : driver->target_render_passes[clear_flags];
}

static int onix_vk_create_render_pass(OnixGPUDriver *driver) {
	if (!onix_vk_create_render_pass_with_load(
		driver,
		VK_ATTACHMENT_LOAD_OP_CLEAR,
		VK_IMAGE_LAYOUT_UNDEFINED, 1,
		&driver->render_pass) ||
		!onix_vk_create_render_pass_with_load(
			driver,
			VK_ATTACHMENT_LOAD_OP_LOAD,
			VK_IMAGE_LAYOUT_PRESENT_SRC_KHR, 1,
			&driver->load_render_pass) ||
		!onix_vk_create_render_pass_with_load(driver, VK_ATTACHMENT_LOAD_OP_CLEAR, VK_IMAGE_LAYOUT_UNDEFINED, 0, &driver->color_only_surface_passes[0]) ||
		!onix_vk_create_render_pass_with_load(driver, VK_ATTACHMENT_LOAD_OP_LOAD, VK_IMAGE_LAYOUT_PRESENT_SRC_KHR, 0, &driver->color_only_surface_passes[1])) return 0;
	for (uint32_t flags = 0; flags < 4; flags++) {
		if (!onix_vk_create_target_render_pass(driver, flags, driver->swapchain_format, 1, &driver->target_render_passes[flags]) ||
			!onix_vk_create_target_render_pass(driver, flags, VK_FORMAT_R32_UINT, 1, &driver->picking_target_render_passes[flags])) return 0;
	}
	return 1;
}

static int onix_vk_create_framebuffers(OnixGPUDriver *driver) {
	for (uint32_t index = 0; index < driver->image_count; index++) {
		VkFramebufferCreateInfo info = {
			.sType = VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO,
			.renderPass = driver->color_only_surface_passes[0],
			.attachmentCount = 1,
			.pAttachments = &driver->image_views[index],
			.width = driver->extent.width,
			.height = driver->extent.height,
			.layers = 1,
		};
		if (!onix_vk_result(vkCreateFramebuffer(driver->device, &info, NULL, &driver->framebuffers[index]), "vkCreateFramebuffer(color)")) return 0;
	}
	return 1;
}

// Each swapchain image receives depth only on its first surface mesh draw.
static int onix_vk_create_surface_depth(OnixGPUDriver *driver) {
	uint32_t index = driver->image_index;
	if (driver->depth_framebuffers[index] != VK_NULL_HANDLE) return 1;
	VkImageCreateInfo depth_image = {
		.sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
		.imageType = VK_IMAGE_TYPE_2D,
		.format = VK_FORMAT_D32_SFLOAT,
		.extent = {driver->extent.width, driver->extent.height, 1},
		.mipLevels = 1,
		.arrayLayers = 1,
		.samples = VK_SAMPLE_COUNT_1_BIT,
		.tiling = VK_IMAGE_TILING_OPTIMAL,
		.usage = VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT,
		.sharingMode = VK_SHARING_MODE_EXCLUSIVE,
		.initialLayout = VK_IMAGE_LAYOUT_UNDEFINED,
	};
	if (!onix_vk_result(vkCreateImage(driver->device, &depth_image, NULL, &driver->depth_images[index]), "vkCreateImage(depth)")) goto fail;
	VkMemoryRequirements requirements;
	vkGetImageMemoryRequirements(driver->device, driver->depth_images[index], &requirements);
	uint32_t memory_type = onix_vk_find_memory_type(driver, requirements.memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT);
	VkMemoryAllocateInfo allocation = {
		.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
		.allocationSize = requirements.size,
		.memoryTypeIndex = memory_type,
	};
	if (memory_type == UINT32_MAX ||
		!onix_vk_result(vkAllocateMemory(driver->device, &allocation, NULL, &driver->depth_memories[index]), "vkAllocateMemory(depth)") ||
		!onix_vk_result(vkBindImageMemory(driver->device, driver->depth_images[index], driver->depth_memories[index], 0), "vkBindImageMemory(depth)")) goto fail;
	VkImageViewCreateInfo depth_view = {
		.sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO,
		.image = driver->depth_images[index],
		.viewType = VK_IMAGE_VIEW_TYPE_2D,
		.format = VK_FORMAT_D32_SFLOAT,
		.subresourceRange = {VK_IMAGE_ASPECT_DEPTH_BIT, 0, 1, 0, 1},
	};
	if (!onix_vk_result(vkCreateImageView(driver->device, &depth_view, NULL, &driver->depth_views[index]), "vkCreateImageView(depth)")) goto fail;
	VkImageView attachments[] = {driver->image_views[index], driver->depth_views[index]};
	VkFramebufferCreateInfo info = {
		.sType = VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO,
		.renderPass = driver->render_pass,
		.attachmentCount = 2,
		.pAttachments = attachments,
		.width = driver->extent.width,
		.height = driver->extent.height,
		.layers = 1,
	};
	if (!onix_vk_result(vkCreateFramebuffer(driver->device, &info, NULL, &driver->depth_framebuffers[index]), "vkCreateFramebuffer")) {
		goto fail;
	}
	return 1;
fail:
	if (driver->depth_views[index]) vkDestroyImageView(driver->device, driver->depth_views[index], NULL);
	if (driver->depth_images[index]) vkDestroyImage(driver->device, driver->depth_images[index], NULL);
	if (driver->depth_memories[index]) vkFreeMemory(driver->device, driver->depth_memories[index], NULL);
	driver->depth_views[index] = VK_NULL_HANDLE;
	driver->depth_images[index] = VK_NULL_HANDLE;
	driver->depth_memories[index] = VK_NULL_HANDLE;
	return 0;
}

static VkShaderModule onix_vk_shader_module(OnixGPUDriver *driver, OnixGPUShader shader) {
    VkShaderModuleCreateInfo info = {
        .sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO,
        .codeSize = shader.size,
        .pCode = shader.code,
    };
    VkShaderModule module = VK_NULL_HANDLE;
    if (!onix_vk_result(vkCreateShaderModule(driver->device, &info, NULL, &module), "vkCreateShaderModule")) {
        return VK_NULL_HANDLE;
    }
    return module;
}

static VkPipeline onix_vk_create_pipeline(
    OnixGPUDriver *driver,
    VkShaderModule vertex,
    VkShaderModule fragment,
    VkPipelineLayout layout,
	int kind) {
	int no_depth = (kind & ONIX_VK_PIPELINE_NO_DEPTH) != 0;
	int r8 = (kind & ONIX_VK_PIPELINE_R8) != 0;
	int compact_sprite = (kind & ONIX_VK_PIPELINE_COMPACT_SPRITE) != 0;
	kind &= ~(ONIX_VK_PIPELINE_NO_DEPTH | ONIX_VK_PIPELINE_COMPACT_SPRITE | ONIX_VK_PIPELINE_R8);
    VkPipelineShaderStageCreateInfo stages[] = {
        {
            .sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO,
            .stage = VK_SHADER_STAGE_VERTEX_BIT,
            .module = vertex,
            .pName = "main",
        },
        {
            .sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO,
            .stage = VK_SHADER_STAGE_FRAGMENT_BIT,
            .module = fragment,
            .pName = "main",
        },
    };
    VkVertexInputBindingDescription sprite_bindings[] = {
        {.binding = 0, .stride = 20, .inputRate = VK_VERTEX_INPUT_RATE_VERTEX},
        {.binding = 1, .stride = sizeof(OnixGPUInstance), .inputRate = VK_VERTEX_INPUT_RATE_INSTANCE},
    };
	VkVertexInputAttributeDescription sprite_attributes[] = {
		{.location = 0, .binding = 0, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 0},
		{.location = 1, .binding = 0, .format = VK_FORMAT_R32G32_SFLOAT, .offset = 12},
		{.location = 2, .binding = 1, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 0},
		{.location = 3, .binding = 1, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 12},
		{.location = 4, .binding = 1, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 24},
		{.location = 5, .binding = 1, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 36},
		{.location = 6, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 48},
		{.location = 7, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 64},
		{.location = 8, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 80},
		{.location = 9, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 96},
		{.location = 10, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 112},
		{.location = 11, .binding = 1, .format = VK_FORMAT_R32_SFLOAT, .offset = 128},
	};
	if (compact_sprite) {
		sprite_bindings[1].stride = ONIX_SPRITE_COMPACT_BYTES;
		for (uint32_t index = 9; index < 12; index++) sprite_attributes[index].offset -= 32;
	}
	VkVertexInputBindingDescription shape_bindings[] = {
		{.binding = 0, .stride = 20, .inputRate = VK_VERTEX_INPUT_RATE_VERTEX},
		{.binding = 1, .stride = sizeof(OnixGPUShapeInstance), .inputRate = VK_VERTEX_INPUT_RATE_INSTANCE},
	};
	VkVertexInputAttributeDescription shape_attributes[] = {
		{.location = 0, .binding = 0, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 0},
		{.location = 1, .binding = 0, .format = VK_FORMAT_R32G32_SFLOAT, .offset = 12},
		{.location = 2, .binding = 1, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 0},
		{.location = 3, .binding = 1, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 12},
		{.location = 4, .binding = 1, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 24},
		{.location = 5, .binding = 1, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 36},
		{.location = 6, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 48},
		{.location = 7, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 64},
		{.location = 8, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 80},
		{.location = 9, .binding = 1, .format = VK_FORMAT_R32_UINT, .offset = 96},
	};
	VkVertexInputBindingDescription mesh_bindings[] = {
		{.binding = 0, .stride = sizeof(OnixGPUVertex3D), .inputRate = VK_VERTEX_INPUT_RATE_VERTEX},
		{.binding = 1, .stride = sizeof(OnixGPUInstance3D), .inputRate = VK_VERTEX_INPUT_RATE_INSTANCE},
	};
	VkVertexInputAttributeDescription mesh_attributes[] = {
		{.location = 0, .binding = 0, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 0},
		{.location = 1, .binding = 0, .format = VK_FORMAT_R32G32B32_SFLOAT, .offset = 12},
		{.location = 2, .binding = 0, .format = VK_FORMAT_R32G32_SFLOAT, .offset = 24},
		{.location = 15, .binding = 0, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 32},
		{.location = 3, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 0},
		{.location = 4, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 16},
		{.location = 5, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 32},
		{.location = 6, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 48},
		{.location = 7, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 64},
		{.location = 8, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 80},
		{.location = 9, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 96},
		{.location = 10, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 112},
		{.location = 11, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 128},
		{.location = 12, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 144},
		{.location = 13, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 160},
		{.location = 14, .binding = 1, .format = VK_FORMAT_R32G32B32A32_SFLOAT, .offset = 176},
	};
	const VkVertexInputBindingDescription *bindings = sprite_bindings;
	const VkVertexInputAttributeDescription *attributes = sprite_attributes;
	uint32_t binding_count = 2;
	uint32_t attribute_count = (uint32_t)(sizeof(sprite_attributes) / sizeof(sprite_attributes[0]));
	if (kind == 1 || kind == 3) {
		bindings = shape_bindings;
		attributes = shape_attributes;
		attribute_count = (uint32_t)(sizeof(shape_attributes) / sizeof(shape_attributes[0]));
	}
	if (kind == 2 || kind == 8 || kind == 9 || (kind >= 4 && kind < 4 + ONIX_MESH_BLEND_MODE_COUNT)) {
		bindings = mesh_bindings;
		attributes = mesh_attributes;
		attribute_count = (uint32_t)(sizeof(mesh_attributes) / sizeof(mesh_attributes[0]));
	}
	VkPipelineVertexInputStateCreateInfo vertex_input = {
		.sType = VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO,
		.vertexBindingDescriptionCount = binding_count,
		.pVertexBindingDescriptions = bindings,
		.vertexAttributeDescriptionCount = attribute_count,
		.pVertexAttributeDescriptions = attributes,
    };
    VkPipelineInputAssemblyStateCreateInfo assembly = {
        .sType = VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO,
        .topology = VK_PRIMITIVE_TOPOLOGY_TRIANGLE_LIST,
    };
    VkPipelineViewportStateCreateInfo viewport = {
        .sType = VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO,
        .viewportCount = 1,
        .scissorCount = 1,
    };
    VkPipelineRasterizationStateCreateInfo rasterization = {
        .sType = VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO,
        .polygonMode = VK_POLYGON_MODE_FILL,
        .cullMode = VK_CULL_MODE_NONE,
        .frontFace = VK_FRONT_FACE_COUNTER_CLOCKWISE,
        .lineWidth = 1.0f,
    };
	if (kind == 2 || kind == 9) {
		// The material flags select single/double-sided rendering in the fragment stage.
		// Sky backgrounds render the inside of the cube in this same pipeline.
		rasterization.cullMode = VK_CULL_MODE_NONE;
		rasterization.frontFace = VK_FRONT_FACE_COUNTER_CLOCKWISE;
	} else if (kind >= 4 && kind < 4 + ONIX_MESH_BLEND_MODE_COUNT) {
		// Transparent meshes are double-sided. Spine animations commonly use reflected bones,
		// which reverse triangle winding without changing attachment visibility.
		rasterization.cullMode = VK_CULL_MODE_NONE;
		rasterization.frontFace = VK_FRONT_FACE_COUNTER_CLOCKWISE;
	}
    VkPipelineMultisampleStateCreateInfo multisample = {
        .sType = VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO,
        .rasterizationSamples = VK_SAMPLE_COUNT_1_BIT,
    };
    VkPipelineColorBlendAttachmentState attachment = {
        .blendEnable = VK_TRUE,
        .srcColorBlendFactor = VK_BLEND_FACTOR_ONE,
        .dstColorBlendFactor = VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA,
        .colorBlendOp = VK_BLEND_OP_ADD,
        .srcAlphaBlendFactor = VK_BLEND_FACTOR_ONE,
        .dstAlphaBlendFactor = VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA,
        .alphaBlendOp = VK_BLEND_OP_ADD,
        .colorWriteMask = VK_COLOR_COMPONENT_R_BIT | VK_COLOR_COMPONENT_G_BIT |
            VK_COLOR_COMPONENT_B_BIT | VK_COLOR_COMPONENT_A_BIT,
    };
	if (kind == 3) {
		attachment.srcColorBlendFactor = VK_BLEND_FACTOR_ZERO;
		attachment.dstColorBlendFactor = VK_BLEND_FACTOR_SRC_ALPHA;
		attachment.srcAlphaBlendFactor = VK_BLEND_FACTOR_ZERO;
		attachment.dstAlphaBlendFactor = VK_BLEND_FACTOR_SRC_ALPHA;
	}
	if (kind == 5) {
		attachment.dstColorBlendFactor = VK_BLEND_FACTOR_ONE;
		attachment.dstAlphaBlendFactor = VK_BLEND_FACTOR_ONE;
	}
	if (kind == 6) {
		attachment.srcColorBlendFactor = VK_BLEND_FACTOR_DST_COLOR;
		attachment.srcAlphaBlendFactor = VK_BLEND_FACTOR_DST_COLOR;
	}
	if (kind == 10) {
		attachment.srcColorBlendFactor = VK_BLEND_FACTOR_CONSTANT_COLOR;
		attachment.dstColorBlendFactor = VK_BLEND_FACTOR_ONE_MINUS_SRC_COLOR;
		attachment.srcAlphaBlendFactor = VK_BLEND_FACTOR_ZERO;
		attachment.dstAlphaBlendFactor = VK_BLEND_FACTOR_ONE;
	}
	if (kind == 8 || r8) attachment.colorWriteMask = VK_COLOR_COMPONENT_R_BIT;
	if (kind == 2 || kind == 8 || kind == 9) attachment.blendEnable = VK_FALSE;
	VkPipelineDepthStencilStateCreateInfo depth_stencil = {
		.sType = VK_STRUCTURE_TYPE_PIPELINE_DEPTH_STENCIL_STATE_CREATE_INFO,
		.depthTestEnable = VK_TRUE,
		.depthWriteEnable = (kind == 2 || kind == 8 || kind == 9) ? VK_TRUE : VK_FALSE,
		.depthCompareOp = VK_COMPARE_OP_GREATER,
		.minDepthBounds = 0.0f,
		.maxDepthBounds = 1.0f,
	};
    VkPipelineColorBlendStateCreateInfo blend = {
        .sType = VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO,
        .attachmentCount = 1,
        .pAttachments = &attachment,
    };
    VkDynamicState dynamic_states[] = {VK_DYNAMIC_STATE_VIEWPORT, VK_DYNAMIC_STATE_SCISSOR, VK_DYNAMIC_STATE_BLEND_CONSTANTS};
    VkPipelineDynamicStateCreateInfo dynamic = {
        .sType = VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO,
        .dynamicStateCount = 3,
        .pDynamicStates = dynamic_states,
    };
    VkGraphicsPipelineCreateInfo pipeline = {
        .sType = VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO,
        .stageCount = 2,
        .pStages = stages,
        .pVertexInputState = &vertex_input,
        .pInputAssemblyState = &assembly,
        .pViewportState = &viewport,
        .pRasterizationState = &rasterization,
        .pMultisampleState = &multisample,
		.pDepthStencilState = !no_depth && (kind == 2 || kind == 8 || kind == 9 || (kind >= 4 && kind < 4 + ONIX_MESH_BLEND_MODE_COUNT)) ? &depth_stencil : NULL,
        .pColorBlendState = &blend,
        .pDynamicState = &dynamic,
        .layout = layout,
		.renderPass = no_depth ? (driver->active_render_target == 0 ? driver->color_only_surface_passes[0] : driver->color_only_target_passes[r8 ? 2 : kind == 8][1]) : kind == 8 ? driver->picking_target_render_passes[3] : driver->render_pass,
        .subpass = 0,
    };
    VkPipeline result = VK_NULL_HANDLE;
    if (!onix_vk_result(vkCreateGraphicsPipelines(driver->device, driver->pipeline_cache, 1, &pipeline, NULL, &result), "vkCreateGraphicsPipelines")) {
        if (result != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, result, NULL);
        return VK_NULL_HANDLE;
    }
    return result;
}

static VkPipeline onix_vk_create_builtin_pipeline(
	OnixGPUDriver *driver,
	OnixGPUShader vertex_shader,
	OnixGPUShader fragment_shader,
	VkPipelineLayout layout,
	int kind) {
	VkShaderModule vertex = onix_vk_shader_module(driver, vertex_shader);
	VkShaderModule fragment = onix_vk_shader_module(driver, fragment_shader);
	if (vertex == VK_NULL_HANDLE || fragment == VK_NULL_HANDLE) {
		if (vertex != VK_NULL_HANDLE) vkDestroyShaderModule(driver->device, vertex, NULL);
		if (fragment != VK_NULL_HANDLE) vkDestroyShaderModule(driver->device, fragment, NULL);
		return VK_NULL_HANDLE;
	}
	VkPipeline pipeline = onix_vk_create_pipeline(driver, vertex, fragment, layout,
		kind | (layout == driver->sprite_layout ? ONIX_VK_PIPELINE_COMPACT_SPRITE : 0));
	vkDestroyShaderModule(driver->device, vertex, NULL);
	vkDestroyShaderModule(driver->device, fragment, NULL);
	return pipeline;
}

static VkPipeline onix_vk_2d_pipeline(OnixGPUDriver *driver, int kind, VkPipeline *normal) {
	VkPipeline *pipeline = onix_vk_color_only(driver) ? &driver->color_only_pipelines[onix_vk_r8_target(driver)][kind] : normal;
	if (*pipeline == VK_NULL_HANDLE) {
		int shape = kind == 1 || kind == 3;
		*pipeline = onix_vk_create_builtin_pipeline(driver,
			shape ? driver->shape_vertex_shader : driver->sprite_vertex_shader,
			shape ? driver->shape_fragment_shader : driver->sprite_fragment_shader,
			shape ? driver->shape_layout : driver->sprite_layout,
			kind | (onix_vk_color_only(driver) ? ONIX_VK_PIPELINE_NO_DEPTH : 0) | (onix_vk_r8_target(driver) ? ONIX_VK_PIPELINE_R8 : 0));
	}
	return *pipeline;
}

static int onix_vk_ensure_shape_pipeline(OnixGPUDriver *driver) {
	return onix_vk_2d_pipeline(driver, 1, &driver->shape_pipeline) != VK_NULL_HANDLE;
}

static int onix_vk_ensure_corner_mask_pipeline(OnixGPUDriver *driver) {
	return onix_vk_2d_pipeline(driver, 3, &driver->corner_mask_pipeline) != VK_NULL_HANDLE;
}

static int onix_vk_ensure_mesh_resources(OnixGPUDriver *driver);

static int onix_vk_ensure_picking_pipeline(OnixGPUDriver *driver) {
	if (driver->mesh_picking_pipeline != VK_NULL_HANDLE) return 1;
	if (!onix_vk_ensure_mesh_resources(driver)) return 0;
	driver->mesh_picking_pipeline = onix_vk_create_builtin_pipeline(driver, driver->picking_vertex_shader,
		driver->picking_fragment_shader, driver->mesh_layout, 8);
	return driver->mesh_picking_pipeline != VK_NULL_HANDLE;
}

static VkDescriptorSet onix_vk_deformation_descriptor(OnixGPUDriver *driver, OnixVkFrame *frame) {
	if (frame->deformation_set_index == ONIX_VK_DEFORMATION_SETS) {
		onix_vk_set_error("Vulkan deformation descriptor capacity exceeded");
		return VK_NULL_HANDLE;
	}
	if (frame->deformation_set_index == frame->deformation_set_count) {
		VkDescriptorPool *owner = &frame->deformation_pools[frame->deformation_set_count / ONIX_VK_DEFORMATION_POOL_SETS];
		VkDescriptorPool handle = *owner;
		if (handle == VK_NULL_HANDLE) {
			VkDescriptorPoolSize sizes[] = {
				{VK_DESCRIPTOR_TYPE_STORAGE_BUFFER, ONIX_VK_DEFORMATION_POOL_SETS},
				{VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, ONIX_VK_DEFORMATION_POOL_SETS},
			};
			VkDescriptorPoolCreateInfo create = {
				.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO,
				.maxSets = ONIX_VK_DEFORMATION_POOL_SETS,
				.poolSizeCount = sizeof(sizes) / sizeof(sizes[0]),
				.pPoolSizes = sizes,
			};
			if (!onix_vk_result(vkCreateDescriptorPool(driver->device, &create, NULL, &handle), "vkCreateDescriptorPool(deformation)")) return VK_NULL_HANDLE;
		}
		VkDescriptorSetAllocateInfo allocate = {
			.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO,
			.descriptorPool = handle,
			.descriptorSetCount = 1,
			.pSetLayouts = &driver->deformation_layout,
		};
		VkDescriptorSet descriptor = VK_NULL_HANDLE;
		if (!onix_vk_result(vkAllocateDescriptorSets(driver->device, &allocate, &descriptor), "vkAllocateDescriptorSets(deformation)")) {
			if (*owner == VK_NULL_HANDLE) vkDestroyDescriptorPool(driver->device, handle, NULL);
			return VK_NULL_HANDLE;
		}
		*owner = handle;
		frame->deformation_sets[frame->deformation_set_count++] = descriptor;
	}
	return frame->deformation_sets[frame->deformation_set_index++];
}

// The owning frame fence has completed; retained descriptor handles stay valid.
static void onix_vk_reset_deformation_descriptors(OnixGPUDriver *driver, OnixVkFrame *frame) {
	uint32_t capacity = (frame->deformation_set_count + ONIX_VK_DEFORMATION_POOL_SETS - 1) & ~(ONIX_VK_DEFORMATION_POOL_SETS - 1);
	uint64_t target = onix_buffer_trim_capacity(&frame->deformation_retention,
		capacity, frame->deformation_set_index, ONIX_VK_DEFORMATION_POOL_SETS);
	frame->deformation_set_index = 0;
	if (!target) return;
	capacity = ((uint32_t)target + ONIX_VK_DEFORMATION_POOL_SETS - 1) & ~(ONIX_VK_DEFORMATION_POOL_SETS - 1);
	for (uint32_t index = capacity / ONIX_VK_DEFORMATION_POOL_SETS; index < ONIX_VK_DEFORMATION_SETS / ONIX_VK_DEFORMATION_POOL_SETS; index++) {
		if (frame->deformation_pools[index]) vkDestroyDescriptorPool(driver->device, frame->deformation_pools[index], NULL);
		frame->deformation_pools[index] = VK_NULL_HANDLE;
	}
	if (frame->deformation_set_count > capacity) frame->deformation_set_count = capacity;
}

static int onix_vk_resize_surface_cache(OnixVkFrame *frame, uint32_t capacity) {
	uint32_t *buckets = calloc(capacity * 2, sizeof(*buckets));
	if (buckets == NULL) {
		onix_vk_set_error("failed to grow Vulkan surface cache");
		return 0;
	}
	OnixVkSurfaceDescriptor *sets = realloc(frame->surface_sets, capacity * sizeof(*sets));
	if (sets == NULL) {
		free(buckets);
		onix_vk_set_error("failed to grow Vulkan surface descriptors");
		return 0;
	}
	if (capacity > frame->surface_capacity) memset(sets + frame->surface_capacity, 0, (capacity - frame->surface_capacity) * sizeof(*sets));
	for (uint32_t index = 0; index < frame->surface_set_index; index++) {
		uint32_t bucket = (uint32_t)sets[index].hash & (capacity * 2 - 1);
		while (buckets[bucket] != 0) bucket = (bucket + 1) & (capacity * 2 - 1);
		buckets[bucket] = index + 1;
	}
	free(frame->surface_buckets);
	frame->surface_sets = sets;
	frame->surface_buckets = buckets;
	frame->surface_capacity = capacity;
	return 1;
}

static int onix_vk_grow_surface_cache(OnixVkFrame *frame) {
	return onix_vk_resize_surface_cache(frame, frame->surface_capacity ? frame->surface_capacity * 2 : 16);
}

// Call after the owning frame fence. Descriptor handles in retained pools stay
// valid; pools beyond the new table capacity own only unused tail entries.
static void onix_vk_reset_surface_cache(OnixGPUDriver *driver, OnixVkFrame *frame) {
	uint64_t target = onix_buffer_trim_capacity(&frame->surface_retention,
		frame->surface_capacity, frame->surface_set_index, ONIX_GPU_SURFACE_POOL_SETS);
	frame->surface_set_index = 0;
	if (target) {
		uint32_t capacity = ONIX_GPU_SURFACE_POOL_SETS;
		while (capacity < target) capacity *= 2;
		if (onix_vk_resize_surface_cache(frame, capacity)) {
			for (uint32_t pool = capacity / ONIX_GPU_SURFACE_POOL_SETS; pool < ONIX_GPU_SURFACE_SETS / ONIX_GPU_SURFACE_POOL_SETS; pool++) {
				if (frame->surface_pools[pool]) vkDestroyDescriptorPool(driver->device, frame->surface_pools[pool], NULL);
				frame->surface_pools[pool] = VK_NULL_HANDLE;
			}
			if (frame->surface_set_count > capacity) frame->surface_set_count = capacity;
			return;
		}
	}
	if (frame->surface_capacity) memset(frame->surface_buckets, 0, (size_t)frame->surface_capacity * 2 * sizeof(*frame->surface_buckets));
}

static VkDescriptorSet onix_vk_surface_descriptor(OnixGPUDriver *driver, OnixVkFrame *frame, const OnixNativeMeshDraw3D *draw) {
	uint32_t textures[ONIX_GPU_SURFACE_TEXTURES];
	uint64_t hash = UINT64_C(14695981039346656037);
	// Texture views and samplers are immutable until retirement; slots are only
	// recycled after the frame fence, when the cache is cleared.
	for (uint32_t slot = 0; slot < ONIX_GPU_SURFACE_TEXTURES; slot++) {
		uint32_t id = slot == 0 ? draw->metallic_roughness_texture : draw->extra_textures[slot - 1];
		if (id == 0) id = draw->texture;
		if (id == 0 || id > driver->texture_slots.count || driver->textures[id - 1].image == VK_NULL_HANDLE || driver->textures[id - 1].destroy_pending) {
			onix_vk_set_error("invalid surface texture");
			return VK_NULL_HANDLE;
		}
		textures[slot] = id;
		hash = (hash ^ id) * UINT64_C(1099511628211);
	}
	uint32_t static_mesh = draw->deformation_index == 0 ? 0 : draw->mesh;
	if (draw->deformation_index != 0 && (static_mesh == 0 || static_mesh > driver->mesh_slots.count || (driver->meshes[static_mesh-1].deformation_buffer == VK_NULL_HANDLE || driver->meshes[static_mesh-1].destroy_pending))) { onix_vk_set_error("missing static mesh deformation"); return VK_NULL_HANDLE; }
	hash = (hash ^ static_mesh) * UINT64_C(1099511628211);
	hash ^= hash >> 33;
	hash *= UINT64_C(0xff51afd7ed558ccd);
	hash ^= hash >> 33;
	if (frame->surface_capacity == 0 && !onix_vk_grow_surface_cache(frame)) return VK_NULL_HANDLE;
	uint32_t mask = frame->surface_capacity * 2 - 1;
	uint32_t bucket = (uint32_t)hash & mask;
	while (frame->surface_buckets[bucket] != 0) {
		OnixVkSurfaceDescriptor *entry = &frame->surface_sets[frame->surface_buckets[bucket] - 1];
		if (entry->hash == hash && entry->static_mesh == static_mesh && memcmp(entry->textures, textures, sizeof(textures)) == 0) return entry->descriptor;
		bucket = (bucket + 1) & mask;
	}
	// Look up before checking capacity: existing combinations still work at the limit.
	if (frame->surface_set_index == ONIX_GPU_SURFACE_SETS) {
		onix_vk_set_error("surface descriptor capacity exceeded");
		return VK_NULL_HANDLE;
	}
	if (frame->surface_set_index == frame->surface_capacity) {
		if (!onix_vk_grow_surface_cache(frame)) return VK_NULL_HANDLE;
		mask = frame->surface_capacity * 2 - 1;
		bucket = (uint32_t)hash & mask;
		while (frame->surface_buckets[bucket] != 0) bucket = (bucket + 1) & mask;
	}
	OnixVkSurfaceDescriptor *entry = &frame->surface_sets[frame->surface_set_index];
	if (frame->surface_set_index == frame->surface_set_count) {
		VkDescriptorPool *pool = &frame->surface_pools[frame->surface_set_count / ONIX_GPU_SURFACE_POOL_SETS];
		if (*pool == VK_NULL_HANDLE) {
			VkDescriptorPoolSize sizes[] = {{.type = VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, .descriptorCount = ONIX_GPU_SURFACE_POOL_SETS * ONIX_GPU_SURFACE_TEXTURES}, {.type=VK_DESCRIPTOR_TYPE_STORAGE_BUFFER,.descriptorCount=ONIX_GPU_SURFACE_POOL_SETS}};
			VkDescriptorPoolCreateInfo create = {
				.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO,
				.maxSets = ONIX_GPU_SURFACE_POOL_SETS,
				.poolSizeCount = 2,
				.pPoolSizes = sizes,
			};
			if (!onix_vk_result(vkCreateDescriptorPool(driver->device, &create, NULL, pool), "vkCreateDescriptorPool(surface)")) return VK_NULL_HANDLE;
		}
		VkDescriptorSetAllocateInfo allocate = {
			.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO,
			.descriptorPool = *pool,
			.descriptorSetCount = 1,
			.pSetLayouts = &driver->surface_layout,
		};
		if (!onix_vk_result(vkAllocateDescriptorSets(driver->device, &allocate, &entry->descriptor), "vkAllocateDescriptorSets(surface)")) return VK_NULL_HANDLE;
		frame->surface_set_count++;
	}
	VkDescriptorImageInfo images[ONIX_GPU_SURFACE_TEXTURES];
	VkWriteDescriptorSet writes[ONIX_GPU_SURFACE_TEXTURES + 1];
	for (uint32_t slot = 0; slot < ONIX_GPU_SURFACE_TEXTURES; slot++) {
		OnixVkTexture *source = &driver->textures[textures[slot] - 1];
		images[slot] = (VkDescriptorImageInfo){.sampler = driver->texture_samplers[source->sampler_index], .imageView = source->view, .imageLayout = VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL};
		writes[slot] = (VkWriteDescriptorSet){.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET, .dstSet = entry->descriptor, .dstBinding = slot, .descriptorCount = 1, .descriptorType = VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, .pImageInfo = &images[slot]};
	}
	VkDescriptorBufferInfo static_info = {.buffer=static_mesh == 0 ? driver->empty_deformation_buffer : driver->meshes[static_mesh-1].deformation_buffer,.offset=static_mesh == 0 ? 0 : driver->meshes[static_mesh-1].deformation_offset,.range=static_mesh == 0 ? 4 : driver->meshes[static_mesh-1].deformation_size};
	writes[ONIX_GPU_SURFACE_TEXTURES] = (VkWriteDescriptorSet){.sType=VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET,.dstSet=entry->descriptor,.dstBinding=14,.descriptorCount=1,.descriptorType=VK_DESCRIPTOR_TYPE_STORAGE_BUFFER,.pBufferInfo=&static_info};
	vkUpdateDescriptorSets(driver->device, ONIX_GPU_SURFACE_TEXTURES+1, writes, 0, NULL);
	entry->static_mesh = static_mesh;
	entry->hash = hash;
	memcpy(entry->textures, textures, sizeof(textures));
	frame->surface_buckets[bucket] = ++frame->surface_set_index;
	return entry->descriptor;
}

// Compile only the variant consumed by this draw. Guides, sky and unlit
// sprites never pull the PBR lighting loops into their first pipeline.
static VkPipeline onix_vk_mesh_pipeline(OnixGPUDriver *driver, const OnixNativeMeshDraw3D *draw, const OnixGPUInstance3D *instances, int pass) {
    int variant = draw->shader ? ONIX_MESH_FULL : onix_mesh_builtin_variant(instances, draw->first_instance, draw->instance_count, pass);
    OnixVkMeshPipeline *variants = draw->shader ? &driver->mesh_pipelines[draw->shader - 1] : &driver->mesh_builtin[variant];
	int kind = draw->alpha_mode == 2 ? 4 + (int)draw->blend_mode : driver->active_render_target == 0 ? 9 : 2;
	VkPipeline *pipeline = draw->alpha_mode == 2 ? &variants->transparent[draw->blend_mode]
		: driver->active_render_target == 0 ? &variants->opaque_surface : &variants->opaque;
	if (onix_vk_color_only(driver)) {
		pipeline = &variants->color_only[onix_vk_r8_target(driver)][draw->alpha_mode == 2 ? 1 + draw->blend_mode : 0];
		kind |= ONIX_VK_PIPELINE_NO_DEPTH | (onix_vk_r8_target(driver) ? ONIX_VK_PIPELINE_R8 : 0);
	}
	if (*pipeline == VK_NULL_HANDLE) {
		*pipeline = draw->shader
			? onix_vk_create_pipeline(driver, variants->vertex, variants->fragment, driver->mesh_layout, kind | (onix_vk_r8_target(driver) ? ONIX_VK_PIPELINE_R8 : 0))
			: onix_vk_create_builtin_pipeline(driver, driver->mesh_vertex_shader,
				(OnixGPUShader[]){driver->mesh_fragment_shader, driver->mesh_unlit_fragment_shader, driver->mesh_depth_fragment_shader, driver->mesh_basic_fragment_shader}[variant], driver->mesh_layout, kind);
	}
	return *pipeline;
}

// A 2D window never needs mesh layouts or the fallback deformation buffer.
// Publish them together so a failed first use can be retried without leaks.
static int onix_vk_ensure_mesh_resources(OnixGPUDriver *driver) {
	if (driver->mesh_layout != VK_NULL_HANDLE) return 1;
	VkDescriptorSetLayout deformation = VK_NULL_HANDLE, surface = VK_NULL_HANDLE;
	VkBuffer empty_buffer = VK_NULL_HANDLE;
	VkDeviceMemory empty_memory = VK_NULL_HANDLE;
	VkPipelineLayout layout = VK_NULL_HANDLE;
	VkDescriptorSetLayoutBinding deformation_binding = {
		.binding = 0,
		.descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER,
		.descriptorCount = 1,
		.stageFlags = VK_SHADER_STAGE_VERTEX_BIT,
	};
	VkDescriptorSetLayoutBinding deformation_bindings[] = {deformation_binding, { .binding = 1, .descriptorType = VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, .descriptorCount = 1, .stageFlags = VK_SHADER_STAGE_FRAGMENT_BIT }};
	VkDescriptorSetLayoutCreateInfo deformation_layout = {
		.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO,
		.bindingCount = 2,
		.pBindings = deformation_bindings,
	};
	if (!onix_vk_result(vkCreateDescriptorSetLayout(driver->device, &deformation_layout, NULL, &deformation), "vkCreateDescriptorSetLayout(deformation)")) goto failed;
	VkDescriptorSetLayoutBinding surface_bindings[15];
	for (uint32_t i = 0; i < 14; i++) surface_bindings[i] = (VkDescriptorSetLayoutBinding){.binding=i, .descriptorType=VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, .descriptorCount=1, .stageFlags=VK_SHADER_STAGE_FRAGMENT_BIT};
	surface_bindings[14] = (VkDescriptorSetLayoutBinding){.binding=14,.descriptorType=VK_DESCRIPTOR_TYPE_STORAGE_BUFFER,.descriptorCount=1,.stageFlags=VK_SHADER_STAGE_VERTEX_BIT};
	VkDescriptorSetLayoutCreateInfo surface_layout = {.sType=VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO, .bindingCount=15, .pBindings=surface_bindings};
	if (!onix_vk_result(vkCreateDescriptorSetLayout(driver->device, &surface_layout, NULL, &surface), "surface descriptor layout")) goto failed;
	uint32_t empty_deformation = 0;
	if (!onix_vk_create_buffer(driver, 4, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, &empty_buffer, &empty_memory) || !onix_vk_upload_memory(driver, empty_memory, &empty_deformation, 4)) goto failed;
	VkPushConstantRange mesh_push = {
		.stageFlags = VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT,
		.size = sizeof(OnixGPUMeshPush),
	};
	VkPipelineLayoutCreateInfo mesh_layout = {
		.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO,
		.setLayoutCount = 4,
		.pSetLayouts = (VkDescriptorSetLayout[]){driver->texture_layout, driver->texture_layout, deformation, surface},
		.pushConstantRangeCount = 1,
		.pPushConstantRanges = &mesh_push,
	};
	if (!onix_vk_result(vkCreatePipelineLayout(driver->device, &mesh_layout, NULL, &layout), "vkCreatePipelineLayout(mesh)")) goto failed;
	driver->deformation_layout = deformation;
	driver->surface_layout = surface;
	driver->empty_deformation_buffer = empty_buffer;
	driver->empty_deformation_memory = empty_memory;
	driver->mesh_layout = layout;
	return 1;
failed:
	if (layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(driver->device, layout, NULL);
	if (empty_buffer != VK_NULL_HANDLE) vkDestroyBuffer(driver->device, empty_buffer, NULL);
	if (empty_memory != VK_NULL_HANDLE) vkFreeMemory(driver->device, empty_memory, NULL);
	if (surface != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(driver->device, surface, NULL);
	if (deformation != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(driver->device, deformation, NULL);
	return 0;
}

static int onix_vk_create_pipeline_resources(OnixGPUDriver *driver) {
	VkPipelineCacheCreateInfo cache = {.sType = VK_STRUCTURE_TYPE_PIPELINE_CACHE_CREATE_INFO};
	if (!onix_vk_result(vkCreatePipelineCache(driver->device, &cache, NULL, &driver->pipeline_cache), "vkCreatePipelineCache")) return 0;
	VkDescriptorSetLayoutBinding texture_bindings[] = {
		{.binding = 0, .descriptorType = VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, .descriptorCount = 1, .stageFlags = VK_SHADER_STAGE_FRAGMENT_BIT},
		{.binding = 1, .descriptorType = VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE, .descriptorCount = 1, .stageFlags = VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT},
		{.binding = 2, .descriptorType = VK_DESCRIPTOR_TYPE_SAMPLER, .descriptorCount = 1, .stageFlags = VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT},
	};
	VkDescriptorSetLayoutCreateInfo descriptor_layout = {
		.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO,
		.bindingCount = sizeof(texture_bindings) / sizeof(texture_bindings[0]),
		.pBindings = texture_bindings,
	};
	if (!onix_vk_result(vkCreateDescriptorSetLayout(driver->device, &descriptor_layout, NULL, &driver->texture_layout), "vkCreateDescriptorSetLayout")) return 0;
    VkPushConstantRange sprite_push = {
        .stageFlags = VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT,
        .offset = 0,
        .size = sizeof(OnixGPUSpritePush),
    };
    VkPipelineLayoutCreateInfo sprite_layout = {
        .sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO,
        .setLayoutCount = 4,
        .pSetLayouts = (VkDescriptorSetLayout[]){driver->texture_layout, driver->texture_layout, driver->texture_layout, driver->texture_layout},
        .pushConstantRangeCount = 1,
        .pPushConstantRanges = &sprite_push,
    };
	if (!onix_vk_result(vkCreatePipelineLayout(driver->device, &sprite_layout, NULL, &driver->sprite_layout), "vkCreatePipelineLayout(sprite)")) {
		return 0;
	}
	VkPipelineLayoutCreateInfo shape_layout = {
		.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO,
		.pushConstantRangeCount = 1,
		.pPushConstantRanges = &sprite_push,
	};
	if (!onix_vk_result(vkCreatePipelineLayout(driver->device, &shape_layout, NULL, &driver->shape_layout), "vkCreatePipelineLayout(shape)")) {
		return 0;
	}
	// The caller imports the persisted cache after driver creation. Compile all
	// pipelines on first draw so even the initial sprite pipeline uses it.
	return 1;
}

const uint8_t *onix_gpu_pipeline_cache_uuid(OnixGPUDriver *driver) {
	return driver->physical_device_properties.pipelineCacheUUID;
}

void onix_gpu_load_pipeline_cache(OnixGPUDriver *driver, const void *data, size_t size) {
	// Vulkan requires initial data to match this device and driver. Validate the
	// version-one header before passing a persisted blob to the implementation.
	if (size < 32 || data == NULL) return;
	uint32_t header[4];
	memcpy(header, data, sizeof(header));
	const VkPhysicalDeviceProperties *properties = &driver->physical_device_properties;
	if (header[0] < 32 || header[0] > size || header[1] != VK_PIPELINE_CACHE_HEADER_VERSION_ONE ||
		header[2] != properties->vendorID || header[3] != properties->deviceID ||
		memcmp((const uint8_t *)data + 16, properties->pipelineCacheUUID, VK_UUID_SIZE) != 0) return;
	VkPipelineCacheCreateInfo info = {.sType = VK_STRUCTURE_TYPE_PIPELINE_CACHE_CREATE_INFO, .initialDataSize = size, .pInitialData = data};
	VkPipelineCache loaded = VK_NULL_HANDLE;
	if (vkCreatePipelineCache(driver->device, &info, NULL, &loaded) != VK_SUCCESS) return;
	vkMergePipelineCaches(driver->device, driver->pipeline_cache, 1, &loaded);
	vkDestroyPipelineCache(driver->device, loaded, NULL);
}

size_t onix_gpu_pipeline_cache_data(OnixGPUDriver *driver, void *data, size_t size) {
	if (vkGetPipelineCacheData(driver->device, driver->pipeline_cache, &size, data) != VK_SUCCESS) return 0;
	return size;
}

uint32_t onix_gpu_create_mesh_shader(OnixGPUDriver *driver, OnixGPUShader vertex_shader, OnixGPUShader fragment_shader) {
	if (driver == NULL || vertex_shader.code == NULL || vertex_shader.size == 0 || fragment_shader.code == NULL || fragment_shader.size == 0) {
		onix_vk_set_error("invalid Vulkan mesh shader");
		return 0;
	}
	VkShaderModule vertex = onix_vk_shader_module(driver, vertex_shader);
	VkShaderModule fragment = onix_vk_shader_module(driver, fragment_shader);
	if (vertex == VK_NULL_HANDLE || fragment == VK_NULL_HANDLE) {
		if (vertex != VK_NULL_HANDLE) vkDestroyShaderModule(driver->device, vertex, NULL);
		if (fragment != VK_NULL_HANDLE) vkDestroyShaderModule(driver->device, fragment, NULL);
		return 0;
	}
	uint32_t index;
	if (!onix_vk_mesh_shaders_acquire(&driver->mesh_pipelines, &driver->mesh_shader_slots, &index)) {
		vkDestroyShaderModule(driver->device, vertex, NULL);
		vkDestroyShaderModule(driver->device, fragment, NULL);
		onix_vk_set_error("failed to grow Vulkan mesh shader storage");
		return 0;
	}
	onix_vk_mesh_shaders_insert(driver->mesh_pipelines, &driver->mesh_shader_slots, index,
		(OnixVkMeshPipeline){.vertex = vertex, .fragment = fragment});
	return index + 1;
}

int onix_gpu_destroy_mesh_shader(OnixGPUDriver *driver, uint32_t shader) {
	if (driver == NULL || shader == 0 || shader > driver->mesh_shader_slots.count) {
		onix_vk_set_error("invalid Vulkan mesh shader");
		return 0;
	}
	OnixVkMeshPipeline *pipeline = &driver->mesh_pipelines[shader - 1];
	if (pipeline->vertex == VK_NULL_HANDLE || pipeline->destroy_pending) {
		onix_vk_set_error("Vulkan mesh shader is already destroyed");
		return 0;
	}
	pipeline->destroy_pending = 1;
	pipeline->retire_frame = driver->frame_active
		? driver->frame_index
		: (driver->frame_index + ONIX_VK_FRAMES_IN_FLIGHT - 1) % ONIX_VK_FRAMES_IN_FLIGHT;
	pipeline->retire_next = driver->frames[pipeline->retire_frame].retired_mesh_pipelines;
	driver->frames[pipeline->retire_frame].retired_mesh_pipelines = shader;
	return 1;
}

uint32_t onix_gpu_create_material2d_shader(OnixGPUDriver *driver, OnixGPUShader vertex_shader, OnixGPUShader fragment_shader) {
	if (driver == NULL || vertex_shader.code == NULL || vertex_shader.size == 0 || fragment_shader.code == NULL || fragment_shader.size == 0) {
		onix_vk_set_error("invalid Vulkan Material2D shader");
		return 0;
	}
	VkShaderModule vertex = onix_vk_shader_module(driver, vertex_shader);
	VkShaderModule fragment = onix_vk_shader_module(driver, fragment_shader);
	if (vertex == VK_NULL_HANDLE || fragment == VK_NULL_HANDLE) {
		if (vertex != VK_NULL_HANDLE) vkDestroyShaderModule(driver->device, vertex, NULL);
		if (fragment != VK_NULL_HANDLE) vkDestroyShaderModule(driver->device, fragment, NULL);
		return 0;
	}
	uint32_t index;
	if (!onix_vk_material2d_shaders_acquire(&driver->material2d_pipelines, &driver->material2d_shader_slots, &index)) {
		vkDestroyShaderModule(driver->device, vertex, NULL);
		vkDestroyShaderModule(driver->device, fragment, NULL);
		onix_vk_set_error("failed to grow Vulkan Material2D shader storage");
		return 0;
	}
	onix_vk_material2d_shaders_insert(driver->material2d_pipelines, &driver->material2d_shader_slots, index,
		(OnixVkMaterial2DPipeline){.vertex = vertex, .fragment = fragment});
	return index + 1;
}

int onix_gpu_destroy_material2d_shader(OnixGPUDriver *driver, uint32_t shader) {
	if (driver == NULL || shader == 0 || shader > driver->material2d_shader_slots.count) {
		onix_vk_set_error("invalid Vulkan Material2D shader");
		return 0;
	}
	OnixVkMaterial2DPipeline *pipeline = &driver->material2d_pipelines[shader - 1];
	if (pipeline->vertex == VK_NULL_HANDLE || pipeline->destroy_pending) {
		onix_vk_set_error("Vulkan Material2D shader is already destroyed");
		return 0;
	}
	pipeline->destroy_pending = 1;
	pipeline->retire_frame = driver->frame_active
		? driver->frame_index
		: (driver->frame_index + ONIX_VK_FRAMES_IN_FLIGHT - 1) % ONIX_VK_FRAMES_IN_FLIGHT;
	pipeline->retire_next = driver->frames[pipeline->retire_frame].retired_material2d_pipelines;
	driver->frames[pipeline->retire_frame].retired_material2d_pipelines = shader;
	return 1;
}

static int onix_vk_create_commands(OnixGPUDriver *driver) {
    VkCommandPoolCreateInfo pool = {
        .sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,
        .flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT,
        .queueFamilyIndex = driver->queue_family,
    };
    if (!onix_vk_result(vkCreateCommandPool(driver->device, &pool, NULL, &driver->command_pool), "vkCreateCommandPool")) {
        return 0;
    }
	VkCommandBufferAllocateInfo allocation = {
		.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
		.commandPool = driver->command_pool,
		.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY,
		.commandBufferCount = ONIX_VK_FRAMES_IN_FLIGHT,
	};
	VkCommandBuffer command_buffers[ONIX_VK_FRAMES_IN_FLIGHT] = {0};
	if (!onix_vk_result(vkAllocateCommandBuffers(driver->device, &allocation, command_buffers), "vkAllocateCommandBuffers")) {
		return 0;
	}
	VkSemaphoreCreateInfo semaphore = {.sType = VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO};
    VkFenceCreateInfo fence = {
        .sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO,
        .flags = VK_FENCE_CREATE_SIGNALED_BIT,
    };
	for (uint32_t index = 0; index < ONIX_VK_FRAMES_IN_FLIGHT; index++) {
		OnixVkFrame *frame = &driver->frames[index];
		frame->command_buffer = command_buffers[index];
		if (!onix_vk_result(vkCreateSemaphore(driver->device, &semaphore, NULL, &frame->image_available), "vkCreateSemaphore(image)") ||
			!onix_vk_result(vkCreateSemaphore(driver->device, &semaphore, NULL, &frame->render_finished), "vkCreateSemaphore(render)") ||
			!onix_vk_result(vkCreateFence(driver->device, &fence, NULL, &frame->fence), "vkCreateFence")) {
			return 0;
		}
	}
	return 1;
}

static int onix_vk_create_geometry(OnixGPUDriver *driver) {
    const float vertices[] = {
        0, 0, 0, 0, 0,
        1, 0, 0, 1, 0,
        1, 1, 0, 1, 1,
        0, 1, 0, 0, 1,
    };
    const uint16_t indices[] = {0, 1, 2, 0, 2, 3};
    VkMemoryPropertyFlags host = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
    if (!onix_vk_create_buffer(driver, sizeof(vertices), VK_BUFFER_USAGE_VERTEX_BUFFER_BIT, host, &driver->quad_vertex_buffer, &driver->quad_vertex_memory) ||
        !onix_vk_upload_memory(driver, driver->quad_vertex_memory, vertices, sizeof(vertices)) ||
        !onix_vk_create_buffer(driver, sizeof(indices), VK_BUFFER_USAGE_INDEX_BUFFER_BIT, host, &driver->quad_index_buffer, &driver->quad_index_memory) ||
        !onix_vk_upload_memory(driver, driver->quad_index_memory, indices, sizeof(indices))) {
        return 0;
    }
    return 1;
}

OnixGPUDriver *onix_gpu_create(
    OnixGPUSurface surface,
    OnixGPUShader sprite_vertex,
	OnixGPUShader sprite_fragment,
	OnixGPUShader shape_vertex,
	OnixGPUShader shape_fragment,
	OnixGPUShader mesh_vertex,
	OnixGPUShader mesh_fragment,
	OnixGPUShader mesh_unlit_fragment,
	OnixGPUShader mesh_depth_fragment,
	OnixGPUShader mesh_basic_fragment,
	OnixGPUShader picking_vertex,
	OnixGPUShader picking_fragment) {
    onix_vk_error[0] = '\0';
#if defined(_WIN32)
    if (!onix_vk_load_library()) {
        return NULL;
    }
#endif
    OnixGPUDriver *driver = calloc(1, sizeof(OnixGPUDriver));
    if (driver == NULL) {
        onix_vk_set_error("failed to allocate Vulkan driver");
        return NULL;
    }
    driver->native_surface = surface;
    driver->vsync_enabled = 1;
	driver->sprite_vertex_shader = sprite_vertex;
	driver->sprite_fragment_shader = sprite_fragment;
	driver->shape_vertex_shader = shape_vertex;
	driver->shape_fragment_shader = shape_fragment;
	driver->mesh_vertex_shader = mesh_vertex;
	driver->mesh_fragment_shader = mesh_fragment;
	driver->mesh_unlit_fragment_shader = mesh_unlit_fragment;
	driver->mesh_depth_fragment_shader = mesh_depth_fragment;
	driver->mesh_basic_fragment_shader = mesh_basic_fragment;
	driver->picking_vertex_shader = picking_vertex;
	driver->picking_fragment_shader = picking_fragment;
    if (!onix_vk_create_instance(driver) ||
        !onix_vk_create_surface(driver) ||
        !onix_vk_select_device(driver) ||
        !onix_vk_create_device(driver) ||
        !onix_vk_create_swapchain(driver) ||
        !onix_vk_create_render_pass(driver) ||
        !onix_vk_create_framebuffers(driver) ||
        !onix_vk_create_pipeline_resources(driver) ||
        !onix_vk_create_commands(driver) ||
        !onix_vk_create_geometry(driver)) {
        onix_gpu_destroy(driver);
        return NULL;
    }
    return driver;
}

static void onix_vk_destroy_buffer(OnixGPUDriver *driver, VkBuffer *buffer, VkDeviceMemory *memory) {
    if (*buffer != VK_NULL_HANDLE) {
        vkDestroyBuffer(driver->device, *buffer, NULL);
        *buffer = VK_NULL_HANDLE;
    }
    if (*memory != VK_NULL_HANDLE) {
        vkFreeMemory(driver->device, *memory, NULL);
        *memory = VK_NULL_HANDLE;
    }
}

static int onix_vk_mesh_storage(OnixGPUDriver *driver, VkDeviceSize size, OnixVkMeshLease *lease, VkBuffer *buffer, VkDeviceSize *offset, void **mapping) {
	uint32_t available = driver->mesh_block_count;
	for (uint32_t index = 0; index < driver->mesh_block_count; index++) {
		OnixVkMeshBlock *block = &driver->mesh_blocks[index];
		if (block->buffer == VK_NULL_HANDLE) { if (available == driver->mesh_block_count) available = index; continue; }
		uint64_t start, node = onix_buddy_take(&block->buddy, size, &start);
		if (node) {
			block->idle_frames = 0;
			*lease = (OnixVkMeshLease){.block = index, .node = node};
			*buffer = block->buffer;
			*offset = start;
			*mapping = (uint8_t *)block->mapping + start;
			return 1;
		}
	}
	if (available == driver->mesh_block_capacity) {
		uint32_t capacity = driver->mesh_block_capacity ? driver->mesh_block_capacity * 2 : 4;
		if (capacity < driver->mesh_block_capacity || (uint64_t)capacity * sizeof(*driver->mesh_blocks) > SIZE_MAX) return 0;
		OnixVkMeshBlock *blocks = realloc(driver->mesh_blocks, (size_t)capacity * sizeof(*blocks));
		if (!blocks) { onix_vk_set_error("Vulkan mesh arena table allocation failed"); return 0; }
		driver->mesh_blocks = blocks;
		driver->mesh_block_capacity = capacity;
	}
	VkDeviceSize minimum = driver->physical_device_properties.limits.minStorageBufferOffsetAlignment;
	if (minimum < 256) minimum = 256;
	VkDeviceSize capacity = ONIX_VK_MESH_BLOCK_SIZE;
	if (capacity < minimum) capacity = minimum;
	while (capacity < size && capacity <= UINT64_MAX / 2) capacity *= 2;
	if (capacity < size) return 0;
	OnixVkMeshBlock block = {0};
	if (!onix_buddy_init(&block.buddy, capacity, minimum)) { onix_vk_set_error("Vulkan mesh arena metadata allocation failed"); return 0; }
	VkMemoryPropertyFlags host = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
	if (!onix_vk_create_buffer(driver, capacity, VK_BUFFER_USAGE_VERTEX_BUFFER_BIT | VK_BUFFER_USAGE_INDEX_BUFFER_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, host, &block.buffer, &block.memory) ||
		!onix_vk_result(vkMapMemory(driver->device, block.memory, 0, capacity, 0, &block.mapping), "vkMapMemory(mesh arena)")) {
		onix_vk_destroy_buffer(driver, &block.buffer, &block.memory);
		free(block.buddy.states);
		return 0;
	}
	uint32_t index = available;
	if (index == driver->mesh_block_count) driver->mesh_block_count++;
	driver->mesh_blocks[index] = block;
	uint64_t start, node = onix_buddy_take(&driver->mesh_blocks[index].buddy, size, &start);
	*lease = (OnixVkMeshLease){.block = index, .node = node};
	*buffer = block.buffer;
	*offset = start;
	*mapping = (uint8_t *)block.mapping + start;
	return 1;
}

// Retirement has already waited for the last frame using each lease. Keep one
// small empty block warm, and release surplus empty arenas after sustained idle.
static void onix_vk_trim_mesh_storage(OnixGPUDriver *driver) {
	int warm = 0;
	for (uint32_t index = 0; index < driver->mesh_block_count; index++) {
		OnixVkMeshBlock *block = &driver->mesh_blocks[index];
		if (block->buffer == VK_NULL_HANDLE) continue;
		if (block->buddy.states[1] != 0) { block->idle_frames = 0; continue; }
		if (!warm && block->buddy.size == ONIX_VK_MESH_BLOCK_SIZE) { warm = 1; block->idle_frames = 0; continue; }
		if (++block->idle_frames < ONIX_BUFFER_TRIM_DELAY) continue;
		onix_vk_retire_host_memory(driver, block->buddy.size);
		vkUnmapMemory(driver->device, block->memory);
		onix_vk_destroy_buffer(driver, &block->buffer, &block->memory);
		free(block->buddy.states);
		*block = (OnixVkMeshBlock){0};
	}
}

static void onix_vk_release_mesh(OnixGPUDriver *driver, OnixVkMesh *mesh) {
	if (mesh == NULL) return;
	if (mesh->geometry.node) onix_buddy_release(&driver->mesh_blocks[mesh->geometry.block].buddy, mesh->geometry.node);
	if (mesh->deformation.node) onix_buddy_release(&driver->mesh_blocks[mesh->deformation.block].buddy, mesh->deformation.node);
	*mesh = (OnixVkMesh){0};
}

static int onix_vk_acquire_mesh_slot(OnixGPUDriver *driver, uint32_t *slot) {
	if (onix_vk_meshes_acquire(&driver->meshes, &driver->mesh_slots, slot)) return 1;
	onix_vk_set_error("failed to acquire Vulkan mesh slot");
	return 0;
}

static void onix_vk_destroy_instance_buffers(OnixGPUDriver *driver, OnixVkFrame *frame) {
	for (uint32_t index = 0; index < frame->instance_buffer_count; index++) {
		OnixVkInstanceBuffer *buffer = &frame->instance_buffers[index];
        if (buffer->mapping != NULL) {
            vkUnmapMemory(driver->device, buffer->memory);
        }
        onix_vk_destroy_buffer(driver, &buffer->buffer, &buffer->memory);
    }
	free(frame->instance_buffers);
	frame->instance_buffers = NULL;
	frame->instance_buffer_count = 0;
	frame->instance_buffer_capacity = 0;
	frame->instance_capacity = 0;
	frame->instance_bytes_used = 0;
	frame->instance_retention = (OnixBufferRetention){0};
	frame->instance_table_retention = (OnixBufferRetention){0};
}

static void onix_vk_destroy_staging_buffers(OnixGPUDriver *driver, OnixVkFrame *frame) {
	for (uint32_t index = 0; index < frame->staging_buffer_count; index++) {
		OnixVkStagingBuffer *buffer = &frame->staging_buffers[index];
		if (buffer->mapping != NULL) {
			vkUnmapMemory(driver->device, buffer->memory);
		}
		onix_vk_destroy_buffer(driver, &buffer->buffer, &buffer->memory);
	}
	free(frame->staging_buffers);
	frame->staging_buffers = NULL;
	frame->staging_buffer_count = 0;
	frame->staging_buffer_capacity = 0;
	frame->staging_capacity = 0;
	frame->staging_retention = (OnixBufferRetention){0};
	frame->staging_table_retention = (OnixBufferRetention){0};
}

// onix_vk_staging_bytes_reserved reports the staging bytes the frame reserved for
// its texture uploads. The caller must run it before the offsets are reset.
static VkDeviceSize onix_vk_staging_bytes_reserved(const OnixVkFrame *frame) {
	return frame->staging_reserved;
}

// Only called after the frame fence completes. Compact the pool in one pass,
// retaining existing blocks that fit the budget; large blocks are recreated
// lazily on demand rather than keeping a historical peak forever.
static uint32_t onix_vk_trim_buffer_pool(OnixGPUDriver *driver, OnixVkInstanceBuffer *buffers, uint32_t count, VkDeviceSize budget, VkDeviceSize *capacity) {
	VkDeviceSize retained = 0;
	uint32_t keep = 0;
	for (uint32_t index = 0; index < count; index++) {
		OnixVkInstanceBuffer *buffer = &buffers[index];
		if (buffer->capacity <= budget - retained) {
			retained += buffer->capacity;
			buffers[keep++] = *buffer;
		} else {
			onix_vk_retire_host_memory(driver, buffer->capacity);
			if (buffer->mapping != NULL) vkUnmapMemory(driver->device, buffer->memory);
			onix_vk_destroy_buffer(driver, &buffer->buffer, &buffer->memory);
		}
	}
	*capacity = retained;
	return keep;
}

static void onix_vk_trim_staging_buffers(OnixGPUDriver *driver, OnixVkFrame *frame) {
	VkDeviceSize budget = onix_buffer_trim_capacity(&frame->staging_retention,
		frame->staging_capacity, frame->staging_reserved, ONIX_VK_STAGING_BUFFER_BLOCK_SIZE);
	if (budget) {
		budget = (budget + ONIX_VK_STAGING_BUFFER_BLOCK_SIZE - 1) & ~(VkDeviceSize)(ONIX_VK_STAGING_BUFFER_BLOCK_SIZE - 1);
		frame->staging_buffer_count = onix_vk_trim_buffer_pool(driver, frame->staging_buffers,
			frame->staging_buffer_count, budget, &frame->staging_capacity);
		frame->staging_buffer_index = 0;
	}
	frame->staging_buffers = onix_buffer_trim_array(frame->staging_buffers, &frame->staging_buffer_capacity,
		frame->staging_buffer_count, sizeof(*frame->staging_buffers), 2, &frame->staging_table_retention);
}

static void onix_vk_trim_instance_buffers(OnixGPUDriver *driver, OnixVkFrame *frame) {
	VkDeviceSize budget = onix_buffer_trim_capacity(&frame->instance_retention,
		frame->instance_capacity, frame->instance_bytes_used, ONIX_VK_INSTANCE_BUFFER_BLOCK_SIZE);
	if (budget) {
		budget = (budget + ONIX_VK_INSTANCE_BUFFER_BLOCK_SIZE - 1) & ~(VkDeviceSize)(ONIX_VK_INSTANCE_BUFFER_BLOCK_SIZE - 1);
		frame->instance_buffer_count = onix_vk_trim_buffer_pool(driver, frame->instance_buffers,
			frame->instance_buffer_count, budget, &frame->instance_capacity);
	}
	frame->instance_buffers = onix_buffer_trim_array(frame->instance_buffers, &frame->instance_buffer_capacity,
		frame->instance_buffer_count, sizeof(*frame->instance_buffers), 4, &frame->instance_table_retention);
}

static OnixVkInstanceBuffer *onix_vk_append_instance_buffer(OnixGPUDriver *driver, OnixVkFrame *frame, VkDeviceSize required) {
	if (frame->instance_buffer_count == frame->instance_buffer_capacity) {
		uint32_t capacity = frame->instance_buffer_capacity == 0 ? 4 : frame->instance_buffer_capacity * 2;
		OnixVkInstanceBuffer *buffers = realloc(frame->instance_buffers, capacity * sizeof(OnixVkInstanceBuffer));
        if (buffers == NULL) {
            onix_vk_set_error("failed to grow Vulkan instance buffer table");
            return NULL;
        }
		frame->instance_buffers = buffers;
		frame->instance_buffer_capacity = capacity;
    }
    VkDeviceSize capacity = onix_buffer_block_capacity(required, ONIX_VK_INSTANCE_BUFFER_BLOCK_SIZE,
        frame->instance_buffer_count ? frame->instance_buffers[frame->instance_buffer_count - 1].capacity : 0, 1024u * 1024u);
    if (!capacity) return NULL;
    OnixVkInstanceBuffer buffer = {.capacity = capacity};
    VkMemoryPropertyFlags host = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
    if (!onix_vk_create_buffer(driver, capacity, VK_BUFFER_USAGE_VERTEX_BUFFER_BIT | VK_BUFFER_USAGE_INDEX_BUFFER_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_UNIFORM_BUFFER_BIT, host, &buffer.buffer, &buffer.memory) ||
        !onix_vk_result(vkMapMemory(driver->device, buffer.memory, 0, capacity, 0, &buffer.mapping), "vkMapMemory(instance buffer)")) {
        onix_vk_destroy_buffer(driver, &buffer.buffer, &buffer.memory);
        return NULL;
    }
	frame->instance_buffers[frame->instance_buffer_count++] = buffer;
	frame->instance_capacity += capacity;
	return &frame->instance_buffers[frame->instance_buffer_count - 1];
}

static OnixVkInstanceBuffer *onix_vk_acquire_instance_buffer_aligned(OnixGPUDriver *driver, VkDeviceSize size, VkDeviceSize alignment, VkDeviceSize *offset) {
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (alignment < 1) alignment = 1;
	for (uint32_t index = frame->instance_buffer_index; index < frame->instance_buffer_count; index++) {
		OnixVkInstanceBuffer *buffer = &frame->instance_buffers[index];
		VkDeviceSize aligned = (buffer->offset + alignment - 1) & ~(alignment - 1);
        if (aligned + size <= buffer->capacity) {
			frame->instance_buffer_index = index;
			frame->instance_bytes_used += aligned + size - buffer->offset;
            buffer->offset = aligned + size;
            *offset = aligned;
            return buffer;
        }
    }
	OnixVkInstanceBuffer *buffer = onix_vk_append_instance_buffer(driver, frame, size);
	if (buffer != NULL) {
		frame->instance_buffer_index = frame->instance_buffer_count - 1;
		frame->instance_bytes_used += size;
        buffer->offset = size;
        *offset = 0;
    }
    return buffer;
}

static OnixVkStagingBuffer *onix_vk_append_staging_buffer(OnixGPUDriver *driver, OnixVkFrame *frame, VkDeviceSize required) {
	if (frame->staging_buffer_count == frame->staging_buffer_capacity) {
		uint32_t capacity = frame->staging_buffer_capacity == 0 ? 2 : frame->staging_buffer_capacity * 2;
		OnixVkStagingBuffer *buffers = realloc(frame->staging_buffers, capacity * sizeof(OnixVkStagingBuffer));
		if (buffers == NULL) {
			onix_vk_set_error("failed to grow Vulkan staging buffer table");
			return NULL;
		}
		frame->staging_buffers = buffers;
		frame->staging_buffer_capacity = capacity;
	}
	VkDeviceSize capacity = onix_buffer_block_capacity(required, ONIX_VK_STAGING_BUFFER_BLOCK_SIZE,
		frame->staging_buffer_count ? frame->staging_buffers[frame->staging_buffer_count - 1].capacity : 0, 4u * 1024u * 1024u);
	if (!capacity) return NULL;
	OnixVkStagingBuffer buffer = {.capacity = capacity};
	VkMemoryPropertyFlags host = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
	if (!onix_vk_create_buffer(driver, capacity, VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host, &buffer.buffer, &buffer.memory) ||
		!onix_vk_result(vkMapMemory(driver->device, buffer.memory, 0, capacity, 0, &buffer.mapping), "vkMapMemory(staging buffer)")) {
		onix_vk_destroy_buffer(driver, &buffer.buffer, &buffer.memory);
		return NULL;
	}
	frame->staging_buffers[frame->staging_buffer_count++] = buffer;
	frame->staging_capacity += capacity;
	return &frame->staging_buffers[frame->staging_buffer_count - 1];
}

static OnixVkStagingBuffer *onix_vk_acquire_staging_buffer(OnixGPUDriver *driver, VkDeviceSize size, VkDeviceSize *offset) {
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (!frame->staging_prepared) {
		if (!onix_vk_result(vkWaitForFences(driver->device, 1, &frame->fence, VK_TRUE, UINT64_MAX), "vkWaitForFences(staging)")) return NULL;
		frame->staging_buffer_index = 0;
		frame->staging_reserved = 0;
		for (uint32_t index = 0; index < frame->staging_buffer_count; index++) {
			frame->staging_buffers[index].offset = 0;
		}
		frame->staging_prepared = 1;
	}
	for (uint32_t index = frame->staging_buffer_index; index < frame->staging_buffer_count; index++) {
		OnixVkStagingBuffer *buffer = &frame->staging_buffers[index];
		VkDeviceSize aligned = (buffer->offset + 3u) & ~(VkDeviceSize)3u;
		if (aligned + size <= buffer->capacity) {
			frame->staging_buffer_index = index;
			frame->staging_reserved += aligned + size - buffer->offset;
			buffer->offset = aligned + size;
			*offset = aligned;
			return buffer;
		}
	}
	OnixVkStagingBuffer *buffer = onix_vk_append_staging_buffer(driver, frame, size);
	if (buffer != NULL) {
		frame->staging_buffer_index = frame->staging_buffer_count - 1;
		frame->staging_reserved += size;
		buffer->offset = size;
		*offset = 0;
	}
	return buffer;
}

static OnixVkInstanceBuffer *onix_vk_acquire_instance_buffer(OnixGPUDriver *driver, VkDeviceSize size, VkDeviceSize *offset) {
	return onix_vk_acquire_instance_buffer_aligned(driver, size, 16, offset);
}

static int onix_vk_mesh_upload(OnixGPUDriver *driver, const void *data, VkDeviceSize size, VkDeviceSize alignment, int cached, OnixNativeUpload *upload) {
	if (size > SIZE_MAX) { onix_vk_set_error("Vulkan upload exceeds addressable memory"); return 0; }
	OnixNativeUploadCache *cache = &driver->frames[driver->frame_index].mesh_uploads;
	uint64_t hash = cached ? onix_upload_hash(data, (size_t)size) : 0;
	if (cached && onix_upload_cache_find(cache, data, (size_t)size, hash, alignment ? (size_t)alignment : 1, upload)) return 1;
	VkDeviceSize offset;
	OnixVkInstanceBuffer *buffer = onix_vk_acquire_instance_buffer_aligned(driver, size, alignment, &offset);
	if (!buffer) return 0;
	void *destination = (uint8_t *)buffer->mapping + offset;
	memcpy(destination, data, (size_t)size);
	*upload = (OnixNativeUpload){.resource = (uint64_t)buffer->buffer, .offset = offset, .hash = hash, .size = (size_t)size, .data = destination};
	if (cached && !onix_upload_cache_insert(cache, *upload)) { onix_vk_set_error("Vulkan mesh upload cache allocation failed"); return 0; }
	return 1;
}

static int onix_vk_grow_uploads(OnixGPUDriver *driver, uint32_t additional) {
	uint64_t required = (uint64_t)driver->upload_count + additional;
	if (required <= driver->upload_capacity) {
		return 1;
	}
	uint64_t max_capacity = SIZE_MAX / sizeof(OnixVkTextureUpload);
	if (max_capacity > UINT32_MAX) max_capacity = UINT32_MAX;
	if (required > max_capacity) {
		onix_vk_set_error("Vulkan texture upload queue is too large");
		return 0;
	}
	uint32_t capacity = driver->upload_capacity == 0 ? 32 : driver->upload_capacity;
	while (capacity < required) {
		if ((uint64_t)capacity * 2 > max_capacity) {
			capacity = (uint32_t)required;
			break;
		}
		capacity *= 2;
	}
	OnixVkTextureUpload *uploads = realloc(driver->uploads, (size_t)capacity * sizeof(OnixVkTextureUpload));
	if (uploads == NULL) {
		onix_vk_set_error("failed to grow Vulkan texture upload queue");
		return 0;
	}
	driver->uploads = uploads;
	driver->upload_capacity = capacity;
	return 1;
}

// Queue records contain indices, not pointers into this allocation. Observe the
// largest queue since the previous frame, including queues already flushed.
static void onix_vk_trim_uploads(OnixGPUDriver *driver) {
	driver->uploads = onix_buffer_trim_array(driver->uploads, &driver->upload_capacity,
		driver->upload_peak, sizeof(*driver->uploads), 32, &driver->upload_retention);
	driver->upload_peak = driver->upload_count;
}

/* The encoded size of a format is driven by its texel block, not by a byte
 * count per pixel: block-compressed formats address 4x4 texel groups and round
 * partial groups up to a whole block. onix_gpu_texture_chain_size is checked
 * against the Go side's render.TextureFormat.ChainSize by a test, so the two
 * implementations cannot drift apart. */
typedef struct {
	uint32_t block_width;
	uint32_t block_height;
	uint32_t block_bytes;
} OnixVkTextureBlockLayout;

static OnixVkTextureBlockLayout onix_vk_texture_block_layout(uint32_t format) {
	switch (format) {
	case ONIX_TEXTURE_FORMAT_R8:
		return (OnixVkTextureBlockLayout){1, 1, 1};
	case ONIX_TEXTURE_FORMAT_BC1:
	case ONIX_TEXTURE_FORMAT_BC1_SRGB:
	case ONIX_TEXTURE_FORMAT_BC4:
		return (OnixVkTextureBlockLayout){4, 4, 8};
	case ONIX_TEXTURE_FORMAT_BC3:
	case ONIX_TEXTURE_FORMAT_BC3_SRGB:
	case ONIX_TEXTURE_FORMAT_BC5:
	case ONIX_TEXTURE_FORMAT_BC7:
	case ONIX_TEXTURE_FORMAT_BC7_SRGB:
	case ONIX_TEXTURE_FORMAT_ASTC_4X4:
	case ONIX_TEXTURE_FORMAT_ASTC_4X4_SRGB:
		return (OnixVkTextureBlockLayout){4, 4, 16};
	default:
		return (OnixVkTextureBlockLayout){1, 1, 4};
	}
}

static int onix_vk_texture_format_is_compressed(uint32_t format) {
	return format >= ONIX_TEXTURE_FORMAT_BC1 && format < ONIX_TEXTURE_FORMAT_COUNT;
}

static uint64_t onix_vk_texture_level_size(uint32_t format, uint32_t width, uint32_t height) {
	OnixVkTextureBlockLayout layout = onix_vk_texture_block_layout(format);
	if (width == 0 || height == 0) return 0;
	uint64_t blocks = (uint64_t)((width + layout.block_width - 1) / layout.block_width) *
		(uint64_t)((height + layout.block_height - 1) / layout.block_height);
	return blocks * layout.block_bytes;
}

static uint32_t onix_vk_texture_mip_levels(uint32_t width, uint32_t height) {
	uint32_t levels = 1;
	while (width > 1 || height > 1) {
		width = width > 1 ? width / 2 : 1;
		height = height > 1 ? height / 2 : 1;
		levels++;
	}
	return levels;
}

uint32_t onix_gpu_texture_mip_level_count(uint32_t width, uint32_t height) {
	return onix_vk_texture_mip_levels(width, height);
}

uint64_t onix_gpu_texture_chain_size(uint32_t format, uint32_t width, uint32_t height, uint32_t levels) {
	if (width == 0 || height == 0) return 0;
	uint32_t available = onix_vk_texture_mip_levels(width, height);
	if (levels == 0) levels = 1;
	if (levels > available) levels = available;
	uint64_t total = 0;
	for (uint32_t level = 0; level < levels; level++) {
		total += onix_vk_texture_level_size(format, width, height);
		width = width > 1 ? width / 2 : 1;
		height = height > 1 ? height / 2 : 1;
	}
	return total;
}

static VkFormat onix_vk_texture_image_format(uint32_t format) {
	switch (format) {
	case ONIX_TEXTURE_FORMAT_R8: return VK_FORMAT_R8_UNORM;
	case ONIX_TEXTURE_FORMAT_SRGBA8: return VK_FORMAT_R8G8B8A8_SRGB;
	/* BC1 is only selected for fully opaque colour, so the RGB variant is the
	 * accurate one and needs no alpha support from the device. */
	case ONIX_TEXTURE_FORMAT_BC1: return VK_FORMAT_BC1_RGB_UNORM_BLOCK;
	case ONIX_TEXTURE_FORMAT_BC1_SRGB: return VK_FORMAT_BC1_RGB_SRGB_BLOCK;
	case ONIX_TEXTURE_FORMAT_BC3: return VK_FORMAT_BC3_UNORM_BLOCK;
	case ONIX_TEXTURE_FORMAT_BC3_SRGB: return VK_FORMAT_BC3_SRGB_BLOCK;
	case ONIX_TEXTURE_FORMAT_BC4: return VK_FORMAT_BC4_UNORM_BLOCK;
	case ONIX_TEXTURE_FORMAT_BC5: return VK_FORMAT_BC5_UNORM_BLOCK;
	case ONIX_TEXTURE_FORMAT_BC7: return VK_FORMAT_BC7_UNORM_BLOCK;
	case ONIX_TEXTURE_FORMAT_BC7_SRGB: return VK_FORMAT_BC7_SRGB_BLOCK;
	case ONIX_TEXTURE_FORMAT_ASTC_4X4: return VK_FORMAT_ASTC_4x4_UNORM_BLOCK;
	case ONIX_TEXTURE_FORMAT_ASTC_4X4_SRGB: return VK_FORMAT_ASTC_4x4_SRGB_BLOCK;
	default: return VK_FORMAT_R8G8B8A8_UNORM;
	}
}

static int onix_vk_submit_staging_burst(OnixGPUDriver *driver);

// Per-texture heads are one-based queue indices. Resetting the epoch expires all
// heads at once; cancellation leaves holes so surviving uploads keep their order.
static void onix_vk_reset_uploads(OnixGPUDriver *driver) {
	driver->upload_count = 0;
	driver->upload_live_count = 0;
	driver->upload_epoch++;
}

static void onix_vk_append_texture_upload(OnixGPUDriver *driver, OnixVkTexture *texture, OnixVkTextureUpload upload) {
	if (texture->upload_epoch != driver->upload_epoch) {
		texture->upload_epoch = driver->upload_epoch;
		texture->upload_head = 0;
	}
	upload.next = texture->upload_head;
	driver->uploads[driver->upload_count] = upload;
	texture->upload_head = ++driver->upload_count;
	if (driver->upload_count > driver->upload_peak) driver->upload_peak = driver->upload_count;
	driver->upload_live_count++;
}

static int onix_vk_enqueue_texture_upload(
	OnixGPUDriver *driver,
	OnixVkTexture *texture,
	int32_t x,
	int32_t y,
	uint32_t width,
	uint32_t height,
	const uint8_t *pixels,
	size_t length,
	int initialize,
	uint32_t mip_level,
	int generate_mips) {
	if (!onix_vk_grow_uploads(driver, 1)) {
		return 0;
	}
	VkDeviceSize offset;
	OnixVkStagingBuffer *staging = onix_vk_acquire_staging_buffer(driver, length, &offset);
	if (staging == NULL) return 0;
	memcpy((uint8_t *)staging->mapping + offset, pixels, length);
	onix_vk_append_texture_upload(driver, texture, (OnixVkTextureUpload){
		.image = texture->image,
		.x = x,
		.y = y,
		.width = width,
		.height = height,
		.image_width = texture->width,
		.image_height = texture->height,
		.mip_levels = texture->mip_levels,
		.mip_level = mip_level,
		.generate_mips = generate_mips,
		.initialize = initialize,
		.staging = staging->buffer,
		.staging_offset = offset,
	});
	if (!driver->frame_active && onix_vk_staging_bytes_reserved(&driver->frames[driver->frame_index]) >= ONIX_VK_STAGING_BURST_LIMIT) {
		return onix_vk_submit_staging_burst(driver);
	}
	return 1;
}

static void onix_vk_cancel_texture_uploads(OnixGPUDriver *driver, OnixVkTexture *texture) {
	if (texture->upload_epoch != driver->upload_epoch) return;
	while (texture->upload_head != 0) {
		OnixVkTextureUpload *upload = &driver->uploads[texture->upload_head - 1];
		texture->upload_head = upload->next;
		upload->image = VK_NULL_HANDLE;
		driver->upload_live_count--;
	}
	if (driver->upload_live_count == 0) onix_vk_reset_uploads(driver);
}

static void onix_vk_record_texture_upload(
	VkCommandBuffer command,
	const OnixVkTextureUpload *upload,
	VkBuffer staging,
	VkDeviceSize staging_offset) {
	VkImageLayout old_layout = upload->initialize
		? VK_IMAGE_LAYOUT_UNDEFINED
		: VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL;
	if (!upload->generate_mips) {
		/* A block-compressed level arrives fully formed. The device cannot blit
		 * or filter block data, so the level is written straight through and
		 * only that level is transitioned. */
		onix_vk_image_barrier(command, upload->image, old_layout, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, upload->mip_level, 1);
		VkBufferImageCopy level_copy = {
			.bufferOffset = staging_offset,
			.imageSubresource = {
				.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT,
				.mipLevel = upload->mip_level,
				.layerCount = 1,
			},
			.imageOffset = {upload->x, upload->y, 0},
			.imageExtent = {upload->width, upload->height, 1},
		};
		vkCmdCopyBufferToImage(command, staging, upload->image, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 1, &level_copy);
		onix_vk_image_barrier(command, upload->image, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, upload->mip_level, 1);
		return;
	}
	onix_vk_image_barrier(command, upload->image, old_layout, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 0, 1);
	if (upload->mip_levels > 1) {
		onix_vk_image_barrier(command, upload->image, old_layout, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 1, upload->mip_levels - 1);
	}
	VkBufferImageCopy copy = {
		.bufferOffset = staging_offset,
		.imageSubresource = {
			.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT,
			.mipLevel = 0,
			.layerCount = 1,
		},
		.imageOffset = {upload->x, upload->y, 0},
		.imageExtent = {upload->width, upload->height, 1},
	};
	vkCmdCopyBufferToImage(command, staging, upload->image, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 1, &copy);
	if (upload->mip_levels == 1) {
		onix_vk_image_barrier(command, upload->image, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, 0, 1);
		return;
	}

	int32_t mip_width = upload->image_width;
	int32_t mip_height = upload->image_height;
	for (uint32_t level = 1; level < upload->mip_levels; level++) {
		onix_vk_image_barrier(command, upload->image, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, level - 1, 1);
		int32_t next_width = mip_width > 1 ? mip_width / 2 : 1;
		int32_t next_height = mip_height > 1 ? mip_height / 2 : 1;
		VkImageBlit blit = {
			.srcSubresource = {VK_IMAGE_ASPECT_COLOR_BIT, level - 1, 0, 1},
			.srcOffsets = {{0, 0, 0}, {mip_width, mip_height, 1}},
			.dstSubresource = {VK_IMAGE_ASPECT_COLOR_BIT, level, 0, 1},
			.dstOffsets = {{0, 0, 0}, {next_width, next_height, 1}},
		};
		vkCmdBlitImage(
			command,
			upload->image,
			VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,
			upload->image,
			VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
			1,
			&blit,
			VK_FILTER_LINEAR);
		onix_vk_image_barrier(command, upload->image, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, level - 1, 1);
		mip_width = next_width;
		mip_height = next_height;
	}
	onix_vk_image_barrier(
		command,
		upload->image,
		VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
		VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
		upload->mip_levels - 1,
		1);
}

static int onix_vk_flush_uploads(OnixGPUDriver *driver) {
	if (driver->upload_count == 0) {
		return 1;
	}
	if (!driver->frame_active) {
		return 1;
	}
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (driver->render_pass_active) {
		vkCmdEndRenderPass(frame->command_buffer);
		driver->render_pass_active = 0;
		if (driver->active_render_target == 0) driver->frame_cleared = 1;
	}
	for (uint32_t index = 0; index < driver->upload_count; index++) {
		OnixVkTextureUpload *upload = &driver->uploads[index];
		if (upload->image == VK_NULL_HANDLE) continue;
		onix_vk_record_texture_upload(frame->command_buffer, upload, upload->staging, upload->staging_offset);
		frame->command_count++;
	}
	onix_vk_reset_uploads(driver);
	return 1;
}

// onix_vk_submit_staging_burst records and submits the queued texture uploads
// outside the frame loop. Importing a map queues every texture before the first
// frame is begun, so without this the staging pool would hold the entire upload
// volume as mapped host memory. Only transfer commands run here, so no swapchain
// image is involved; the frame fence orders the copies before any later frame.
static int onix_vk_submit_staging_burst(OnixGPUDriver *driver) {
	if (driver->upload_count == 0 || driver->frame_active) {
		return 1;
	}
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (!onix_vk_result(vkWaitForFences(driver->device, 1, &frame->fence, VK_TRUE, UINT64_MAX), "vkWaitForFences(staging burst)")) {
		return 0;
	}
	VkCommandBuffer command = frame->command_buffer;
	onix_vk_observe_commands(driver, frame);
	if (!onix_vk_result(vkResetCommandBuffer(command, 0), "vkResetCommandBuffer(staging burst)")) {
		return 0;
	}
	VkCommandBufferBeginInfo begin = {
		.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO,
		.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT,
	};
	if (!onix_vk_result(vkBeginCommandBuffer(command, &begin), "vkBeginCommandBuffer(staging burst)")) {
		return 0;
	}
	for (uint32_t index = 0; index < driver->upload_count; index++) {
		OnixVkTextureUpload *upload = &driver->uploads[index];
		if (upload->image == VK_NULL_HANDLE) continue;
		onix_vk_record_texture_upload(command, upload, upload->staging, upload->staging_offset);
		frame->command_count++;
	}
	if (!onix_vk_result(vkEndCommandBuffer(command), "vkEndCommandBuffer(staging burst)")) {
		return 0;
	}
	VkSubmitInfo submit = {
		.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO,
		.commandBufferCount = 1,
		.pCommandBuffers = &command,
	};
	if (!onix_vk_result(vkResetFences(driver->device, 1, &frame->fence), "vkResetFences(staging burst)") ||
		!onix_vk_result(vkQueueSubmit(driver->queue, 1, &submit, frame->fence), "vkQueueSubmit(staging burst)") ||
		!onix_vk_result(vkWaitForFences(driver->device, 1, &frame->fence, VK_TRUE, UINT64_MAX), "vkWaitForFences(staging burst)")) {
		return 0;
	}
	onix_vk_reset_uploads(driver);
	onix_vk_trim_staging_buffers(driver, frame);
	frame->staging_reserved = 0;
	frame->staging_prepared = 0;
	frame->staging_buffer_index = 0;
	return 1;
}

static void onix_vk_initialize_external_textures(OnixGPUDriver *driver) {
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	for (uint32_t index = 0; index < driver->pending_external_count; index++) {
		OnixVkTexture *texture = &driver->textures[driver->pending_external[index]];
		texture->pending_external_index = 0;
		if (!texture->external || !texture->external_ready || texture->external_initialized ||
			texture->image == VK_NULL_HANDLE || texture->destroy_pending) continue;
		if (driver->render_pass_active) {
			vkCmdEndRenderPass(frame->command_buffer);
			driver->render_pass_active = 0;
			driver->frame_cleared = 1;
		}
		onix_vk_image_barrier(
			frame->command_buffer,
			texture->image,
			VK_IMAGE_LAYOUT_UNDEFINED,
			VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
			0,
			1);
		texture->external_initialized = 1;
	}
	driver->pending_external_count = 0;
}

static int onix_vk_prepare_draw(OnixGPUDriver *driver) {
	if (driver == NULL || !driver->frame_active || !onix_vk_flush_uploads(driver)) {
		return 0;
	}
	onix_vk_initialize_external_textures(driver);
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (driver->render_pass_active) {
		return 1;
	}
	VkClearValue clear[] = {
		{.color = {{driver->clear_color[0], driver->clear_color[1], driver->clear_color[2], driver->clear_color[3]}}},
		{.depthStencil = {0.0f, 0}},
	};
	VkRenderPassBeginInfo render_pass = {
		.sType = VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO,
		.renderPass = onix_vk_color_only(driver) ? driver->color_only_surface_passes[driver->frame_cleared != 0]
			: (driver->frame_cleared ? driver->load_render_pass : driver->render_pass),
		.framebuffer = onix_vk_color_only(driver) ? driver->framebuffers[driver->image_index] : driver->depth_framebuffers[driver->image_index],
		.renderArea = {{0, 0}, driver->extent},
		.clearValueCount = 2,
		.pClearValues = clear,
	};
	// Texture uploads may interrupt an offscreen pass. Resume its attachments
	// with load operations instead of redirecting subsequent draws to the window.
	if (driver->active_render_target != 0) {
		OnixVkRenderTarget *target = &driver->render_targets[driver->active_render_target - 1];
		render_pass.renderPass = onix_vk_target_pass(driver, target, 0);
		render_pass.framebuffer = target->framebuffer;
		render_pass.renderArea.extent = (VkExtent2D){target->width, target->height};
	}
	vkCmdBeginRenderPass(frame->command_buffer, &render_pass, VK_SUBPASS_CONTENTS_INLINE);
	driver->render_pass_active = 1;
	if (driver->active_render_target == 0) driver->frame_cleared = 1;
	return 1;
}

static int onix_vk_require_surface_depth(OnixGPUDriver *driver) {
	if (driver->active_render_target != 0 || driver->depth_framebuffers[driver->image_index] != VK_NULL_HANDLE) return 1;
	if (!onix_vk_create_surface_depth(driver)) return 0;
	VkCommandBuffer command = driver->frames[driver->frame_index].command_buffer;
	if (driver->render_pass_active) {
		vkCmdEndRenderPass(command);
		driver->render_pass_active = 0;
		driver->frame_cleared = 1;
	}
	if (driver->frame_cleared) {
		// A resumed surface pass loads color, but its new depth has no layout yet.
		VkImageMemoryBarrier barrier = {
			.sType = VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,
			.oldLayout = VK_IMAGE_LAYOUT_UNDEFINED,
			.newLayout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
			.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED,
			.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED,
			.image = driver->depth_images[driver->image_index],
			.subresourceRange = {VK_IMAGE_ASPECT_DEPTH_BIT, 0, 1, 0, 1},
			.dstAccessMask = VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT,
		};
		vkCmdPipelineBarrier(command, VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT,
			0, 0, NULL, 0, NULL, 1, &barrier);
	}
	return 1;
}

static void onix_vk_prepare_surface_depth(OnixGPUDriver *driver) {
	if (driver->active_render_target != 0 || driver->surface_depth_cleared) return;
	VkClearAttachment attachment = {
		.aspectMask = VK_IMAGE_ASPECT_DEPTH_BIT,
		.clearValue = {.depthStencil = {0.0f, 0}},
	};
	VkClearRect rect = {.rect = {{0, 0}, driver->extent}, .layerCount = 1};
	vkCmdClearAttachments(driver->frames[driver->frame_index].command_buffer, 1, &attachment, 1, &rect);
	driver->surface_depth_cleared = 1;
}

// onix_vk_pool_spare_index reports a range entry usable for a hole split.
static int onix_vk_pool_spare_index(const OnixVkTexturePool *pool, int first, int second) {
	for (uint32_t index = 0; index < ONIX_VK_TEXTURE_POOL_RANGES; index++) {
		if ((int)index == first || (int)index == second) continue;
		if (pool->ranges[index].size == 0 && !pool->ranges[index].occupied) return (int)index;
	}
	return -1;
}

// onix_vk_pool_merge_ranges folds a released range into its free neighbours.
static void onix_vk_pool_merge_ranges(OnixVkTexturePool *pool, uint32_t index) {
	for (int merged = 1; merged;) {
		merged = 0;
		for (uint32_t next = 0; next < ONIX_VK_TEXTURE_POOL_RANGES; next++) {
			OnixVkMemoryRange *other = &pool->ranges[next];
			OnixVkMemoryRange *range = &pool->ranges[index];
			if (next == index || other->occupied || other->size == 0) continue;
			if (other->offset + other->size == range->offset) {
				range->offset = other->offset;
				range->size += other->size;
			} else if (range->offset + range->size == other->offset) {
				range->size += other->size;
			} else continue;
			other->size = 0;
			merged = 1;
		}
	}
}

// onix_vk_create_texture_pool_block reserves one device-local block for leases.
static int onix_vk_create_texture_pool_block(OnixGPUDriver *driver, OnixVkTexturePool *pool, const VkMemoryRequirements *requirements) {
	uint32_t memory_type = onix_vk_find_memory_type(driver, requirements->memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT);
	if (memory_type == UINT32_MAX) return 0;
	VkMemoryAllocateInfo allocation = {
		.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
		.allocationSize = ONIX_VK_TEXTURE_POOL_BLOCK_SIZE,
		.memoryTypeIndex = memory_type,
	};
	if (!onix_vk_result(vkAllocateMemory(driver->device, &allocation, NULL, &pool->memory), "vkAllocateMemory(texture pool)")) return 0;
	pool->memory_type = memory_type;
	pool->ranges[0] = (OnixVkMemoryRange){.size = ONIX_VK_TEXTURE_POOL_BLOCK_SIZE};
	return 1;
}

// onix_vk_acquire_texture_pool leases a range for one image out of a shared
// block; larger images keep their dedicated allocation.
static int onix_vk_acquire_texture_pool(
    OnixGPUDriver *driver,
    const VkMemoryRequirements *requirements,
    uint32_t *lease,
    VkDeviceMemory *memory,
    VkDeviceSize *offset) {
	if (requirements->size > ONIX_VK_TEXTURE_POOL_MAX_IMAGE) return 0;
	VkDeviceSize granularity = driver->physical_device_properties.limits.bufferImageGranularity;
	if (granularity < requirements->alignment) granularity = requirements->alignment;
	for (uint32_t block = 0; block < ONIX_VK_TEXTURE_POOL_BLOCKS; block++) {
		OnixVkTexturePool *pool = &driver->texture_pools[block];
		if (pool->memory == VK_NULL_HANDLE) {
			if (!onix_vk_create_texture_pool_block(driver, pool, requirements)) continue;
		} else if ((requirements->memoryTypeBits & (1u << pool->memory_type)) == 0) {
			continue;
		}
		for (uint32_t index = 0; index < ONIX_VK_TEXTURE_POOL_RANGES; index++) {
			OnixVkMemoryRange *hole = &pool->ranges[index];
			if (hole->occupied || hole->size == 0) continue;
			VkDeviceSize start = (hole->offset + granularity - 1) & ~(granularity - 1);
			VkDeviceSize payload = (requirements->size + granularity - 1) & ~(granularity - 1);
			VkDeviceSize end = hole->offset + hole->size;
			if (start + payload > end) continue;
			int head = start == hole->offset ? -1 : onix_vk_pool_spare_index(pool, -1, -1);
			if (start != hole->offset && head < 0) continue;
			int tail = end == start + payload ? -1 : onix_vk_pool_spare_index(pool, head, -1);
			if (end != start + payload && tail < 0) continue;
			if (tail >= 0) pool->ranges[tail] = (OnixVkMemoryRange){.offset = start + payload, .size = end - start - payload};
			if (head >= 0) pool->ranges[head] = (OnixVkMemoryRange){.offset = hole->offset, .size = start - hole->offset};
			*hole = (OnixVkMemoryRange){.offset = start, .size = payload, .occupied = 1};
			pool->occupied_count++;
			*lease = (block << 16) | (index + 1);
			*memory = pool->memory;
			*offset = start;
			return 1;
		}
	}
	return 0;
}

// onix_vk_release_texture_pool returns a lease and drops a block once empty.
static void onix_vk_release_texture_pool(OnixGPUDriver *driver, uint32_t lease) {
	uint32_t block = lease >> 16, index = (lease & 0xFFFFu) - 1;
	if (block >= ONIX_VK_TEXTURE_POOL_BLOCKS || index >= ONIX_VK_TEXTURE_POOL_RANGES) return;
	OnixVkTexturePool *pool = &driver->texture_pools[block];
	pool->ranges[index].occupied = 0;
	pool->occupied_count--;
	onix_vk_pool_merge_ranges(pool, index);
	if (pool->occupied_count != 0) return;
	onix_vk_retire_host_memory(driver, ONIX_VK_TEXTURE_POOL_BLOCK_SIZE);
	vkFreeMemory(driver->device, pool->memory, NULL);
	*pool = (OnixVkTexturePool){0};
}

// onix_vk_forget_texture_memory releases whatever owns the texture image.
static void onix_vk_forget_texture_memory(OnixGPUDriver *driver, OnixVkTexture *texture) {
	if (texture->pool != 0) {
		onix_vk_release_texture_pool(driver, texture->pool);
		return;
	}
	if (texture->memory != VK_NULL_HANDLE) vkFreeMemory(driver->device, texture->memory, NULL);
}

static void onix_vk_release_texture(OnixGPUDriver *driver, OnixVkTexture *texture) {
	onix_vk_release_texture_descriptor(driver, texture);
    if (texture->view != VK_NULL_HANDLE) vkDestroyImageView(driver->device, texture->view, NULL);
    if (texture->image != VK_NULL_HANDLE) vkDestroyImage(driver->device, texture->image, NULL);
    onix_vk_forget_texture_memory(driver, texture);
    *texture = (OnixVkTexture){0};
}

static void onix_vk_release_retired_target(OnixGPUDriver *driver, OnixVkRetiredRenderTarget *target) {
	if (target->framebuffer != VK_NULL_HANDLE) vkDestroyFramebuffer(driver->device, target->framebuffer, NULL);
	if (target->depth_view != VK_NULL_HANDLE) vkDestroyImageView(driver->device, target->depth_view, NULL);
	if (target->color_view != VK_NULL_HANDLE) vkDestroyImageView(driver->device, target->color_view, NULL);
	if (target->depth_image != VK_NULL_HANDLE) vkDestroyImage(driver->device, target->depth_image, NULL);
	if (target->depth_memory != VK_NULL_HANDLE) vkFreeMemory(driver->device, target->depth_memory, NULL);
	*target = (OnixVkRetiredRenderTarget){0};
}

static void onix_vk_release_render_target_attachments(OnixGPUDriver *driver, OnixVkRenderTarget *target) {
	OnixVkRetiredRenderTarget retired = {
		.framebuffer = target->framebuffer,
		.depth_image = target->depth_image,
		.depth_view = target->depth_view,
		.color_view = target->color_view,
		.depth_memory = target->depth_memory,
	};
	onix_vk_release_retired_target(driver, &retired);
	target->framebuffer = VK_NULL_HANDLE;
	target->depth_image = VK_NULL_HANDLE;
	target->depth_view = VK_NULL_HANDLE;
	target->color_view = VK_NULL_HANDLE;
	target->depth_memory = VK_NULL_HANDLE;
}

static void onix_vk_release_mesh_pipeline(OnixGPUDriver *driver, OnixVkMeshPipeline *pipeline) {
	if (pipeline->opaque != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, pipeline->opaque, NULL);
	if (pipeline->opaque_surface != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, pipeline->opaque_surface, NULL);
	for (uint32_t blend = 0; blend < ONIX_MESH_BLEND_MODE_COUNT; blend++) {
		if (pipeline->transparent[blend] != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, pipeline->transparent[blend], NULL);
	}
	for (uint32_t format = 0; format < 2; format++) {
		for (uint32_t index = 0; index < 1 + ONIX_MESH_BLEND_MODE_COUNT; index++) {
			if (pipeline->color_only[format][index]) vkDestroyPipeline(driver->device, pipeline->color_only[format][index], NULL);
		}
	}
	if (pipeline->vertex) vkDestroyShaderModule(driver->device, pipeline->vertex, NULL);
	if (pipeline->fragment) vkDestroyShaderModule(driver->device, pipeline->fragment, NULL);
	*pipeline = (OnixVkMeshPipeline){0};
}

static void onix_vk_release_material2d_pipeline(OnixGPUDriver *driver, OnixVkMaterial2DPipeline *pipeline) {
	if (pipeline->pipeline != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, pipeline->pipeline, NULL);
	for (uint32_t format = 0; format < 2; format++) {
		if (pipeline->color_only[format]) vkDestroyPipeline(driver->device, pipeline->color_only[format], NULL);
	}
	if (pipeline->vertex) vkDestroyShaderModule(driver->device, pipeline->vertex, NULL);
	if (pipeline->fragment) vkDestroyShaderModule(driver->device, pipeline->fragment, NULL);
	*pipeline = (OnixVkMaterial2DPipeline){0};
}

static void onix_vk_collect_resources(OnixGPUDriver *driver, uint32_t frame_index) {
	OnixVkFrame *frame = &driver->frames[frame_index];
	for (uint32_t index = 0; index < frame->retired_target_count; index++) onix_vk_release_retired_target(driver, &frame->retired_targets[index]);
	frame->retired_targets = onix_buffer_trim_array(frame->retired_targets, &frame->retired_target_capacity,
		frame->retired_target_count, sizeof(*frame->retired_targets), 4, &frame->retired_target_retention);
	frame->retired_target_count = 0;
	while (frame->retired_textures != 0) {
		uint32_t index = frame->retired_textures - 1;
		OnixVkTexture *resource = &driver->textures[index];
		frame->retired_textures = resource->retire_next;
		onix_vk_release_texture(driver, resource);
		onix_vk_textures_recycle(driver->textures, &driver->texture_slots, index);
	}
	while (frame->retired_mesh_pipelines != 0) {
		uint32_t index = frame->retired_mesh_pipelines - 1;
		OnixVkMeshPipeline *resource = &driver->mesh_pipelines[index];
		frame->retired_mesh_pipelines = resource->retire_next;
		onix_vk_release_mesh_pipeline(driver, resource);
		onix_vk_mesh_shaders_recycle(driver->mesh_pipelines, &driver->mesh_shader_slots, index);
	}
	while (frame->retired_material2d_pipelines != 0) {
		uint32_t index = frame->retired_material2d_pipelines - 1;
		OnixVkMaterial2DPipeline *resource = &driver->material2d_pipelines[index];
		frame->retired_material2d_pipelines = resource->retire_next;
		onix_vk_release_material2d_pipeline(driver, resource);
		onix_vk_material2d_shaders_recycle(driver->material2d_pipelines, &driver->material2d_shader_slots, index);
	}
	while (frame->retired_meshes != 0) {
		uint32_t index = frame->retired_meshes - 1;
		OnixVkMesh *resource = &driver->meshes[index];
		frame->retired_meshes = resource->retire_next;
		onix_vk_release_mesh(driver, resource);
		onix_vk_meshes_recycle(driver->meshes, &driver->mesh_slots, index);
	}
	while (frame->retired_render_targets != 0) {
		uint32_t index = frame->retired_render_targets - 1;
		OnixVkRenderTarget *resource = &driver->render_targets[index];
		frame->retired_render_targets = resource->retire_next;
		onix_vk_targets_recycle(driver->render_targets, &driver->target_slots, index);
	}
}

void onix_gpu_destroy(OnixGPUDriver *driver) {
    if (driver == NULL) {
        return;
    }
    if (driver->device != VK_NULL_HANDLE) {
        vkDeviceWaitIdle(driver->device);
		if (driver->readback.mapping != NULL) vkUnmapMemory(driver->device, driver->readback.memory);
		onix_vk_destroy_buffer(driver, &driver->readback.buffer, &driver->readback.memory);
		if (driver->readback_fence != VK_NULL_HANDLE) vkDestroyFence(driver->device, driver->readback_fence, NULL);
		for (uint32_t index = 0; index < driver->target_slots.count; index++) {
			onix_vk_release_render_target_attachments(driver, &driver->render_targets[index]);
		}
		if (driver->readback_command != VK_NULL_HANDLE) vkFreeCommandBuffers(driver->device, driver->command_pool, 1, &driver->readback_command);
		for (uint32_t index = 0; index < ONIX_VK_FRAMES_IN_FLIGHT; index++) {
			OnixVkFrame *frame = &driver->frames[index];
			for (uint32_t retired = 0; retired < frame->retired_target_count; retired++) {
				onix_vk_release_retired_target(driver, &frame->retired_targets[retired]);
			}
			frame->retired_target_count = 0;
		}
        for (uint32_t index = 0; index < driver->texture_slots.count; index++) {
            OnixVkTexture *texture = &driver->textures[index];
            if (texture->view != VK_NULL_HANDLE) vkDestroyImageView(driver->device, texture->view, NULL);
            if (texture->image != VK_NULL_HANDLE) vkDestroyImage(driver->device, texture->image, NULL);
            if (texture->pool == 0 && texture->memory != VK_NULL_HANDLE) vkFreeMemory(driver->device, texture->memory, NULL);
        }
		for (uint32_t index = 0; index < ONIX_VK_TEXTURE_POOL_BLOCKS; index++) {
			OnixVkTexturePool *block = &driver->texture_pools[index];
			if (block->memory != VK_NULL_HANDLE) vkFreeMemory(driver->device, block->memory, NULL);
			*block = (OnixVkTexturePool){0};
		}
		for (uint32_t index = 0; index < ONIX_VK_FRAMES_IN_FLIGHT; index++) {
			OnixVkFrame *frame = &driver->frames[index];
			onix_upload_cache_destroy(&frame->mesh_uploads);
			onix_vk_destroy_instance_buffers(driver, frame);
			onix_vk_destroy_staging_buffers(driver, frame);
			free(frame->retired_targets);
			free(frame->surface_sets);
			free(frame->surface_buckets);
			for (uint32_t pool = 0; pool < ONIX_GPU_SURFACE_SETS / ONIX_GPU_SURFACE_POOL_SETS; pool++) {
				if (frame->surface_pools[pool] != VK_NULL_HANDLE) vkDestroyDescriptorPool(driver->device, frame->surface_pools[pool], NULL);
			}
			if (frame->fence != VK_NULL_HANDLE) vkDestroyFence(driver->device, frame->fence, NULL);
			if (frame->image_available != VK_NULL_HANDLE) vkDestroySemaphore(driver->device, frame->image_available, NULL);
			if (frame->render_finished != VK_NULL_HANDLE) vkDestroySemaphore(driver->device, frame->render_finished, NULL);
			for (uint32_t pool = 0; pool < ONIX_VK_DEFORMATION_SETS / ONIX_VK_DEFORMATION_POOL_SETS; pool++) {
				if (frame->deformation_pools[pool]) vkDestroyDescriptorPool(driver->device, frame->deformation_pools[pool], NULL);
			}
		}
		for (uint32_t index = 0; index < driver->mesh_slots.count; index++) onix_vk_release_mesh(driver, &driver->meshes[index]);
		for (uint32_t index = 0; index < driver->mesh_block_count; index++) {
			OnixVkMeshBlock *block = &driver->mesh_blocks[index];
			if (block->buffer == VK_NULL_HANDLE) continue;
			vkUnmapMemory(driver->device, block->memory);
			onix_vk_destroy_buffer(driver, &block->buffer, &block->memory);
			free(block->buddy.states);
		}
		free(driver->mesh_blocks);
		onix_vk_destroy_buffer(driver, &driver->quad_vertex_buffer, &driver->quad_vertex_memory);
		onix_vk_destroy_buffer(driver, &driver->quad_index_buffer, &driver->quad_index_memory);
        if (driver->command_pool != VK_NULL_HANDLE) vkDestroyCommandPool(driver->device, driver->command_pool, NULL);
		if (driver->sprite_pipeline != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, driver->sprite_pipeline, NULL);
		if (driver->shape_pipeline != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, driver->shape_pipeline, NULL);
		if (driver->corner_mask_pipeline != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, driver->corner_mask_pipeline, NULL);
		if (driver->mesh_picking_pipeline != VK_NULL_HANDLE) vkDestroyPipeline(driver->device, driver->mesh_picking_pipeline, NULL);
		for (uint32_t variant = 0; variant < ONIX_MESH_VARIANT_COUNT; variant++) onix_vk_release_mesh_pipeline(driver, &driver->mesh_builtin[variant]);
		for (uint32_t index = 0; index < driver->mesh_shader_slots.count; index++) onix_vk_release_mesh_pipeline(driver, &driver->mesh_pipelines[index]);
		for (uint32_t index = 0; index < driver->material2d_shader_slots.count; index++) onix_vk_release_material2d_pipeline(driver, &driver->material2d_pipelines[index]);
		if (driver->lcd_pipeline) vkDestroyPipeline(driver->device, driver->lcd_pipeline, NULL);
		for (uint32_t index = 0; index < 3; index++) {
			for (uint32_t clear = 0; clear < 2; clear++) {
				if (driver->color_only_target_passes[index][clear]) vkDestroyRenderPass(driver->device, driver->color_only_target_passes[index][clear], NULL);
			}
		}
		for (uint32_t format = 0; format < 2; format++) {
			for (uint32_t kind = 0; kind < 11; kind++) {
				if (driver->color_only_pipelines[format][kind]) vkDestroyPipeline(driver->device, driver->color_only_pipelines[format][kind], NULL);
			}
		}
		if (driver->sprite_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(driver->device, driver->sprite_layout, NULL);
		if (driver->shape_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(driver->device, driver->shape_layout, NULL);
		if (driver->mesh_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(driver->device, driver->mesh_layout, NULL);
        for (int i=0;i<18;i++) if (driver->texture_samplers[i] != VK_NULL_HANDLE) vkDestroySampler(driver->device, driver->texture_samplers[i], NULL);
		for (uint32_t index = 0; index < driver->texture_descriptor_pool_count; index++) {
			if (driver->texture_descriptor_pools[index].pool) vkDestroyDescriptorPool(driver->device, driver->texture_descriptor_pools[index].pool, NULL);
		}
		onix_vk_destroy_buffer(driver, &driver->empty_deformation_buffer, &driver->empty_deformation_memory);
		if (driver->deformation_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(driver->device, driver->deformation_layout, NULL);
		if (driver->surface_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(driver->device, driver->surface_layout, NULL);
        if (driver->texture_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(driver->device, driver->texture_layout, NULL);
        onix_vk_destroy_swapchain(driver);
		if (driver->render_pass != VK_NULL_HANDLE) vkDestroyRenderPass(driver->device, driver->render_pass, NULL);
		if (driver->load_render_pass != VK_NULL_HANDLE) vkDestroyRenderPass(driver->device, driver->load_render_pass, NULL);
		for (uint32_t index = 0; index < 2; index++) if (driver->color_only_surface_passes[index]) vkDestroyRenderPass(driver->device, driver->color_only_surface_passes[index], NULL);
		for (uint32_t index = 0; index < 4; index++) {
			if (driver->target_render_passes[index] != VK_NULL_HANDLE) vkDestroyRenderPass(driver->device, driver->target_render_passes[index], NULL);
			if (driver->picking_target_render_passes[index] != VK_NULL_HANDLE) vkDestroyRenderPass(driver->device, driver->picking_target_render_passes[index], NULL);
		}
		if (driver->pipeline_cache != VK_NULL_HANDLE) vkDestroyPipelineCache(driver->device, driver->pipeline_cache, NULL);
        vkDestroyDevice(driver->device, NULL);
    }
    if (driver->surface != VK_NULL_HANDLE && driver->instance != VK_NULL_HANDLE) {
        vkDestroySurfaceKHR(driver->instance, driver->surface, NULL);
    }
#if defined(ONIX_VK_DRM)
    if (driver->display_acquired) {
        driver->release_display(driver->physical_device, driver->drm_display);
    }
#endif
    if (driver->instance != VK_NULL_HANDLE) {
        vkDestroyInstance(driver->instance, NULL);
    }
	free(driver->uploads);
	free(driver->textures);
	free(driver->render_targets);
	free(driver->mesh_pipelines);
	free(driver->material2d_pipelines);
	free(driver->meshes);
	free(driver);
}

int onix_gpu_resize(OnixGPUDriver *driver, uint32_t width, uint32_t height) {
    if (driver == NULL || width == 0 || height == 0) {
        return 1;
    }
    driver->native_surface.width = width;
    driver->native_surface.height = height;
    if (driver->frame_active) {
        return 1;
    }
    vkDeviceWaitIdle(driver->device);
    onix_vk_destroy_swapchain(driver);
    return onix_vk_create_swapchain(driver) && onix_vk_create_framebuffers(driver);
}

// Readback is synchronous, but a failed wait may leave a submission pending.
static void onix_vk_trim_readback(OnixGPUDriver *driver) {
	if (driver->readback_pending) return;
	if (onix_buffer_trim_capacity(&driver->readback_retention, driver->readback.capacity, driver->readback_used, 64u * 1024u)) {
		onix_vk_retire_host_memory(driver, driver->readback.capacity);
		if (driver->readback.mapping) vkUnmapMemory(driver->device, driver->readback.memory);
		onix_vk_destroy_buffer(driver, &driver->readback.buffer, &driver->readback.memory);
		driver->readback = (OnixVkInstanceBuffer){0};
	}
	driver->readback_used = 0;
}

int onix_gpu_begin_frame(OnixGPUDriver *driver, const float clear_color[4]) {
    if (driver == NULL || driver->frame_active) {
        return driver != NULL;
    }
	if (driver->swapchain_dirty && !onix_gpu_resize(driver, driver->native_surface.width, driver->native_surface.height)) {
		return 0;
	}
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (!onix_vk_result(vkWaitForFences(driver->device, 1, &frame->fence, VK_TRUE, UINT64_MAX), "vkWaitForFences")) {
		return 0;
	}
	onix_vk_observe_commands(driver, frame);
	onix_vk_trim_readback(driver);
	onix_vk_trim_uploads(driver);
	onix_upload_cache_reset(&frame->mesh_uploads);
	onix_vk_reset_deformation_descriptors(driver, frame);
	onix_vk_reset_surface_cache(driver, frame);
	if (driver->upload_count == 0) {
		onix_vk_trim_staging_buffers(driver, frame);
		frame->staging_buffer_index = 0;
		frame->staging_reserved = 0;
		for (uint32_t index = 0; index < frame->staging_buffer_count; index++) {
			frame->staging_buffers[index].offset = 0;
		}
	}
	frame->staging_prepared = 1;
	onix_vk_collect_resources(driver, driver->frame_index);
	onix_vk_trim_texture_descriptors(driver);
	onix_vk_trim_mesh_storage(driver);
	onix_vk_release_idle_host_memory(driver);
	VkResult acquire = vkAcquireNextImageKHR(driver->device, driver->swapchain, UINT64_MAX, frame->image_available, VK_NULL_HANDLE, &driver->image_index);
    if (acquire == VK_ERROR_OUT_OF_DATE_KHR) {
        if (!onix_gpu_resize(driver, driver->native_surface.width, driver->native_surface.height)) {
            return 0;
        }
		acquire = vkAcquireNextImageKHR(driver->device, driver->swapchain, UINT64_MAX, frame->image_available, VK_NULL_HANDLE, &driver->image_index);
    }
    if (acquire != VK_SUCCESS && acquire != VK_SUBOPTIMAL_KHR) {
        return onix_vk_result(acquire, "vkAcquireNextImageKHR");
    }
	vkResetCommandBuffer(frame->command_buffer, 0);
	VkCommandBufferBeginInfo begin = {.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO};
	if (!onix_vk_result(vkBeginCommandBuffer(frame->command_buffer, &begin), "vkBeginCommandBuffer(frame)")) {
        return 0;
    }
	onix_vk_trim_instance_buffers(driver, frame);
	frame->instance_bytes_used = 0;
	frame->instance_buffer_index = 0;
	for (uint32_t index = 0; index < frame->instance_buffer_count; index++) {
		frame->instance_buffers[index].offset = 0;
	}
	memcpy(driver->clear_color, clear_color, sizeof(driver->clear_color));
	driver->render_pass_active = 0;
	driver->active_render_target = 0;
	driver->frame_cleared = 0;
	driver->surface_depth_cleared = 0;
	driver->frame_active = 1;
    return 1;
}

void onix_gpu_set_sprite_blend(OnixGPUDriver *driver, int mode, float red, float green, float blue, float alpha) {
	if (driver == NULL) return;
	driver->sprite_blend = mode == 1;
	driver->blend_constant[0] = red;
	driver->blend_constant[1] = green;
	driver->blend_constant[2] = blue;
	driver->blend_constant[3] = alpha;
}

void onix_gpu_set_viewport(OnixGPUDriver *driver, int32_t x, int32_t y, int32_t width, int32_t height) {
    if (driver == NULL || width <= 0 || height <= 0) {
        return;
    }
    driver->viewport_rect.offset.x = x;
    driver->viewport_rect.offset.y = y;
    driver->viewport_rect.extent.width = (uint32_t)width;
    driver->viewport_rect.extent.height = (uint32_t)height;
}

void onix_gpu_set_scissor(OnixGPUDriver *driver, int enabled, int32_t x, int32_t y, int32_t width, int32_t height) {
    if (driver == NULL) {
        return;
    }
    driver->scissor_enabled = enabled;
    if (enabled) {
        driver->scissor_rect.offset.x = x;
        driver->scissor_rect.offset.y = y;
        driver->scissor_rect.extent.width = width > 0 ? (uint32_t)width : 0;
        driver->scissor_rect.extent.height = height > 0 ? (uint32_t)height : 0;
    }
}

void onix_gpu_set_vsync(OnixGPUDriver *driver, int enabled) {
    if (driver == NULL || driver->vsync_enabled == (enabled != 0)) {
        return;
    }
    driver->vsync_enabled = enabled != 0;
    driver->swapchain_dirty = 1;
}

void onix_gpu_set_corner_radius(OnixGPUDriver *driver, int radius) {
	if (driver == NULL) {
		return;
	}
	radius = radius > 0 ? radius : 0;
	if (driver->corner_radius == radius) {
		return;
	}
	driver->corner_radius = radius;
	driver->swapchain_dirty = 1;
}

static int onix_vk_apply_corner_mask(OnixGPUDriver *driver, VkCommandBuffer command_buffer) {
	int radius = driver->corner_radius;
	if (radius <= 0) {
		return 1;
	}
	VkPipeline pipeline = onix_vk_2d_pipeline(driver, 3, &driver->corner_mask_pipeline);
	if (pipeline == VK_NULL_HANDLE) {
		return 0;
	}
	if ((uint32_t)(radius * 2) > driver->extent.width) radius = (int)driver->extent.width / 2;
	if ((uint32_t)(radius * 2) > driver->extent.height) radius = (int)driver->extent.height / 2;
	OnixGPUShapeInstance instance = {
		.model = {2, 0, -1, 0, 2, -1},
		.local_model = {1, 0, 0, 0, 1, 0},
		.fill = {1, 1, 1, 1},
		.params = {(float)driver->extent.width, (float)driver->extent.height, (float)radius, 0},
		.kind = 1,
	};
	VkDeviceSize offset;
	OnixVkInstanceBuffer *instance_buffer = onix_vk_acquire_instance_buffer(driver, sizeof(instance), &offset);
	if (instance_buffer == NULL) {
		return 0;
	}
	memcpy((uint8_t *)instance_buffer->mapping + offset, &instance, sizeof(instance));
	OnixGPUSpritePush push = {.projection = {
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}};
	VkViewport viewport = {
		.x = 0,
		.y = 0,
		.width = (float)driver->extent.width,
		.height = (float)driver->extent.height,
		.minDepth = 0,
		.maxDepth = 1,
	};
	VkRect2D scissor = {{0, 0}, driver->extent};
	vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_GRAPHICS, pipeline);
	vkCmdSetViewport(command_buffer, 0, 1, &viewport);
	vkCmdSetScissor(command_buffer, 0, 1, &scissor);
	VkBuffer vertex_buffers[] = {driver->quad_vertex_buffer, instance_buffer->buffer};
	VkDeviceSize offsets[] = {0, offset};
	vkCmdBindVertexBuffers(command_buffer, 0, 2, vertex_buffers, offsets);
	vkCmdBindIndexBuffer(command_buffer, driver->quad_index_buffer, 0, VK_INDEX_TYPE_UINT16);
	vkCmdPushConstants(command_buffer, driver->shape_layout, VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT, 0, sizeof(push), &push);
	vkCmdDrawIndexed(command_buffer, 6, 1, 0, 0, 0);
	driver->frames[driver->frame_index].command_count++;
	return 1;
}

static void onix_vk_apply_dynamic_state(OnixGPUDriver *driver) {
	vkCmdSetBlendConstants(driver->frames[driver->frame_index].command_buffer, driver->blend_constant);
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	VkExtent2D extent = driver->extent;
	if (driver->active_render_target > 0 && driver->active_render_target <= driver->target_slots.count) {
		OnixVkRenderTarget *target = &driver->render_targets[driver->active_render_target - 1];
		extent = (VkExtent2D){target->width, target->height};
	}
    VkViewport viewport = {
        .x = (float)driver->viewport_rect.offset.x,
        .y = (float)driver->viewport_rect.offset.y,
        .width = (float)driver->viewport_rect.extent.width,
        .height = (float)driver->viewport_rect.extent.height,
        .minDepth = 0.0f,
        .maxDepth = 1.0f,
    };
	vkCmdSetViewport(frame->command_buffer, 0, 1, &viewport);
    VkRect2D scissor = driver->scissor_enabled
        ? driver->scissor_rect
		: (VkRect2D){{0, 0}, extent};
    if (scissor.offset.x < 0) {
        scissor.extent.width = scissor.extent.width > (uint32_t)-scissor.offset.x ? scissor.extent.width + scissor.offset.x : 0;
        scissor.offset.x = 0;
    }
    if (scissor.offset.y < 0) {
        scissor.extent.height = scissor.extent.height > (uint32_t)-scissor.offset.y ? scissor.extent.height + scissor.offset.y : 0;
        scissor.offset.y = 0;
    }
	if ((uint32_t)scissor.offset.x + scissor.extent.width > extent.width) {
		scissor.extent.width = extent.width > (uint32_t)scissor.offset.x ? extent.width - (uint32_t)scissor.offset.x : 0;
    }
	if ((uint32_t)scissor.offset.y + scissor.extent.height > extent.height) {
		scissor.extent.height = extent.height > (uint32_t)scissor.offset.y ? extent.height - (uint32_t)scissor.offset.y : 0;
    }
	vkCmdSetScissor(frame->command_buffer, 0, 1, &scissor);
}

int onix_gpu_present(OnixGPUDriver *driver) {
    if (driver == NULL || !driver->frame_active) {
        return driver != NULL;
	}
	if (driver->active_render_target != 0) {
		onix_vk_set_error("cannot present while a Vulkan render target is active");
		return 0;
	}
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (!onix_vk_prepare_draw(driver)) {
		driver->frame_active = 0;
		return 0;
	}
	if (!onix_vk_apply_corner_mask(driver, frame->command_buffer)) {
		driver->frame_active = 0;
		return 0;
	}
	vkCmdEndRenderPass(frame->command_buffer);
	driver->render_pass_active = 0;
	if (!onix_vk_result(vkEndCommandBuffer(frame->command_buffer), "vkEndCommandBuffer(frame)")) {
        driver->frame_active = 0;
        return 0;
    }
    VkPipelineStageFlags wait_stage = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT;
	VkSubmitInfo submit = {
        .sType = VK_STRUCTURE_TYPE_SUBMIT_INFO,
        .waitSemaphoreCount = 1,
		.pWaitSemaphores = &frame->image_available,
        .pWaitDstStageMask = &wait_stage,
        .commandBufferCount = 1,
		.pCommandBuffers = &frame->command_buffer,
        .signalSemaphoreCount = 1,
		.pSignalSemaphores = &frame->render_finished,
	};
#if defined(_WIN32)
	VkDeviceMemory external_memories[ONIX_VK_MAX_EXTERNAL_TEXTURES];
	uint64_t external_keys[ONIX_VK_MAX_EXTERNAL_TEXTURES] = {0};
	uint32_t external_timeouts[ONIX_VK_MAX_EXTERNAL_TEXTURES];
	uint32_t external_count = 0;
	for (uint32_t index = 0; index < driver->external_texture_count; index++) {
		OnixVkTexture *texture = &driver->textures[driver->external_textures[index]];
		if (texture->external && texture->external_ready && texture->image != VK_NULL_HANDLE && !texture->destroy_pending) {
			external_memories[external_count] = texture->memory;
			external_timeouts[external_count] = UINT32_MAX;
			external_count++;
		}
	}
	VkWin32KeyedMutexAcquireReleaseInfoKHR keyed_mutex = {
		.sType = VK_STRUCTURE_TYPE_WIN32_KEYED_MUTEX_ACQUIRE_RELEASE_INFO_KHR,
		.acquireCount = external_count,
		.pAcquireSyncs = external_memories,
		.pAcquireKeys = external_keys,
		.pAcquireTimeouts = external_timeouts,
		.releaseCount = external_count,
		.pReleaseSyncs = external_memories,
		.pReleaseKeys = external_keys,
	};
	if (external_count != 0) submit.pNext = &keyed_mutex;
#endif
	if (!onix_vk_result(vkResetFences(driver->device, 1, &frame->fence), "vkResetFences")) {
		driver->frame_active = 0;
		return 0;
	}
	if (!onix_vk_result(vkQueueSubmit(driver->queue, 1, &submit, frame->fence), "vkQueueSubmit(frame)")) {
        driver->frame_active = 0;
        return 0;
    }
	frame->staging_prepared = 0;
#if defined(_WIN32)
	if (external_count != 0) {
		for (uint32_t index = 0; index < driver->external_texture_count; index++) {
			OnixVkTexture *texture = &driver->textures[driver->external_textures[index]];
			texture->external_ready = 0;
		}
	}
#endif
    VkPresentInfoKHR present = {
        .sType = VK_STRUCTURE_TYPE_PRESENT_INFO_KHR,
        .waitSemaphoreCount = 1,
		.pWaitSemaphores = &frame->render_finished,
        .swapchainCount = 1,
        .pSwapchains = &driver->swapchain,
        .pImageIndices = &driver->image_index,
    };
	VkResult result = vkQueuePresentKHR(driver->queue, &present);
	driver->frame_active = 0;
	driver->frame_index = (driver->frame_index + 1) % ONIX_VK_FRAMES_IN_FLIGHT;
    if (result == VK_ERROR_OUT_OF_DATE_KHR || result == VK_SUBOPTIMAL_KHR) {
        driver->swapchain_dirty = 1;
        return 1;
    }
    return onix_vk_result(result, "vkQueuePresentKHR");
}

static int onix_vk_acquire_texture_slot(OnixGPUDriver *driver, uint32_t *slot) {
	if (onix_vk_textures_acquire(&driver->textures, &driver->texture_slots, slot)) return 1;
	onix_vk_set_error("failed to acquire Vulkan texture slot");
	return 0;
}

static int onix_vk_ensure_texture_sampler(OnixGPUDriver *driver, uint32_t index) {
	if (driver->texture_samplers[index] != VK_NULL_HANDLE) return 1;
	const VkSamplerAddressMode modes[] = {VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE, VK_SAMPLER_ADDRESS_MODE_REPEAT, VK_SAMPLER_ADDRESS_MODE_MIRRORED_REPEAT};
	VkSamplerCreateInfo sampler = {
		.sType = VK_STRUCTURE_TYPE_SAMPLER_CREATE_INFO,
		.magFilter = VK_FILTER_LINEAR,
		.minFilter = VK_FILTER_LINEAR,
		.mipmapMode = VK_SAMPLER_MIPMAP_MODE_LINEAR,
		.addressModeU = modes[index % 9 / 3],
		.addressModeV = modes[index % 3],
		.addressModeW = VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE,
		.maxLod = index >= 9 ? 1000.0f : 0.0f,
	};
	return onix_vk_result(vkCreateSampler(driver->device, &sampler, NULL, &driver->texture_samplers[index]), "vkCreateSampler");
}

static int onix_vk_create_texture_descriptor(OnixGPUDriver *driver, OnixVkTexture *texture, VkFormat format) {
	if (!onix_vk_ensure_texture_sampler(driver, texture->sampler_index)) return 0;
    VkImageViewCreateInfo view = {
        .sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO,
        .image = texture->image,
        .viewType = VK_IMAGE_VIEW_TYPE_2D,
        .format = format,
        .subresourceRange = {VK_IMAGE_ASPECT_COLOR_BIT, 0, texture->mip_levels, 0, 1},
    };
	if (texture->format == ONIX_TEXTURE_FORMAT_R8) {
		view.components = (VkComponentMapping){
			VK_COMPONENT_SWIZZLE_ONE,
			VK_COMPONENT_SWIZZLE_ONE,
			VK_COMPONENT_SWIZZLE_ONE,
			VK_COMPONENT_SWIZZLE_R,
		};
	}
    if (!onix_vk_result(vkCreateImageView(driver->device, &view, NULL, &texture->view), "vkCreateImageView(texture)")) return 0;
	if (!onix_vk_allocate_texture_descriptor(driver, texture)) return 0;
    VkDescriptorImageInfo image_info = {
        .sampler = driver->texture_samplers[texture->sampler_index],
        .imageView = texture->view,
        .imageLayout = VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
    };
    VkWriteDescriptorSet writes[] = {
        {.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET, .dstSet = texture->descriptor, .dstBinding = 0,
         .descriptorCount = 1, .descriptorType = VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, .pImageInfo = &image_info},
        {.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET, .dstSet = texture->descriptor, .dstBinding = 1,
         .descriptorCount = 1, .descriptorType = VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE, .pImageInfo = &image_info},
        {.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET, .dstSet = texture->descriptor, .dstBinding = 2,
         .descriptorCount = 1, .descriptorType = VK_DESCRIPTOR_TYPE_SAMPLER, .pImageInfo = &image_info},
    };
    vkUpdateDescriptorSets(driver->device, sizeof(writes) / sizeof(writes[0]), writes, 0, NULL);
    return 1;
}

uint32_t onix_gpu_create_texture_with_format(OnixGPUDriver *driver, uint32_t width, uint32_t height, uint32_t levels, uint32_t format, int mipmap, const uint8_t *pixels, size_t length, uint32_t wrap_u, uint32_t wrap_v) {
	if (wrap_u > 2 || wrap_v > 2 || mipmap < 0 || mipmap > 1 || driver == NULL || pixels == NULL || width == 0 || height == 0 ||
		format >= ONIX_TEXTURE_FORMAT_COUNT) {
		onix_vk_set_error("invalid Vulkan texture data");
		return 0;
	}
	int compressed = onix_vk_texture_format_is_compressed(format);
	if (levels == 0) levels = 1;
	if (levels > onix_vk_texture_mip_levels(width, height)) {
		onix_vk_set_error("invalid Vulkan texture mip levels");
		return 0;
	}
	/* A compressed payload carries every level; an uncompressed payload carries
	 * the base level and the device blits the rest of the chain from it. */
	size_t expected = (size_t)onix_gpu_texture_chain_size(format, width, height, compressed ? levels : 1);
	if (length != expected) {
		onix_vk_set_error("invalid Vulkan texture data");
		return 0;
	}
	VkFormat image_format = onix_vk_texture_image_format(format);
	if (compressed) {
		VkFormatProperties properties;
		vkGetPhysicalDeviceFormatProperties(driver->physical_device, image_format, &properties);
		const VkFormatFeatureFlags required = VK_FORMAT_FEATURE_SAMPLED_IMAGE_BIT | VK_FORMAT_FEATURE_TRANSFER_DST_BIT;
		if ((properties.optimalTilingFeatures & required) != required) {
			onix_vk_set_error("Vulkan device cannot sample this block-compressed texture format");
			return 0;
		}
	}
    uint32_t slot;
    if (!onix_vk_acquire_texture_slot(driver, &slot)) {
        return 0;
    }
	uint32_t mip_levels = levels;
	OnixVkTexture texture = {.width = width, .height = height, .mip_levels = mip_levels, .format = format, .sampler_index = (uint32_t)mipmap*9+wrap_u*3+wrap_v};
    VkImageCreateInfo image = {
        .sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
        .imageType = VK_IMAGE_TYPE_2D,
        .format = image_format,
        .extent = {width, height, 1},
		.mipLevels = mip_levels,
        .arrayLayers = 1,
        .samples = VK_SAMPLE_COUNT_1_BIT,
        .tiling = VK_IMAGE_TILING_OPTIMAL,
		.usage = VK_IMAGE_USAGE_TRANSFER_SRC_BIT | VK_IMAGE_USAGE_TRANSFER_DST_BIT | VK_IMAGE_USAGE_SAMPLED_BIT,
        .sharingMode = VK_SHARING_MODE_EXCLUSIVE,
        .initialLayout = VK_IMAGE_LAYOUT_UNDEFINED,
    };
    if (!onix_vk_result(vkCreateImage(driver->device, &image, NULL, &texture.image), "vkCreateImage(texture)")) {
        return 0;
    }
    VkMemoryRequirements requirements;
    vkGetImageMemoryRequirements(driver->device, texture.image, &requirements);
    VkDeviceSize bind_offset = 0;
    if (!onix_vk_acquire_texture_pool(driver, &requirements, &texture.pool, &texture.memory, &bind_offset)) {
        uint32_t memory_type = onix_vk_find_memory_type(driver, requirements.memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT);
        VkMemoryAllocateInfo allocation = {
            .sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
            .allocationSize = requirements.size,
            .memoryTypeIndex = memory_type,
        };
        if (memory_type == UINT32_MAX ||
            !onix_vk_result(vkAllocateMemory(driver->device, &allocation, NULL, &texture.memory), "vkAllocateMemory(texture)")) {
            vkDestroyImage(driver->device, texture.image, NULL);
            return 0;
        }
    }
    if (!onix_vk_result(vkBindImageMemory(driver->device, texture.image, texture.memory, bind_offset), "vkBindImageMemory")) {
        onix_vk_forget_texture_memory(driver, &texture);
        vkDestroyImage(driver->device, texture.image, NULL);
        return 0;
    }
	if (!onix_vk_create_texture_descriptor(driver, &texture, image_format)) {
		onix_vk_release_texture(driver, &texture);
		return 0;
	}
	if (compressed) {
		/* One upload per level: the payload is already the chain the sampler
		 * wants, so nothing here generates mips. */
		size_t offset = 0;
		uint32_t level_width = width, level_height = height;
		for (uint32_t level = 0; level < mip_levels; level++) {
			size_t level_bytes = (size_t)onix_vk_texture_level_size(format, level_width, level_height);
			if (!onix_vk_enqueue_texture_upload(driver, &texture, 0, 0, level_width, level_height, pixels + offset, level_bytes, 1, level, 0)) {
				onix_vk_cancel_texture_uploads(driver, &texture);
				onix_vk_release_texture(driver, &texture);
				return 0;
			}
			offset += level_bytes;
			level_width = level_width > 1 ? level_width / 2 : 1;
			level_height = level_height > 1 ? level_height / 2 : 1;
		}
	} else if (!onix_vk_enqueue_texture_upload(driver, &texture, 0, 0, width, height, pixels, length, 1, 0, 1)) {
		onix_vk_cancel_texture_uploads(driver, &texture);
		onix_vk_release_texture(driver, &texture);
		return 0;
	}
    onix_vk_textures_insert(driver->textures, &driver->texture_slots, slot, texture);
    return slot + 1;
}

static int onix_vk_acquire_render_target_slot(OnixGPUDriver *driver, uint32_t *slot) {
	if (onix_vk_targets_acquire(&driver->render_targets, &driver->target_slots, slot)) return 1;
	onix_vk_set_error("failed to acquire Vulkan render_target slot");
	return 0;
}

static int onix_vk_create_render_target_resources(
	OnixGPUDriver *driver,
	uint32_t width,
	uint32_t height,
	uint32_t color_format,
	int depth,
	OnixVkRenderTarget *target) {
	if (width == 0 || height == 0 || color_format > 2 || (color_format == 2 && depth)) {
		onix_vk_set_error("unsupported Vulkan render target descriptor");
		return 0;
	}
	if (!depth) {
		for (uint32_t clear = 0; clear < 2; clear++) {
			VkRenderPass *pass = &driver->color_only_target_passes[color_format][clear];
			if (*pass == VK_NULL_HANDLE && !onix_vk_create_target_render_pass(driver, clear,
				color_format == 2 ? VK_FORMAT_R8_UNORM : color_format == 1 ? VK_FORMAT_R32_UINT : driver->swapchain_format, 0, pass)) return 0;
		}
	}
	uint32_t texture_slot;
	if (!onix_vk_acquire_texture_slot(driver, &texture_slot)) return 0;
	VkFormat target_color_format = color_format == 2 ? VK_FORMAT_R8_UNORM : color_format == 1 ? VK_FORMAT_R32_UINT : driver->swapchain_format;
	OnixVkTexture color = {.width = width, .height = height, .mip_levels = 1, .format = color_format == 2 ? ONIX_TEXTURE_FORMAT_R8 : ONIX_TEXTURE_FORMAT_RGBA8};
	VkImageCreateInfo color_info = {
		.sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
		.imageType = VK_IMAGE_TYPE_2D,
		.format = target_color_format,
		.extent = {width, height, 1},
		.mipLevels = 1,
		.arrayLayers = 1,
		.samples = VK_SAMPLE_COUNT_1_BIT,
		.tiling = VK_IMAGE_TILING_OPTIMAL,
		.usage = VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT | VK_IMAGE_USAGE_SAMPLED_BIT | VK_IMAGE_USAGE_TRANSFER_SRC_BIT,
		.sharingMode = VK_SHARING_MODE_EXCLUSIVE,
		.initialLayout = VK_IMAGE_LAYOUT_UNDEFINED,
	};
	if (!onix_vk_result(vkCreateImage(driver->device, &color_info, NULL, &color.image), "vkCreateImage(render target color)")) return 0;
	VkMemoryRequirements requirements;
	vkGetImageMemoryRequirements(driver->device, color.image, &requirements);
	uint32_t memory_type = onix_vk_find_memory_type(driver, requirements.memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT);
	VkMemoryAllocateInfo allocation = {
		.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
		.allocationSize = requirements.size,
		.memoryTypeIndex = memory_type,
	};
	if (memory_type == UINT32_MAX ||
		!onix_vk_result(vkAllocateMemory(driver->device, &allocation, NULL, &color.memory), "vkAllocateMemory(render target color)") ||
		!onix_vk_result(vkBindImageMemory(driver->device, color.image, color.memory, 0), "vkBindImageMemory(render target color)") ||
		!onix_vk_create_texture_descriptor(driver, &color, target_color_format)) {
		onix_vk_release_texture(driver, &color);
		return 0;
	}

	OnixVkRenderTarget result = {.width = width, .height = height, .color_texture = texture_slot + 1, .color_format = color_format};
	if (depth) {
		VkImageCreateInfo depth_info = {
			.sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
			.imageType = VK_IMAGE_TYPE_2D,
			.format = VK_FORMAT_D32_SFLOAT,
			.extent = {width, height, 1},
			.mipLevels = 1,
			.arrayLayers = 1,
			.samples = VK_SAMPLE_COUNT_1_BIT,
			.tiling = VK_IMAGE_TILING_OPTIMAL,
			.usage = VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT,
			.sharingMode = VK_SHARING_MODE_EXCLUSIVE,
			.initialLayout = VK_IMAGE_LAYOUT_UNDEFINED,
		};
		if (!onix_vk_result(vkCreateImage(driver->device, &depth_info, NULL, &result.depth_image), "vkCreateImage(render target depth)")) {
			onix_vk_release_texture(driver, &color);
			return 0;
		}
		vkGetImageMemoryRequirements(driver->device, result.depth_image, &requirements);
		memory_type = onix_vk_find_memory_type(driver, requirements.memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT);
		allocation.allocationSize = requirements.size;
		allocation.memoryTypeIndex = memory_type;
		if (memory_type == UINT32_MAX ||
			!onix_vk_result(vkAllocateMemory(driver->device, &allocation, NULL, &result.depth_memory), "vkAllocateMemory(render target depth)") ||
			!onix_vk_result(vkBindImageMemory(driver->device, result.depth_image, result.depth_memory, 0), "vkBindImageMemory(render target depth)")) {
			onix_vk_release_texture(driver, &color);
			onix_vk_release_render_target_attachments(driver, &result);
			return 0;
		}
		VkImageViewCreateInfo depth_view = {
			.sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO,
			.image = result.depth_image,
			.viewType = VK_IMAGE_VIEW_TYPE_2D,
			.format = VK_FORMAT_D32_SFLOAT,
			.subresourceRange = {VK_IMAGE_ASPECT_DEPTH_BIT, 0, 1, 0, 1},
		};
		if (!onix_vk_result(vkCreateImageView(driver->device, &depth_view, NULL, &result.depth_view), "vkCreateImageView(render target depth)")) {
			onix_vk_release_texture(driver, &color);
			onix_vk_release_render_target_attachments(driver, &result);
			return 0;
		}
	}
	if (color_format == 2) {
		VkImageViewCreateInfo view = {
			.sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO,
			.image = color.image, .viewType = VK_IMAGE_VIEW_TYPE_2D,
			.format = target_color_format,
			.subresourceRange = {VK_IMAGE_ASPECT_COLOR_BIT, 0, 1, 0, 1},
		};
		if (!onix_vk_result(vkCreateImageView(driver->device, &view, NULL, &result.color_view), "vkCreateImageView(R8 attachment)")) {
			onix_vk_release_texture(driver, &color);
			onix_vk_release_render_target_attachments(driver, &result);
			return 0;
		}
	}
	VkImageView attachments[] = {result.color_view ? result.color_view : color.view, result.depth_view};
	VkFramebufferCreateInfo framebuffer = {
		.sType = VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO,
		.renderPass = onix_vk_target_pass(driver, &result, 3),
		.attachmentCount = depth ? 2 : 1,
		.pAttachments = attachments,
		.width = width,
		.height = height,
		.layers = 1,
	};
	if (!onix_vk_result(vkCreateFramebuffer(driver->device, &framebuffer, NULL, &result.framebuffer), "vkCreateFramebuffer(render target)")) {
		onix_vk_release_texture(driver, &color);
		onix_vk_release_render_target_attachments(driver, &result);
		return 0;
	}
	onix_vk_textures_insert(driver->textures, &driver->texture_slots, texture_slot, color);
	*target = result;
	return 1;
}

static int onix_vk_retire_render_target_attachments(OnixGPUDriver *driver, OnixVkRenderTarget *target) {
	if (target->framebuffer == VK_NULL_HANDLE && target->depth_image == VK_NULL_HANDLE) return 1;
	if (!driver->frame_active) {
		onix_vk_release_render_target_attachments(driver, target);
		return 1;
	}
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (frame->retired_target_count == frame->retired_target_capacity) {
		uint32_t capacity = frame->retired_target_capacity == 0 ? 4 : frame->retired_target_capacity * 2;
		OnixVkRetiredRenderTarget *targets = realloc(frame->retired_targets, (size_t)capacity * sizeof(*targets));
		if (targets == NULL) {
			onix_vk_set_error("failed to grow Vulkan retired render target list");
			return 0;
		}
		frame->retired_targets = targets;
		frame->retired_target_capacity = capacity;
	}
	frame->retired_targets[frame->retired_target_count++] = (OnixVkRetiredRenderTarget){
		.framebuffer = target->framebuffer,
		.depth_image = target->depth_image,
		.depth_view = target->depth_view,
		.color_view = target->color_view,
		.depth_memory = target->depth_memory,
	};
	target->framebuffer = VK_NULL_HANDLE;
	target->depth_image = VK_NULL_HANDLE;
	target->depth_view = VK_NULL_HANDLE;
	target->color_view = VK_NULL_HANDLE;
	target->depth_memory = VK_NULL_HANDLE;
	return 1;
}

uint32_t onix_gpu_create_render_target(
	OnixGPUDriver *driver,
	uint32_t width,
	uint32_t height,
	uint32_t color_format,
	int depth,
	uint32_t *color_texture) {
	if (driver == NULL || color_texture == NULL) {
		onix_vk_set_error("invalid Vulkan render target creation");
		return 0;
	}
	uint32_t slot;
	if (!onix_vk_acquire_render_target_slot(driver, &slot)) return 0;
	OnixVkRenderTarget target;
	if (!onix_vk_create_render_target_resources(driver, width, height, color_format, depth, &target)) return 0;
	onix_vk_targets_insert(driver->render_targets, &driver->target_slots, slot, target);
	*color_texture = target.color_texture;
	return slot + 1;
}

int onix_gpu_resize_render_target(
	OnixGPUDriver *driver,
	uint32_t handle,
	uint32_t width,
	uint32_t height,
	uint32_t color_format,
	int depth,
	uint32_t *color_texture) {
	if (driver == NULL || handle == 0 || handle > driver->target_slots.count || color_texture == NULL || driver->active_render_target == handle) {
		onix_vk_set_error("invalid Vulkan render target resize");
		return 0;
	}
	OnixVkRenderTarget *target = &driver->render_targets[handle - 1];
	if (target->framebuffer == VK_NULL_HANDLE || target->destroy_pending) {
		onix_vk_set_error("Vulkan render target is unavailable");
		return 0;
	}
	OnixVkRenderTarget replacement;
	if (!onix_vk_create_render_target_resources(driver, width, height, color_format, depth, &replacement)) return 0;
	if (!onix_vk_retire_render_target_attachments(driver, target)) {
		onix_vk_release_texture(driver, &driver->textures[replacement.color_texture - 1]);
		onix_vk_textures_recycle(driver->textures, &driver->texture_slots, replacement.color_texture - 1);
		onix_vk_release_render_target_attachments(driver, &replacement);
		return 0;
	}
	if (!onix_gpu_destroy_texture(driver, target->color_texture)) {
		onix_vk_release_texture(driver, &driver->textures[replacement.color_texture - 1]);
		onix_vk_textures_recycle(driver->textures, &driver->texture_slots, replacement.color_texture - 1);
		onix_vk_release_render_target_attachments(driver, &replacement);
		return 0;
	}
	*target = replacement;
	*color_texture = replacement.color_texture;
	return 1;
}

int onix_gpu_destroy_render_target(OnixGPUDriver *driver, uint32_t handle) {
	if (driver == NULL || handle == 0 || handle > driver->target_slots.count || driver->active_render_target == handle) {
		onix_vk_set_error("invalid Vulkan render target destruction");
		return 0;
	}
	OnixVkRenderTarget *target = &driver->render_targets[handle - 1];
	if (target->framebuffer == VK_NULL_HANDLE || target->destroy_pending) {
		onix_vk_set_error("Vulkan render target is already destroyed");
		return 0;
	}
	if (!onix_vk_retire_render_target_attachments(driver, target) || !onix_gpu_destroy_texture(driver, target->color_texture)) return 0;
	target->color_texture = 0;
	target->destroy_pending = 1;
	target->retire_frame = driver->frame_active
		? driver->frame_index
		: (driver->frame_index + ONIX_VK_FRAMES_IN_FLIGHT - 1) % ONIX_VK_FRAMES_IN_FLIGHT;
	target->retire_next = driver->frames[target->retire_frame].retired_render_targets;
	driver->frames[target->retire_frame].retired_render_targets = handle;
	return 1;
}

int onix_gpu_begin_render_target(
	OnixGPUDriver *driver,
	uint32_t handle,
	const float clear_color[4],
	float clear_depth,
	int clear_color_enabled,
	int clear_depth_enabled) {
	if (driver == NULL || !driver->frame_active || driver->active_render_target != 0 || handle == 0 || handle > driver->target_slots.count) {
		onix_vk_set_error("invalid Vulkan render target begin");
		return 0;
	}
	OnixVkRenderTarget *target = &driver->render_targets[handle - 1];
	if (target->framebuffer == VK_NULL_HANDLE || target->destroy_pending || !onix_vk_flush_uploads(driver)) {
		onix_vk_set_error("Vulkan render target is unavailable");
		return 0;
	}
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	if (driver->render_pass_active) {
		vkCmdEndRenderPass(frame->command_buffer);
		driver->render_pass_active = 0;
		driver->frame_cleared = 1;
	}
	onix_vk_initialize_external_textures(driver);
	uint32_t clear_flags = (clear_color_enabled ? 1u : 0u) | (clear_depth_enabled ? 2u : 0u);
	if (!target->initialized) clear_flags = 3;
	VkClearValue clear[] = {
		{.color = {{clear_color[0], clear_color[1], clear_color[2], clear_color[3]}}},
		{.depthStencil = {clear_depth, 0}},
	};
	VkRenderPassBeginInfo begin = {
		.sType = VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO,
		.renderPass = onix_vk_target_pass(driver, target, clear_flags),
		.framebuffer = target->framebuffer,
		.renderArea = {{0, 0}, {target->width, target->height}},
		.clearValueCount = 2,
		.pClearValues = clear,
	};
	vkCmdBeginRenderPass(frame->command_buffer, &begin, VK_SUBPASS_CONTENTS_INLINE);
	driver->render_pass_active = 1;
	driver->active_render_target = handle;
	target->initialized = 1;
	return 1;
}

int onix_gpu_end_render_target(OnixGPUDriver *driver) {
	if (driver == NULL || !driver->frame_active || !driver->render_pass_active || driver->active_render_target == 0) {
		onix_vk_set_error("invalid Vulkan render target end");
		return 0;
	}
	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	vkCmdEndRenderPass(frame->command_buffer);
	driver->render_pass_active = 0;
	driver->active_render_target = 0;
	return 1;
}

static int onix_vk_prepare_readback(OnixGPUDriver *driver, VkDeviceSize size) {
	if (driver->readback_pending) {
		if (!onix_vk_result(vkWaitForFences(driver->device, 1, &driver->readback_fence, VK_TRUE, UINT64_MAX), "vkWaitForFences(readback)")) return 0;
		driver->readback_pending = 0;
	}
	if (size > driver->readback_used) driver->readback_used = size;
	if (driver->readback.capacity < size) {
		VkDeviceSize capacity = (size + 4095u) & ~(VkDeviceSize)4095u;
		OnixVkInstanceBuffer buffer = {.capacity = capacity};
		VkMemoryPropertyFlags host = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
		if (!onix_vk_create_buffer(driver, capacity, VK_BUFFER_USAGE_TRANSFER_DST_BIT, host, &buffer.buffer, &buffer.memory) ||
			!onix_vk_result(vkMapMemory(driver->device, buffer.memory, 0, capacity, 0, &buffer.mapping), "vkMapMemory(readback)")) {
			onix_vk_destroy_buffer(driver, &buffer.buffer, &buffer.memory);
			return 0;
		}
		if (driver->readback.mapping != NULL) vkUnmapMemory(driver->device, driver->readback.memory);
		onix_vk_destroy_buffer(driver, &driver->readback.buffer, &driver->readback.memory);
		driver->readback = buffer;
	}
	if (driver->readback_command == VK_NULL_HANDLE) {
		VkCommandBufferAllocateInfo allocation = {
			.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
			.commandPool = driver->command_pool,
			.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY,
			.commandBufferCount = 1,
		};
		if (!onix_vk_result(vkAllocateCommandBuffers(driver->device, &allocation, &driver->readback_command), "vkAllocateCommandBuffers(readback)")) return 0;
	}
	if (driver->readback_fence == VK_NULL_HANDLE) {
		VkFenceCreateInfo fence = {.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO};
		if (!onix_vk_result(vkCreateFence(driver->device, &fence, NULL, &driver->readback_fence), "vkCreateFence(readback)")) return 0;
	}
	return onix_vk_result(vkResetCommandBuffer(driver->readback_command, 0), "vkResetCommandBuffer(readback)");
}

int onix_gpu_read_render_target(OnixGPUDriver *driver, uint32_t handle, int32_t x, int32_t y, uint32_t width, uint32_t height, uint8_t *destination, size_t length) {
	if (driver == NULL || driver->frame_active || handle == 0 || handle > driver->target_slots.count || destination == NULL || width == 0 || height == 0 ||
		x < 0 || y < 0) {
		onix_vk_set_error("invalid Vulkan render target readback");
		return 0;
	}
	OnixVkRenderTarget *target = &driver->render_targets[handle - 1];
	if (length != (size_t)width * height * (target->color_format == 2 ? 1 : 4) || target->framebuffer == VK_NULL_HANDLE || target->destroy_pending || !target->initialized || target->color_texture == 0 || (uint64_t)(uint32_t)x + width > target->width || (uint64_t)(uint32_t)y + height > target->height) {
		onix_vk_set_error("Vulkan render target readback is outside the image");
		return 0;
	}
	OnixVkTexture *texture = &driver->textures[target->color_texture - 1];
	if (!onix_vk_prepare_readback(driver, length)) return 0;
	VkCommandBuffer command = driver->readback_command;
	VkCommandBufferBeginInfo begin = {.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO, .flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT};
	if (!onix_vk_result(vkBeginCommandBuffer(command, &begin), "vkBeginCommandBuffer(readback)")) return 0;
	onix_vk_image_barrier(command, texture->image, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, 0, 1);
	VkBufferImageCopy copy = {
		.bufferOffset = 0,
		.imageSubresource = {VK_IMAGE_ASPECT_COLOR_BIT, 0, 0, 1},
		.imageOffset = {x, y, 0},
		.imageExtent = {width, height, 1},
	};
	vkCmdCopyImageToBuffer(command, texture->image, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, driver->readback.buffer, 1, &copy);
	onix_vk_image_barrier(command, texture->image, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, 0, 1);
	if (!onix_vk_result(vkEndCommandBuffer(command), "vkEndCommandBuffer(readback)")) return 0;
	VkSubmitInfo submit = {.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO, .commandBufferCount = 1, .pCommandBuffers = &command};
	if (!onix_vk_result(vkResetFences(driver->device, 1, &driver->readback_fence), "vkResetFences(readback)")) return 0;
	if (!onix_vk_result(vkQueueSubmit(driver->queue, 1, &submit, driver->readback_fence), "vkQueueSubmit(readback)")) return 0;
	driver->readback_pending = 1;
	if (!onix_vk_result(vkWaitForFences(driver->device, 1, &driver->readback_fence, VK_TRUE, UINT64_MAX), "vkWaitForFences(readback)")) return 0;
	driver->readback_pending = 0;
	memcpy(destination, driver->readback.mapping, length);
	return 1;
}

uint32_t onix_gpu_create_external_d3d11_texture(OnixGPUDriver *driver, uintptr_t handle, uint32_t width, uint32_t height) {
#if defined(_WIN32)
    if (driver == NULL || !driver->external_d3d11_supported || handle == 0 || width == 0 || height == 0) {
        onix_vk_set_error("Vulkan D3D11 external textures are unavailable");
        return 0;
    }
    if (driver->external_texture_count >= ONIX_VK_MAX_EXTERNAL_TEXTURES) {
        onix_vk_set_error("Vulkan external texture capacity exceeded %u slots", ONIX_VK_MAX_EXTERNAL_TEXTURES);
        return 0;
    }
    ID3D11Texture2D *native = (ID3D11Texture2D *)handle;
    D3D11_TEXTURE2D_DESC native_description;
    ID3D11Texture2D_GetDesc(native, &native_description);
    UINT shared_flags = D3D11_RESOURCE_MISC_SHARED_KEYEDMUTEX;
    if (native_description.Width != width || native_description.Height != height ||
        native_description.Format != DXGI_FORMAT_B8G8R8A8_UNORM ||
        (native_description.MiscFlags & shared_flags) != shared_flags) {
        onix_vk_set_error("D3D11 texture is not a compatible shared BGRA surface");
        return 0;
    }
    IDXGIResource *resource = NULL;
    HRESULT result = IUnknown_QueryInterface((IUnknown *)native, &IID_IDXGIResource, (void **)&resource);
    HANDLE shared = NULL;
    if (SUCCEEDED(result)) {
        result = IDXGIResource_GetSharedHandle(resource, &shared);
        IDXGIResource_Release(resource);
    }
    if (FAILED(result) || shared == NULL) {
        onix_vk_set_error("create D3D11 shared texture handle failed (0x%08lx)", (unsigned long)result);
        return 0;
    }
    uint32_t slot;
    if (!onix_vk_acquire_texture_slot(driver, &slot)) {
        return 0;
    }
    OnixVkTexture texture = {
        .width = width, .height = height, .mip_levels = 1,
        .external = 1, .external_ready = 1,
    };
    VkExternalMemoryImageCreateInfo external_image = {
        .sType = VK_STRUCTURE_TYPE_EXTERNAL_MEMORY_IMAGE_CREATE_INFO,
        .handleTypes = VK_EXTERNAL_MEMORY_HANDLE_TYPE_D3D11_TEXTURE_KMT_BIT,
    };
    VkImageCreateInfo image = {
        .sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
        .pNext = &external_image,
        .imageType = VK_IMAGE_TYPE_2D,
        .format = VK_FORMAT_B8G8R8A8_UNORM,
        .extent = {width, height, 1},
        .mipLevels = 1,
        .arrayLayers = 1,
        .samples = VK_SAMPLE_COUNT_1_BIT,
        .tiling = VK_IMAGE_TILING_OPTIMAL,
        .usage = VK_IMAGE_USAGE_SAMPLED_BIT,
        .sharingMode = VK_SHARING_MODE_EXCLUSIVE,
        .initialLayout = VK_IMAGE_LAYOUT_UNDEFINED,
    };
    if (!onix_vk_result(vkCreateImage(driver->device, &image, NULL, &texture.image), "vkCreateImage(D3D11)")) {
        CloseHandle(shared);
        return 0;
    }
    VkMemoryRequirements requirements;
    vkGetImageMemoryRequirements(driver->device, texture.image, &requirements);
    uint32_t memory_type = onix_vk_find_memory_type(driver, requirements.memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT);
    VkMemoryDedicatedAllocateInfo dedicated = {
        .sType = VK_STRUCTURE_TYPE_MEMORY_DEDICATED_ALLOCATE_INFO,
        .image = texture.image,
    };
    VkImportMemoryWin32HandleInfoKHR import = {
        .sType = VK_STRUCTURE_TYPE_IMPORT_MEMORY_WIN32_HANDLE_INFO_KHR,
        .pNext = &dedicated,
        .handleType = VK_EXTERNAL_MEMORY_HANDLE_TYPE_D3D11_TEXTURE_KMT_BIT,
        .handle = shared,
    };
    VkMemoryAllocateInfo allocation = {
        .sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
        .pNext = &import,
        .allocationSize = requirements.size,
        .memoryTypeIndex = memory_type,
    };
    int allocated = memory_type != UINT32_MAX &&
        onix_vk_result(vkAllocateMemory(driver->device, &allocation, NULL, &texture.memory), "vkAllocateMemory(D3D11)");
    if (!allocated) {
        CloseHandle(shared);
        onix_vk_release_texture(driver, &texture);
        return 0;
    }
    if (!onix_vk_result(vkBindImageMemory(driver->device, texture.image, texture.memory, 0), "vkBindImageMemory(D3D11)")) {
        onix_vk_release_texture(driver, &texture);
        return 0;
    }
    if (!onix_vk_create_texture_descriptor(driver, &texture, VK_FORMAT_B8G8R8A8_UNORM)) {
        onix_vk_release_texture(driver, &texture);
        return 0;
    }
    onix_vk_textures_insert(driver->textures, &driver->texture_slots, slot, texture);
    driver->pending_external[driver->pending_external_count++] = slot;
    driver->textures[slot].pending_external_index = driver->pending_external_count;
    driver->external_textures[driver->external_texture_count++] = slot;
    return slot + 1;
#else
    (void)driver; (void)handle; (void)width; (void)height;
    onix_vk_set_error("D3D11 external textures require Windows");
    return 0;
#endif
}

uint32_t onix_gpu_create_external_dmabuf_texture(OnixGPUDriver *driver, int fd, uint32_t width, uint32_t height, uint32_t stride, uint32_t offset, uint64_t modifier) {
#if defined(__linux__) && !defined(__ANDROID__)
    if (driver == NULL || !driver->external_dmabuf_supported || fd < 0 || width == 0 || height == 0 || stride < width * 4) {
        onix_vk_set_error("Vulkan DMA-BUF external textures are unavailable");
        if (fd >= 0) close(fd);
        return 0;
    }
    uint32_t slot;
    if (!onix_vk_acquire_texture_slot(driver, &slot)) {
        close(fd);
        return 0;
    }
    OnixVkTexture texture = {.width = width, .height = height, .mip_levels = 1, .external = 1, .external_ready = 1};
    VkExternalMemoryImageCreateInfo external_image = {
        .sType = VK_STRUCTURE_TYPE_EXTERNAL_MEMORY_IMAGE_CREATE_INFO,
        .handleTypes = VK_EXTERNAL_MEMORY_HANDLE_TYPE_DMA_BUF_BIT_EXT,
    };
    VkSubresourceLayout plane = {.offset = offset, .rowPitch = stride, .arrayPitch = 0, .depthPitch = 0, .size = 0};
    VkImageDrmFormatModifierExplicitCreateInfoEXT modifier_info = {
        .sType = VK_STRUCTURE_TYPE_IMAGE_DRM_FORMAT_MODIFIER_EXPLICIT_CREATE_INFO_EXT,
        .drmFormatModifier = modifier,
        .drmFormatModifierPlaneCount = 1,
        .pPlaneLayouts = &plane,
    };
    VkImageCreateInfo image = {
        .sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
        .pNext = &external_image,
        .imageType = VK_IMAGE_TYPE_2D,
        .format = VK_FORMAT_B8G8R8A8_UNORM,
        .extent = {width, height, 1},
        .mipLevels = 1,
        .arrayLayers = 1,
        .samples = VK_SAMPLE_COUNT_1_BIT,
        .tiling = modifier == 0 ? VK_IMAGE_TILING_LINEAR : VK_IMAGE_TILING_DRM_FORMAT_MODIFIER_EXT,
        .usage = VK_IMAGE_USAGE_SAMPLED_BIT,
        .sharingMode = VK_SHARING_MODE_EXCLUSIVE,
        .initialLayout = VK_IMAGE_LAYOUT_UNDEFINED,
    };
    if (modifier != 0) {
        modifier_info.pNext = image.pNext;
        image.pNext = &modifier_info;
    }
    if (!onix_vk_result(vkCreateImage(driver->device, &image, NULL, &texture.image), "vkCreateImage(DMA-BUF)")) {
        close(fd);
        return 0;
    }
    VkMemoryRequirements requirements;
    vkGetImageMemoryRequirements(driver->device, texture.image, &requirements);
    uint32_t memory_type = onix_vk_find_memory_type(driver, requirements.memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT);
    VkImportMemoryFdInfoKHR import = {
        .sType = VK_STRUCTURE_TYPE_IMPORT_MEMORY_FD_INFO_KHR,
        .handleType = VK_EXTERNAL_MEMORY_HANDLE_TYPE_DMA_BUF_BIT_EXT,
        .fd = fd,
    };
    VkMemoryAllocateInfo allocation = {
        .sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
        .pNext = &import,
        .allocationSize = requirements.size,
        .memoryTypeIndex = memory_type,
    };
    if (memory_type == UINT32_MAX || !onix_vk_result(vkAllocateMemory(driver->device, &allocation, NULL, &texture.memory), "vkAllocateMemory(DMA-BUF)")) {
        close(fd);
        onix_vk_release_texture(driver, &texture);
        return 0;
    }
    if (!onix_vk_result(vkBindImageMemory(driver->device, texture.image, texture.memory, 0), "vkBindImageMemory(DMA-BUF)")) {
        onix_vk_release_texture(driver, &texture);
        return 0;
    }
    if (!onix_vk_create_texture_descriptor(driver, &texture, VK_FORMAT_B8G8R8A8_UNORM)) {
        onix_vk_release_texture(driver, &texture);
        return 0;
    }
    onix_vk_textures_insert(driver->textures, &driver->texture_slots, slot, texture);
    driver->pending_external[driver->pending_external_count++] = slot;
    driver->textures[slot].pending_external_index = driver->pending_external_count;
    return slot + 1;
#else
    (void)driver; (void)fd; (void)width; (void)height; (void)stride; (void)offset; (void)modifier;
    onix_vk_set_error("DMA-BUF external textures require Linux");
    return 0;
#endif
}

int onix_gpu_refresh_external_texture(OnixGPUDriver *driver, uint32_t handle) {
    if (driver == NULL || handle == 0 || handle > driver->texture_slots.count) {
        onix_vk_set_error("invalid external Vulkan texture refresh");
        return 0;
    }
    OnixVkTexture *texture = &driver->textures[handle - 1];
    if (texture->image == VK_NULL_HANDLE || !texture->external || texture->destroy_pending) {
        onix_vk_set_error("Vulkan texture is not an active external texture");
        return 0;
    }
    texture->external_ready = 1;
    return 1;
}

static int onix_vk_validate_texture_updates(OnixGPUDriver *driver, const OnixGPUTextureUpdate *updates, uint32_t update_count, size_t length, int aligned) {
	if (driver == NULL || updates == NULL || update_count == 0 || length == 0) {
		onix_vk_set_error("invalid Vulkan texture updates");
		return 0;
	}
	for (uint32_t index = 0; index < update_count; index++) {
		const OnixGPUTextureUpdate *update = &updates[index];
		if (update->texture == 0 || update->texture > driver->texture_slots.count || update->x < 0 || update->y < 0 || update->width == 0 || update->height == 0 ||
			update->data_offset > length || update->data_length > length - update->data_offset || (aligned && (update->data_offset & 3u) != 0)) {
			onix_vk_set_error("invalid Vulkan texture update");
			return 0;
		}
		OnixVkTexture *texture = &driver->textures[update->texture - 1];
		/* Compressed textures are GPU-only: a region update cannot address
		 * whole texel blocks, so it is rejected rather than silently ignored. */
		if (onix_vk_texture_format_is_compressed(texture->format)) {
			onix_vk_set_error("Vulkan cannot update a block-compressed texture");
			return 0;
		}
		uint64_t bytes_per_pixel = texture->format == ONIX_TEXTURE_FORMAT_R8 ? 1 : 4;
		if (update->data_length != (uint64_t)update->width * update->height * bytes_per_pixel ||
			texture->image == VK_NULL_HANDLE || texture->destroy_pending ||
			(uint64_t)(uint32_t)update->x + update->width > texture->width ||
			(uint64_t)(uint32_t)update->y + update->height > texture->height) {
			onix_vk_set_error("Vulkan texture update is outside the image");
			return 0;
		}
	}
	return 1;
}

uint8_t *onix_gpu_prepare_texture_updates(OnixGPUDriver *driver, const OnixGPUTextureUpdate *updates, uint32_t update_count, size_t length) {
	if (!onix_vk_validate_texture_updates(driver, updates, update_count, length, 1)) return NULL;
	// Reserve the entire queue before acquiring storage, so failure cannot leave
	// pending uploads pointing at a batch Go has not filled yet.
	if (!onix_vk_grow_uploads(driver, update_count)) return NULL;
	VkDeviceSize offset;
	OnixVkStagingBuffer *staging = onix_vk_acquire_staging_buffer(driver, length, &offset);
	if (staging == NULL) return NULL;
	for (uint32_t index = 0; index < update_count; index++) {
		const OnixGPUTextureUpdate *update = &updates[index];
		OnixVkTexture *texture = &driver->textures[update->texture - 1];
		onix_vk_append_texture_upload(driver, texture, (OnixVkTextureUpload){
			.image = texture->image,
			.x = update->x,
			.y = update->y,
			.width = update->width,
			.height = update->height,
			.image_width = texture->width,
			.image_height = texture->height,
			.mip_levels = texture->mip_levels,
			.generate_mips = 1,
			.staging = staging->buffer,
			.staging_offset = offset + update->data_offset,
		});
	}
	return (uint8_t *)staging->mapping + offset;
}

int onix_gpu_commit_texture_updates(OnixGPUDriver *driver) {
	if (driver == NULL) return 0;
	if (!driver->frame_active && onix_vk_staging_bytes_reserved(&driver->frames[driver->frame_index]) >= ONIX_VK_STAGING_BURST_LIMIT) {
		return onix_vk_submit_staging_burst(driver);
	}
	return 1;
}

int onix_gpu_update_textures(OnixGPUDriver *driver, const OnixGPUTextureUpdate *updates, uint32_t update_count, const uint8_t *pixels, size_t length) {
	if (pixels == NULL) {
		onix_vk_set_error("invalid Vulkan texture update pixels");
		return 0;
	}
	if (!onix_vk_validate_texture_updates(driver, updates, update_count, length, 0)) return 0;
	for (uint32_t index = 0; index < update_count; index++) {
		const OnixGPUTextureUpdate *update = &updates[index];
		OnixVkTexture *texture = &driver->textures[update->texture - 1];
		if (!onix_vk_enqueue_texture_upload(
			driver,
			texture,
			update->x,
			update->y,
			update->width,
			update->height,
			pixels + (size_t)update->data_offset,
			(size_t)update->data_length,
			0,
			0,
			1)) {
			return 0;
		}
	}
	return 1;
}

int onix_gpu_destroy_texture(OnixGPUDriver *driver, uint32_t handle) {
	if (driver == NULL || handle == 0 || handle > driver->texture_slots.count) {
		onix_vk_set_error("invalid Vulkan texture destruction");
		return 0;
	}
	OnixVkTexture *texture = &driver->textures[handle - 1];
	if (texture->image == VK_NULL_HANDLE || texture->destroy_pending) {
		onix_vk_set_error("Vulkan texture is already destroyed");
		return 0;
	}
	onix_vk_cancel_texture_uploads(driver, texture);
	if (texture->pending_external_index != 0) {
		uint32_t index = texture->pending_external_index - 1;
		uint32_t moved = driver->pending_external[--driver->pending_external_count];
		driver->pending_external[index] = moved;
		driver->textures[moved].pending_external_index = index + 1;
		texture->pending_external_index = 0;
	}
#if defined(_WIN32)
	if (texture->external) {
		for (uint32_t index = 0; index < driver->external_texture_count; index++) {
			if (driver->external_textures[index] == handle-1) { driver->external_textures[index] = driver->external_textures[--driver->external_texture_count]; break; }
		}
	}
#endif
	texture->destroy_pending = 1;
	texture->retire_frame = driver->frame_active
		? driver->frame_index
		: (driver->frame_index + ONIX_VK_FRAMES_IN_FLIGHT - 1) % ONIX_VK_FRAMES_IN_FLIGHT;
	texture->retire_next = driver->frames[texture->retire_frame].retired_textures;
	driver->frames[texture->retire_frame].retired_textures = handle;
	return 1;
}

uint32_t onix_gpu_create_mesh(
	OnixGPUDriver *driver,
	const OnixGPUVertex3D *vertices,
	uint32_t vertex_count,
	const uint32_t *indices,
	uint32_t index_count) {
	if (driver == NULL || vertices == NULL || vertex_count == 0 || indices == NULL || index_count == 0) {
		onix_vk_set_error("invalid Vulkan mesh data");
		return 0;
	}
	uint32_t slot;
	if (!onix_vk_acquire_mesh_slot(driver, &slot)) return 0;
	OnixVkMesh mesh = {.vertex_count = vertex_count, .index_count = index_count};
	VkDeviceSize vertex_size = (VkDeviceSize)vertex_count * sizeof(OnixGPUVertex3D);
	VkDeviceSize index_size = (VkDeviceSize)index_count * sizeof(uint32_t);
	VkDeviceSize index_base = (vertex_size + 15u) & ~(VkDeviceSize)15u;
	if (index_base + index_size > SIZE_MAX) { onix_vk_set_error("Vulkan mesh exceeds addressable memory"); return 0; }
	void *mapping;
	if (!onix_vk_mesh_storage(driver, index_base + index_size, &mesh.geometry, &mesh.vertex_buffer, &mesh.vertex_offset, &mapping)) return 0;
	mesh.index_buffer = mesh.vertex_buffer;
	mesh.index_offset = mesh.vertex_offset + index_base;
	memcpy(mapping, vertices, (size_t)vertex_size);
	memcpy((uint8_t *)mapping + index_base, indices, (size_t)index_size);

	onix_vk_meshes_insert(driver->meshes, &driver->mesh_slots, slot, mesh);
	return slot + 1;
}

int onix_gpu_set_mesh_deformation(OnixGPUDriver *driver, uint32_t handle, const uint32_t *words, uint32_t word_count, uint32_t vertex_count) {
	if (driver == NULL || handle == 0 || handle > driver->mesh_slots.count || words == NULL || word_count == 0) { onix_vk_set_error("invalid static mesh deformation"); return 0; }
	OnixVkMesh *mesh = &driver->meshes[handle-1];
	VkDeviceSize bytes = (VkDeviceSize)word_count * 4;
	if (mesh->vertex_buffer == VK_NULL_HANDLE || mesh->destroy_pending || mesh->deformation_buffer != VK_NULL_HANDLE || mesh->vertex_count != vertex_count || bytes > driver->physical_device_properties.limits.maxStorageBufferRange) { onix_vk_set_error("invalid static mesh deformation resource"); return 0; }
	void *mapping;
	if (!onix_vk_mesh_storage(driver, bytes, &mesh->deformation, &mesh->deformation_buffer, &mesh->deformation_offset, &mapping)) return 0;
	memcpy(mapping, words, (size_t)bytes);
	mesh->deformation_size = bytes;
	return 1;
}

int onix_gpu_destroy_mesh(OnixGPUDriver *driver, uint32_t handle) {
	if (driver == NULL || handle == 0 || handle > driver->mesh_slots.count) {
		onix_vk_set_error("invalid Vulkan mesh destruction");
		return 0;
	}
	OnixVkMesh *mesh = &driver->meshes[handle - 1];
	if (mesh->vertex_buffer == VK_NULL_HANDLE || mesh->destroy_pending) {
		onix_vk_set_error("Vulkan mesh is already destroyed");
		return 0;
	}
	mesh->destroy_pending = 1;
	mesh->retire_frame = driver->frame_active
		? driver->frame_index
		: (driver->frame_index + ONIX_VK_FRAMES_IN_FLIGHT - 1) % ONIX_VK_FRAMES_IN_FLIGHT;
	mesh->retire_next = driver->frames[mesh->retire_frame].retired_meshes;
	driver->frames[mesh->retire_frame].retired_meshes = handle;
	return 1;
}

int onix_gpu_draw_sprites(
	OnixGPUDriver *driver,
	const OnixGPUSpritePush *push,
	const OnixGPUInstance *instances,
	uint32_t instance_count,
	const OnixNativeSpriteDraw *draws,
	uint32_t draw_count) {
	if (driver == NULL || !driver->frame_active || push == NULL || instances == NULL || instance_count == 0 || draws == NULL || draw_count == 0) {
		onix_vk_set_error("invalid Vulkan sprite draw");
		return 0;
	}
	if (!onix_vk_prepare_draw(driver)) {
		return 0;
	}
	VkDeviceSize size = 0;
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeSpriteDraw *draw = &draws[index];
		if (draw->instance_count == 0 || (uint64_t)draw->first_instance + draw->instance_count > instance_count) break;
		if (draw->shader && size < (VkDeviceSize)draw->first_instance * ONIX_SPRITE_CUSTOM_BYTES) size = (VkDeviceSize)draw->first_instance * ONIX_SPRITE_CUSTOM_BYTES;
		size += (VkDeviceSize)draw->instance_count * onix_sprite_stride(draw->shader);
	}
	if (size == 0) { onix_vk_set_error("invalid Vulkan sprite draw range"); return 0; }
	VkDeviceSize offset;
	OnixVkInstanceBuffer *instance_buffer = onix_vk_acquire_instance_buffer(driver, size, &offset);
    if (instance_buffer == NULL) {
        return 0;
	}
	VkDeviceSize packed_offset = 0;
	VkCommandBuffer command_buffer = driver->frames[driver->frame_index].command_buffer;
	onix_vk_apply_dynamic_state(driver);
	VkBuffer vertex_buffers[] = {driver->quad_vertex_buffer, instance_buffer->buffer};
	VkDeviceSize offsets[] = {0, offset};
	vkCmdBindVertexBuffers(command_buffer, 0, 2, vertex_buffers, offsets);
	vkCmdBindIndexBuffer(command_buffer, driver->quad_index_buffer, 0, VK_INDEX_TYPE_UINT16);
	vkCmdPushConstants(command_buffer, driver->sprite_layout, VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT, 0, sizeof(*push), push);
	VkPipeline pipeline = VK_NULL_HANDLE;
	VkDescriptorSet bound_descriptors[4] = {VK_NULL_HANDLE};
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeSpriteDraw *draw = &draws[index];
		if (draw->instance_count == 0 || (uint64_t)draw->first_instance + draw->instance_count > instance_count ||
			(draw->shader > 0 && (draw->shader > driver->material2d_shader_slots.count ||
				driver->material2d_pipelines[draw->shader - 1].vertex == VK_NULL_HANDLE || driver->material2d_pipelines[draw->shader - 1].destroy_pending))) {
			onix_vk_set_error("invalid Vulkan sprite draw range");
			return 0;
		}
		VkDescriptorSet descriptors[4];
		int descriptors_changed = 0;
		for (uint32_t slot = 0; slot < 4; slot++) {
			uint32_t texture = slot ? draw->additional_textures[slot - 1] : draw->texture;
			if (texture == 0) texture = draw->texture;
			if (texture == 0 || texture > driver->texture_slots.count || driver->textures[texture - 1].image == VK_NULL_HANDLE || driver->textures[texture - 1].destroy_pending) {
				onix_vk_set_error("invalid Vulkan sprite texture slot");
				return 0;
			}
			descriptors[slot] = driver->textures[texture - 1].descriptor;
			descriptors_changed |= descriptors[slot] != bound_descriptors[slot];
		}
		VkPipeline next_pipeline = VK_NULL_HANDLE;
		if (draw->shader == 0) {
			int kind = driver->sprite_blend ? 9 + driver->sprite_blend : 0;
			VkPipeline *normal = driver->sprite_blend ? &driver->lcd_pipeline : &driver->sprite_pipeline;
			next_pipeline = onix_vk_2d_pipeline(driver, kind, normal);
		} else {
			OnixVkMaterial2DPipeline *custom = &driver->material2d_pipelines[draw->shader - 1];
			VkPipeline *variant = onix_vk_color_only(driver) ? &custom->color_only[onix_vk_r8_target(driver)] : &custom->pipeline;
			if (*variant == VK_NULL_HANDLE) *variant = onix_vk_create_pipeline(driver, custom->vertex, custom->fragment,
				driver->sprite_layout, (onix_vk_color_only(driver) ? ONIX_VK_PIPELINE_NO_DEPTH : 0) | (onix_vk_r8_target(driver) ? ONIX_VK_PIPELINE_R8 : 0));
			next_pipeline = *variant;
		}
		if (next_pipeline == VK_NULL_HANDLE) return 0;
		if (pipeline != next_pipeline) {
			pipeline = next_pipeline;
			vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_GRAPHICS, pipeline);
		}
		if (descriptors_changed) {
			vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_GRAPHICS, driver->sprite_layout, 0, 4, descriptors, 0, NULL);
			memcpy(bound_descriptors, descriptors, sizeof(descriptors));
		}
		if (draw->shader && packed_offset < (VkDeviceSize)draw->first_instance * ONIX_SPRITE_CUSTOM_BYTES) packed_offset = (VkDeviceSize)draw->first_instance * ONIX_SPRITE_CUSTOM_BYTES;
		onix_sprite_copy((uint8_t *)instance_buffer->mapping + offset + packed_offset, instances + draw->first_instance, draw->instance_count, draw->shader);
		uint32_t first_instance = draw->shader ? draw->first_instance : 0;
		VkDeviceSize draw_offset = offset + packed_offset - (VkDeviceSize)first_instance * onix_sprite_stride(draw->shader);
		vkCmdBindVertexBuffers(command_buffer, 1, 1, &instance_buffer->buffer, &draw_offset);
		vkCmdDrawIndexed(command_buffer, 6, draw->instance_count, 0, 0, first_instance);
		packed_offset += (VkDeviceSize)draw->instance_count * onix_sprite_stride(draw->shader);
		driver->frames[driver->frame_index].command_count++;
	}
	return 1;
}

int onix_gpu_draw_shapes(
	OnixGPUDriver *driver,
	const OnixGPUSpritePush *push,
	const OnixGPUShapeInstance *instances,
	uint32_t instance_count) {
	if (driver == NULL || !driver->frame_active || push == NULL || instances == NULL || instance_count == 0) {
		onix_vk_set_error("invalid Vulkan shape draw");
		return 0;
	}
	if (!onix_vk_ensure_shape_pipeline(driver)) {
		return 0;
	}
	if (!onix_vk_prepare_draw(driver)) {
		return 0;
	}
	VkDeviceSize size = (VkDeviceSize)instance_count * sizeof(OnixGPUShapeInstance);
	VkDeviceSize offset;
	OnixVkInstanceBuffer *instance_buffer = onix_vk_acquire_instance_buffer(driver, size, &offset);
	if (instance_buffer == NULL) {
		return 0;
	}
	memcpy((uint8_t *)instance_buffer->mapping + offset, instances, (size_t)size);
	VkCommandBuffer command_buffer = driver->frames[driver->frame_index].command_buffer;
	onix_vk_apply_dynamic_state(driver);
	VkBuffer vertex_buffers[] = {driver->quad_vertex_buffer, instance_buffer->buffer};
	VkDeviceSize offsets[] = {0, offset};
	vkCmdBindVertexBuffers(command_buffer, 0, 2, vertex_buffers, offsets);
	vkCmdBindIndexBuffer(command_buffer, driver->quad_index_buffer, 0, VK_INDEX_TYPE_UINT16);
	vkCmdPushConstants(command_buffer, driver->shape_layout, VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT, 0, sizeof(*push), push);
	uint32_t start = 0;
	while (start < instance_count) {
		uint32_t kind = instances[start].kind == 3 ? 3 : 1;
		uint32_t end = start + 1;
		while (end < instance_count && (instances[end].kind == 3 ? 3 : 1) == kind) end++;
		if (kind == 3 && !onix_vk_ensure_corner_mask_pipeline(driver)) return 0;
		VkPipeline pipeline = onix_vk_2d_pipeline(driver, kind, kind == 3 ? &driver->corner_mask_pipeline : &driver->shape_pipeline);
		vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_GRAPHICS, pipeline);
		vkCmdDrawIndexed(command_buffer, 6, end - start, 0, 0, start);
		driver->frames[driver->frame_index].command_count++;
		start = end;
	}
	return 1;
}

int onix_gpu_draw_meshes(
	OnixGPUDriver *driver,
	const OnixGPUMeshPush *push,
	const OnixGPUVertex3D *vertices,
	uint32_t vertex_count,
	const uint32_t *indices,
	uint32_t index_count,
	const OnixGPUInstance3D *instances,
	uint32_t instance_count,
	const uint32_t *deformation_words,
	uint32_t deformation_word_count,
	const OnixNativeMeshDraw3D *draws,
	uint32_t draw_count, const float *lighting) {
	if (driver == NULL || !driver->frame_active || push == NULL || instances == NULL || instance_count == 0 || deformation_words == NULL || deformation_word_count == 0 || draws == NULL || draw_count == 0) {
		onix_vk_set_error("invalid Vulkan mesh draw");
		return 0;
	}
	int dynamic = vertices != NULL || vertex_count != 0 || indices != NULL || index_count != 0;
	if (dynamic && (vertices == NULL || vertex_count == 0 || indices == NULL || index_count == 0)) {
		onix_vk_set_error("incomplete Vulkan dynamic mesh data");
		return 0;
	}
	if (!onix_vk_ensure_mesh_resources(driver) || !onix_vk_require_surface_depth(driver) || !onix_vk_prepare_draw(driver)) return 0;
	onix_vk_prepare_surface_depth(driver);

	OnixNativeUpload vertex_upload = {0}, index_upload = {0}, instance_upload;
	if (dynamic && (!onix_vk_mesh_upload(driver, vertices, (VkDeviceSize)vertex_count * sizeof(*vertices), 16, 1, &vertex_upload) ||
		!onix_vk_mesh_upload(driver, indices, (VkDeviceSize)index_count * sizeof(*indices), 16, 1, &index_upload))) return 0;
	if (!onix_vk_mesh_upload(driver, instances, (VkDeviceSize)instance_count * sizeof(*instances), 16, 0, &instance_upload)) return 0;
	VkBuffer vertex_handle = (VkBuffer)vertex_upload.resource, index_handle = (VkBuffer)index_upload.resource;
	VkBuffer instance_handle = (VkBuffer)instance_upload.resource;
	VkDeviceSize vertex_offset = vertex_upload.offset, index_offset = index_upload.offset, instance_offset = instance_upload.offset;
	VkDeviceSize deformation_size = (VkDeviceSize)deformation_word_count * sizeof(uint32_t);
	if (deformation_size > driver->physical_device_properties.limits.maxStorageBufferRange) {
		onix_vk_set_error("Vulkan deformation buffer exceeds maxStorageBufferRange"); return 0;
	}
	OnixNativeUpload deformation_upload;
	if (!onix_vk_mesh_upload(driver, deformation_words, deformation_size, driver->physical_device_properties.limits.minStorageBufferOffsetAlignment, 1, &deformation_upload)) return 0;
	VkBuffer deformation_handle = (VkBuffer)deformation_upload.resource;
	VkDeviceSize deformation_offset = deformation_upload.offset;

	OnixVkFrame *frame = &driver->frames[driver->frame_index];
	VkDescriptorSet deformation_descriptor = onix_vk_deformation_descriptor(driver, frame);
	if (deformation_descriptor == VK_NULL_HANDLE) return 0;
	VkDescriptorBufferInfo deformation_info = {.buffer = deformation_handle, .offset = deformation_offset, .range = deformation_size};
	VkWriteDescriptorSet deformation_write = {
		.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET,
		.dstSet = deformation_descriptor,
		.dstBinding = 0,
		.descriptorCount = 1,
		.descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER,
		.pBufferInfo = &deformation_info,
	};
	VkDeviceSize lighting_offset;
	OnixVkInstanceBuffer *lighting_buffer = onix_vk_acquire_instance_buffer_aligned(driver, ONIX_GPU_MESH_LIGHTING_BYTES, driver->physical_device_properties.limits.minUniformBufferOffsetAlignment, &lighting_offset);
	if (lighting_buffer == NULL) return 0;
	float *lighting_dst = (float *)((uint8_t *)lighting_buffer->mapping + lighting_offset);
	if (lighting != NULL) memcpy(lighting_dst, lighting, ONIX_GPU_MESH_LIGHTING_BYTES);
	else { memset(lighting_dst, 0, ONIX_GPU_MESH_LIGHTING_BYTES); memcpy(lighting_dst, push, sizeof(*push)); }
	// Both surfaces use the vertex shader's Y flip and the same front face.
	lighting_dst[299] = 0.0f;
	VkDescriptorBufferInfo lighting_info = {.buffer=lighting_buffer->buffer,.offset=lighting_offset,.range=ONIX_GPU_MESH_LIGHTING_BYTES};
	VkWriteDescriptorSet lighting_write = {.sType=VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET,.dstSet=deformation_descriptor,.dstBinding=1,.descriptorCount=1,.descriptorType=VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER,.pBufferInfo=&lighting_info};
	VkWriteDescriptorSet writes[] = {deformation_write, lighting_write};
	vkUpdateDescriptorSets(driver->device, 2, writes, 0, NULL);

	VkCommandBuffer command_buffer = driver->frames[driver->frame_index].command_buffer;
	onix_vk_apply_dynamic_state(driver);
	VkPipeline pipeline = VK_NULL_HANDLE;
	uint32_t bound_mesh = UINT32_MAX;
	VkDescriptorSet bound_descriptors[4] = {0};
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeMeshDraw3D *draw = &draws[index];
		if (draw->texture == 0 || draw->texture > driver->texture_slots.count || draw->normal_texture > driver->texture_slots.count ||
			driver->textures[draw->texture - 1].image == VK_NULL_HANDLE || driver->textures[draw->texture - 1].destroy_pending || draw->index_count == 0 || draw->instance_count == 0 ||
			(draw->normal_texture != 0 && (driver->textures[draw->normal_texture - 1].image == VK_NULL_HANDLE || driver->textures[draw->normal_texture - 1].destroy_pending)) ||
			(uint64_t)draw->first_instance + draw->instance_count > instance_count || draw->alpha_mode > 2 || draw->blend_mode >= ONIX_MESH_BLEND_MODE_COUNT ||
			(draw->shader > 0 && (draw->shader > driver->mesh_shader_slots.count || driver->mesh_pipelines[draw->shader - 1].vertex == VK_NULL_HANDLE ||
				driver->mesh_pipelines[draw->shader - 1].destroy_pending))) {
			onix_vk_set_error("invalid Vulkan mesh draw range");
			return 0;
		}
		if (draw->mesh == 0) {
			if (!dynamic || (uint64_t)draw->first_index + draw->index_count > index_count || draw->base_vertex < 0 || (uint64_t)(uint32_t)draw->base_vertex >= vertex_count) {
				onix_vk_set_error("invalid Vulkan dynamic mesh draw range");
				return 0;
			}
		} else {
			if (draw->mesh > driver->mesh_slots.count) {
				onix_vk_set_error("invalid Vulkan static mesh handle");
				return 0;
			}
			OnixVkMesh *mesh = &driver->meshes[draw->mesh - 1];
			if (mesh->vertex_buffer == VK_NULL_HANDLE || mesh->destroy_pending || (uint64_t)draw->first_index + draw->index_count > mesh->index_count ||
				draw->base_vertex < 0 || (uint64_t)(uint32_t)draw->base_vertex >= mesh->vertex_count) {
				onix_vk_set_error("invalid Vulkan static mesh draw range");
				return 0;
			}
		}
		VkDescriptorSet surface_set = onix_vk_surface_descriptor(driver, frame, draw);
		if (surface_set == VK_NULL_HANDLE) return 0;
		if (draw->mesh != bound_mesh) {
			VkBuffer geometry_vertex = draw->mesh == 0 ? vertex_handle : driver->meshes[draw->mesh - 1].vertex_buffer;
			VkBuffer geometry_index = draw->mesh == 0 ? index_handle : driver->meshes[draw->mesh - 1].index_buffer;
			VkDeviceSize geometry_vertex_offset = draw->mesh == 0 ? vertex_offset : driver->meshes[draw->mesh - 1].vertex_offset;
			VkDeviceSize geometry_index_offset = draw->mesh == 0 ? index_offset : driver->meshes[draw->mesh - 1].index_offset;
			VkBuffer vertex_buffers[] = {geometry_vertex, instance_handle};
			VkDeviceSize offsets[] = {geometry_vertex_offset, instance_offset};
			vkCmdBindVertexBuffers(command_buffer, 0, 2, vertex_buffers, offsets);
			vkCmdBindIndexBuffer(command_buffer, geometry_index, geometry_index_offset, VK_INDEX_TYPE_UINT32);
			bound_mesh = draw->mesh;
		}
		VkPipeline next_pipeline = onix_vk_mesh_pipeline(driver, draw, instances, (int)((const OnixNativeMeshUniforms *)lighting_dst)->lighting[5]);
		if (next_pipeline == VK_NULL_HANDLE) return 0;
		if (pipeline != next_pipeline) {
			pipeline = next_pipeline;
			vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_GRAPHICS, pipeline);
		}
		OnixVkTexture *texture = &driver->textures[draw->texture - 1];
		OnixVkTexture *normal_texture = draw->normal_texture == 0 ? texture : &driver->textures[draw->normal_texture - 1];
		VkDescriptorSet descriptors[] = {texture->descriptor, normal_texture->descriptor, deformation_descriptor, surface_set};
		if (memcmp(bound_descriptors, descriptors, sizeof(descriptors)) != 0) {
			vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_GRAPHICS, driver->mesh_layout, 0, 4, descriptors, 0, NULL);
			memcpy(bound_descriptors, descriptors, sizeof(descriptors));
		}
		OnixGPUMeshPush draw_push = *push;
		if (draw->deformation_index != 0) draw_push.directional_light[3] = -(float)draw->deformation_index;
		vkCmdPushConstants(command_buffer, driver->mesh_layout, VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT, 0, sizeof(draw_push), &draw_push);
		vkCmdDrawIndexed(command_buffer, draw->index_count, draw->instance_count, draw->first_index, draw->base_vertex, draw->first_instance);
		frame->command_count++;
	}
	return 1;
}

int onix_gpu_draw_mesh_picking(
	OnixGPUDriver *driver,
	const OnixGPUMeshPush *push,
	const OnixGPUVertex3D *vertices,
	uint32_t vertex_count,
	const uint32_t *indices,
	uint32_t index_count,
	const OnixGPUInstance3D *instances,
	uint32_t instance_count,
	const OnixNativeMeshDraw3D *draws,
	uint32_t draw_count) {
	if (driver == NULL || !driver->frame_active || driver->active_render_target == 0 ||
		push == NULL || instances == NULL || instance_count == 0 || draws == NULL || draw_count == 0) {
		onix_vk_set_error("invalid Vulkan mesh picking draw");
		return 0;
	}
	OnixVkRenderTarget *target = &driver->render_targets[driver->active_render_target - 1];
	if (target->color_format != 1 || target->depth_image == VK_NULL_HANDLE || !driver->render_pass_active) {
		onix_vk_set_error("Vulkan mesh picking requires an active R32Uint target with depth");
		return 0;
	}
	int dynamic = vertices != NULL || vertex_count != 0 || indices != NULL || index_count != 0;
	if (dynamic && (vertices == NULL || vertex_count == 0 || indices == NULL || index_count == 0)) {
		onix_vk_set_error("incomplete Vulkan dynamic mesh picking data");
		return 0;
	}
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeMeshDraw3D *draw = &draws[index];
		if (draw->index_count == 0 || draw->instance_count == 0 ||
			(uint64_t)draw->first_instance + draw->instance_count > instance_count || draw->base_vertex < 0) {
			onix_vk_set_error("invalid Vulkan mesh picking draw range");
			return 0;
		}
		if (draw->mesh == 0) {
			if (!dynamic || (uint64_t)draw->first_index + draw->index_count > index_count ||
				(uint64_t)(uint32_t)draw->base_vertex >= vertex_count) {
				onix_vk_set_error("invalid Vulkan dynamic mesh picking draw range");
				return 0;
			}
		} else {
			if (draw->mesh > driver->mesh_slots.count) {
				onix_vk_set_error("invalid Vulkan static mesh picking handle");
				return 0;
			}
			OnixVkMesh *mesh = &driver->meshes[draw->mesh - 1];
			if (mesh->vertex_buffer == VK_NULL_HANDLE || mesh->destroy_pending ||
				(uint64_t)draw->first_index + draw->index_count > mesh->index_count ||
				(uint64_t)(uint32_t)draw->base_vertex >= mesh->vertex_count) {
				onix_vk_set_error("invalid Vulkan static mesh picking draw range");
				return 0;
			}
		}
	}
	if (!onix_vk_ensure_picking_pipeline(driver) || !onix_vk_prepare_draw(driver)) return 0;
	OnixNativeUpload vertex_upload = {0}, index_upload = {0}, instance_upload;
	if (dynamic && (!onix_vk_mesh_upload(driver, vertices, (VkDeviceSize)vertex_count * sizeof(*vertices), 16, 1, &vertex_upload) ||
		!onix_vk_mesh_upload(driver, indices, (VkDeviceSize)index_count * sizeof(*indices), 16, 1, &index_upload))) return 0;
	if (!onix_vk_mesh_upload(driver, instances, (VkDeviceSize)instance_count * sizeof(*instances), 16, 0, &instance_upload)) return 0;
	VkBuffer vertex_handle = (VkBuffer)vertex_upload.resource, index_handle = (VkBuffer)index_upload.resource;
	VkBuffer instance_handle = (VkBuffer)instance_upload.resource;
	VkDeviceSize vertex_offset = vertex_upload.offset, index_offset = index_upload.offset, instance_offset = instance_upload.offset;

	VkCommandBuffer command_buffer = driver->frames[driver->frame_index].command_buffer;
	onix_vk_apply_dynamic_state(driver);
	vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_GRAPHICS, driver->mesh_picking_pipeline);
	uint32_t bound_mesh = UINT32_MAX;
	for (uint32_t index = 0; index < draw_count; index++) {
		const OnixNativeMeshDraw3D *draw = &draws[index];
		if (draw->mesh != bound_mesh) {
			VkBuffer geometry_vertex = draw->mesh == 0 ? vertex_handle : driver->meshes[draw->mesh - 1].vertex_buffer;
			VkBuffer geometry_index = draw->mesh == 0 ? index_handle : driver->meshes[draw->mesh - 1].index_buffer;
			VkDeviceSize geometry_vertex_offset = draw->mesh == 0 ? vertex_offset : driver->meshes[draw->mesh - 1].vertex_offset;
			VkDeviceSize geometry_index_offset = draw->mesh == 0 ? index_offset : driver->meshes[draw->mesh - 1].index_offset;
			VkBuffer vertex_buffers[] = {geometry_vertex, instance_handle};
			VkDeviceSize offsets[] = {geometry_vertex_offset, instance_offset};
			vkCmdBindVertexBuffers(command_buffer, 0, 2, vertex_buffers, offsets);
			vkCmdBindIndexBuffer(command_buffer, geometry_index, geometry_index_offset, VK_INDEX_TYPE_UINT32);
			bound_mesh = draw->mesh;
		}
		vkCmdPushConstants(command_buffer, driver->mesh_layout, VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT, 0, sizeof(*push), push);
		vkCmdDrawIndexed(command_buffer, draw->index_count, draw->instance_count, draw->first_index, draw->base_vertex, draw->first_instance);
		driver->frames[driver->frame_index].command_count++;
	}
	return 1;
}
