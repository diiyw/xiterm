#define ONIX_VK_DRM
#include <assert.h>
#include <string.h>
#include "../vulkan/vulkan.h"

#define HANDLE(type, value) ((type)(uintptr_t)(value))
#define GPU HANDLE(VkPhysicalDevice, 2)
#define DISPLAY HANDLE(VkDisplayKHR, 3)

static int missing_extension, wrong_device, deny_acquire, no_modes, no_present, fail_surface;
static int reject_routed_plane, reject_all_planes;
static int acquired, released, destroyed_surface, destroyed_instance;
static VkDisplaySurfaceCreateInfoKHR created_surface;

static VkResult test_create_instance(const VkInstanceCreateInfo *info, const VkAllocationCallbacks *allocator, VkInstance *instance) {
    (void)allocator;
    assert(info->enabledExtensionCount == 4);
    assert(strcmp(info->ppEnabledExtensionNames[1], VK_KHR_DISPLAY_EXTENSION_NAME) == 0);
    assert(strcmp(info->ppEnabledExtensionNames[2], VK_EXT_DIRECT_MODE_DISPLAY_EXTENSION_NAME) == 0);
    assert(strcmp(info->ppEnabledExtensionNames[3], VK_EXT_ACQUIRE_DRM_DISPLAY_EXTENSION_NAME) == 0);
    *instance = HANDLE(VkInstance, 1);
    return VK_SUCCESS;
}

static VkResult test_devices(VkInstance instance, uint32_t *count, VkPhysicalDevice *devices) {
    (void)instance;
    if (devices != NULL) {
        devices[0] = HANDLE(VkPhysicalDevice, 9);
        devices[1] = GPU;
    }
    *count = 2;
    return VK_SUCCESS;
}

static VkResult test_get_display(VkPhysicalDevice device, int32_t fd, uint32_t connector, VkDisplayKHR *display) {
    assert(fd == 0 && connector == 43);
    if (wrong_device || device != GPU) return VK_ERROR_UNKNOWN;
    *display = DISPLAY;
    return VK_SUCCESS;
}

static VkResult test_acquire(VkPhysicalDevice device, int32_t fd, VkDisplayKHR display) {
    assert(device == GPU && fd == 0 && display == DISPLAY);
    if (deny_acquire) return VK_ERROR_INITIALIZATION_FAILED;
    acquired++;
    return VK_SUCCESS;
}

static VkResult test_release(VkPhysicalDevice device, VkDisplayKHR display) {
    assert(device == GPU && display == DISPLAY);
    assert(destroyed_instance == 0);
    if (created_surface.sType != 0 && !fail_surface) assert(destroyed_surface == 1);
    released++;
    return VK_SUCCESS;
}

static PFN_vkVoidFunction test_proc(VkInstance instance, const char *name) {
    (void)instance;
    if (missing_extension) return NULL;
    if (strcmp(name, "vkGetDrmDisplayEXT") == 0) return (PFN_vkVoidFunction)test_get_display;
    if (strcmp(name, "vkAcquireDrmDisplayEXT") == 0) return (PFN_vkVoidFunction)test_acquire;
    if (strcmp(name, "vkReleaseDisplayEXT") == 0) return (PFN_vkVoidFunction)test_release;
    assert(0);
    return NULL;
}

static VkResult test_displays(VkPhysicalDevice device, uint32_t *count, VkDisplayPropertiesKHR *displays) {
    assert(device == GPU && acquired == 1);
    if (displays != NULL) displays[0] = (VkDisplayPropertiesKHR){.display = DISPLAY, .supportedTransforms = VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR};
    *count = 1;
    return VK_SUCCESS;
}

static void test_properties(VkPhysicalDevice device, VkPhysicalDeviceProperties *properties) {
    memset(properties, 0, sizeof(*properties));
    properties->deviceType = device == GPU ? VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU : VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU;
    properties->limits.maxImageDimension2D = 8192;
}

static const VkDisplayModePropertiesKHR modes[] = {
    {.displayMode = HANDLE(VkDisplayModeKHR, 10), .parameters = {{1280, 720}, 60000}},
    {.displayMode = HANDLE(VkDisplayModeKHR, 11), .parameters = {{1920, 1080}, 60000}},
    {.displayMode = HANDLE(VkDisplayModeKHR, 12), .parameters = {{1920, 1080}, 59940}},
    {.displayMode = HANDLE(VkDisplayModeKHR, 13), .parameters = {{1920, 1080}, 144000}},
};

static VkResult test_modes(VkPhysicalDevice device, VkDisplayKHR display, uint32_t *count, VkDisplayModePropertiesKHR *properties) {
    assert(device == GPU && display == DISPLAY);
    *count = no_modes ? 0 : 4;
    if (properties != NULL) memcpy(properties, modes, *count * sizeof(*properties));
    return VK_SUCCESS;
}

static VkResult test_planes(VkPhysicalDevice device, uint32_t *count, VkDisplayPlanePropertiesKHR *planes) {
    assert(device == GPU);
    *count = 3;
    if (planes != NULL) {
        planes[0] = (VkDisplayPlanePropertiesKHR){HANDLE(VkDisplayKHR, 99), 0};
        planes[1] = (VkDisplayPlanePropertiesKHR){VK_NULL_HANDLE, 1};
        planes[2] = (VkDisplayPlanePropertiesKHR){DISPLAY, 2};
    }
    return VK_SUCCESS;
}

static VkResult test_supported_displays(VkPhysicalDevice device, uint32_t plane, uint32_t *count, VkDisplayKHR *displays) {
    assert(device == GPU && plane != 0);
    *count = 1;
    if (displays != NULL) displays[0] = DISPLAY;
    return VK_SUCCESS;
}

static VkResult test_caps(VkPhysicalDevice device, VkDisplayModeKHR mode, uint32_t plane, VkDisplayPlaneCapabilitiesKHR *caps) {
    assert(device == GPU && mode == modes[2].displayMode);
    *caps = (VkDisplayPlaneCapabilitiesKHR){
        .supportedAlpha = plane == 1 ? VK_DISPLAY_PLANE_ALPHA_GLOBAL_BIT_KHR : VK_DISPLAY_PLANE_ALPHA_OPAQUE_BIT_KHR,
        .maxSrcExtent = {3840, 2160}, .maxDstExtent = {3840, 2160},
    };
    if (reject_all_planes || (reject_routed_plane && plane == 2)) caps->maxDstExtent.width = 1280;
    return VK_SUCCESS;
}

static VkResult test_create_surface(VkInstance instance, const VkDisplaySurfaceCreateInfoKHR *info, const VkAllocationCallbacks *allocator, VkSurfaceKHR *surface) {
    (void)instance;
    (void)allocator;
    created_surface = *info;
    assert(info->planeIndex == info->planeStackIndex);
    assert(info->transform == VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR);
    assert(info->imageExtent.width == 1920 && info->imageExtent.height == 1080);
    if (fail_surface) return VK_ERROR_INITIALIZATION_FAILED;
    *surface = HANDLE(VkSurfaceKHR, 4);
    return VK_SUCCESS;
}

static void test_queues(VkPhysicalDevice device, uint32_t *count, VkQueueFamilyProperties *queues) {
    assert(device == GPU);
    *count = 1;
    if (queues != NULL) queues[0] = (VkQueueFamilyProperties){.queueFlags = VK_QUEUE_GRAPHICS_BIT, .queueCount = 1};
}

static VkResult test_present_support(VkPhysicalDevice device, uint32_t queue, VkSurfaceKHR surface, VkBool32 *present) {
    assert(device == GPU && queue == 0 && surface == HANDLE(VkSurfaceKHR, 4));
    *present = !no_present;
    return VK_SUCCESS;
}

static void test_destroy_surface(VkInstance instance, VkSurfaceKHR surface, const VkAllocationCallbacks *allocator) {
    (void)instance;
    (void)surface;
    (void)allocator;
    assert(released == 0);
    destroyed_surface++;
}

static void test_destroy_instance(VkInstance instance, const VkAllocationCallbacks *allocator) {
    (void)instance;
    (void)allocator;
    assert(released == acquired);
    destroyed_instance++;
}

#define vkCreateInstance test_create_instance
#define vkEnumeratePhysicalDevices test_devices
#define vkGetInstanceProcAddr test_proc
#define vkGetPhysicalDeviceDisplayPropertiesKHR test_displays
#define vkGetPhysicalDeviceProperties test_properties
#define vkGetDisplayModePropertiesKHR test_modes
#define vkGetPhysicalDeviceDisplayPlanePropertiesKHR test_planes
#define vkGetDisplayPlaneSupportedDisplaysKHR test_supported_displays
#define vkGetDisplayPlaneCapabilitiesKHR test_caps
#define vkCreateDisplayPlaneSurfaceKHR test_create_surface
#define vkGetPhysicalDeviceQueueFamilyProperties test_queues
#define vkGetPhysicalDeviceSurfaceSupportKHR test_present_support
#define vkDestroySurfaceKHR test_destroy_surface
#define vkDestroyInstance test_destroy_instance
#include "../vulkan_native.c"

static OnixGPUDriver *new_driver(void) {
    missing_extension = wrong_device = deny_acquire = no_modes = no_present = fail_surface = 0;
    reject_routed_plane = reject_all_planes = 0;
    acquired = released = destroyed_surface = destroyed_instance = 0;
    memset(&created_surface, 0, sizeof(created_surface));
    OnixGPUDriver *driver = calloc(1, sizeof(*driver));
    assert(driver != NULL);
    driver->native_surface = (OnixGPUSurface){.kind = ONIX_GPU_SURFACE_DRM, .drm_fd = 0, .drm_connector_id = 43,
        .width = 1920, .height = 1080, .drm_refresh_rate = 59941};
    assert(onix_vk_create_instance(driver));
    return driver;
}

int main(void) {
    OnixGPUDriver *driver = new_driver();
    assert(onix_vk_create_surface(driver));
    assert(created_surface.planeIndex == 2);
    assert(onix_vk_select_device(driver));
    assert(driver->physical_device == GPU);
    onix_gpu_destroy(driver);
    assert(released == 1 && destroyed_surface == 1 && destroyed_instance == 1);

    driver = new_driver();
    OnixGPUSurface surface = driver->native_surface;
    surface.drm_refresh_rate = 0;
    assert(onix_vk_display_mode(modes, 4, surface) == modes[3].displayMode);
    surface.width = 800;
    assert(onix_vk_display_mode(modes, 4, surface) == VK_NULL_HANDLE);
    reject_routed_plane = 1;
    assert(onix_vk_create_surface(driver));
    assert(created_surface.planeIndex == 1 && created_surface.alphaMode == VK_DISPLAY_PLANE_ALPHA_GLOBAL_BIT_KHR);
    onix_gpu_destroy(driver);

    for (int failure = 0; failure < 6; failure++) {
        driver = new_driver();
        switch (failure) {
            case 0: missing_extension = 1; break;
            case 1: wrong_device = 1; break;
            case 2: deny_acquire = 1; break;
            case 3: no_modes = 1; break;
            case 4: reject_all_planes = 1; break;
            case 5: fail_surface = 1; break;
        }
        assert(!onix_vk_create_surface(driver));
        assert(onix_gpu_last_error()[0] != '\0');
        onix_gpu_destroy(driver);
        assert(released == (failure >= 3));
        assert(destroyed_surface == 0 && destroyed_instance == 1);
    }
    driver = new_driver();
    assert(onix_vk_create_surface(driver));
    no_present = 1;
    assert(!onix_vk_select_device(driver));
    onix_gpu_destroy(driver);
    assert(released == 1);
    return 0;
}
