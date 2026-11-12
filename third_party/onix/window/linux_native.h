#ifndef ONIX_ENGINE_WINDOW_LINUX_NATIVE_H
#define ONIX_ENGINE_WINDOW_LINUX_NATIVE_H

#include <stdint.h>
#include "cursor_native.h"

typedef enum {
	ONIX_BACKEND_NONE = 0,
	ONIX_BACKEND_X11 = 1,
	ONIX_BACKEND_WAYLAND = 2,
} OnixBackend;

typedef enum {
	ONIX_EVENT_NONE = 0,
	ONIX_EVENT_CLOSE = 1,
	ONIX_EVENT_WINDOW_SIZE = 2,
	ONIX_EVENT_FRAMEBUFFER_SIZE = 3,
	ONIX_EVENT_KEY_DOWN = 4,
	ONIX_EVENT_KEY_UP = 5,
	ONIX_EVENT_MOUSE_MOVE = 6,
	ONIX_EVENT_MOUSE_DOWN = 7,
	ONIX_EVENT_MOUSE_UP = 8,
	ONIX_EVENT_MOUSE_WHEEL = 9,
	ONIX_EVENT_TEXT_INPUT = 10,
	ONIX_EVENT_TEXT_COMPOSITION = 11,
	ONIX_EVENT_TEXT_COMMAND = 12,
	ONIX_EVENT_WINDOW_STATE = 13,
	ONIX_EVENT_FILES_DROPPED = 14,
	ONIX_EVENT_MOUSE_DELTA = 15,
	ONIX_EVENT_MOUSE_CAPTURE = 16,
} OnixEventKind;

typedef struct {
	int kind;
	int arg0;
	int arg1;
	double x;
	double y;
	char text[4096];
} OnixEvent;

typedef struct {
	int x;
	int y;
	int width;
	int height;
} OnixInputRect;

typedef struct OnixNativeWindow OnixNativeWindow;

const char *onix_last_error(void);

OnixNativeWindow *onix_window_create(int width, int height, const char *title, int client_decorations, int require_x11);
void onix_window_destroy(OnixNativeWindow *window);

int onix_window_backend(OnixNativeWindow *window);
int onix_window_should_close(OnixNativeWindow *window);
void onix_window_cancel_close(OnixNativeWindow *window);
int onix_window_window_width(OnixNativeWindow *window);
int onix_window_window_height(OnixNativeWindow *window);
int onix_window_framebuffer_width(OnixNativeWindow *window);
int onix_window_framebuffer_height(OnixNativeWindow *window);
void *onix_window_native_display(OnixNativeWindow *window);
uintptr_t onix_window_native_surface(OnixNativeWindow *window);

void onix_window_set_title(OnixNativeWindow *window, const char *title);
void onix_window_set_icon(OnixNativeWindow *window, int width, int height, const uint32_t *pixels);
void onix_window_set_cursor(OnixNativeWindow *window, int cursor);
void onix_window_set_vsync(OnixNativeWindow *window, int enabled);
int onix_window_set_mouse_captured(OnixNativeWindow *window, int captured);
void onix_window_set_text_input_config(OnixNativeWindow *window, int active, int multiline, int kind);
void onix_window_set_decorations(OnixNativeWindow *window, int mode);
void onix_window_set_border_visible(OnixNativeWindow *window, int visible);
void onix_window_set_shadow_visible(OnixNativeWindow *window, int visible);
void onix_window_set_resizable(OnixNativeWindow *window, int enabled);
int onix_window_is_maximized(OnixNativeWindow *window);
int onix_window_is_fullscreen(OnixNativeWindow *window);
void onix_window_set_taskbar_visible(OnixNativeWindow *window, int visible);
void onix_window_set_always_on_top(OnixNativeWindow *window, int enabled);
void onix_window_set_input_region(OnixNativeWindow *window, int enabled, const OnixInputRect *rects, int count);
void onix_window_minimize(OnixNativeWindow *window);
void onix_window_toggle_maximize(OnixNativeWindow *window);
void onix_window_toggle_fullscreen(OnixNativeWindow *window);
void onix_window_start_drag(OnixNativeWindow *window);
int onix_window_set_clipboard_text(OnixNativeWindow *window, const char *text);
int onix_window_get_clipboard_text(OnixNativeWindow *window, char *buffer, int capacity);
void onix_window_poll_events(OnixNativeWindow *window);
void onix_window_wait_events(OnixNativeWindow *window, int wake_fd, int timeout_ms);
int onix_window_pop_event(OnixNativeWindow *window, OnixEvent *event);

#endif
