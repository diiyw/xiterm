//go:build linux && cgo && onix_drm

#include "drm_native.h"

#include <dlfcn.h>
#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#define ONIX_DRM_MODE_CONNECTED 1
#define ONIX_DRM_MODE_TYPE_PREFERRED (1u << 3)
#define ONIX_DRM_MODE_PAGE_FLIP_EVENT 0x01
#define ONIX_DRM_EVENT_CONTEXT_VERSION 2
#define ONIX_GBM_BO_USE_SCANOUT (1u << 0)
#define ONIX_GBM_BO_USE_RENDERING (1u << 2)
#define ONIX_GBM_FORMAT_XRGB8888 0x34325258u

typedef struct gbm_device gbm_device;
typedef struct gbm_surface gbm_surface;
typedef struct gbm_bo gbm_bo;

typedef union {
	void *ptr;
	int32_t s32;
	uint32_t u32;
	uint64_t u64;
} OnixGBMHandle;

typedef struct {
	uint32_t clock;
	uint16_t hdisplay;
	uint16_t hsync_start;
	uint16_t hsync_end;
	uint16_t htotal;
	uint16_t hskew;
	uint16_t vdisplay;
	uint16_t vsync_start;
	uint16_t vsync_end;
	uint16_t vtotal;
	uint16_t vscan;
	uint32_t vrefresh;
	uint32_t flags;
	uint32_t type;
	char name[32];
} OnixDRMModeInfo;

typedef struct {
	int count_fbs;
	uint32_t *fbs;
	int count_crtcs;
	uint32_t *crtcs;
	int count_connectors;
	uint32_t *connectors;
	int count_encoders;
	uint32_t *encoders;
	uint32_t min_width;
	uint32_t max_width;
	uint32_t min_height;
	uint32_t max_height;
} OnixDRMResources;

typedef struct {
	uint32_t connector_id;
	uint32_t encoder_id;
	uint32_t connector_type;
	uint32_t connector_type_id;
	int connection;
	uint32_t mm_width;
	uint32_t mm_height;
	int subpixel;
	int count_modes;
	OnixDRMModeInfo *modes;
	int count_props;
	uint32_t *props;
	uint64_t *prop_values;
	int count_encoders;
	uint32_t *encoders;
} OnixDRMConnector;

typedef struct {
	uint32_t encoder_id;
	uint32_t encoder_type;
	uint32_t crtc_id;
	uint32_t possible_crtcs;
	uint32_t possible_clones;
} OnixDRMEncoder;

typedef struct {
	uint32_t crtc_id;
	uint32_t buffer_id;
	uint32_t x;
	uint32_t y;
	uint32_t width;
	uint32_t height;
	int mode_valid;
	OnixDRMModeInfo mode;
	int gamma_size;
} OnixDRMCrtc;

typedef void (*OnixDRMVBlankHandler)(int, unsigned int, unsigned int, unsigned int, void *);
typedef struct {
	int version;
	OnixDRMVBlankHandler vblank_handler;
	OnixDRMVBlankHandler page_flip_handler;
} OnixDRMEventContext;

typedef OnixDRMResources *(*OnixDRMGetResources)(int);
typedef void (*OnixDRMFreeResources)(OnixDRMResources *);
typedef OnixDRMConnector *(*OnixDRMGetConnector)(int, uint32_t);
typedef void (*OnixDRMFreeConnector)(OnixDRMConnector *);
typedef OnixDRMEncoder *(*OnixDRMGetEncoder)(int, uint32_t);
typedef void (*OnixDRMFreeEncoder)(OnixDRMEncoder *);
typedef OnixDRMCrtc *(*OnixDRMGetCrtc)(int, uint32_t);
typedef void (*OnixDRMFreeCrtc)(OnixDRMCrtc *);
typedef int (*OnixDRMSetCrtc)(int, uint32_t, uint32_t, uint32_t, uint32_t, uint32_t *, int, OnixDRMModeInfo *);
typedef int (*OnixDRMAddFB2)(int, uint32_t, uint32_t, uint32_t, const uint32_t[4], const uint32_t[4], const uint32_t[4], uint32_t *, uint32_t);
typedef int (*OnixDRMRmFB)(int, uint32_t);
typedef int (*OnixDRMPageFlip)(int, uint32_t, uint32_t, uint32_t, void *);
typedef int (*OnixDRMHandleEvent)(int, OnixDRMEventContext *);

typedef gbm_device *(*OnixGBMCreateDevice)(int);
typedef void (*OnixGBMDestroyDevice)(gbm_device *);
typedef gbm_surface *(*OnixGBMSurfaceCreate)(gbm_device *, uint32_t, uint32_t, uint32_t, uint32_t);
typedef void (*OnixGBMSurfaceDestroy)(gbm_surface *);
typedef gbm_bo *(*OnixGBMSurfaceLockFrontBuffer)(gbm_surface *);
typedef void (*OnixGBMSurfaceReleaseBuffer)(gbm_surface *, gbm_bo *);
typedef OnixGBMHandle (*OnixGBMBOGetHandle)(gbm_bo *);
typedef uint32_t (*OnixGBMBOGetStride)(gbm_bo *);

typedef struct {
	gbm_bo *buffer;
	uint32_t framebuffer;
} OnixDRMFramebuffer;

typedef struct {
	OnixDRMCrtc *crtc;
	uint32_t *connectors;
	int connector_count;
} OnixDRMOutputState;

struct OnixDRMDisplay {
	int fd;
	void *drm_library;
	void *gbm_library;
	OnixDRMGetResources get_resources;
	OnixDRMFreeResources free_resources;
	OnixDRMGetConnector get_connector;
	OnixDRMFreeConnector free_connector;
	OnixDRMGetEncoder get_encoder;
	OnixDRMFreeEncoder free_encoder;
	OnixDRMGetCrtc get_crtc;
	OnixDRMFreeCrtc free_crtc;
	OnixDRMSetCrtc set_crtc;
	OnixDRMAddFB2 add_fb2;
	OnixDRMRmFB rm_fb;
	OnixDRMPageFlip page_flip;
	OnixDRMHandleEvent handle_event;
	OnixGBMCreateDevice create_device;
	OnixGBMDestroyDevice destroy_device;
	OnixGBMSurfaceCreate create_surface;
	OnixGBMSurfaceDestroy destroy_surface;
	OnixGBMSurfaceLockFrontBuffer lock_front_buffer;
	OnixGBMSurfaceReleaseBuffer release_buffer;
	OnixGBMBOGetHandle get_handle;
	OnixGBMBOGetStride get_stride;
	gbm_device *gbm;
	gbm_surface *surface;
	OnixDRMCrtc *saved_crtc;
	OnixDRMModeInfo mode;
	uint32_t connector_id;
	uint32_t crtc_id;
	gbm_bo *current_bo;
	uint32_t current_fb;
	gbm_bo *pending_bo;
	uint32_t pending_fb;
	OnixDRMFramebuffer framebuffers[8];
	int modeset;
	int flip_pending;
	OnixDRMOutputState *outputs;
	int output_count;
	int restore_outputs;
};

static __thread char onix_drm_error[512];

static void onix_drm_set_error(const char *format, ...) {
	va_list args;
	va_start(args, format);
	vsnprintf(onix_drm_error, sizeof(onix_drm_error), format, args);
	va_end(args);
}

const char *onix_drm_last_error(void) {
	return onix_drm_error;
}

static int onix_drm_load_symbol(void *library, const char *name, void **target) {
	*target = dlsym(library, name);
	if (*target == NULL) {
		onix_drm_set_error("load %s: %s", name, dlerror());
		return 0;
	}
	return 1;
}

#define ONIX_LOAD(display, library, field, name) \
	onix_drm_load_symbol((display)->library, name, (void **)&(display)->field)

static int onix_drm_load_libraries(OnixDRMDisplay *display) {
	display->drm_library = dlopen("libdrm.so.2", RTLD_NOW | RTLD_LOCAL);
	if (display->drm_library == NULL) {
		onix_drm_set_error("load libdrm.so.2: %s", dlerror());
		return 0;
	}
	return ONIX_LOAD(display, drm_library, get_resources, "drmModeGetResources") &&
		ONIX_LOAD(display, drm_library, free_resources, "drmModeFreeResources") &&
		ONIX_LOAD(display, drm_library, get_connector, "drmModeGetConnector") &&
		ONIX_LOAD(display, drm_library, free_connector, "drmModeFreeConnector") &&
		ONIX_LOAD(display, drm_library, get_encoder, "drmModeGetEncoder") &&
		ONIX_LOAD(display, drm_library, free_encoder, "drmModeFreeEncoder") &&
		ONIX_LOAD(display, drm_library, get_crtc, "drmModeGetCrtc") &&
		ONIX_LOAD(display, drm_library, free_crtc, "drmModeFreeCrtc") &&
		ONIX_LOAD(display, drm_library, set_crtc, "drmModeSetCrtc") &&
		ONIX_LOAD(display, drm_library, add_fb2, "drmModeAddFB2") &&
		ONIX_LOAD(display, drm_library, rm_fb, "drmModeRmFB") &&
		ONIX_LOAD(display, drm_library, page_flip, "drmModePageFlip") &&
		ONIX_LOAD(display, drm_library, handle_event, "drmHandleEvent");
}

static int onix_drm_load_gbm(OnixDRMDisplay *display) {
	display->gbm_library = dlopen("libgbm.so.1", RTLD_NOW | RTLD_LOCAL);
	if (display->gbm_library == NULL) {
		onix_drm_set_error("load libgbm.so.1: %s", dlerror());
		return 0;
	}
	return ONIX_LOAD(display, gbm_library, create_device, "gbm_create_device") &&
		ONIX_LOAD(display, gbm_library, destroy_device, "gbm_device_destroy") &&
		ONIX_LOAD(display, gbm_library, create_surface, "gbm_surface_create") &&
		ONIX_LOAD(display, gbm_library, destroy_surface, "gbm_surface_destroy") &&
		ONIX_LOAD(display, gbm_library, lock_front_buffer, "gbm_surface_lock_front_buffer") &&
		ONIX_LOAD(display, gbm_library, release_buffer, "gbm_surface_release_buffer") &&
		ONIX_LOAD(display, gbm_library, get_handle, "gbm_bo_get_handle") &&
		ONIX_LOAD(display, gbm_library, get_stride, "gbm_bo_get_stride");
}

static int onix_drm_mode_index(const OnixDRMConnector *connector) {
	for (int index = 0; index < connector->count_modes; index++) {
		if ((connector->modes[index].type & ONIX_DRM_MODE_TYPE_PREFERRED) != 0) {
			return index;
		}
	}
	return 0;
}

static uint32_t onix_drm_find_crtc(OnixDRMDisplay *display, OnixDRMResources *resources, OnixDRMConnector *connector) {
	if (connector->encoder_id != 0) {
		OnixDRMEncoder *encoder = display->get_encoder(display->fd, connector->encoder_id);
		if (encoder != NULL) {
			uint32_t crtc = encoder->crtc_id;
			display->free_encoder(encoder);
			if (crtc != 0) {
				return crtc;
			}
		}
	}
	for (int encoder_index = 0; encoder_index < connector->count_encoders; encoder_index++) {
		OnixDRMEncoder *encoder = display->get_encoder(display->fd, connector->encoders[encoder_index]);
		if (encoder == NULL) {
			continue;
		}
		for (int crtc_index = 0; crtc_index < resources->count_crtcs && crtc_index < 32; crtc_index++) {
			if ((encoder->possible_crtcs & (1u << crtc_index)) != 0) {
				uint32_t crtc = resources->crtcs[crtc_index];
				display->free_encoder(encoder);
				return crtc;
			}
		}
		display->free_encoder(encoder);
	}
	return 0;
}

static int onix_drm_select_output(OnixDRMDisplay *display, uint32_t requested_connector) {
	OnixDRMResources *resources = display->get_resources(display->fd);
	if (resources == NULL) {
		onix_drm_set_error("query DRM resources: %s", strerror(errno));
		return 0;
	}
	int selected = 0;
	for (int index = 0; index < resources->count_connectors && !selected; index++) {
		uint32_t connector_id = resources->connectors[index];
		if (requested_connector != 0 && connector_id != requested_connector) {
			continue;
		}
		OnixDRMConnector *connector = display->get_connector(display->fd, connector_id);
		if (connector == NULL) {
			continue;
		}
		if (connector->connection == ONIX_DRM_MODE_CONNECTED && connector->count_modes > 0) {
			uint32_t crtc_id = onix_drm_find_crtc(display, resources, connector);
			if (crtc_id != 0) {
				display->connector_id = connector_id;
				display->crtc_id = crtc_id;
				display->mode = connector->modes[onix_drm_mode_index(connector)];
				selected = 1;
			}
		}
		display->free_connector(connector);
	}
	display->free_resources(resources);
	if (!selected) {
		if (requested_connector != 0) {
			onix_drm_set_error("DRM connector %u is not connected or has no usable CRTC", requested_connector);
		} else {
			onix_drm_set_error("no connected DRM connector with a usable CRTC");
		}
	}
	return selected;
}

OnixDRMDisplay *onix_drm_create(const char *device, uint32_t connector_id) {
	onix_drm_error[0] = '\0';
	if (device == NULL || device[0] == '\0') {
		onix_drm_set_error("DRM device path is empty");
		return NULL;
	}
	OnixDRMDisplay *display = calloc(1, sizeof(OnixDRMDisplay));
	if (display == NULL) {
		onix_drm_set_error("allocate DRM display");
		return NULL;
	}
	display->fd = -1;
	if (!onix_drm_load_libraries(display)) {
		onix_drm_destroy(display);
		return NULL;
	}
	display->fd = open(device, O_RDWR | O_CLOEXEC);
	if (display->fd < 0) {
		onix_drm_set_error("open %s: %s", device, strerror(errno));
		onix_drm_destroy(display);
		return NULL;
	}
	if (!onix_drm_select_output(display, connector_id)) {
		onix_drm_destroy(display);
		return NULL;
	}
	display->saved_crtc = display->get_crtc(display->fd, display->crtc_id);
	if (display->saved_crtc == NULL) {
		onix_drm_set_error("query DRM CRTC %u: %s", display->crtc_id, strerror(errno));
		onix_drm_destroy(display);
		return NULL;
	}
	return display;
}

int onix_drm_create_gbm(OnixDRMDisplay *display) {
	if (display == NULL || !onix_drm_load_gbm(display)) return 0;
	display->gbm = display->create_device(display->fd);
	if (display->gbm == NULL) {
		onix_drm_set_error("create GBM device");
		return 0;
	}
	display->surface = display->create_surface(
		display->gbm,
		display->mode.hdisplay,
		display->mode.vdisplay,
		ONIX_GBM_FORMAT_XRGB8888,
		ONIX_GBM_BO_USE_SCANOUT | ONIX_GBM_BO_USE_RENDERING);
	if (display->surface == NULL) {
		onix_drm_set_error("create %ux%u GBM scanout surface", display->mode.hdisplay, display->mode.vdisplay);
		return 0;
	}
	return 1;
}

int onix_drm_fd(OnixDRMDisplay *display) {
	return display != NULL ? display->fd : -1;
}

uint32_t onix_drm_connector_id(OnixDRMDisplay *display) {
	return display != NULL ? display->connector_id : 0;
}

uint32_t onix_drm_refresh_rate(OnixDRMDisplay *display) {
	if (display == NULL) return 0;
	const OnixDRMModeInfo *mode = &display->mode;
	if (mode->clock == 0 || mode->htotal == 0 || mode->vtotal == 0) return mode->vrefresh * 1000;
	uint64_t numerator = (uint64_t)mode->clock * 1000000;
	uint64_t denominator = (uint64_t)mode->htotal * mode->vtotal;
	if (mode->flags & (1u << 4)) numerator *= 2; // DRM_MODE_FLAG_INTERLACE
	if (mode->flags & (1u << 5)) denominator *= 2; // DRM_MODE_FLAG_DBLSCAN
	if (mode->vscan > 1) denominator *= mode->vscan;
	return (uint32_t)((numerator + denominator / 2) / denominator);
}

int onix_drm_save_outputs(OnixDRMDisplay *display) {
	if (display == NULL || display->outputs != NULL) return 0;
	OnixDRMResources *resources = display->get_resources(display->fd);
	if (resources == NULL) {
		onix_drm_set_error("query DRM outputs: %s", strerror(errno));
		return 0;
	}
	int success = 0;
	display->outputs = calloc(resources->count_crtcs, sizeof(*display->outputs));
	if (display->outputs == NULL) goto done;
	display->output_count = resources->count_crtcs;
	for (int index = 0; index < display->output_count; index++) {
		OnixDRMOutputState *output = &display->outputs[index];
		output->crtc = display->get_crtc(display->fd, resources->crtcs[index]);
		output->connectors = calloc(resources->count_connectors, sizeof(*output->connectors));
		if (output->crtc == NULL || output->connectors == NULL) goto done;
	}
	for (int index = 0; index < resources->count_connectors; index++) {
		OnixDRMConnector *connector = display->get_connector(display->fd, resources->connectors[index]);
		if (connector == NULL) goto done;
		uint32_t encoder_id = connector->encoder_id;
		display->free_connector(connector);
		if (encoder_id == 0) continue;
		OnixDRMEncoder *encoder = display->get_encoder(display->fd, encoder_id);
		if (encoder == NULL) goto done;
		uint32_t crtc_id = encoder->crtc_id;
		display->free_encoder(encoder);
		for (int output_index = 0; output_index < display->output_count; output_index++) {
			OnixDRMOutputState *output = &display->outputs[output_index];
			if (output->crtc->crtc_id == crtc_id) {
				output->connectors[output->connector_count++] = resources->connectors[index];
				break;
			}
		}
	}
	success = 1;
done:
	display->free_resources(resources);
	if (!success) onix_drm_set_error("save DRM output configuration: %s", strerror(errno));
	return success;
}

void onix_drm_restore_on_destroy(OnixDRMDisplay *display) {
	if (display != NULL) display->restore_outputs = 1;
}

void *onix_drm_gbm_device(OnixDRMDisplay *display) {
	return display != NULL ? display->gbm : NULL;
}

void *onix_drm_gbm_surface(OnixDRMDisplay *display) {
	return display != NULL ? display->surface : NULL;
}

int onix_drm_width(OnixDRMDisplay *display) {
	return display != NULL ? display->mode.hdisplay : 0;
}

int onix_drm_height(OnixDRMDisplay *display) {
	return display != NULL ? display->mode.vdisplay : 0;
}

static int onix_drm_framebuffer(OnixDRMDisplay *display, gbm_bo *buffer, uint32_t *framebuffer) {
	for (int index = 0; index < 8; index++) {
		if (display->framebuffers[index].buffer == buffer) {
			*framebuffer = display->framebuffers[index].framebuffer;
			return 1;
		}
	}
	int slot = -1;
	for (int index = 0; index < 8; index++) {
		if (display->framebuffers[index].buffer == NULL) {
			slot = index;
			break;
		}
	}
	if (slot < 0) {
		onix_drm_set_error("GBM surface uses more than 8 scanout buffers");
		return 0;
	}
	uint32_t handles[4] = {display->get_handle(buffer).u32, 0, 0, 0};
	uint32_t pitches[4] = {display->get_stride(buffer), 0, 0, 0};
	uint32_t offsets[4] = {0, 0, 0, 0};
	if (display->add_fb2(
		display->fd,
		display->mode.hdisplay,
		display->mode.vdisplay,
		ONIX_GBM_FORMAT_XRGB8888,
		handles,
		pitches,
		offsets,
		framebuffer,
		0) != 0) {
		onix_drm_set_error("create DRM framebuffer: %s", strerror(errno));
		return 0;
	}
	display->framebuffers[slot] = (OnixDRMFramebuffer){buffer, *framebuffer};
	return 1;
}

static void onix_drm_page_flip_handler(int fd, unsigned int frame, unsigned int seconds, unsigned int microseconds, void *data) {
	(void)fd;
	(void)frame;
	(void)seconds;
	(void)microseconds;
	((OnixDRMDisplay *)data)->flip_pending = 0;
}

static int onix_drm_wait_for_flip(OnixDRMDisplay *display) {
	OnixDRMEventContext context = {
		.version = ONIX_DRM_EVENT_CONTEXT_VERSION,
		.page_flip_handler = onix_drm_page_flip_handler,
	};
	while (display->flip_pending) {
		struct pollfd descriptor = {.fd = display->fd, .events = POLLIN};
		int ready;
		do {
			ready = poll(&descriptor, 1, 3000);
		} while (ready < 0 && errno == EINTR);
		if (ready == 0) {
			onix_drm_set_error("DRM page flip timed out");
			return 0;
		}
		if (ready < 0 || (descriptor.revents & (POLLERR | POLLHUP | POLLNVAL)) != 0) {
			onix_drm_set_error("wait for DRM page flip: %s", ready < 0 ? strerror(errno) : "device disconnected");
			return 0;
		}
		if ((descriptor.revents & POLLIN) != 0 && display->handle_event(display->fd, &context) != 0) {
			onix_drm_set_error("handle DRM page flip: %s", strerror(errno));
			return 0;
		}
	}
	return 1;
}

int onix_drm_present(OnixDRMDisplay *display) {
	if (display == NULL || display->surface == NULL) {
		onix_drm_set_error("invalid DRM display");
		return 0;
	}
	gbm_bo *next_bo = display->lock_front_buffer(display->surface);
	if (next_bo == NULL) {
		onix_drm_set_error("lock GBM front buffer");
		return 0;
	}
	uint32_t next_fb = 0;
	if (!onix_drm_framebuffer(display, next_bo, &next_fb)) {
		display->release_buffer(display->surface, next_bo);
		return 0;
	}
	if (!display->modeset) {
		if (display->set_crtc(display->fd, display->crtc_id, next_fb, 0, 0, &display->connector_id, 1, &display->mode) != 0) {
			onix_drm_set_error("set DRM mode %s: %s", display->mode.name, strerror(errno));
			display->release_buffer(display->surface, next_bo);
			return 0;
		}
		display->modeset = 1;
	} else {
		display->pending_bo = next_bo;
		display->pending_fb = next_fb;
		display->flip_pending = 1;
		if (display->page_flip(display->fd, display->crtc_id, next_fb, ONIX_DRM_MODE_PAGE_FLIP_EVENT, display) != 0) {
			display->flip_pending = 0;
			onix_drm_set_error("queue DRM page flip: %s", strerror(errno));
			display->release_buffer(display->surface, next_bo);
			display->pending_bo = NULL;
			display->pending_fb = 0;
			return 0;
		}
		if (!onix_drm_wait_for_flip(display)) {
			return 0;
		}
	}
	if (display->current_bo != NULL) {
		display->release_buffer(display->surface, display->current_bo);
	}
	display->current_fb = next_fb;
	display->current_bo = next_bo;
	display->pending_bo = NULL;
	display->pending_fb = 0;
	return 1;
}

void onix_drm_destroy(OnixDRMDisplay *display) {
	if (display == NULL) {
		return;
	}
	// Vulkan can route a different CRTC. Restore the snapshot after releasing its display.
	if (display->restore_outputs && display->fd >= 0 && display->set_crtc != NULL) {
		for (int active = 0; active < 2; active++) {
			for (int index = 0; index < display->output_count; index++) {
				OnixDRMOutputState *output = &display->outputs[index];
				OnixDRMCrtc *crtc = output->crtc;
				if (crtc == NULL || !!crtc->mode_valid != active) continue;
				display->set_crtc(display->fd, crtc->crtc_id, active ? crtc->buffer_id : 0,
					active ? crtc->x : 0, active ? crtc->y : 0,
					active ? output->connectors : NULL, active ? output->connector_count : 0,
					active ? &crtc->mode : NULL);
			}
		}
	}
	if (display->fd >= 0 && display->modeset && display->saved_crtc != NULL && display->set_crtc != NULL) {
		if (display->saved_crtc->mode_valid) {
			display->set_crtc(
				display->fd,
				display->saved_crtc->crtc_id,
				display->saved_crtc->buffer_id,
				display->saved_crtc->x,
				display->saved_crtc->y,
				&display->connector_id,
				1,
				&display->saved_crtc->mode);
		} else {
			display->set_crtc(display->fd, display->crtc_id, 0, 0, 0, NULL, 0, NULL);
		}
	}
	if (display->surface != NULL && display->pending_bo != NULL && display->pending_bo != display->current_bo && display->release_buffer != NULL) {
		display->release_buffer(display->surface, display->pending_bo);
	}
	if (display->surface != NULL && display->current_bo != NULL && display->release_buffer != NULL) {
		display->release_buffer(display->surface, display->current_bo);
	}
	if (display->fd >= 0 && display->rm_fb != NULL) {
		for (int index = 0; index < 8; index++) {
			if (display->framebuffers[index].framebuffer != 0) {
				display->rm_fb(display->fd, display->framebuffers[index].framebuffer);
			}
		}
	}
	if (display->surface != NULL && display->destroy_surface != NULL) {
		display->destroy_surface(display->surface);
	}
	if (display->gbm != NULL && display->destroy_device != NULL) {
		display->destroy_device(display->gbm);
	}
	if (display->saved_crtc != NULL && display->free_crtc != NULL) {
		display->free_crtc(display->saved_crtc);
	}
	for (int index = 0; index < display->output_count; index++) {
		if (display->outputs[index].crtc != NULL) display->free_crtc(display->outputs[index].crtc);
		free(display->outputs[index].connectors);
	}
	free(display->outputs);
	if (display->fd >= 0) {
		close(display->fd);
	}
	if (display->gbm_library != NULL) {
		dlclose(display->gbm_library);
	}
	if (display->drm_library != NULL) {
		dlclose(display->drm_library);
	}
	free(display);
}
