#ifndef ONIX_ENGINE_WINDOW_DRM_NATIVE_H
#define ONIX_ENGINE_WINDOW_DRM_NATIVE_H

#include <stdint.h>

typedef struct OnixDRMDisplay OnixDRMDisplay;

const char *onix_drm_last_error(void);
OnixDRMDisplay *onix_drm_create(const char *device, uint32_t connector_id);
int onix_drm_create_gbm(OnixDRMDisplay *display);
int onix_drm_fd(OnixDRMDisplay *display);
uint32_t onix_drm_connector_id(OnixDRMDisplay *display);
uint32_t onix_drm_refresh_rate(OnixDRMDisplay *display);
int onix_drm_save_outputs(OnixDRMDisplay *display);
void onix_drm_restore_on_destroy(OnixDRMDisplay *display);
void *onix_drm_gbm_device(OnixDRMDisplay *display);
void *onix_drm_gbm_surface(OnixDRMDisplay *display);
int onix_drm_width(OnixDRMDisplay *display);
int onix_drm_height(OnixDRMDisplay *display);
int onix_drm_present(OnixDRMDisplay *display);
void onix_drm_destroy(OnixDRMDisplay *display);

#endif
