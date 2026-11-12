#ifndef ONIX_ENGINE_RENDER_VULKAN_DISPLAY_H
#define ONIX_ENGINE_RENDER_VULKAN_DISPLAY_H

static VkDisplayModeKHR onix_vk_display_mode(const VkDisplayModePropertiesKHR *modes, uint32_t count, OnixVkSurface surface) {
    VkDisplayModeKHR selected = VK_NULL_HANDLE;
    uint32_t best_distance = UINT32_MAX;
    uint32_t best_rate = 0;
    for (uint32_t index = 0; index < count; index++) {
        const VkDisplayModeParametersKHR *mode = &modes[index].parameters;
        if (mode->visibleRegion.width != surface.width || mode->visibleRegion.height != surface.height) continue;
        uint32_t distance = mode->refreshRate > surface.drm_refresh_rate
            ? mode->refreshRate - surface.drm_refresh_rate : surface.drm_refresh_rate - mode->refreshRate;
        if (selected == VK_NULL_HANDLE ||
            (surface.drm_refresh_rate != 0 && distance < best_distance) ||
            (surface.drm_refresh_rate == 0 && mode->refreshRate > best_rate)) {
            selected = modes[index].displayMode;
            best_distance = distance;
            best_rate = mode->refreshRate;
        }
    }
    return selected;
}

static int onix_vk_display_extent_supported(VkExtent2D extent, const VkDisplayPlaneCapabilitiesKHR *caps) {
    return caps->minSrcPosition.x <= 0 && caps->maxSrcPosition.x >= 0 &&
        caps->minSrcPosition.y <= 0 && caps->maxSrcPosition.y >= 0 &&
        caps->minDstPosition.x <= 0 && caps->maxDstPosition.x >= 0 &&
        caps->minDstPosition.y <= 0 && caps->maxDstPosition.y >= 0 &&
        extent.width >= caps->minSrcExtent.width && extent.width <= caps->maxSrcExtent.width &&
        extent.height >= caps->minSrcExtent.height && extent.height <= caps->maxSrcExtent.height &&
        extent.width >= caps->minDstExtent.width && extent.width <= caps->maxDstExtent.width &&
        extent.height >= caps->minDstExtent.height && extent.height <= caps->maxDstExtent.height;
}

static int onix_vk_display_plane_supports(OnixVkDriver *driver, uint32_t plane) {
    uint32_t count = 0;
    if (vkGetDisplayPlaneSupportedDisplaysKHR(driver->physical_device, plane, &count, NULL) != VK_SUCCESS || count == 0) return 0;
    VkDisplayKHR *displays = calloc(count, sizeof(*displays));
    if (displays == NULL) return 0;
    int supported = 0;
    if (vkGetDisplayPlaneSupportedDisplaysKHR(driver->physical_device, plane, &count, displays) == VK_SUCCESS) {
        for (uint32_t index = 0; index < count; index++) {
            if (displays[index] == driver->drm_display) {
                supported = 1;
                break;
            }
        }
    }
    free(displays);
    return supported;
}

static int onix_vk_create_display_plane(OnixVkDriver *driver, VkDisplayModeKHR mode) {
    uint32_t count = 0;
    if (!onix_vk_result(vkGetPhysicalDeviceDisplayPlanePropertiesKHR(driver->physical_device, &count, NULL), "query Vulkan display planes")) return 0;
    if (count == 0) {
        onix_vk_set_error("Vulkan DRM display has no planes");
        return 0;
    }
    VkDisplayPlanePropertiesKHR *planes = calloc(count, sizeof(*planes));
    if (planes == NULL) {
        onix_vk_set_error("allocate Vulkan display planes");
        return 0;
    }
    if (!onix_vk_result(vkGetPhysicalDeviceDisplayPlanePropertiesKHR(driver->physical_device, &count, planes), "query Vulkan display planes")) {
        free(planes);
        return 0;
    }
    VkDisplaySurfaceCreateInfoKHR info = {
        .sType = VK_STRUCTURE_TYPE_DISPLAY_SURFACE_CREATE_INFO_KHR,
        .displayMode = mode,
        .transform = VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR,
        .globalAlpha = 1.0f,
        .imageExtent = {driver->native_surface.width, driver->native_surface.height},
    };
    int found = 0;
    // Prefer an already routed plane, then consider unassigned planes.
    for (int pass = 0; pass < 2 && !found; pass++) {
        for (uint32_t index = 0; index < count; index++) {
            VkDisplayKHR expected = pass == 0 ? driver->drm_display : VK_NULL_HANDLE;
            if (planes[index].currentDisplay != expected || !onix_vk_display_plane_supports(driver, index)) continue;
            VkDisplayPlaneCapabilitiesKHR caps;
            if (vkGetDisplayPlaneCapabilitiesKHR(driver->physical_device, mode, index, &caps) != VK_SUCCESS ||
                !onix_vk_display_extent_supported(info.imageExtent, &caps)) continue;
            if ((caps.supportedAlpha & VK_DISPLAY_PLANE_ALPHA_OPAQUE_BIT_KHR) != 0) {
                info.alphaMode = VK_DISPLAY_PLANE_ALPHA_OPAQUE_BIT_KHR;
            } else if ((caps.supportedAlpha & VK_DISPLAY_PLANE_ALPHA_GLOBAL_BIT_KHR) != 0) {
                info.alphaMode = VK_DISPLAY_PLANE_ALPHA_GLOBAL_BIT_KHR;
            } else {
                continue;
            }
            info.planeIndex = index;
            info.planeStackIndex = planes[index].currentStackIndex;
            found = 1;
            break;
        }
    }
    free(planes);
    if (!found) {
        onix_vk_set_error("Vulkan DRM display has no compatible opaque plane for %ux%u", info.imageExtent.width, info.imageExtent.height);
        return 0;
    }
    return onix_vk_result(vkCreateDisplayPlaneSurfaceKHR(driver->instance, &info, NULL, &driver->surface), "create Vulkan DRM display surface");
}

static int onix_vk_display_supports_identity(OnixVkDriver *driver) {
    uint32_t count = 0;
    if (!onix_vk_result(vkGetPhysicalDeviceDisplayPropertiesKHR(driver->physical_device, &count, NULL), "query Vulkan displays")) return 0;
    if (count == 0) {
        onix_vk_set_error("Vulkan DRM display is unavailable");
        return 0;
    }
    VkDisplayPropertiesKHR *displays = calloc(count, sizeof(*displays));
    if (displays == NULL) {
        onix_vk_set_error("allocate Vulkan displays");
        return 0;
    }
    if (!onix_vk_result(vkGetPhysicalDeviceDisplayPropertiesKHR(driver->physical_device, &count, displays), "query Vulkan displays")) {
        free(displays);
        return 0;
    }
    int supported = 0;
    for (uint32_t index = 0; index < count; index++) {
        if (displays[index].display == driver->drm_display) {
            supported = (displays[index].supportedTransforms & VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR) != 0;
            break;
        }
    }
    free(displays);
    if (!supported) onix_vk_set_error("Vulkan DRM display does not support unrotated scanout");
    return supported;
}

static int onix_vk_create_drm_surface(OnixVkDriver *driver) {
    const OnixVkSurface *surface = &driver->native_surface;
    if (surface->kind != ONIX_GPU_SURFACE_DRM || surface->drm_fd < 0 || surface->drm_connector_id == 0 || surface->transparent) {
        onix_vk_set_error("DRM Vulkan driver requires an opaque DRM connector surface");
        return 0;
    }
    PFN_vkGetDrmDisplayEXT get_display = (PFN_vkGetDrmDisplayEXT)vkGetInstanceProcAddr(driver->instance, "vkGetDrmDisplayEXT");
    PFN_vkAcquireDrmDisplayEXT acquire_display = (PFN_vkAcquireDrmDisplayEXT)vkGetInstanceProcAddr(driver->instance, "vkAcquireDrmDisplayEXT");
    driver->release_display = (PFN_vkReleaseDisplayEXT)vkGetInstanceProcAddr(driver->instance, "vkReleaseDisplayEXT");
    if (get_display == NULL || acquire_display == NULL || driver->release_display == NULL) {
        onix_vk_set_error("Vulkan DRM display acquisition extensions are unavailable");
        return 0;
    }
    uint32_t count = 0;
    if (!onix_vk_result(vkEnumeratePhysicalDevices(driver->instance, &count, NULL), "query Vulkan DRM devices")) return 0;
    if (count == 0) {
        onix_vk_set_error("no Vulkan physical device is available");
        return 0;
    }
    VkPhysicalDevice *devices = calloc(count, sizeof(*devices));
    if (devices == NULL) {
        onix_vk_set_error("allocate Vulkan DRM devices");
        return 0;
    }
    if (!onix_vk_result(vkEnumeratePhysicalDevices(driver->instance, &count, devices), "query Vulkan DRM devices")) {
        free(devices);
        return 0;
    }
    for (uint32_t index = 0; index < count; index++) {
        VkDisplayKHR display = VK_NULL_HANDLE;
        if (get_display(devices[index], surface->drm_fd, surface->drm_connector_id, &display) == VK_SUCCESS && display != VK_NULL_HANDLE) {
            driver->physical_device = devices[index];
            driver->drm_display = display;
            break;
        }
    }
    free(devices);
    if (driver->drm_display == VK_NULL_HANDLE) {
        onix_vk_set_error("no Vulkan device exposes DRM connector %u on fd %d", surface->drm_connector_id, surface->drm_fd);
        return 0;
    }
    if (!onix_vk_result(acquire_display(driver->physical_device, surface->drm_fd, driver->drm_display), "acquire Vulkan DRM display (DRM master required)")) return 0;
    driver->display_acquired = 1;
    if (!onix_vk_display_supports_identity(driver)) return 0;
    vkGetPhysicalDeviceProperties(driver->physical_device, &driver->physical_device_properties);
    if (surface->width > driver->physical_device_properties.limits.maxImageDimension2D ||
        surface->height > driver->physical_device_properties.limits.maxImageDimension2D) {
        onix_vk_set_error("DRM mode exceeds Vulkan image dimensions");
        return 0;
    }
    count = 0;
    if (!onix_vk_result(vkGetDisplayModePropertiesKHR(driver->physical_device, driver->drm_display, &count, NULL), "query Vulkan DRM modes")) return 0;
    if (count == 0) {
        onix_vk_set_error("Vulkan DRM display has no modes");
        return 0;
    }
    VkDisplayModePropertiesKHR *modes = calloc(count, sizeof(*modes));
    if (modes == NULL) {
        onix_vk_set_error("allocate Vulkan DRM modes");
        return 0;
    }
    if (!onix_vk_result(vkGetDisplayModePropertiesKHR(driver->physical_device, driver->drm_display, &count, modes), "query Vulkan DRM modes")) {
        free(modes);
        return 0;
    }
    VkDisplayModeKHR mode = onix_vk_display_mode(modes, count, *surface);
    free(modes);
    if (mode == VK_NULL_HANDLE) {
        onix_vk_set_error("Vulkan DRM display has no %ux%u mode", surface->width, surface->height);
        return 0;
    }
    return onix_vk_create_display_plane(driver, mode);
}

#endif
