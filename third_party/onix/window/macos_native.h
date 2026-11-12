#ifndef ONIX_ENGINE_WINDOW_MACOS_NATIVE_H
#define ONIX_ENGINE_WINDOW_MACOS_NATIVE_H

#include <stdint.h>
#include "cursor_native.h"

enum {
    ONIX_MAC_EVENT_CLOSE = 1,
    ONIX_MAC_EVENT_RESIZE = 2,
    ONIX_MAC_EVENT_KEY_DOWN = 3,
    ONIX_MAC_EVENT_KEY_UP = 4,
    ONIX_MAC_EVENT_TEXT = 5,
    ONIX_MAC_EVENT_MOUSE_DOWN = 6,
    ONIX_MAC_EVENT_MOUSE_UP = 7,
    ONIX_MAC_EVENT_MOUSE_MOVE = 8,
    ONIX_MAC_EVENT_MOUSE_WHEEL = 9,
    ONIX_MAC_EVENT_TEXT_COMPOSITION = 10,
    ONIX_MAC_EVENT_TEXT_COMMAND = 11,
    ONIX_MAC_EVENT_FILES_DROPPED = 12,
    ONIX_MAC_EVENT_MOUSE_DELTA = 13,
    ONIX_MAC_EVENT_MOUSE_CAPTURE = 14,
    ONIX_MAC_EVENT_REDRAW = 15,
};

enum {
    ONIX_MAC_TEXT_COMMAND_BACKSPACE = 1,
    ONIX_MAC_TEXT_COMMAND_ENTER = 2,
};

typedef struct OnixMacWindow OnixMacWindow;

// onix_macos_file_dialog returns the number of selected paths, zero when the
// panel is cancelled, or a negative value on allocation/native failure.
int onix_macos_file_dialog(
    OnixMacWindow *window,
    int mode,
    const char *title,
    const char *default_path,
    const char *extensions,
    int extension_bytes,
    char ***paths
);
void onix_macos_file_dialog_free(char **paths, int count);

typedef struct {
    // Retained native press; ownership transfers to the caller of pop_event.
    void *mouse_down;
    int kind;
    int arg0;
    double x;
    double y;
    char text[4096];
} OnixMacEvent;

typedef struct {
    int x;
    int y;
    int width;
    int height;
} OnixMacInputRect;

const char *onix_macos_last_error(void);
OnixMacWindow *onix_macos_create(int width, int height, const char *title, int transparent, int decorations, double title_bar_height, int shadow, int resizable);
void onix_macos_destroy(OnixMacWindow *window);
void onix_macos_set_cursor(OnixMacWindow *window, int cursor);
int onix_macos_set_mouse_captured(OnixMacWindow *window, int captured);
void onix_macos_poll(OnixMacWindow *window);
void onix_macos_wait(OnixMacWindow *window, int timeout_ms);
void onix_macos_wake(void);
int onix_macos_pop_event(OnixMacWindow *window, OnixMacEvent *event);
void onix_macos_release_event(OnixMacEvent *event);
int onix_macos_should_close(OnixMacWindow *window);
void onix_macos_cancel_close(OnixMacWindow *window);
int onix_macos_width(OnixMacWindow *window);
int onix_macos_height(OnixMacWindow *window);
double onix_macos_scale(OnixMacWindow *window);
void *onix_macos_view(OnixMacWindow *window);
void *onix_macos_layer(OnixMacWindow *window);
void *onix_macos_native_window(OnixMacWindow *window);
void onix_macos_set_title(OnixMacWindow *window, const char *title);
void onix_macos_set_decorations(OnixMacWindow *window, int mode);
void onix_macos_set_title_bar_height(OnixMacWindow *window, double height);
void onix_macos_set_shadow_visible(OnixMacWindow *window, int visible);
void onix_macos_set_resizable(OnixMacWindow *window, int enabled);
void onix_macos_set_taskbar_visible(OnixMacWindow *window, int visible);
void onix_macos_set_always_on_top(OnixMacWindow *window, int enabled);
void onix_macos_set_input_region(OnixMacWindow *window, int enabled, const OnixMacInputRect *rects, int count);
void onix_macos_minimize(OnixMacWindow *window);
void onix_macos_toggle_maximize(OnixMacWindow *window);
void onix_macos_toggle_fullscreen(OnixMacWindow *window);
void onix_macos_start_drag(OnixMacWindow *window, const OnixMacEvent *event);
void onix_macos_set_text_input_config(
    OnixMacWindow *window,
    uint64_t session,
    int active,
    int selection_start,
    int selection_end,
    double caret_x,
    double caret_y,
    double caret_width,
    double caret_height
);
int onix_macos_set_clipboard_text(const char *text);
int onix_macos_get_clipboard_text(char *buffer, int capacity);

#endif
