#ifndef ONIX_ENGINE_WINDOW_DRM_INPUT_H
#define ONIX_ENGINE_WINDOW_DRM_INPUT_H

typedef struct OnixDRMInput OnixDRMInput;

typedef enum {
	ONIX_DRM_INPUT_NONE = 0,
	ONIX_DRM_INPUT_KEY_DOWN = 1,
	ONIX_DRM_INPUT_KEY_UP = 2,
	ONIX_DRM_INPUT_MOUSE_MOVE = 3,
	ONIX_DRM_INPUT_MOUSE_DOWN = 4,
	ONIX_DRM_INPUT_MOUSE_UP = 5,
	ONIX_DRM_INPUT_MOUSE_WHEEL = 6,
	ONIX_DRM_INPUT_TOUCH_START = 7,
	ONIX_DRM_INPUT_TOUCH_MOVE = 8,
	ONIX_DRM_INPUT_TOUCH_END = 9,
} OnixDRMInputKind;

typedef struct {
	int kind;
	int code;
	double x;
	double y;
} OnixDRMInputEvent;

OnixDRMInput *onix_drm_input_create(int width, int height);
void onix_drm_input_poll(OnixDRMInput *input);
void onix_drm_input_wait(OnixDRMInput *input, int wake_fd, int timeout_ms);
int onix_drm_input_pop(OnixDRMInput *input, OnixDRMInputEvent *event);
int onix_drm_input_has_keyboard(OnixDRMInput *input);
int onix_drm_input_has_pointer(OnixDRMInput *input);
void onix_drm_input_destroy(OnixDRMInput *input);

#endif
