//go:build linux && cgo && !android && !ios && !js && !wasm && !onix_drm

#include "linux_native.h"

#include <X11/Xatom.h>
#include <X11/cursorfont.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/Xrender.h>
#include <X11/extensions/shape.h>
#include <X11/extensions/XInput2.h>
#include <X11/keysym.h>
#include <errno.h>
#include <linux/input-event-codes.h>
#include <fcntl.h>
#include <locale.h>
#include <poll.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>
#include <wayland-client.h>
#include <wayland-cursor.h>
#include <xkbcommon/xkbcommon-compose.h>
#include <xkbcommon/xkbcommon-names.h>
#include <xkbcommon/xkbcommon.h>

#include "text-input-unstable-v3-client-protocol.h"
#include "xdg-decoration-unstable-v1-client-protocol.h"
#include "xdg-shell-client-protocol.h"
#include "relative-pointer-unstable-v1-client-protocol.h"
#include "pointer-constraints-unstable-v1-client-protocol.h"

#ifndef Button6
#define Button6 6
#endif

#ifndef Button7
#define Button7 7
#endif

#define ONIX_EVENT_QUEUE_CAPACITY 256
#define ONIX_KEY_ENTER 257
#define ONIX_KEY_BACKSPACE 259
#define ONIX_TEXT_COMMAND_BACKSPACE 1
#define ONIX_TEXT_COMMAND_ENTER 2

typedef struct {
	int head;
	int tail;
	OnixEvent items[ONIX_EVENT_QUEUE_CAPACITY];
} OnixEventQueue;

typedef struct {
	Display *display;
	Window window;
	Colormap colormap;
	unsigned long border_pixel;
	Atom wm_delete;
	XIM im;
	XIC ic;
	int focused;
	int xi_opcode;
	Cursor hidden_cursor;
	Cursor cursors[ONIX_CURSOR_COUNT];
	Atom clipboard;
	Atom targets;
	Atom utf8_string;
	Atom net_wm_state;
	Atom maximized_horz;
	Atom maximized_vert;
	Atom fullscreen;
	Atom xdnd_aware;
	Atom xdnd_enter;
	Atom xdnd_position;
	Atom xdnd_status;
	Atom xdnd_leave;
	Atom xdnd_drop;
	Atom xdnd_finished;
	Atom xdnd_selection;
	Atom xdnd_type_list;
	Atom xdnd_action_copy;
	Atom text_uri_list;
	Atom xdnd_property;
	Window xdnd_source;
	int xdnd_accept;
	int xdnd_x;
	int xdnd_y;
	Time xdnd_time;
} OnixX11State;

typedef struct {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_compositor *compositor;
	struct wl_surface *surface;
	struct wl_seat *seat;
	struct wl_pointer *pointer;
	struct wl_shm *shm;
	struct wl_cursor_theme *cursor_theme;
	struct wl_surface *cursor_surface;
	struct zwp_relative_pointer_manager_v1 *relative_pointer_manager;
	struct zwp_pointer_constraints_v1 *pointer_constraints;
	struct zwp_relative_pointer_v1 *relative_pointer;
	struct zwp_locked_pointer_v1 *locked_pointer;
	uint32_t pointer_serial;
	int pointer_entered;
	struct wl_keyboard *keyboard;
	struct wl_data_device_manager *data_device_manager;
	struct wl_data_device *data_device;
	struct wl_data_source *data_source;
	struct wl_data_offer *selection_offer;
	struct wl_data_offer *pending_offer;
	int pending_offer_utf8;
	int pending_offer_uri;
	int selection_offer_utf8;
	struct wl_data_offer *dnd_offer;
	int dnd_offer_uri;
	uint32_t dnd_serial;
	double dnd_x;
	double dnd_y;
	int dnd_fd;
	char *dnd_buffer;
	size_t dnd_length;
	size_t dnd_capacity;
	struct xdg_wm_base *wm_base;
	struct xdg_surface *xdg_surface;
	struct xdg_toplevel *xdg_toplevel;
	struct zxdg_decoration_manager_v1 *decoration_manager;
	struct zxdg_toplevel_decoration_v1 *toplevel_decoration;
	struct xkb_context *xkb_context;
	struct xkb_keymap *xkb_keymap;
	struct xkb_state *xkb_state;
	struct xkb_compose_table *xkb_compose_table;
	struct xkb_compose_state *xkb_compose_state;
	struct zwp_text_input_manager_v3 *text_input_manager;
	struct zwp_text_input_v3 *text_input;
	int configured;
	int text_input_entered;
	uint32_t text_input_commit_count;
	uint32_t text_input_session_serial;
	char *pending_preedit;
	char *pending_commit;
	int has_pending_preedit;
	int has_pending_commit;
	double mouse_x;
	double mouse_y;
	uint32_t last_serial;
} OnixWaylandState;

struct OnixNativeWindow {
	OnixBackend backend;
	int should_close;
	int vsync_enabled;
	int window_width;
	int window_height;
	int framebuffer_width;
	int framebuffer_height;
	int text_input_active;
	int text_input_multiline;
	int text_input_kind;
	int maximized;
	int fullscreen;
	int mouse_captured;
	int cursor;
	uint8_t modifiers;
	char *clipboard_text;
	OnixEventQueue queue;
	union {
		OnixX11State x11;
		OnixWaylandState wayland;
	} state;
};

static __thread char onix_error_message[512];

static void onix_set_error(const char *fmt, ...) {
	va_list args;
	va_start(args, fmt);
	vsnprintf(onix_error_message, sizeof(onix_error_message), fmt, args);
	va_end(args);
}

const char *onix_last_error(void) {
	return onix_error_message;
}

static void onix_clear_error(void) {
	onix_error_message[0] = '\0';
}

static OnixNativeWindow *onix_alloc_window(int width, int height) {
	OnixNativeWindow *window = calloc(1, sizeof(OnixNativeWindow));
	if (window == NULL) {
		onix_set_error("failed to allocate Linux window state");
		return NULL;
	}
	window->vsync_enabled = 1;
	window->window_width = width;
	window->window_height = height;
	window->framebuffer_width = width;
	window->framebuffer_height = height;
	return window;
}

static void onix_push_event(OnixNativeWindow *window, OnixEventKind kind, int arg0, int arg1, double x, double y) {
	if (window == NULL) {
		return;
	}
	if (arg0 >= 340 && arg0 <= 347) {
		uint8_t bit = 1u << (arg0 - 340);
		if (kind == ONIX_EVENT_KEY_DOWN) window->modifiers |= bit;
		else if (kind == ONIX_EVENT_KEY_UP) window->modifiers &= ~bit;
	}
	// High-rate relative motion must not overflow the fixed event queue.
	if (kind == ONIX_EVENT_MOUSE_DELTA && window->queue.head != window->queue.tail) {
		OnixEvent *previous = &window->queue.items[(window->queue.tail + ONIX_EVENT_QUEUE_CAPACITY - 1) % ONIX_EVENT_QUEUE_CAPACITY];
		if (previous->kind == kind) {
			previous->x += x;
			previous->y += y;
			return;
		}
	}
	int next = (window->queue.tail + 1) % ONIX_EVENT_QUEUE_CAPACITY;
	if (next == window->queue.head) {
		window->queue.head = (window->queue.head + 1) % ONIX_EVENT_QUEUE_CAPACITY;
	}
	window->queue.items[window->queue.tail].kind = kind;
	window->queue.items[window->queue.tail].arg0 = arg0;
	window->queue.items[window->queue.tail].arg1 = arg1;
	window->queue.items[window->queue.tail].x = x;
	window->queue.items[window->queue.tail].y = y;
	window->queue.items[window->queue.tail].text[0] = '\0';
	window->queue.tail = next;
}

static void onix_release_modifiers(OnixNativeWindow *window, uint8_t mask) {
	uint8_t released = window->modifiers & mask;
	for (int index = 0; index < 8; index++) {
		if (released & (1u << index)) onix_push_event(window, ONIX_EVENT_KEY_UP, 340 + index, 0, 0, 0);
	}
}

// Reconcile with the native mask: IMEs and focus changes can consume key events.
// The low four bits represent Shift, Control, Alt and Super, in that order.
static void onix_sync_modifiers(OnixNativeWindow *window, uint8_t active) {
	onix_release_modifiers(window, ~(active | (active << 4)));
	for (int index = 0; index < 4; index++) {
		uint8_t bit = 1u << index;
		if ((active & bit) && !(window->modifiers & (bit | (bit << 4)))) {
			onix_push_event(window, ONIX_EVENT_KEY_DOWN, 340 + index, 0, 0, 0);
		}
	}
}

static void onix_push_text_event_chunk(OnixNativeWindow *window, OnixEventKind kind, int active, const char *text, int len) {
	if (window == NULL || text == NULL || len < 0 || len >= (int)sizeof(window->queue.items[0].text)) {
		return;
	}
	int next = (window->queue.tail + 1) % ONIX_EVENT_QUEUE_CAPACITY;
	if (next == window->queue.head) {
		window->queue.head = (window->queue.head + 1) % ONIX_EVENT_QUEUE_CAPACITY;
	}
	OnixEvent *event = &window->queue.items[window->queue.tail];
	memset(event, 0, sizeof(*event));
	event->kind = kind;
	event->arg0 = active;
	memcpy(event->text, text, (size_t)len);
	event->text[len] = '\0';
	window->queue.tail = next;
}

static void onix_push_text_event(OnixNativeWindow *window, OnixEventKind kind, int active, const char *text, int len) {
	if (window == NULL || text == NULL || len < 0) {
		return;
	}
	if (len == 0) {
		onix_push_text_event_chunk(window, kind, active, text, 0);
		return;
	}
	while (len > 0) {
		int chunk = len;
		if (chunk >= (int)sizeof(window->queue.items[0].text)) {
			chunk = (int)sizeof(window->queue.items[0].text) - 1;
			while (chunk > 0 && (((unsigned char)text[chunk] & 0xC0) == 0x80)) {
				chunk--;
			}
		}
		if (chunk <= 0) {
			chunk = 1;
		}
		onix_push_text_event_chunk(window, kind, active, text, chunk);
		text += chunk;
		len -= chunk;
	}
}

static void onix_push_drop_event(OnixNativeWindow *window, double x, double y, const char *path, int len) {
	if (window == NULL || path == NULL || len <= 0 || len >= (int)sizeof(window->queue.items[0].text)) return;
	int next = (window->queue.tail + 1) % ONIX_EVENT_QUEUE_CAPACITY;
	if (next == window->queue.head) window->queue.head = (window->queue.head + 1) % ONIX_EVENT_QUEUE_CAPACITY;
	OnixEvent *event = &window->queue.items[window->queue.tail];
	memset(event, 0, sizeof(*event));
	event->kind = ONIX_EVENT_FILES_DROPPED;
	event->x = x;
	event->y = y;
	memcpy(event->text, path, (size_t)len);
	event->text[len] = '\0';
	window->queue.tail = next;
}

static void onix_push_text_input_event(OnixNativeWindow *window, const char *text, int len) {
	if (window == NULL || text == NULL || len <= 0) {
		return;
	}
	char stack[256];
	char *clean = len <= (int)sizeof(stack) ? stack : malloc((size_t)len);
	if (clean == NULL) {
		return;
	}
	int clean_len = 0;
	for (int i = 0; i < len; i++) {
		unsigned char value = (unsigned char)text[i];
		if (value >= 0x20 && value != 0x7F) {
			clean[clean_len++] = text[i];
		}
	}
	if (clean_len > 0) {
		onix_push_text_event(window, ONIX_EVENT_TEXT_INPUT, 0, clean, clean_len);
	}
	if (clean != stack) {
		free(clean);
	}
}

static void onix_push_text_command(OnixNativeWindow *window, int command, int count) {
	if (window == NULL || command <= 0 || count <= 0) {
		return;
	}
	onix_push_event(window, ONIX_EVENT_TEXT_COMMAND, command, count, 0, 0);
}

static void onix_replace_string(char **target, const char *value) {
	if (target == NULL) {
		return;
	}
	char *replacement = value != NULL ? strdup(value) : NULL;
	if (value != NULL && replacement == NULL) {
		return;
	}
	free(*target);
	*target = replacement;
}

static int onix_is_line_break_text(const char *text) {
	if (text == NULL || text[0] == '\0') {
		return 0;
	}
	for (const char *cursor = text; *cursor != '\0'; cursor++) {
		if (*cursor != '\n' && *cursor != '\r') {
			return 0;
		}
	}
	return 1;
}

static void onix_clear_wayland_text_pending(OnixNativeWindow *window) {
	if (window == NULL) {
		return;
	}
	free(window->state.wayland.pending_preedit);
	free(window->state.wayland.pending_commit);
	window->state.wayland.pending_preedit = NULL;
	window->state.wayland.pending_commit = NULL;
	window->state.wayland.has_pending_preedit = 0;
	window->state.wayland.has_pending_commit = 0;
}

static int onix_keysym_to_keycode(uint32_t sym) {
	if (sym >= '0' && sym <= '9') {
		return (int)sym;
	}
	if (sym >= 'A' && sym <= 'Z') {
		return (int)sym;
	}
	if (sym >= 'a' && sym <= 'z') {
		return (int)(sym - 'a' + 'A');
	}
	if (sym >= XK_F1 && sym <= XK_F25) {
		return 290 + (int)(sym - XK_F1);
	}
	if (sym >= XK_KP_0 && sym <= XK_KP_9) {
		return 320 + (int)(sym - XK_KP_0);
	}

	switch (sym) {
	case XK_space:
		return 32;
	case XK_apostrophe:
		return 39;
	case XK_comma:
		return 44;
	case XK_minus:
		return 45;
	case XK_period:
		return 46;
	case XK_slash:
		return 47;
	case XK_semicolon:
		return 59;
	case XK_equal:
		return 61;
	case XK_bracketleft:
		return 91;
	case XK_backslash:
		return 92;
	case XK_bracketright:
		return 93;
	case XK_grave:
		return 96;
	case XK_Escape:
		return 256;
	case XK_Return:
	case XK_ISO_Enter:
		return 257;
	case XK_Tab:
	case XK_ISO_Left_Tab:
		return 258;
	case XK_BackSpace:
		return 259;
	case XK_Insert:
		return 260;
	case XK_Delete:
		return 261;
	case XK_Right:
		return 262;
	case XK_Left:
		return 263;
	case XK_Down:
		return 264;
	case XK_Up:
		return 265;
	case XK_Page_Up:
		return 266;
	case XK_Page_Down:
		return 267;
	case XK_Home:
		return 268;
	case XK_End:
		return 269;
	case XK_Caps_Lock:
		return 280;
	case XK_Scroll_Lock:
		return 281;
	case XK_Num_Lock:
		return 282;
	case XK_Print:
		return 283;
	case XK_Pause:
		return 284;
	case XK_KP_Decimal:
		return 330;
	case XK_KP_Divide:
		return 331;
	case XK_KP_Multiply:
		return 332;
	case XK_KP_Subtract:
		return 333;
	case XK_KP_Add:
		return 334;
	case XK_KP_Enter:
		return 335;
	case XK_KP_Equal:
		return 336;
	case XK_Shift_L:
		return 340;
	case XK_Control_L:
		return 341;
	case XK_Alt_L:
	case XK_Meta_L:
		return 342;
	case XK_Super_L:
		return 343;
	case XK_Shift_R:
		return 344;
	case XK_Control_R:
		return 345;
	case XK_Alt_R:
	case XK_Meta_R:
		return 346;
	case XK_Super_R:
		return 347;
	case XK_Menu:
		return 348;
	default:
		return 0;
	}
}

static int onix_mouse_button_from_linux(uint32_t button) {
	switch (button) {
	case BTN_LEFT:
		return 0;
	case BTN_RIGHT:
		return 1;
	case BTN_MIDDLE:
		return 2;
	default:
		return -1;
	}
}

static int onix_mouse_button_from_x11(unsigned int button) {
	switch (button) {
	case Button1:
		return 0;
	case Button3:
		return 1;
	case Button2:
		return 2;
	default:
		return -1;
	}
}

static int onix_x11_is_maximized(OnixNativeWindow *window) {
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	if (XGetWindowProperty(
			window->state.x11.display,
			window->state.x11.window,
			window->state.x11.net_wm_state,
			0,
			32,
			False,
			XA_ATOM,
			&actual_type,
			&actual_format,
			&item_count,
			&bytes_after,
			&data) != Success) {
		return 0;
	}
	int horizontal = 0;
	int vertical = 0;
	Atom *states = (Atom *)data;
	for (unsigned long index = 0; index < item_count; index++) {
		horizontal |= states[index] == window->state.x11.maximized_horz;
		vertical |= states[index] == window->state.x11.maximized_vert;
	}
	if (data != NULL) {
		XFree(data);
	}
	return horizontal && vertical;
}

static int onix_x11_is_fullscreen(OnixNativeWindow *window) {
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	if (XGetWindowProperty(
			window->state.x11.display,
			window->state.x11.window,
			window->state.x11.net_wm_state,
			0,
			32,
			False,
			XA_ATOM,
			&actual_type,
			&actual_format,
			&item_count,
			&bytes_after,
			&data) != Success) {
		return 0;
	}
	int fullscreen = 0;
	Atom *states = (Atom *)data;
	for (unsigned long index = 0; index < item_count; index++) {
		fullscreen |= states[index] == window->state.x11.fullscreen;
	}
	if (data != NULL) {
		XFree(data);
	}
	return fullscreen;
}

static int onix_x11_argb_visual(Display *display, int screen, XVisualInfo *result) {
	XVisualInfo match = {
		.screen = screen,
		.depth = 32,
		.class = TrueColor,
	};
	int count = 0;
	XVisualInfo *visuals = XGetVisualInfo(display, VisualScreenMask | VisualDepthMask | VisualClassMask, &match, &count);
	for (int index = 0; index < count; index++) {
		XRenderPictFormat *format = XRenderFindVisualFormat(display, visuals[index].visual);
		if (format != NULL && format->type == PictTypeDirect && format->direct.alphaMask != 0) {
			*result = visuals[index];
			XFree(visuals);
			return 1;
		}
	}
	if (visuals != NULL) {
		XFree(visuals);
	}
	return 0;
}

static int onix_x11_default_visual(Display *display, int screen, XVisualInfo *result) {
	XVisualInfo match = {
		.visualid = XVisualIDFromVisual(DefaultVisual(display, screen)),
	};
	int count = 0;
	XVisualInfo *visuals = XGetVisualInfo(display, VisualIDMask, &match, &count);
	if (visuals == NULL || count == 0) {
		if (visuals != NULL) {
			XFree(visuals);
		}
		return 0;
	}
	*result = visuals[0];
	XFree(visuals);
	return 1;
}

static void onix_x11_set_decorations(OnixNativeWindow *window, int client_decorations) {
	if (window == NULL || window->state.x11.display == NULL || window->state.x11.window == 0) {
		return;
	}
	typedef struct { unsigned long flags, functions, decorations; long input_mode; unsigned long status; } MotifHints;
	MotifHints hints = { .flags = 2, .decorations = client_decorations ? 0 : 1 };
	Atom property = XInternAtom(window->state.x11.display, "_MOTIF_WM_HINTS", False);
	XChangeProperty(window->state.x11.display, window->state.x11.window, property, property, 32, PropModeReplace, (unsigned char *)&hints, 5);
}

static void onix_x11_set_border_visible(OnixNativeWindow *window, int visible) {
	if (window == NULL || window->state.x11.display == NULL || window->state.x11.window == 0) {
		return;
	}
	Display *display = window->state.x11.display;
	XSetWindowBorderWidth(display, window->state.x11.window, visible != 0 ? 1 : 0);
	XSetWindowBorder(display, window->state.x11.window, window->state.x11.border_pixel);
}

static unsigned long onix_x11_channel_pixel(unsigned long mask, unsigned int value) {
	if (mask == 0) {
		return 0;
	}
	unsigned int shift = 0;
	while ((mask & 1UL) == 0) {
		mask >>= 1;
		shift++;
	}
	return (((unsigned long)value * mask + 127) / 255) << shift;
}

static unsigned long onix_x11_border_pixel(const XVisualInfo *visual_info) {
	if (visual_info == NULL) {
		return 0;
	}
	/* Default-screen pixels are transparent in the selected ARGB visual. */
	unsigned long visual_mask = visual_info->red_mask | visual_info->green_mask | visual_info->blue_mask;
	unsigned long depth_mask = ~0UL;
	if (visual_info->depth < (int)(sizeof(unsigned long) * 8)) {
		depth_mask = (1UL << visual_info->depth) - 1;
	}
	return (depth_mask & ~visual_mask)
		| onix_x11_channel_pixel(visual_info->red_mask, 82)
		| onix_x11_channel_pixel(visual_info->green_mask, 82)
		| onix_x11_channel_pixel(visual_info->blue_mask, 82);
}

static int onix_init_x11(OnixNativeWindow *window, int width, int height, const char *title, int client_decorations) {
	setlocale(LC_CTYPE, "");
	XInitThreads();
	XSetLocaleModifiers("");

	Display *display = XOpenDisplay(NULL);
	if (display == NULL) {
		onix_set_error("XOpenDisplay failed");
		return 0;
	}

	window->backend = ONIX_BACKEND_X11;
	window->state.x11.display = display;

	int screen = DefaultScreen(display);
	Window root = RootWindow(display, screen);
	XVisualInfo visual_info;
	if (!onix_x11_argb_visual(display, screen, &visual_info)) {
		/*
		 * XQuartz exposes 32-bit pixmap formats but no 32-bit ARGB visual.
		 * Opaque windows can use the default visual instead.
		 */
		if (!onix_x11_default_visual(display, screen, &visual_info)) {
			onix_set_error("X11 server does not expose a usable visual");
			return 0;
		}
	}
	Visual *visual = visual_info.visual;
	int depth = visual_info.depth;
	window->state.x11.border_pixel = onix_x11_border_pixel(&visual_info);

	XSetWindowAttributes attrs;
	memset(&attrs, 0, sizeof(attrs));
	attrs.colormap = XCreateColormap(display, root, visual, AllocNone);
	attrs.background_pixel = 0;
	attrs.border_pixel = window->state.x11.border_pixel;
	attrs.event_mask = ExposureMask | StructureNotifyMask | PropertyChangeMask | FocusChangeMask | KeyPressMask | KeyReleaseMask | PointerMotionMask | ButtonPressMask | ButtonReleaseMask;
	window->state.x11.colormap = attrs.colormap;
	int initial_x = (DisplayWidth(display, screen) - width) / 2;
	int initial_y = (DisplayHeight(display, screen) - height) / 2;

	window->state.x11.window = XCreateWindow(
		display,
		root,
		initial_x,
		initial_y,
		(unsigned int)width,
		(unsigned int)height,
		0,
		depth,
		InputOutput,
		visual,
		CWColormap | CWBackPixel | CWBorderPixel | CWEventMask,
		&attrs);
	if (window->state.x11.window == 0) {
		onix_set_error("XCreateWindow failed");
		return 0;
	}

	if (title != NULL && title[0] != '\0') {
		XStoreName(display, window->state.x11.window, title);
	}
	XSizeHints initial_hints = {0};
	initial_hints.flags = PPosition | PSize;
	initial_hints.x = initial_x;
	initial_hints.y = initial_y;
	initial_hints.width = width;
	initial_hints.height = height;
	XSetWMNormalHints(display, window->state.x11.window, &initial_hints);
	window->state.x11.wm_delete = XInternAtom(display, "WM_DELETE_WINDOW", False);
	window->state.x11.clipboard = XInternAtom(display, "CLIPBOARD", False);
	window->state.x11.targets = XInternAtom(display, "TARGETS", False);
	window->state.x11.utf8_string = XInternAtom(display, "UTF8_STRING", False);
	window->state.x11.net_wm_state = XInternAtom(display, "_NET_WM_STATE", False);
	window->state.x11.maximized_horz = XInternAtom(display, "_NET_WM_STATE_MAXIMIZED_HORZ", False);
	window->state.x11.maximized_vert = XInternAtom(display, "_NET_WM_STATE_MAXIMIZED_VERT", False);
	window->state.x11.fullscreen = XInternAtom(display, "_NET_WM_STATE_FULLSCREEN", False);
	window->state.x11.xdnd_aware = XInternAtom(display, "XdndAware", False);
	window->state.x11.xdnd_enter = XInternAtom(display, "XdndEnter", False);
	window->state.x11.xdnd_position = XInternAtom(display, "XdndPosition", False);
	window->state.x11.xdnd_status = XInternAtom(display, "XdndStatus", False);
	window->state.x11.xdnd_leave = XInternAtom(display, "XdndLeave", False);
	window->state.x11.xdnd_drop = XInternAtom(display, "XdndDrop", False);
	window->state.x11.xdnd_finished = XInternAtom(display, "XdndFinished", False);
	window->state.x11.xdnd_selection = XInternAtom(display, "XdndSelection", False);
	window->state.x11.xdnd_type_list = XInternAtom(display, "XdndTypeList", False);
	window->state.x11.xdnd_action_copy = XInternAtom(display, "XdndActionCopy", False);
	window->state.x11.text_uri_list = XInternAtom(display, "text/uri-list", False);
	window->state.x11.xdnd_property = XInternAtom(display, "ONIX_XDND_DATA", False);
	unsigned long xdnd_version = 5;
	XChangeProperty(display, window->state.x11.window, window->state.x11.xdnd_aware, XA_ATOM, 32, PropModeReplace, (unsigned char *)&xdnd_version, 1);
	XSetWMProtocols(display, window->state.x11.window, &window->state.x11.wm_delete, 1);
	onix_x11_set_decorations(window, client_decorations);

	window->state.x11.im = XOpenIM(display, NULL, NULL, NULL);
	if (window->state.x11.im != NULL) {
		window->state.x11.ic = XCreateIC(
			window->state.x11.im,
			XNInputStyle,
			XIMPreeditNothing | XIMStatusNothing,
			XNClientWindow,
			window->state.x11.window,
			XNFocusWindow,
			window->state.x11.window,
			NULL);
		if (window->state.x11.ic != NULL) {
			unsigned long filter = 0;
			if (XGetICValues(window->state.x11.ic, XNFilterEvents, &filter, NULL) == NULL) {
				XSelectInput(display, window->state.x11.window, attrs.event_mask | filter);
			}
		}
	}

	XMapWindow(display, window->state.x11.window);
	XFlush(display);

	return 1;
}

static void onix_wayland_registry_global(void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version);
static void onix_wayland_registry_remove(void *data, struct wl_registry *registry, uint32_t name);
static void onix_wayland_wm_base_ping(void *data, struct xdg_wm_base *wm_base, uint32_t serial);
static void onix_wayland_xdg_surface_configure(void *data, struct xdg_surface *surface, uint32_t serial);
static void onix_wayland_toplevel_configure(void *data, struct xdg_toplevel *toplevel, int32_t width, int32_t height, struct wl_array *states);
static void onix_wayland_toplevel_close(void *data, struct xdg_toplevel *toplevel);
static void onix_wayland_seat_capabilities(void *data, struct wl_seat *seat, uint32_t capabilities);
static void onix_wayland_seat_name(void *data, struct wl_seat *seat, const char *name);
static void onix_wayland_pointer_enter(void *data, struct wl_pointer *pointer, uint32_t serial, struct wl_surface *surface, wl_fixed_t sx, wl_fixed_t sy);
static void onix_wayland_pointer_leave(void *data, struct wl_pointer *pointer, uint32_t serial, struct wl_surface *surface);
static void onix_wayland_pointer_motion(void *data, struct wl_pointer *pointer, uint32_t time, wl_fixed_t sx, wl_fixed_t sy);
static void onix_wayland_pointer_button(void *data, struct wl_pointer *pointer, uint32_t serial, uint32_t time, uint32_t button, uint32_t state);
static void onix_wayland_pointer_axis(void *data, struct wl_pointer *pointer, uint32_t time, uint32_t axis, wl_fixed_t value);
static void onix_wayland_keyboard_keymap(void *data, struct wl_keyboard *keyboard, uint32_t format, int fd, uint32_t size);
static void onix_wayland_keyboard_enter(void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface, struct wl_array *keys);
static void onix_wayland_keyboard_leave(void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface);
static void onix_wayland_keyboard_key(void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t time, uint32_t key, uint32_t state);
static void onix_wayland_keyboard_modifiers(void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t depressed, uint32_t latched, uint32_t locked, uint32_t group);
static void onix_wayland_keyboard_repeat_info(void *data, struct wl_keyboard *keyboard, int32_t rate, int32_t delay);
static void onix_wayland_text_input_enter(void *data, struct zwp_text_input_v3 *text_input, struct wl_surface *surface);
static void onix_wayland_text_input_leave(void *data, struct zwp_text_input_v3 *text_input, struct wl_surface *surface);
static void onix_wayland_text_input_preedit(void *data, struct zwp_text_input_v3 *text_input, const char *text, int32_t cursor_begin, int32_t cursor_end);
static void onix_wayland_text_input_commit(void *data, struct zwp_text_input_v3 *text_input, const char *text);
static void onix_wayland_text_input_delete(void *data, struct zwp_text_input_v3 *text_input, uint32_t before_length, uint32_t after_length);
static void onix_wayland_text_input_done(void *data, struct zwp_text_input_v3 *text_input, uint32_t serial);
static void onix_wayland_maybe_create_data_device(OnixNativeWindow *window);

static const struct wl_registry_listener onix_wayland_registry_listener = {
	.global = onix_wayland_registry_global,
	.global_remove = onix_wayland_registry_remove,
};

static const struct xdg_wm_base_listener onix_wayland_wm_base_listener = {
	.ping = onix_wayland_wm_base_ping,
};

static const struct xdg_surface_listener onix_wayland_xdg_surface_listener = {
	.configure = onix_wayland_xdg_surface_configure,
};

static const struct xdg_toplevel_listener onix_wayland_toplevel_listener = {
	.configure = onix_wayland_toplevel_configure,
	.close = onix_wayland_toplevel_close,
};

static const struct wl_seat_listener onix_wayland_seat_listener = {
	.capabilities = onix_wayland_seat_capabilities,
	.name = onix_wayland_seat_name,
};

static const struct wl_pointer_listener onix_wayland_pointer_listener = {
	.enter = onix_wayland_pointer_enter,
	.leave = onix_wayland_pointer_leave,
	.motion = onix_wayland_pointer_motion,
	.button = onix_wayland_pointer_button,
	.axis = onix_wayland_pointer_axis,
};

static const struct wl_keyboard_listener onix_wayland_keyboard_listener = {
	.keymap = onix_wayland_keyboard_keymap,
	.enter = onix_wayland_keyboard_enter,
	.leave = onix_wayland_keyboard_leave,
	.key = onix_wayland_keyboard_key,
	.modifiers = onix_wayland_keyboard_modifiers,
	.repeat_info = onix_wayland_keyboard_repeat_info,
};

static const struct zwp_text_input_v3_listener onix_wayland_text_input_listener = {
	.enter = onix_wayland_text_input_enter,
	.leave = onix_wayland_text_input_leave,
	.preedit_string = onix_wayland_text_input_preedit,
	.commit_string = onix_wayland_text_input_commit,
	.delete_surrounding_text = onix_wayland_text_input_delete,
	.done = onix_wayland_text_input_done,
};

static void onix_wayland_data_source_target(void *data, struct wl_data_source *source, const char *mime_type) {
	(void)data; (void)source; (void)mime_type;
}

static void onix_wayland_data_source_send(void *data, struct wl_data_source *source, const char *mime_type, int32_t fd) {
	(void)source; (void)mime_type;
	OnixNativeWindow *window = data;
	const char *text = window->clipboard_text != NULL ? window->clipboard_text : "";
	size_t remaining = strlen(text);
	while (remaining > 0) {
		ssize_t written = write(fd, text, remaining);
		if (written <= 0) break;
		text += written;
		remaining -= (size_t)written;
	}
	close(fd);
}

static void onix_wayland_data_source_cancelled(void *data, struct wl_data_source *source) {
	OnixNativeWindow *window = data;
	if (window->state.wayland.data_source == source) window->state.wayland.data_source = NULL;
	wl_data_source_destroy(source);
}

static void onix_wayland_data_source_drop(void *data, struct wl_data_source *source) { (void)data; (void)source; }
static void onix_wayland_data_source_finished(void *data, struct wl_data_source *source) { (void)data; (void)source; }
static void onix_wayland_data_source_action(void *data, struct wl_data_source *source, uint32_t action) { (void)data; (void)source; (void)action; }

static const struct wl_data_source_listener onix_wayland_data_source_listener = {
	.target = onix_wayland_data_source_target,
	.send = onix_wayland_data_source_send,
	.cancelled = onix_wayland_data_source_cancelled,
	.dnd_drop_performed = onix_wayland_data_source_drop,
	.dnd_finished = onix_wayland_data_source_finished,
	.action = onix_wayland_data_source_action,
};

static void onix_wayland_offer_mime(void *data, struct wl_data_offer *offer, const char *mime_type) {
	OnixNativeWindow *window = data;
	if (offer == window->state.wayland.pending_offer && strcmp(mime_type, "text/plain;charset=utf-8") == 0) window->state.wayland.pending_offer_utf8 = 1;
	if (offer == window->state.wayland.pending_offer && strcmp(mime_type, "text/uri-list") == 0) window->state.wayland.pending_offer_uri = 1;
}
static void onix_wayland_offer_source_actions(void *data, struct wl_data_offer *offer, uint32_t actions) { (void)data; (void)offer; (void)actions; }
static void onix_wayland_offer_action(void *data, struct wl_data_offer *offer, uint32_t action) { (void)data; (void)offer; (void)action; }
static const struct wl_data_offer_listener onix_wayland_data_offer_listener = {
	.offer = onix_wayland_offer_mime,
	.source_actions = onix_wayland_offer_source_actions,
	.action = onix_wayland_offer_action,
};

static void onix_wayland_data_offer(void *data, struct wl_data_device *device, struct wl_data_offer *offer) {
	(void)device;
	OnixNativeWindow *window = data;
	window->state.wayland.pending_offer = offer;
	window->state.wayland.pending_offer_utf8 = 0;
	window->state.wayland.pending_offer_uri = 0;
	wl_data_offer_add_listener(offer, &onix_wayland_data_offer_listener, window);
}
static void onix_wayland_data_enter(void *data, struct wl_data_device *device, uint32_t serial, struct wl_surface *surface, wl_fixed_t x, wl_fixed_t y, struct wl_data_offer *offer) {
	(void)device;
	OnixNativeWindow *window = data;
	if (surface != window->state.wayland.surface) return;
	window->state.wayland.dnd_offer = offer;
	window->state.wayland.dnd_offer_uri = offer == window->state.wayland.pending_offer && window->state.wayland.pending_offer_uri;
	window->state.wayland.dnd_serial = serial;
	window->state.wayland.dnd_x = wl_fixed_to_double(x);
	window->state.wayland.dnd_y = wl_fixed_to_double(y);
	if (offer != NULL) {
		wl_data_offer_accept(offer, serial, window->state.wayland.dnd_offer_uri ? "text/uri-list" : NULL);
		if (wl_proxy_get_version((struct wl_proxy *)offer) >= 3) {
			uint32_t action = window->state.wayland.dnd_offer_uri ? WL_DATA_DEVICE_MANAGER_DND_ACTION_COPY : WL_DATA_DEVICE_MANAGER_DND_ACTION_NONE;
			wl_data_offer_set_actions(offer, action, action);
		}
	}
}
static void onix_wayland_data_leave(void *data, struct wl_data_device *device) {
	(void)device;
	OnixNativeWindow *window = data;
	if (window->state.wayland.dnd_offer != NULL && window->state.wayland.dnd_offer != window->state.wayland.selection_offer) {
		wl_data_offer_destroy(window->state.wayland.dnd_offer);
	}
	if (window->state.wayland.pending_offer == window->state.wayland.dnd_offer) window->state.wayland.pending_offer = NULL;
	window->state.wayland.dnd_offer = NULL;
	window->state.wayland.dnd_offer_uri = 0;
}
static void onix_wayland_data_motion(void *data, struct wl_data_device *device, uint32_t time, wl_fixed_t x, wl_fixed_t y) {
	(void)device; (void)time;
	OnixNativeWindow *window = data;
	window->state.wayland.dnd_x = wl_fixed_to_double(x);
	window->state.wayland.dnd_y = wl_fixed_to_double(y);
}
static void onix_wayland_data_drop(void *data, struct wl_data_device *device) {
	(void)device;
	OnixNativeWindow *window = data;
	struct wl_data_offer *offer = window->state.wayland.dnd_offer;
	if (offer == NULL || !window->state.wayland.dnd_offer_uri || window->state.wayland.dnd_fd >= 0) return;
	int fds[2];
	if (pipe(fds) != 0) return;
	if (fcntl(fds[0], F_SETFL, O_NONBLOCK) != 0) {
		close(fds[0]);
		close(fds[1]);
		return;
	}
	window->state.wayland.dnd_fd = fds[0];
	window->state.wayland.dnd_length = 0;
	window->state.wayland.dnd_capacity = 0;
	free(window->state.wayland.dnd_buffer);
	window->state.wayland.dnd_buffer = NULL;
	wl_data_offer_receive(offer, "text/uri-list", fds[1]);
	close(fds[1]);
	wl_display_flush(window->state.wayland.display);
}

static void onix_wayland_emit_drop_uris(OnixNativeWindow *window) {
	char *cursor = window->state.wayland.dnd_buffer;
	char *end = cursor + window->state.wayland.dnd_length;
	while (cursor != NULL && cursor < end) {
		char *line = cursor;
		char *newline = memchr(cursor, '\n', (size_t)(end - cursor));
		if (newline == NULL) {
			cursor = end;
		} else {
			*newline = '\0';
			cursor = newline + 1;
		}
		if (line[0] == '#') continue;
		size_t length = strlen(line);
		while (length > 0 && line[length - 1] == '\r') line[--length] = '\0';
		if (length > 0) onix_push_drop_event(window, window->state.wayland.dnd_x, window->state.wayland.dnd_y, line, (int)length);
	}
}

static void onix_wayland_poll_drop(OnixNativeWindow *window) {
	int fd = window->state.wayland.dnd_fd;
	if (fd < 0) return;
	struct pollfd pfd = {.fd = fd, .events = POLLIN};
	if (poll(&pfd, 1, 0) <= 0) return;
	for (;;) {
		char chunk[4096];
		ssize_t count = read(fd, chunk, sizeof(chunk));
		if (count > 0) {
			size_t required = window->state.wayland.dnd_length + (size_t)count + 1;
			if (required > 1 << 20) {
				close(fd);
				window->state.wayland.dnd_fd = -1;
				free(window->state.wayland.dnd_buffer);
				window->state.wayland.dnd_buffer = NULL;
				return;
			}
			if (required > window->state.wayland.dnd_capacity) {
				size_t capacity = window->state.wayland.dnd_capacity == 0 ? 4096 : window->state.wayland.dnd_capacity;
				while (capacity < required) capacity *= 2;
				char *buffer = realloc(window->state.wayland.dnd_buffer, capacity);
				if (buffer == NULL) {
					close(fd);
					window->state.wayland.dnd_fd = -1;
					free(window->state.wayland.dnd_buffer);
					window->state.wayland.dnd_buffer = NULL;
					return;
				}
				window->state.wayland.dnd_buffer = buffer;
				window->state.wayland.dnd_capacity = capacity;
			}
			memcpy(window->state.wayland.dnd_buffer + window->state.wayland.dnd_length, chunk, (size_t)count);
			window->state.wayland.dnd_length += (size_t)count;
			window->state.wayland.dnd_buffer[window->state.wayland.dnd_length] = '\0';
			continue;
		}
		if (count == 0) {
			close(fd);
			window->state.wayland.dnd_fd = -1;
			if (window->state.wayland.dnd_buffer != NULL) onix_wayland_emit_drop_uris(window);
			if (window->state.wayland.dnd_offer != NULL) {
				struct wl_data_offer *offer = window->state.wayland.dnd_offer;
				if (wl_proxy_get_version((struct wl_proxy *)offer) >= 3) wl_data_offer_finish(offer);
				if (offer != window->state.wayland.selection_offer) wl_data_offer_destroy(offer);
				if (window->state.wayland.pending_offer == offer) window->state.wayland.pending_offer = NULL;
				window->state.wayland.dnd_offer = NULL;
			}
			free(window->state.wayland.dnd_buffer);
			window->state.wayland.dnd_buffer = NULL;
			window->state.wayland.dnd_length = 0;
			window->state.wayland.dnd_capacity = 0;
			return;
		}
		if (errno == EAGAIN || errno == EWOULDBLOCK) return;
		close(fd);
		window->state.wayland.dnd_fd = -1;
		return;
	}
}
static void onix_wayland_data_selection(void *data, struct wl_data_device *device, struct wl_data_offer *offer) {
	(void)device;
	OnixNativeWindow *window = data;
	if (window->state.wayland.selection_offer != NULL && window->state.wayland.selection_offer != offer) wl_data_offer_destroy(window->state.wayland.selection_offer);
	window->state.wayland.selection_offer = offer;
	window->state.wayland.selection_offer_utf8 = offer == window->state.wayland.pending_offer && window->state.wayland.pending_offer_utf8;
	if (offer == window->state.wayland.pending_offer) window->state.wayland.pending_offer = NULL;
}

static const struct wl_data_device_listener onix_wayland_data_device_listener = {
	.data_offer = onix_wayland_data_offer,
	.enter = onix_wayland_data_enter,
	.leave = onix_wayland_data_leave,
	.motion = onix_wayland_data_motion,
	.drop = onix_wayland_data_drop,
	.selection = onix_wayland_data_selection,
};

static void onix_wayland_maybe_create_data_device(OnixNativeWindow *window) {
	if (window->state.wayland.data_device != NULL || window->state.wayland.data_device_manager == NULL || window->state.wayland.seat == NULL) return;
	window->state.wayland.data_device = wl_data_device_manager_get_data_device(window->state.wayland.data_device_manager, window->state.wayland.seat);
	if (window->state.wayland.data_device != NULL) wl_data_device_add_listener(window->state.wayland.data_device, &onix_wayland_data_device_listener, window);
}

static uint32_t onix_wayland_commit_text_input(OnixNativeWindow *window) {
	if (window == NULL || window->state.wayland.text_input == NULL) {
		return 0;
	}
	zwp_text_input_v3_commit(window->state.wayland.text_input);
	window->state.wayland.text_input_commit_count++;
	return window->state.wayland.text_input_commit_count;
}

static void onix_wayland_apply_text_input_config(OnixNativeWindow *window) {
	if (window == NULL || window->state.wayland.text_input == NULL || !window->state.wayland.text_input_entered) {
		return;
	}
	if (window->state.wayland.xkb_compose_state != NULL) {
		xkb_compose_state_reset(window->state.wayland.xkb_compose_state);
	}
	onix_clear_wayland_text_pending(window);
	if (!window->text_input_active) {
		zwp_text_input_v3_disable(window->state.wayland.text_input);
		onix_wayland_commit_text_input(window);
		window->state.wayland.text_input_session_serial = 0;
		onix_push_text_event(window, ONIX_EVENT_TEXT_COMPOSITION, 0, "", 0);
		return;
	}
	zwp_text_input_v3_enable(window->state.wayland.text_input);
	uint32_t hint = window->text_input_multiline ? ZWP_TEXT_INPUT_V3_CONTENT_HINT_MULTILINE : ZWP_TEXT_INPUT_V3_CONTENT_HINT_NONE;
	uint32_t purpose = window->text_input_kind == 0 ? ZWP_TEXT_INPUT_V3_CONTENT_PURPOSE_NORMAL : ZWP_TEXT_INPUT_V3_CONTENT_PURPOSE_NUMBER;
	zwp_text_input_v3_set_content_type(window->state.wayland.text_input, hint, purpose);
	window->state.wayland.text_input_session_serial = onix_wayland_commit_text_input(window);
}

static void onix_wayland_maybe_create_text_input(OnixNativeWindow *window) {
	if (window == NULL || window->state.wayland.text_input != NULL || window->state.wayland.text_input_manager == NULL || window->state.wayland.seat == NULL) {
		return;
	}
	window->state.wayland.text_input = zwp_text_input_manager_v3_get_text_input(window->state.wayland.text_input_manager, window->state.wayland.seat);
	if (window->state.wayland.text_input != NULL) {
		zwp_text_input_v3_add_listener(window->state.wayland.text_input, &onix_wayland_text_input_listener, window);
	}
}

static void onix_wayland_text_input_enter(void *data, struct zwp_text_input_v3 *text_input, struct wl_surface *surface) {
	(void)text_input;
	OnixNativeWindow *window = data;
	window->state.wayland.text_input_entered = surface == window->state.wayland.surface;
	if (window->state.wayland.text_input_entered) {
		onix_wayland_apply_text_input_config(window);
	}
}

static void onix_wayland_text_input_leave(void *data, struct zwp_text_input_v3 *text_input, struct wl_surface *surface) {
	(void)text_input;
	(void)surface;
	OnixNativeWindow *window = data;
	window->state.wayland.text_input_entered = 0;
	window->state.wayland.text_input_session_serial = 0;
	onix_clear_wayland_text_pending(window);
	onix_push_text_event(window, ONIX_EVENT_TEXT_COMPOSITION, 0, "", 0);
}

static void onix_wayland_text_input_preedit(void *data, struct zwp_text_input_v3 *text_input, const char *text, int32_t cursor_begin, int32_t cursor_end) {
	(void)text_input;
	(void)cursor_begin;
	(void)cursor_end;
	OnixNativeWindow *window = data;
	onix_replace_string(&window->state.wayland.pending_preedit, text);
	window->state.wayland.has_pending_preedit = 1;
}

static void onix_wayland_text_input_commit(void *data, struct zwp_text_input_v3 *text_input, const char *text) {
	(void)text_input;
	OnixNativeWindow *window = data;
	onix_replace_string(&window->state.wayland.pending_commit, text);
	window->state.wayland.has_pending_commit = 1;
}

static void onix_wayland_text_input_delete(void *data, struct zwp_text_input_v3 *text_input, uint32_t before_length, uint32_t after_length) {
	(void)data;
	(void)text_input;
	(void)before_length;
	(void)after_length;
}

static void onix_wayland_text_input_done(void *data, struct zwp_text_input_v3 *text_input, uint32_t serial) {
	(void)text_input;
	OnixNativeWindow *window = data;
	if (!window->text_input_active || serial != window->state.wayland.text_input_session_serial) {
		onix_clear_wayland_text_pending(window);
		return;
	}
	if (window->state.wayland.has_pending_commit && window->state.wayland.pending_commit != NULL) {
		if (!window->text_input_multiline && onix_is_line_break_text(window->state.wayland.pending_commit)) {
			onix_push_text_command(window, ONIX_TEXT_COMMAND_ENTER, 1);
		} else if (window->state.wayland.pending_commit[0] != '\0') {
			onix_push_text_event(window, ONIX_EVENT_TEXT_INPUT, 0, window->state.wayland.pending_commit, (int)strlen(window->state.wayland.pending_commit));
		}
	}
	if (window->state.wayland.has_pending_preedit && window->state.wayland.pending_preedit != NULL && window->state.wayland.pending_preedit[0] != '\0') {
		onix_push_text_event(window, ONIX_EVENT_TEXT_COMPOSITION, 1, window->state.wayland.pending_preedit, (int)strlen(window->state.wayland.pending_preedit));
	} else {
		onix_push_text_event(window, ONIX_EVENT_TEXT_COMPOSITION, 0, "", 0);
	}
	onix_clear_wayland_text_pending(window);
}

static void onix_set_mouse_capture_state(OnixNativeWindow *window, int captured) {
	if (window->mouse_captured == captured) return;
	window->mouse_captured = captured;
	onix_push_event(window, ONIX_EVENT_MOUSE_CAPTURE, captured, 0, 0, 0);
}

static void onix_wayland_update_cursor(OnixNativeWindow *window) {
	OnixWaylandState *state = &window->state.wayland;
	if (!state->pointer_entered || state->pointer == NULL) return;
	if (window->mouse_captured || window->cursor == ONIX_CURSOR_NONE) {
		wl_pointer_set_cursor(state->pointer, state->pointer_serial, NULL, 0, 0);
		return;
	}
	if (state->shm == NULL || state->compositor == NULL) return;
	if (state->cursor_theme == NULL) state->cursor_theme = wl_cursor_theme_load(NULL, 24, state->shm);
	if (state->cursor_surface == NULL) state->cursor_surface = wl_compositor_create_surface(state->compositor);
	if (state->cursor_theme == NULL || state->cursor_surface == NULL) return;
	static const char *names[] = {"left_ptr", "xterm", "hand2", "crosshair", "fleur", "sb_h_double_arrow", "sb_v_double_arrow", "size_bdiag", "size_fdiag", "openhand", "closedhand", "not-allowed", "watch", "left_ptr_watch", "question_arrow", "left_ptr", "vertical-text", "all-scroll", "context-menu", "cell", "copy", "alias", "zoom-in", "zoom-out"};
	struct wl_cursor *cursor = wl_cursor_theme_get_cursor(state->cursor_theme, names[window->cursor]);
	if (cursor == NULL) cursor = wl_cursor_theme_get_cursor(state->cursor_theme, "left_ptr");
	if (cursor == NULL || cursor->image_count == 0) return;
	struct wl_cursor_image *image = cursor->images[0];
	struct wl_buffer *buffer = wl_cursor_image_get_buffer(image);
	if (buffer == NULL) return;
	wl_pointer_set_cursor(state->pointer, state->pointer_serial, state->cursor_surface, image->hotspot_x, image->hotspot_y);
	wl_surface_attach(state->cursor_surface, buffer, 0, 0);
	wl_surface_damage(state->cursor_surface, 0, 0, image->width, image->height);
	wl_surface_commit(state->cursor_surface);
}

void onix_window_set_cursor(OnixNativeWindow *window, int cursor) {
	if (window == NULL || cursor < 0 || cursor >= ONIX_CURSOR_COUNT) return;
	window->cursor = cursor;
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		onix_wayland_update_cursor(window);
		wl_display_flush(window->state.wayland.display);
	} else if (window->backend == ONIX_BACKEND_X11) {
		OnixX11State *state = &window->state.x11;
		static const unsigned int shapes[] = {XC_left_ptr, XC_xterm, XC_hand2, XC_crosshair, XC_fleur, XC_sb_h_double_arrow, XC_sb_v_double_arrow, XC_bottom_left_corner, XC_bottom_right_corner, XC_hand1, XC_hand2, XC_X_cursor, XC_watch, XC_watch, XC_question_arrow, 0, XC_xterm, XC_fleur, XC_left_ptr, XC_plus, XC_hand2, XC_hand2, XC_plus, XC_circle};
		if (state->cursors[cursor] == None) {
			if (cursor == ONIX_CURSOR_NONE) {
				char data = 0;
				Pixmap bitmap = XCreateBitmapFromData(state->display, state->window, &data, 1, 1);
				XColor color = {0};
				state->cursors[cursor] = XCreatePixmapCursor(state->display, bitmap, bitmap, &color, &color, 0, 0);
				XFreePixmap(state->display, bitmap);
			} else state->cursors[cursor] = XCreateFontCursor(state->display, shapes[cursor]);
		}
		XDefineCursor(state->display, state->window, state->cursors[cursor]);
		XFlush(state->display);
	}
}

static void onix_wayland_relative_motion(void *data, struct zwp_relative_pointer_v1 *pointer,
		uint32_t hi, uint32_t lo, wl_fixed_t dx, wl_fixed_t dy, wl_fixed_t raw_dx, wl_fixed_t raw_dy) {
	(void)pointer; (void)hi; (void)lo; (void)dx; (void)dy;
	OnixNativeWindow *window = data;
	if (window->mouse_captured) onix_push_event(window, ONIX_EVENT_MOUSE_DELTA, 0, 0, wl_fixed_to_double(raw_dx), wl_fixed_to_double(raw_dy));
}

static const struct zwp_relative_pointer_v1_listener onix_relative_pointer_listener = {
	.relative_motion = onix_wayland_relative_motion,
};

static void onix_wayland_pointer_locked(void *data, struct zwp_locked_pointer_v1 *pointer) {
	(void)pointer;
	OnixNativeWindow *window = data;
	onix_set_mouse_capture_state(window, 1);
	onix_wayland_update_cursor(window);
}

static void onix_wayland_pointer_unlocked(void *data, struct zwp_locked_pointer_v1 *pointer) {
	(void)pointer;
	onix_window_set_mouse_captured(data, 0);
}

static const struct zwp_locked_pointer_v1_listener onix_locked_pointer_listener = {
	.locked = onix_wayland_pointer_locked,
	.unlocked = onix_wayland_pointer_unlocked,
};

int onix_window_set_mouse_captured(OnixNativeWindow *window, int captured) {
	if (window == NULL) return 0;
	captured = captured != 0;
	if (window->backend == ONIX_BACKEND_X11) {
		OnixX11State *state = &window->state.x11;
		if (state->display == NULL || state->window == 0) return 0;
		if (captured == window->mouse_captured) return 1;
		unsigned char mask[XIMaskLen(XI_RawMotion)] = {0};
		XIEventMask events = {.deviceid = XIAllMasterDevices, .mask_len = sizeof(mask), .mask = mask};
		if (captured) {
			if (!state->focused) return 0;
			int event, error, major = 2, minor = 1;
			if (!XQueryExtension(state->display, "XInputExtension", &state->xi_opcode, &event, &error) ||
				XIQueryVersion(state->display, &major, &minor) != Success || (major == 2 && minor < 1)) return 0;
			if (state->hidden_cursor == None) {
				char pixels[1] = {0};
				Pixmap bitmap = XCreateBitmapFromData(state->display, state->window, pixels, 1, 1);
				XColor color = {0};
				state->hidden_cursor = XCreatePixmapCursor(state->display, bitmap, bitmap, &color, &color, 0, 0);
				XFreePixmap(state->display, bitmap);
			}
			if (XGrabPointer(state->display, state->window, False, ButtonPressMask | ButtonReleaseMask,
				GrabModeAsync, GrabModeAsync, state->window, state->hidden_cursor, CurrentTime) != GrabSuccess) return 0;
			XISetMask(mask, XI_RawMotion);
		} else {
			XUngrabPointer(state->display, CurrentTime);
		}
		XISelectEvents(state->display, DefaultRootWindow(state->display), &events, 1);
		XFlush(state->display);
		onix_set_mouse_capture_state(window, captured);
		return 1;
	}
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		OnixWaylandState *state = &window->state.wayland;
		if (!captured) {
			if (state->locked_pointer != NULL) zwp_locked_pointer_v1_destroy(state->locked_pointer);
			if (state->relative_pointer != NULL) zwp_relative_pointer_v1_destroy(state->relative_pointer);
			state->locked_pointer = NULL;
			state->relative_pointer = NULL;
			onix_set_mouse_capture_state(window, 0);
			onix_wayland_update_cursor(window);
			return 1;
		}
		if (state->locked_pointer != NULL) return 1;
		if (state->pointer == NULL || state->pointer_constraints == NULL || state->relative_pointer_manager == NULL ||
			state->shm == NULL || state->surface == NULL) return 0;
		if (state->cursor_theme == NULL) state->cursor_theme = wl_cursor_theme_load(NULL, 24, state->shm);
		if (state->cursor_theme == NULL) return 0;
		if (state->cursor_surface == NULL) state->cursor_surface = wl_compositor_create_surface(state->compositor);
		state->relative_pointer = zwp_relative_pointer_manager_v1_get_relative_pointer(state->relative_pointer_manager, state->pointer);
		zwp_relative_pointer_v1_add_listener(state->relative_pointer, &onix_relative_pointer_listener, window);
		state->locked_pointer = zwp_pointer_constraints_v1_lock_pointer(state->pointer_constraints, state->surface,
			state->pointer, NULL, ZWP_POINTER_CONSTRAINTS_V1_LIFETIME_ONESHOT);
		zwp_locked_pointer_v1_add_listener(state->locked_pointer, &onix_locked_pointer_listener, window);
		wl_surface_commit(state->surface);
		wl_display_flush(state->display);
		return 1;
	}
	return 0;
}

static void onix_wayland_registry_global(void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
	OnixNativeWindow *window = data;
	if (strcmp(interface, zwp_relative_pointer_manager_v1_interface.name) == 0) {
		window->state.wayland.relative_pointer_manager = wl_registry_bind(registry, name, &zwp_relative_pointer_manager_v1_interface, 1);
		return;
	}
	if (strcmp(interface, zwp_pointer_constraints_v1_interface.name) == 0) {
		window->state.wayland.pointer_constraints = wl_registry_bind(registry, name, &zwp_pointer_constraints_v1_interface, 1);
		return;
	}
	if (strcmp(interface, wl_shm_interface.name) == 0) {
		window->state.wayland.shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
		return;
	}
	if (strcmp(interface, wl_compositor_interface.name) == 0) {
		uint32_t bind_version = version < 4 ? version : 4;
		window->state.wayland.compositor = wl_registry_bind(registry, name, &wl_compositor_interface, bind_version);
		return;
	}
	if (strcmp(interface, wl_seat_interface.name) == 0) {
		uint32_t bind_version = version < 5 ? version : 5;
		window->state.wayland.seat = wl_registry_bind(registry, name, &wl_seat_interface, bind_version);
		onix_wayland_maybe_create_text_input(window);
		onix_wayland_maybe_create_data_device(window);
		return;
	}
	if (strcmp(interface, wl_data_device_manager_interface.name) == 0) {
		window->state.wayland.data_device_manager = wl_registry_bind(registry, name, &wl_data_device_manager_interface, version < 3 ? version : 3);
		onix_wayland_maybe_create_data_device(window);
		return;
	}
	if (strcmp(interface, zwp_text_input_manager_v3_interface.name) == 0) {
		uint32_t bind_version = version < 1 ? version : 1;
		window->state.wayland.text_input_manager = wl_registry_bind(registry, name, &zwp_text_input_manager_v3_interface, bind_version);
		onix_wayland_maybe_create_text_input(window);
		return;
	}
	if (strcmp(interface, xdg_wm_base_interface.name) == 0) {
		uint32_t bind_version = version < 1 ? version : 1;
		window->state.wayland.wm_base = wl_registry_bind(registry, name, &xdg_wm_base_interface, bind_version);
		return;
	}
	if (strcmp(interface, zxdg_decoration_manager_v1_interface.name) == 0) {
		uint32_t bind_version = version < 1 ? version : 1;
		window->state.wayland.decoration_manager = wl_registry_bind(registry, name, &zxdg_decoration_manager_v1_interface, bind_version);
	}
}

static void onix_wayland_registry_remove(void *data, struct wl_registry *registry, uint32_t name) {
	(void)data;
	(void)registry;
	(void)name;
}

static void onix_wayland_wm_base_ping(void *data, struct xdg_wm_base *wm_base, uint32_t serial) {
	(void)data;
	xdg_wm_base_pong(wm_base, serial);
}

static void onix_wayland_xdg_surface_configure(void *data, struct xdg_surface *surface, uint32_t serial) {
	OnixNativeWindow *window = data;
	xdg_surface_ack_configure(surface, serial);
	window->state.wayland.configured = 1;
	wl_surface_commit(window->state.wayland.surface);
}

static void onix_wayland_toplevel_configure(void *data, struct xdg_toplevel *toplevel, int32_t width, int32_t height, struct wl_array *states) {
	(void)toplevel;
	OnixNativeWindow *window = data;
	window->maximized = 0;
	window->fullscreen = 0;
	uint32_t *state;
	wl_array_for_each(state, states) {
		if (*state == XDG_TOPLEVEL_STATE_MAXIMIZED) {
			window->maximized = 1;
		}
		if (*state == XDG_TOPLEVEL_STATE_FULLSCREEN) {
			window->fullscreen = 1;
		}
	}
	onix_push_event(window, ONIX_EVENT_WINDOW_STATE, window->maximized, 0, 0, 0);
	if (width <= 0 || height <= 0) {
		return;
	}
	window->window_width = width;
	window->window_height = height;
	window->framebuffer_width = width;
	window->framebuffer_height = height;
	onix_push_event(window, ONIX_EVENT_WINDOW_SIZE, width, height, 0, 0);
	onix_push_event(window, ONIX_EVENT_FRAMEBUFFER_SIZE, width, height, 0, 0);
}

static void onix_wayland_toplevel_close(void *data, struct xdg_toplevel *toplevel) {
	(void)toplevel;
	OnixNativeWindow *window = data;
	window->should_close = 1;
	onix_push_event(window, ONIX_EVENT_CLOSE, 0, 0, 0, 0);
}

static void onix_wayland_seat_capabilities(void *data, struct wl_seat *seat, uint32_t capabilities) {
	OnixNativeWindow *window = data;
	if ((capabilities & WL_SEAT_CAPABILITY_POINTER) != 0) {
		if (window->state.wayland.pointer == NULL) {
			window->state.wayland.pointer = wl_seat_get_pointer(seat);
			wl_pointer_add_listener(window->state.wayland.pointer, &onix_wayland_pointer_listener, window);
		}
	} else if (window->state.wayland.pointer != NULL) {
		onix_window_set_mouse_captured(window, 0);
		wl_pointer_destroy(window->state.wayland.pointer);
		window->state.wayland.pointer = NULL;
	}
	if ((capabilities & WL_SEAT_CAPABILITY_KEYBOARD) != 0) {
		if (window->state.wayland.keyboard == NULL) {
			window->state.wayland.keyboard = wl_seat_get_keyboard(seat);
			wl_keyboard_add_listener(window->state.wayland.keyboard, &onix_wayland_keyboard_listener, window);
		}
	} else if (window->state.wayland.keyboard != NULL) {
		wl_keyboard_destroy(window->state.wayland.keyboard);
		window->state.wayland.keyboard = NULL;
	}
}

static void onix_wayland_seat_name(void *data, struct wl_seat *seat, const char *name) {
	(void)data;
	(void)seat;
	(void)name;
}

static void onix_wayland_pointer_enter(void *data, struct wl_pointer *pointer, uint32_t serial, struct wl_surface *surface, wl_fixed_t sx, wl_fixed_t sy) {
	(void)pointer;
	(void)serial;
	(void)surface;
	OnixNativeWindow *window = data;
	window->state.wayland.last_serial = serial;
	window->state.wayland.mouse_x = wl_fixed_to_double(sx);
	window->state.wayland.mouse_y = wl_fixed_to_double(sy);
	window->state.wayland.pointer_serial = serial;
	window->state.wayland.pointer_entered = 1;
	onix_wayland_update_cursor(window);
	onix_push_event(window, ONIX_EVENT_MOUSE_MOVE, 0, 0, wl_fixed_to_double(sx), wl_fixed_to_double(sy));
}

static void onix_wayland_pointer_leave(void *data, struct wl_pointer *pointer, uint32_t serial, struct wl_surface *surface) {
	OnixNativeWindow *window = data;
	window->state.wayland.pointer_entered = 0;
	onix_window_set_mouse_captured(window, 0);
	(void)pointer;
	(void)serial;
	(void)surface;
}

static void onix_wayland_pointer_motion(void *data, struct wl_pointer *pointer, uint32_t time, wl_fixed_t sx, wl_fixed_t sy) {
	(void)pointer;
	(void)time;
	OnixNativeWindow *window = data;
	if (window->mouse_captured) return;
	window->state.wayland.mouse_x = wl_fixed_to_double(sx);
	window->state.wayland.mouse_y = wl_fixed_to_double(sy);
	onix_push_event(window, ONIX_EVENT_MOUSE_MOVE, 0, 0, window->state.wayland.mouse_x, window->state.wayland.mouse_y);
}

static void onix_wayland_pointer_button(void *data, struct wl_pointer *pointer, uint32_t serial, uint32_t time, uint32_t button, uint32_t state) {
	(void)pointer;
	(void)serial;
	(void)time;
	OnixNativeWindow *window = data;
	window->state.wayland.last_serial = serial;
	int mapped = onix_mouse_button_from_linux(button);
	if (mapped < 0) {
		return;
	}
	if (state == WL_POINTER_BUTTON_STATE_PRESSED) {
		onix_push_event(window, ONIX_EVENT_MOUSE_DOWN, mapped, 0, window->state.wayland.mouse_x, window->state.wayland.mouse_y);
		return;
	}
	if (state == WL_POINTER_BUTTON_STATE_RELEASED) {
		onix_push_event(window, ONIX_EVENT_MOUSE_UP, mapped, 0, window->state.wayland.mouse_x, window->state.wayland.mouse_y);
	}
}

static void onix_wayland_pointer_axis(void *data, struct wl_pointer *pointer, uint32_t time, uint32_t axis, wl_fixed_t value) {
	(void)pointer;
	(void)time;
	OnixNativeWindow *window = data;
	double amount = wl_fixed_to_double(value) / 10.0;
	if (axis == WL_POINTER_AXIS_VERTICAL_SCROLL) {
		onix_push_event(window, ONIX_EVENT_MOUSE_WHEEL, 0, 0, 0, -amount);
		return;
	}
	if (axis == WL_POINTER_AXIS_HORIZONTAL_SCROLL) {
		onix_push_event(window, ONIX_EVENT_MOUSE_WHEEL, 0, 0, -amount, 0);
	}
}

static void onix_wayland_keyboard_keymap(void *data, struct wl_keyboard *keyboard, uint32_t format, int fd, uint32_t size) {
	(void)keyboard;
	OnixNativeWindow *window = data;
	if (format != WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1) {
		close(fd);
		return;
	}
	char *map = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
	close(fd);
	if (map == MAP_FAILED) {
		return;
	}
	struct xkb_keymap *keymap = xkb_keymap_new_from_string(window->state.wayland.xkb_context, map, XKB_KEYMAP_FORMAT_TEXT_V1, XKB_KEYMAP_COMPILE_NO_FLAGS);
	munmap(map, size);
	if (keymap == NULL) {
		return;
	}
	struct xkb_state *state = xkb_state_new(keymap);
	if (state == NULL) {
		xkb_keymap_unref(keymap);
		return;
	}
	if (window->state.wayland.xkb_state != NULL) {
		xkb_state_unref(window->state.wayland.xkb_state);
	}
	if (window->state.wayland.xkb_keymap != NULL) {
		xkb_keymap_unref(window->state.wayland.xkb_keymap);
	}
	window->state.wayland.xkb_keymap = keymap;
	window->state.wayland.xkb_state = state;
}

static void onix_wayland_keyboard_enter(void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface, struct wl_array *keys) {
	(void)keyboard;
	OnixNativeWindow *window = data;
	window->state.wayland.last_serial = serial;
	(void)surface;
	(void)keys;
}

static void onix_wayland_keyboard_leave(void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface) {
	(void)keyboard;
	(void)serial;
	(void)surface;
	OnixNativeWindow *window = data;
	onix_release_modifiers(window, 0xff);
	onix_window_set_mouse_captured(window, 0);
	if (window->state.wayland.xkb_compose_state != NULL) {
		xkb_compose_state_reset(window->state.wayland.xkb_compose_state);
	}
}

static void onix_wayland_push_keyboard_text(OnixNativeWindow *window, uint32_t key, xkb_keysym_t sym) {
	if (window->state.wayland.xkb_compose_state != NULL && xkb_compose_state_feed(window->state.wayland.xkb_compose_state, sym) == XKB_COMPOSE_FEED_ACCEPTED) {
		switch (xkb_compose_state_get_status(window->state.wayland.xkb_compose_state)) {
		case XKB_COMPOSE_COMPOSING:
			return;
		case XKB_COMPOSE_CANCELLED:
			xkb_compose_state_reset(window->state.wayland.xkb_compose_state);
			return;
		case XKB_COMPOSE_COMPOSED: {
			char text[64];
			int text_len = xkb_compose_state_get_utf8(window->state.wayland.xkb_compose_state, text, sizeof(text));
			if (text_len > 0 && text_len < (int)sizeof(text)) {
				onix_push_text_input_event(window, text, text_len);
			} else if (text_len >= (int)sizeof(text)) {
				char *large = malloc((size_t)text_len + 1);
				if (large != NULL) {
					int large_len = xkb_compose_state_get_utf8(window->state.wayland.xkb_compose_state, large, (size_t)text_len + 1);
					if (large_len > 0) {
						onix_push_text_input_event(window, large, large_len);
					}
					free(large);
				}
			}
			xkb_compose_state_reset(window->state.wayland.xkb_compose_state);
			return;
		}
		case XKB_COMPOSE_NOTHING:
			break;
		}
	}

	char text[64];
	int text_len = xkb_state_key_get_utf8(window->state.wayland.xkb_state, key + 8, text, sizeof(text));
	if (text_len > 0 && text_len < (int)sizeof(text)) {
		onix_push_text_input_event(window, text, text_len);
	} else if (text_len >= (int)sizeof(text)) {
		char *large = malloc((size_t)text_len + 1);
		if (large != NULL) {
			int large_len = xkb_state_key_get_utf8(window->state.wayland.xkb_state, key + 8, large, (size_t)text_len + 1);
			if (large_len > 0) {
				onix_push_text_input_event(window, large, large_len);
			}
			free(large);
		}
	}
}

static void onix_wayland_keyboard_key(void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t time, uint32_t key, uint32_t state) {
	(void)keyboard;
	(void)serial;
	(void)time;
	OnixNativeWindow *window = data;
	window->state.wayland.last_serial = serial;
	if (window->state.wayland.xkb_state == NULL) {
		return;
	}
	xkb_keysym_t sym = xkb_state_key_get_one_sym(window->state.wayland.xkb_state, key + 8);
	int mapped = onix_keysym_to_keycode(sym);
	if (state == WL_KEYBOARD_KEY_STATE_PRESSED) {
		if (window->text_input_active) {
			onix_wayland_push_keyboard_text(window, key, sym);
			if (mapped == ONIX_KEY_BACKSPACE) {
				onix_push_text_command(window, ONIX_TEXT_COMMAND_BACKSPACE, 1);
			} else if (mapped == ONIX_KEY_ENTER && window->state.wayland.text_input == NULL) {
				onix_push_text_command(window, ONIX_TEXT_COMMAND_ENTER, 1);
			}
		}
		if (mapped != 0) {
			onix_push_event(window, ONIX_EVENT_KEY_DOWN, mapped, 0, 0, 0);
		}
		return;
	}
	if (state == WL_KEYBOARD_KEY_STATE_RELEASED && mapped != 0) {
		onix_push_event(window, ONIX_EVENT_KEY_UP, mapped, 0, 0, 0);
	}
}

static void onix_wayland_keyboard_modifiers(void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t depressed, uint32_t latched, uint32_t locked, uint32_t group) {
	(void)data;
	(void)keyboard;
	(void)serial;
	OnixNativeWindow *window = data;
	if (window->state.wayland.xkb_state == NULL) {
		return;
	}
	xkb_state_update_mask(window->state.wayland.xkb_state, depressed, latched, locked, 0, 0, group);
	const char *names[] = {XKB_MOD_NAME_SHIFT, XKB_MOD_NAME_CTRL, XKB_MOD_NAME_ALT, XKB_MOD_NAME_LOGO};
	uint8_t active = 0;
	for (int index = 0; index < 4; index++) {
		if (xkb_state_mod_name_is_active(window->state.wayland.xkb_state, names[index], XKB_STATE_MODS_EFFECTIVE) > 0) active |= 1u << index;
	}
	onix_sync_modifiers(window, active);
}

static void onix_wayland_keyboard_repeat_info(void *data, struct wl_keyboard *keyboard, int32_t rate, int32_t delay) {
	(void)data;
	(void)keyboard;
	(void)rate;
	(void)delay;
}

static int onix_init_wayland(OnixNativeWindow *window, int width, int height, const char *title, int client_decorations) {
	struct wl_display *display = wl_display_connect(NULL);
	if (display == NULL) {
		onix_set_error("wl_display_connect failed");
		return 0;
	}

	window->backend = ONIX_BACKEND_WAYLAND;
	window->state.wayland.display = display;
	window->state.wayland.dnd_fd = -1;
	window->state.wayland.xkb_context = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
	if (window->state.wayland.xkb_context == NULL) {
		onix_set_error("xkb_context_new failed");
		return 0;
	}
	const char *locale = setlocale(LC_CTYPE, "");
	if (locale != NULL) {
		window->state.wayland.xkb_compose_table = xkb_compose_table_new_from_locale(window->state.wayland.xkb_context, locale, XKB_COMPOSE_COMPILE_NO_FLAGS);
		if (window->state.wayland.xkb_compose_table != NULL) {
			window->state.wayland.xkb_compose_state = xkb_compose_state_new(window->state.wayland.xkb_compose_table, XKB_COMPOSE_STATE_NO_FLAGS);
		}
	}

	window->state.wayland.registry = wl_display_get_registry(display);
	if (window->state.wayland.registry == NULL) {
		onix_set_error("wl_display_get_registry failed");
		return 0;
	}
	wl_registry_add_listener(window->state.wayland.registry, &onix_wayland_registry_listener, window);
	if (wl_display_roundtrip(display) < 0) {
		onix_set_error("wl_display_roundtrip failed while enumerating globals");
		return 0;
	}
	if (window->state.wayland.compositor == NULL || window->state.wayland.wm_base == NULL) {
		onix_set_error("Wayland compositor does not expose wl_compositor + xdg_wm_base");
		return 0;
	}
	if (window->state.wayland.seat != NULL) {
		wl_seat_add_listener(window->state.wayland.seat, &onix_wayland_seat_listener, window);
		if (wl_display_roundtrip(display) < 0) {
			onix_set_error("wl_display_roundtrip failed while binding seat");
			return 0;
		}
	}

	xdg_wm_base_add_listener(window->state.wayland.wm_base, &onix_wayland_wm_base_listener, window);

	window->state.wayland.surface = wl_compositor_create_surface(window->state.wayland.compositor);
	if (window->state.wayland.surface == NULL) {
		onix_set_error("wl_compositor_create_surface failed");
		return 0;
	}
	window->state.wayland.xdg_surface = xdg_wm_base_get_xdg_surface(window->state.wayland.wm_base, window->state.wayland.surface);
	if (window->state.wayland.xdg_surface == NULL) {
		onix_set_error("xdg_wm_base_get_xdg_surface failed");
		return 0;
	}
	xdg_surface_add_listener(window->state.wayland.xdg_surface, &onix_wayland_xdg_surface_listener, window);
	window->state.wayland.xdg_toplevel = xdg_surface_get_toplevel(window->state.wayland.xdg_surface);
	if (window->state.wayland.xdg_toplevel == NULL) {
		onix_set_error("xdg_surface_get_toplevel failed");
		return 0;
	}
	xdg_toplevel_add_listener(window->state.wayland.xdg_toplevel, &onix_wayland_toplevel_listener, window);
	if (window->state.wayland.decoration_manager != NULL) {
		window->state.wayland.toplevel_decoration = zxdg_decoration_manager_v1_get_toplevel_decoration(
			window->state.wayland.decoration_manager,
			window->state.wayland.xdg_toplevel);
		zxdg_toplevel_decoration_v1_set_mode(
			window->state.wayland.toplevel_decoration,
			client_decorations ? ZXDG_TOPLEVEL_DECORATION_V1_MODE_CLIENT_SIDE : ZXDG_TOPLEVEL_DECORATION_V1_MODE_SERVER_SIDE);
	}
	if (title != NULL && title[0] != '\0') {
		xdg_toplevel_set_title(window->state.wayland.xdg_toplevel, title);
	}
	xdg_toplevel_set_app_id(window->state.wayland.xdg_toplevel, "onix");
	wl_surface_commit(window->state.wayland.surface);

	while (!window->state.wayland.configured && !window->should_close) {
		if (wl_display_dispatch(display) < 0) {
			onix_set_error("wl_display_dispatch failed while waiting for initial configure");
			return 0;
		}
	}
	if (window->should_close) {
		onix_set_error("Wayland compositor closed the surface during startup");
		return 0;
	}

	if (window->window_width <= 0) {
		window->window_width = width;
	}
	if (window->window_height <= 0) {
		window->window_height = height;
	}
	window->framebuffer_width = window->window_width;
	window->framebuffer_height = window->window_height;

	return 1;
}

static void onix_x11_send_dnd_message(OnixNativeWindow *window, Window target, Atom type, long first, long second, long third, long fourth, long fifth) {
	XEvent event;
	memset(&event, 0, sizeof(event));
	event.xclient.type = ClientMessage;
	event.xclient.display = window->state.x11.display;
	event.xclient.window = target;
	event.xclient.message_type = type;
	event.xclient.format = 32;
	event.xclient.data.l[0] = first;
	event.xclient.data.l[1] = second;
	event.xclient.data.l[2] = third;
	event.xclient.data.l[3] = fourth;
	event.xclient.data.l[4] = fifth;
	XSendEvent(window->state.x11.display, target, False, NoEventMask, &event);
	XFlush(window->state.x11.display);
}

static int onix_x11_dnd_has_uri_type(OnixNativeWindow *window, Window source, long flags, const long *inline_types) {
	Display *display = window->state.x11.display;
	if ((flags & 1) == 0) {
		for (int index = 0; index < 3; index++) {
			if ((Atom)inline_types[index] == window->state.x11.text_uri_list) return 1;
		}
		return 0;
	}
	Atom actual_type;
	int actual_format;
	unsigned long item_count, bytes_after;
	Atom *types = NULL;
	int result = XGetWindowProperty(display, source, window->state.x11.xdnd_type_list, 0, 256, False, XA_ATOM, &actual_type, &actual_format, &item_count, &bytes_after, (unsigned char **)&types) == Success;
	if (result && actual_format == 32) {
		result = 0;
		for (unsigned long index = 0; index < item_count; index++) {
			if (types[index] == window->state.x11.text_uri_list) {
				result = 1;
				break;
			}
		}
	}
	if (types != NULL) XFree(types);
	return result;
}

static void onix_x11_dnd_emit_uris(OnixNativeWindow *window, const unsigned char *data, unsigned long length, int x, int y) {
	if (data == NULL || length == 0) return;
	char *copy = malloc((size_t)length + 1);
	if (copy == NULL) return;
	memcpy(copy, data, (size_t)length);
	copy[length] = '\0';
	char *cursor = copy;
	char *end = copy + length;
	while (cursor < end) {
		char *line = cursor;
		char *newline = memchr(cursor, '\n', (size_t)(end - cursor));
		if (newline == NULL) cursor = end;
		else { *newline = '\0'; cursor = newline + 1; }
		if (line[0] == '#') continue;
		size_t line_length = strlen(line);
		while (line_length > 0 && line[line_length - 1] == '\r') line[--line_length] = '\0';
		if (line_length > 0) onix_push_drop_event(window, x, y, line, (int)line_length);
	}
	free(copy);
}

static void onix_x11_handle_dnd_client_message(OnixNativeWindow *window, XClientMessageEvent *message) {
	if (message->message_type == window->state.x11.xdnd_enter) {
		Window source = (Window)message->data.l[0];
		int accepts = onix_x11_dnd_has_uri_type(window, source, message->data.l[1], &message->data.l[2]);
		window->state.x11.xdnd_source = source;
		window->state.x11.xdnd_accept = accepts;
		onix_x11_send_dnd_message(window, source, window->state.x11.xdnd_status, (long)window->state.x11.window, accepts ? 1 : 0, 0, 0, accepts ? (long)window->state.x11.xdnd_action_copy : 0);
		return;
	}
	if (message->message_type == window->state.x11.xdnd_position) {
		if ((Window)message->data.l[0] != window->state.x11.xdnd_source) return;
		window->state.x11.xdnd_x = (int16_t)(message->data.l[2] >> 16);
		window->state.x11.xdnd_y = (int16_t)(message->data.l[2] & 0xffff);
		window->state.x11.xdnd_time = (Time)message->data.l[3];
		onix_x11_send_dnd_message(window, window->state.x11.xdnd_source, window->state.x11.xdnd_status, (long)window->state.x11.window, window->state.x11.xdnd_accept ? 1 : 0, 0, 0, window->state.x11.xdnd_accept ? (long)window->state.x11.xdnd_action_copy : 0);
		return;
	}
	if (message->message_type == window->state.x11.xdnd_leave) {
		if ((Window)message->data.l[0] == window->state.x11.xdnd_source) {
			window->state.x11.xdnd_source = 0;
			window->state.x11.xdnd_accept = 0;
		}
		return;
	}
	if (message->message_type == window->state.x11.xdnd_drop) {
		if ((Window)message->data.l[0] != window->state.x11.xdnd_source) return;
		if (!window->state.x11.xdnd_accept) {
			onix_x11_send_dnd_message(window, window->state.x11.xdnd_source, window->state.x11.xdnd_finished, (long)window->state.x11.window, 0, 0, 0, 0);
			window->state.x11.xdnd_source = 0;
			return;
		}
		window->state.x11.xdnd_time = (Time)message->data.l[2];
		XConvertSelection(window->state.x11.display, window->state.x11.xdnd_selection, window->state.x11.text_uri_list, window->state.x11.xdnd_property, window->state.x11.window, window->state.x11.xdnd_time);
		XFlush(window->state.x11.display);
	}
}

static void onix_poll_x11(OnixNativeWindow *window) {
	XEvent event;
	while (XPending(window->state.x11.display) > 0) {
		XNextEvent(window->state.x11.display, &event);
		if (event.type == KeyPress || event.type == KeyRelease) {
			const unsigned int masks[] = {ShiftMask, ControlMask, Mod1Mask, Mod4Mask};
			uint8_t active = 0;
			for (int index = 0; index < 4; index++) {
				if (event.xkey.state & masks[index]) active |= 1u << index;
			}
			onix_sync_modifiers(window, active);
		}
		if (window->state.x11.im != NULL && XFilterEvent(&event, None)) {
			continue;
		}
		switch (event.type) {
		case SelectionRequest: {
			XSelectionRequestEvent *request = &event.xselectionrequest;
			XEvent response;
			memset(&response, 0, sizeof(response));
			response.xselection.type = SelectionNotify;
			response.xselection.display = request->display;
			response.xselection.requestor = request->requestor;
			response.xselection.selection = request->selection;
			response.xselection.target = request->target;
			response.xselection.time = request->time;
			response.xselection.property = None;
			Atom property = request->property != None ? request->property : request->target;
			if (request->target == window->state.x11.targets) {
				Atom targets[] = {window->state.x11.targets, window->state.x11.utf8_string, XA_STRING};
				XChangeProperty(request->display, request->requestor, property, XA_ATOM, 32, PropModeReplace, (unsigned char *)targets, 3);
				response.xselection.property = property;
			} else if (request->target == window->state.x11.utf8_string || request->target == XA_STRING) {
				const char *text = window->clipboard_text != NULL ? window->clipboard_text : "";
				XChangeProperty(request->display, request->requestor, property, request->target, 8, PropModeReplace, (const unsigned char *)text, (int)strlen(text));
				response.xselection.property = property;
			}
			XSendEvent(request->display, request->requestor, False, 0, &response);
			XFlush(request->display);
			break;
		}
		case ClientMessage:
			if ((Atom)event.xclient.data.l[0] == window->state.x11.wm_delete) {
				window->should_close = 1;
				onix_push_event(window, ONIX_EVENT_CLOSE, 0, 0, 0, 0);
			} else {
				onix_x11_handle_dnd_client_message(window, &event.xclient);
			}
			break;
		case SelectionNotify:
			if (event.xselection.selection == window->state.x11.xdnd_selection) {
				Atom actual_type;
				int actual_format;
				unsigned long item_count, bytes_after;
				unsigned char *data = NULL;
				if (event.xselection.property != None && XGetWindowProperty(window->state.x11.display, window->state.x11.window, window->state.x11.xdnd_property, 0, 1 << 20, True, AnyPropertyType, &actual_type, &actual_format, &item_count, &bytes_after, &data) == Success) {
					Window child;
					int local_x = window->state.x11.xdnd_x;
					int local_y = window->state.x11.xdnd_y;
					XTranslateCoordinates(window->state.x11.display, RootWindow(window->state.x11.display, DefaultScreen(window->state.x11.display)), window->state.x11.window, window->state.x11.xdnd_x, window->state.x11.xdnd_y, &local_x, &local_y, &child);
					unsigned long bytes = actual_format == 8 ? item_count : actual_format == 16 ? item_count * 2 : actual_format == 32 ? item_count * 4 : 0;
					onix_x11_dnd_emit_uris(window, data, bytes, local_x, local_y);
				}
				if (data != NULL) XFree(data);
				if (window->state.x11.xdnd_source != 0) {
					onix_x11_send_dnd_message(window, window->state.x11.xdnd_source, window->state.x11.xdnd_finished, (long)window->state.x11.window, event.xselection.property != None ? 1 : 0, 0, 0, (long)window->state.x11.xdnd_action_copy);
				}
				window->state.x11.xdnd_source = 0;
				window->state.x11.xdnd_accept = 0;
			}
			break;
		case ConfigureNotify:
			window->window_width = event.xconfigure.width;
			window->window_height = event.xconfigure.height;
			window->framebuffer_width = event.xconfigure.width;
			window->framebuffer_height = event.xconfigure.height;
			window->maximized = onix_x11_is_maximized(window);
			window->fullscreen = onix_x11_is_fullscreen(window);
			onix_push_event(window, ONIX_EVENT_WINDOW_STATE, window->maximized, 0, 0, 0);
			onix_push_event(window, ONIX_EVENT_WINDOW_SIZE, window->window_width, window->window_height, 0, 0);
			onix_push_event(window, ONIX_EVENT_FRAMEBUFFER_SIZE, window->framebuffer_width, window->framebuffer_height, 0, 0);
			break;
		case PropertyNotify:
			if (event.xproperty.atom == window->state.x11.net_wm_state) {
				window->maximized = onix_x11_is_maximized(window);
				window->fullscreen = onix_x11_is_fullscreen(window);
				onix_push_event(window, ONIX_EVENT_WINDOW_STATE, window->maximized, 0, 0, 0);
			}
			break;
		case GenericEvent:
			if (event.xcookie.extension == window->state.x11.xi_opcode && XGetEventData(window->state.x11.display, &event.xcookie)) {
				if (window->mouse_captured && event.xcookie.evtype == XI_RawMotion) {
					XIRawEvent *motion = event.xcookie.data;
					double *values = motion->raw_values, dx = 0, dy = 0;
					if (XIMaskIsSet(motion->valuators.mask, 0)) dx = *values++;
					if (XIMaskIsSet(motion->valuators.mask, 1)) dy = *values;
					onix_push_event(window, ONIX_EVENT_MOUSE_DELTA, 0, 0, dx, dy);
				}
				XFreeEventData(window->state.x11.display, &event.xcookie);
			}
			break;
		case MotionNotify:
			if (!window->mouse_captured) onix_push_event(window, ONIX_EVENT_MOUSE_MOVE, 0, 0, event.xmotion.x, event.xmotion.y);
			break;
		case FocusIn:
			if (event.xfocus.mode != NotifyNormal) break;
			window->state.x11.focused = 1;
			if (window->text_input_active && window->state.x11.ic != NULL) {
				XSetICFocus(window->state.x11.ic);
			}
			break;
		case FocusOut:
			if (event.xfocus.mode != NotifyNormal) break;
			window->state.x11.focused = 0;
			onix_release_modifiers(window, 0xff);
			onix_window_set_mouse_captured(window, 0);
			if (window->state.x11.ic != NULL) {
				XUnsetICFocus(window->state.x11.ic);
			}
			break;
		case ButtonPress:
			if (event.xbutton.button == Button4) {
				onix_push_event(window, ONIX_EVENT_MOUSE_WHEEL, 0, 0, 0, 1);
				break;
			}
			if (event.xbutton.button == Button5) {
				onix_push_event(window, ONIX_EVENT_MOUSE_WHEEL, 0, 0, 0, -1);
				break;
			}
			if (event.xbutton.button == Button6) {
				onix_push_event(window, ONIX_EVENT_MOUSE_WHEEL, 0, 0, -1, 0);
				break;
			}
			if (event.xbutton.button == Button7) {
				onix_push_event(window, ONIX_EVENT_MOUSE_WHEEL, 0, 0, 1, 0);
				break;
			}
			onix_push_event(window, ONIX_EVENT_MOUSE_DOWN, onix_mouse_button_from_x11(event.xbutton.button), 0, event.xbutton.x, event.xbutton.y);
			break;
		case ButtonRelease:
			if (event.xbutton.button >= Button4 && event.xbutton.button <= Button7) {
				break;
			}
			onix_push_event(window, ONIX_EVENT_MOUSE_UP, onix_mouse_button_from_x11(event.xbutton.button), 0, event.xbutton.x, event.xbutton.y);
			break;
		case KeyPress: {
			KeySym sym = 0;
			char buffer[256];
			int text_len = 0;
			if (window->text_input_active && window->state.x11.ic != NULL) {
				Status status = 0;
				text_len = Xutf8LookupString(window->state.x11.ic, &event.xkey, buffer, sizeof(buffer) - 1, &sym, &status);
				if (status == XBufferOverflow && text_len > 0) {
					char *large = malloc((size_t)text_len + 1);
					if (large != NULL) {
						text_len = Xutf8LookupString(window->state.x11.ic, &event.xkey, large, text_len, &sym, &status);
						if (text_len > 0) {
							onix_push_text_input_event(window, large, text_len);
						}
						free(large);
						text_len = 0;
					}
				}
			} else if (window->text_input_active) {
				text_len = XLookupString(&event.xkey, buffer, sizeof(buffer) - 1, &sym, NULL);
			}
			if (text_len > 0) {
				onix_push_text_input_event(window, buffer, text_len);
			}
			if (sym == 0) {
				sym = XLookupKeysym(&event.xkey, 0);
				}
				int mapped = onix_keysym_to_keycode((uint32_t)sym);
				if (window->text_input_active && mapped == ONIX_KEY_BACKSPACE) {
					onix_push_text_command(window, ONIX_TEXT_COMMAND_BACKSPACE, 1);
				} else if (window->text_input_active && mapped == ONIX_KEY_ENTER) {
					onix_push_text_command(window, ONIX_TEXT_COMMAND_ENTER, 1);
				}
				if (mapped != 0) {
				onix_push_event(window, ONIX_EVENT_KEY_DOWN, mapped, 0, 0, 0);
			}
			break;
		}
		case KeyRelease: {
			if (XEventsQueued(window->state.x11.display, QueuedAfterReading) > 0) {
				XEvent next;
				XPeekEvent(window->state.x11.display, &next);
				if (next.type == KeyPress && next.xkey.time == event.xkey.time && next.xkey.keycode == event.xkey.keycode) {
					break;
				}
			}
			KeySym sym = XLookupKeysym(&event.xkey, 0);
			int mapped = onix_keysym_to_keycode((uint32_t)sym);
			if (mapped != 0) {
				onix_push_event(window, ONIX_EVENT_KEY_UP, mapped, 0, 0, 0);
			}
			break;
		}
		default:
			break;
		}
	}
}

static void onix_poll_wayland(OnixNativeWindow *window) {
	struct wl_display *display = window->state.wayland.display;
	if (display == NULL) {
		return;
	}
	if (wl_display_dispatch_pending(display) < 0) {
		window->should_close = 1;
		onix_push_event(window, ONIX_EVENT_CLOSE, 0, 0, 0, 0);
		return;
	}
	onix_wayland_poll_drop(window);

	for (;;) {
		if (wl_display_prepare_read(display) != 0) {
			if (wl_display_dispatch_pending(display) < 0) {
				window->should_close = 1;
				onix_push_event(window, ONIX_EVENT_CLOSE, 0, 0, 0, 0);
				return;
			}
			continue;
		}

		wl_display_flush(display);

		struct pollfd pfd;
		memset(&pfd, 0, sizeof(pfd));
		pfd.fd = wl_display_get_fd(display);
		pfd.events = POLLIN;

		int ret = poll(&pfd, 1, 0);
		if (ret > 0 && (pfd.revents & POLLIN) != 0) {
			if (wl_display_read_events(display) < 0) {
				wl_display_cancel_read(display);
				window->should_close = 1;
				onix_push_event(window, ONIX_EVENT_CLOSE, 0, 0, 0, 0);
				return;
			}
			if (wl_display_dispatch_pending(display) < 0) {
				window->should_close = 1;
				onix_push_event(window, ONIX_EVENT_CLOSE, 0, 0, 0, 0);
				return;
			}
			continue;
		}

		wl_display_cancel_read(display);
		if (ret < 0 || (pfd.revents & (POLLERR | POLLHUP)) != 0) {
			window->should_close = 1;
			onix_push_event(window, ONIX_EVENT_CLOSE, 0, 0, 0, 0);
		}
		break;
	}
	onix_wayland_poll_drop(window);
}

static int onix_try_backend(const char *backend, int width, int height, const char *title, int client_decorations, OnixNativeWindow **result) {
	OnixNativeWindow *window = onix_alloc_window(width, height);
	if (window == NULL) {
		return 0;
	}
	if (strcmp(backend, "wayland") == 0) {
		if (onix_init_wayland(window, width, height, title, client_decorations)) {
			*result = window;
			return 1;
		}
	} else if (strcmp(backend, "x11") == 0) {
		if (onix_init_x11(window, width, height, title, client_decorations)) {
			*result = window;
			return 1;
		}
	}
	onix_window_destroy(window);
	free(window);
	return 0;
}

OnixNativeWindow *onix_window_create(int width, int height, const char *title, int client_decorations, int require_x11) {
	onix_clear_error();

	const char *forced = getenv("ONIX_LINUX_BACKEND");
	OnixNativeWindow *window = NULL;

	if (forced != NULL && forced[0] != '\0') {
		if (require_x11 && strcmp(forced, "x11") != 0) {
			onix_set_error("embedded WebView requires ONIX_LINUX_BACKEND=x11");
			return NULL;
		}
		if (onix_try_backend(forced, width, height, title, client_decorations, &window)) {
			return window;
		}
		if (onix_error_message[0] == '\0') {
			onix_set_error("unsupported ONIX_LINUX_BACKEND=%s", forced);
		}
		return NULL;
	}

	const char *wayland_display = getenv("WAYLAND_DISPLAY");
	const char *x11_display = getenv("DISPLAY");
	if (require_x11) {
		if (x11_display != NULL && x11_display[0] != '\0' && onix_try_backend("x11", width, height, title, client_decorations, &window)) {
			return window;
		}
		onix_set_error("embedded WebView requires an X11 or XWayland display");
		return NULL;
	}

	if (wayland_display != NULL && wayland_display[0] != '\0' && onix_try_backend("wayland", width, height, title, client_decorations, &window)) {
		return window;
	}
	onix_clear_error();

	if (x11_display != NULL && x11_display[0] != '\0' && onix_try_backend("x11", width, height, title, client_decorations, &window)) {
		return window;
	}

	if (wayland_display != NULL && wayland_display[0] != '\0' && onix_try_backend("wayland", width, height, title, client_decorations, &window)) {
		return window;
	}

	if ((wayland_display == NULL || wayland_display[0] == '\0') && (x11_display == NULL || x11_display[0] == '\0')) {
		onix_set_error("no Linux display backend is available; set WAYLAND_DISPLAY or DISPLAY");
	}
	return NULL;
}

void onix_window_destroy(OnixNativeWindow *window) {
	if (window == NULL) {
		return;
	}

	onix_window_set_mouse_captured(window, 0);
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		if (window->state.wayland.cursor_surface != NULL) wl_surface_destroy(window->state.wayland.cursor_surface);
		if (window->state.wayland.cursor_theme != NULL) wl_cursor_theme_destroy(window->state.wayland.cursor_theme);
		if (window->state.wayland.shm != NULL) wl_shm_destroy(window->state.wayland.shm);
		if (window->state.wayland.pointer_constraints != NULL) zwp_pointer_constraints_v1_destroy(window->state.wayland.pointer_constraints);
		if (window->state.wayland.relative_pointer_manager != NULL) zwp_relative_pointer_manager_v1_destroy(window->state.wayland.relative_pointer_manager);
		onix_clear_wayland_text_pending(window);
		if (window->state.wayland.dnd_fd >= 0) close(window->state.wayland.dnd_fd);
		free(window->state.wayland.dnd_buffer);
		window->state.wayland.dnd_buffer = NULL;
		window->state.wayland.dnd_fd = -1;
		if (window->state.wayland.dnd_offer != NULL && window->state.wayland.dnd_offer != window->state.wayland.selection_offer) wl_data_offer_destroy(window->state.wayland.dnd_offer);
		window->state.wayland.dnd_offer = NULL;
		if (window->state.wayland.data_source != NULL) wl_data_source_destroy(window->state.wayland.data_source);
		if (window->state.wayland.selection_offer != NULL) wl_data_offer_destroy(window->state.wayland.selection_offer);
		if (window->state.wayland.data_device != NULL) wl_data_device_destroy(window->state.wayland.data_device);
		if (window->state.wayland.data_device_manager != NULL) wl_data_device_manager_destroy(window->state.wayland.data_device_manager);
		if (window->state.wayland.keyboard != NULL) {
			wl_keyboard_destroy(window->state.wayland.keyboard);
			window->state.wayland.keyboard = NULL;
		}
		if (window->state.wayland.text_input != NULL) {
			zwp_text_input_v3_destroy(window->state.wayland.text_input);
			window->state.wayland.text_input = NULL;
		}
		if (window->state.wayland.text_input_manager != NULL) {
			zwp_text_input_manager_v3_destroy(window->state.wayland.text_input_manager);
			window->state.wayland.text_input_manager = NULL;
		}
		if (window->state.wayland.pointer != NULL) {
			wl_pointer_destroy(window->state.wayland.pointer);
			window->state.wayland.pointer = NULL;
		}
		if (window->state.wayland.seat != NULL) {
			wl_seat_destroy(window->state.wayland.seat);
			window->state.wayland.seat = NULL;
		}
		if (window->state.wayland.xdg_toplevel != NULL) {
			if (window->state.wayland.toplevel_decoration != NULL) {
				zxdg_toplevel_decoration_v1_destroy(window->state.wayland.toplevel_decoration);
				window->state.wayland.toplevel_decoration = NULL;
			}
			xdg_toplevel_destroy(window->state.wayland.xdg_toplevel);
			window->state.wayland.xdg_toplevel = NULL;
		}
		if (window->state.wayland.xdg_surface != NULL) {
			xdg_surface_destroy(window->state.wayland.xdg_surface);
			window->state.wayland.xdg_surface = NULL;
		}
		if (window->state.wayland.surface != NULL) {
			wl_surface_destroy(window->state.wayland.surface);
			window->state.wayland.surface = NULL;
		}
		if (window->state.wayland.wm_base != NULL) {
			xdg_wm_base_destroy(window->state.wayland.wm_base);
			window->state.wayland.wm_base = NULL;
		}
		if (window->state.wayland.decoration_manager != NULL) {
			zxdg_decoration_manager_v1_destroy(window->state.wayland.decoration_manager);
			window->state.wayland.decoration_manager = NULL;
		}
		if (window->state.wayland.registry != NULL) {
			wl_registry_destroy(window->state.wayland.registry);
			window->state.wayland.registry = NULL;
		}
		if (window->state.wayland.xkb_state != NULL) {
			xkb_state_unref(window->state.wayland.xkb_state);
			window->state.wayland.xkb_state = NULL;
		}
		if (window->state.wayland.xkb_compose_state != NULL) {
			xkb_compose_state_unref(window->state.wayland.xkb_compose_state);
			window->state.wayland.xkb_compose_state = NULL;
		}
		if (window->state.wayland.xkb_compose_table != NULL) {
			xkb_compose_table_unref(window->state.wayland.xkb_compose_table);
			window->state.wayland.xkb_compose_table = NULL;
		}
		if (window->state.wayland.xkb_keymap != NULL) {
			xkb_keymap_unref(window->state.wayland.xkb_keymap);
			window->state.wayland.xkb_keymap = NULL;
		}
		if (window->state.wayland.xkb_context != NULL) {
			xkb_context_unref(window->state.wayland.xkb_context);
			window->state.wayland.xkb_context = NULL;
		}
		if (window->state.wayland.display != NULL) {
			wl_display_disconnect(window->state.wayland.display);
			window->state.wayland.display = NULL;
		}
	}

	if (window->backend == ONIX_BACKEND_X11) {
		for (int i = 0; i < ONIX_CURSOR_COUNT; i++) if (window->state.x11.cursors[i] != None) XFreeCursor(window->state.x11.display, window->state.x11.cursors[i]);
		if (window->state.x11.hidden_cursor != None) XFreeCursor(window->state.x11.display, window->state.x11.hidden_cursor);
		if (window->state.x11.ic != NULL) {
			XDestroyIC(window->state.x11.ic);
			window->state.x11.ic = NULL;
		}
		if (window->state.x11.im != NULL) {
			XCloseIM(window->state.x11.im);
			window->state.x11.im = NULL;
		}
		if (window->state.x11.window != 0 && window->state.x11.display != NULL) {
			XDestroyWindow(window->state.x11.display, window->state.x11.window);
			window->state.x11.window = 0;
		}
		if (window->state.x11.colormap != 0 && window->state.x11.display != NULL) {
			XFreeColormap(window->state.x11.display, window->state.x11.colormap);
			window->state.x11.colormap = 0;
		}
		if (window->state.x11.display != NULL) {
			XCloseDisplay(window->state.x11.display);
			window->state.x11.display = NULL;
		}
	}
	free(window->clipboard_text);
	window->clipboard_text = NULL;
}

int onix_window_set_clipboard_text(OnixNativeWindow *window, const char *text) {
	if (window == NULL || text == NULL) return 0;
	char *replacement = strdup(text);
	if (replacement == NULL) return 0;
	free(window->clipboard_text);
	window->clipboard_text = replacement;
	if (window->backend == ONIX_BACKEND_X11) {
		XSetSelectionOwner(window->state.x11.display, window->state.x11.clipboard, window->state.x11.window, CurrentTime);
		XFlush(window->state.x11.display);
		return XGetSelectionOwner(window->state.x11.display, window->state.x11.clipboard) == window->state.x11.window;
	}
	if (window->backend == ONIX_BACKEND_WAYLAND && window->state.wayland.data_device_manager != NULL && window->state.wayland.data_device != NULL) {
		if (window->state.wayland.data_source != NULL) wl_data_source_destroy(window->state.wayland.data_source);
		window->state.wayland.data_source = wl_data_device_manager_create_data_source(window->state.wayland.data_device_manager);
		if (window->state.wayland.data_source == NULL) return 0;
		wl_data_source_add_listener(window->state.wayland.data_source, &onix_wayland_data_source_listener, window);
		wl_data_source_offer(window->state.wayland.data_source, "text/plain;charset=utf-8");
		wl_data_source_offer(window->state.wayland.data_source, "text/plain");
		wl_data_device_set_selection(window->state.wayland.data_device, window->state.wayland.data_source, window->state.wayland.last_serial);
		wl_display_flush(window->state.wayland.display);
		return 1;
	}
	return 0;
}

static int onix_copy_clipboard_result(const char *text, int length, char *buffer, int capacity) {
	if (text == NULL || length < 0) return -1;
	if (buffer != NULL && capacity > 0) {
		int copied = length < capacity - 1 ? length : capacity - 1;
		memcpy(buffer, text, (size_t)copied);
		buffer[copied] = '\0';
	}
	return length;
}

int onix_window_get_clipboard_text(OnixNativeWindow *window, char *buffer, int capacity) {
	if (window == NULL) return -1;
	if (window->backend == ONIX_BACKEND_X11) {
		Display *display = window->state.x11.display;
		if (XGetSelectionOwner(display, window->state.x11.clipboard) == window->state.x11.window) {
			const char *text = window->clipboard_text != NULL ? window->clipboard_text : "";
			return onix_copy_clipboard_result(text, (int)strlen(text), buffer, capacity);
		}
		Atom property = XInternAtom(display, "ONIX_CLIPBOARD", False);
		XConvertSelection(display, window->state.x11.clipboard, window->state.x11.utf8_string, property, window->state.x11.window, CurrentTime);
		XFlush(display);
		struct pollfd fd = {.fd = ConnectionNumber(display), .events = POLLIN};
		if (poll(&fd, 1, 500) <= 0) return -1;
		XEvent event;
		if (!XCheckTypedWindowEvent(display, window->state.x11.window, SelectionNotify, &event) || event.xselection.property == None) return -1;
		Atom type;
		int format;
		unsigned long count, remaining;
		unsigned char *value = NULL;
		if (XGetWindowProperty(display, window->state.x11.window, property, 0, 1 << 20, True, AnyPropertyType, &type, &format, &count, &remaining, &value) != Success || value == NULL || format != 8) {
			if (value != NULL) XFree(value);
			return -1;
		}
		int result = onix_copy_clipboard_result((const char *)value, (int)count, buffer, capacity);
		XFree(value);
		return result;
	}
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		if (window->state.wayland.data_source != NULL) {
			const char *text = window->clipboard_text != NULL ? window->clipboard_text : "";
			return onix_copy_clipboard_result(text, (int)strlen(text), buffer, capacity);
		}
		struct wl_data_offer *offer = window->state.wayland.selection_offer;
		if (offer == NULL) return -1;
		int fds[2];
		if (pipe(fds) != 0) return -1;
		fcntl(fds[0], F_SETFL, O_NONBLOCK);
		const char *mime = window->state.wayland.selection_offer_utf8 ? "text/plain;charset=utf-8" : "text/plain";
		wl_data_offer_receive(offer, mime, fds[1]);
		close(fds[1]);
		wl_display_flush(window->state.wayland.display);
		char *text = NULL;
		int length = 0;
		struct pollfd fd = {.fd = fds[0], .events = POLLIN | POLLHUP};
		while (poll(&fd, 1, 500) > 0) {
			char chunk[4096];
			ssize_t count = read(fds[0], chunk, sizeof(chunk));
			if (count > 0) {
				char *grown = realloc(text, (size_t)length + (size_t)count);
				if (grown == NULL) { free(text); close(fds[0]); return -1; }
				text = grown;
				memcpy(text + length, chunk, (size_t)count);
				length += (int)count;
				continue;
			}
			if (fd.revents & POLLHUP) break;
		}
		close(fds[0]);
		int result = onix_copy_clipboard_result(text != NULL ? text : "", length, buffer, capacity);
		free(text);
		return result;
	}
	return -1;
}

int onix_window_backend(OnixNativeWindow *window) {
	return window != NULL ? window->backend : ONIX_BACKEND_NONE;
}

int onix_window_should_close(OnixNativeWindow *window) {
	return window != NULL ? window->should_close : 1;
}

void onix_window_cancel_close(OnixNativeWindow *window) {
	if (window != NULL) window->should_close = 0;
}

int onix_window_window_width(OnixNativeWindow *window) {
	return window != NULL ? window->window_width : 0;
}

int onix_window_window_height(OnixNativeWindow *window) {
	return window != NULL ? window->window_height : 0;
}

int onix_window_framebuffer_width(OnixNativeWindow *window) {
	return window != NULL ? window->framebuffer_width : 0;
}

int onix_window_framebuffer_height(OnixNativeWindow *window) {
	return window != NULL ? window->framebuffer_height : 0;
}

void *onix_window_native_display(OnixNativeWindow *window) {
	if (window == NULL) {
		return NULL;
	}
	if (window->backend == ONIX_BACKEND_X11) {
		return window->state.x11.display;
	}
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		return window->state.wayland.display;
	}
	return NULL;
}

uintptr_t onix_window_native_surface(OnixNativeWindow *window) {
	if (window == NULL) {
		return 0;
	}
	if (window->backend == ONIX_BACKEND_X11) {
		return (uintptr_t)window->state.x11.window;
	}
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		return (uintptr_t)window->state.wayland.surface;
	}
	return 0;
}

void onix_window_set_title(OnixNativeWindow *window, const char *title) {
	if (window == NULL || title == NULL) {
		return;
	}
	if (window->backend == ONIX_BACKEND_X11 && window->state.x11.display != NULL && window->state.x11.window != 0) {
		XStoreName(window->state.x11.display, window->state.x11.window, title);
		XFlush(window->state.x11.display);
		return;
	}
	if (window->backend == ONIX_BACKEND_WAYLAND && window->state.wayland.xdg_toplevel != NULL) {
		xdg_toplevel_set_title(window->state.wayland.xdg_toplevel, title);
		wl_surface_commit(window->state.wayland.surface);
	}
}

void onix_window_set_icon(OnixNativeWindow *window, int width, int height, const uint32_t *pixels) {
	if (window == NULL || width <= 0 || height <= 0 || pixels == NULL) {
		return;
	}
	if (window->backend != ONIX_BACKEND_X11 || window->state.x11.display == NULL || window->state.x11.window == 0) {
		return;
	}

	unsigned long count = 2u + (unsigned long)width * (unsigned long)height;
	unsigned long *data = calloc(count, sizeof(unsigned long));
	if (data == NULL) {
		return;
	}

	data[0] = (unsigned long)width;
	data[1] = (unsigned long)height;
	for (unsigned long i = 0; i < count - 2; i++) {
		data[i+2] = (unsigned long)pixels[i];
	}

	Atom property = XInternAtom(window->state.x11.display, "_NET_WM_ICON", False);
	XChangeProperty(
		window->state.x11.display,
		window->state.x11.window,
		property,
		XA_CARDINAL,
		32,
		PropModeReplace,
		(const unsigned char *)data,
		(int)count);
	XFlush(window->state.x11.display);
	free(data);
}

void onix_window_set_vsync(OnixNativeWindow *window, int enabled) {
	if (window == NULL) {
		return;
	}
	window->vsync_enabled = enabled != 0;
}

void onix_window_set_decorations(OnixNativeWindow *window, int mode) {
	if (window == NULL) return;
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		if (window->state.wayland.toplevel_decoration == NULL) return;
		zxdg_toplevel_decoration_v1_set_mode(
			window->state.wayland.toplevel_decoration,
			mode ? ZXDG_TOPLEVEL_DECORATION_V1_MODE_CLIENT_SIDE : ZXDG_TOPLEVEL_DECORATION_V1_MODE_SERVER_SIDE);
		wl_display_flush(window->state.wayland.display);
		return;
	}
	if (window->backend == ONIX_BACKEND_X11) {
		onix_x11_set_decorations(window, mode);
		XFlush(window->state.x11.display);
	}
}

void onix_window_set_border_visible(OnixNativeWindow *window, int visible) {
	if (window == NULL || window->backend != ONIX_BACKEND_X11) return;
	onix_x11_set_border_visible(window, visible);
	XFlush(window->state.x11.display);
}

void onix_window_set_shadow_visible(OnixNativeWindow *window, int visible) {
	if (window == NULL || window->backend != ONIX_BACKEND_X11) return;
	unsigned long value = visible != 0;
	Atom property = XInternAtom(window->state.x11.display, "_COMPTON_SHADOW", False);
	XChangeProperty(window->state.x11.display, window->state.x11.window, property, XA_CARDINAL, 32, PropModeReplace, (unsigned char *)&value, 1);
	XFlush(window->state.x11.display);
}

void onix_window_set_resizable(OnixNativeWindow *window, int enabled) {
	if (window == NULL) return;
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		int width = enabled ? 0 : window->window_width;
		int height = enabled ? 0 : window->window_height;
		xdg_toplevel_set_min_size(window->state.wayland.xdg_toplevel, width, height);
		xdg_toplevel_set_max_size(window->state.wayland.xdg_toplevel, width, height);
		wl_surface_commit(window->state.wayland.surface);
		wl_display_flush(window->state.wayland.display);
		return;
	}
	if (window->backend == ONIX_BACKEND_X11) {
		XSizeHints hints = {0};
		long supplied = 0;
		XGetWMNormalHints(window->state.x11.display, window->state.x11.window, &hints, &supplied);
		if (!enabled) {
			hints.flags |= PMinSize | PMaxSize;
			hints.min_width = hints.max_width = window->window_width;
			hints.min_height = hints.max_height = window->window_height;
		} else {
			hints.flags &= ~(PMinSize | PMaxSize);
		}
		XSetWMNormalHints(window->state.x11.display, window->state.x11.window, &hints);
		XFlush(window->state.x11.display);
	}
}

int onix_window_is_maximized(OnixNativeWindow *window) {
	return window != NULL && window->maximized;
}

int onix_window_is_fullscreen(OnixNativeWindow *window) {
	return window != NULL && window->fullscreen;
}

void onix_window_set_taskbar_visible(OnixNativeWindow *window, int visible) {
	if (window == NULL || window->backend != ONIX_BACKEND_X11) return;
	XEvent event = {0};
	event.xclient.type = ClientMessage;
	event.xclient.window = window->state.x11.window;
	event.xclient.message_type = XInternAtom(window->state.x11.display, "_NET_WM_STATE", False);
	event.xclient.format = 32;
	event.xclient.data.l[0] = visible ? 0 : 1;
	event.xclient.data.l[1] = XInternAtom(window->state.x11.display, "_NET_WM_STATE_SKIP_TASKBAR", False);
	XSendEvent(
		window->state.x11.display,
		DefaultRootWindow(window->state.x11.display),
		False,
		SubstructureRedirectMask | SubstructureNotifyMask,
		&event);
	XFlush(window->state.x11.display);
}

void onix_window_set_always_on_top(OnixNativeWindow *window, int enabled) {
	if (window == NULL || window->backend != ONIX_BACKEND_X11) return;
	XEvent event = {0};
	event.xclient.type = ClientMessage;
	event.xclient.window = window->state.x11.window;
	event.xclient.message_type = XInternAtom(window->state.x11.display, "_NET_WM_STATE", False);
	event.xclient.format = 32;
	event.xclient.data.l[0] = enabled ? 1 : 0;
	event.xclient.data.l[1] = XInternAtom(window->state.x11.display, "_NET_WM_STATE_ABOVE", False);
	event.xclient.data.l[3] = 1;
	XSendEvent(
		window->state.x11.display,
		DefaultRootWindow(window->state.x11.display),
		False,
		SubstructureRedirectMask | SubstructureNotifyMask,
		&event);
	XFlush(window->state.x11.display);
}

void onix_window_set_input_region(OnixNativeWindow *window, int enabled, const OnixInputRect *rects, int count) {
	if (window == NULL || count < 0 || (count > 0 && rects == NULL)) return;
	if (window->backend == ONIX_BACKEND_X11) {
		Display *display = window->state.x11.display;
		Window native = window->state.x11.window;
		if (!enabled) {
			XShapeCombineMask(display, native, ShapeInput, 0, 0, None, ShapeSet);
		} else {
			Region region = XCreateRegion();
			for (int i = 0; i < count; i++) {
				if (rects[i].width <= 0 || rects[i].height <= 0) continue;
				XRectangle rect = {
					.x = (short)rects[i].x,
					.y = (short)rects[i].y,
					.width = (unsigned short)rects[i].width,
					.height = (unsigned short)rects[i].height,
				};
				XUnionRectWithRegion(&rect, region, region);
			}
			XShapeCombineRegion(display, native, ShapeInput, 0, 0, region, ShapeSet);
			XDestroyRegion(region);
		}
		XFlush(display);
		return;
	}
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		struct wl_region *region = NULL;
		if (enabled) {
			region = wl_compositor_create_region(window->state.wayland.compositor);
			if (region == NULL) return;
			for (int i = 0; i < count; i++) {
				if (rects[i].width > 0 && rects[i].height > 0) {
					wl_region_add(region, rects[i].x, rects[i].y, rects[i].width, rects[i].height);
				}
			}
		}
		wl_surface_set_input_region(window->state.wayland.surface, region);
		wl_surface_commit(window->state.wayland.surface);
		if (region != NULL) wl_region_destroy(region);
		wl_display_flush(window->state.wayland.display);
	}
}

void onix_window_minimize(OnixNativeWindow *window) {
	if (window == NULL) return;
	if (window->backend == ONIX_BACKEND_X11) {
		XIconifyWindow(window->state.x11.display, window->state.x11.window, DefaultScreen(window->state.x11.display));
		XFlush(window->state.x11.display);
	} else if (window->backend == ONIX_BACKEND_WAYLAND && window->state.wayland.xdg_toplevel != NULL) {
		xdg_toplevel_set_minimized(window->state.wayland.xdg_toplevel);
		wl_surface_commit(window->state.wayland.surface);
		wl_display_flush(window->state.wayland.display);
	}
}

void onix_window_toggle_maximize(OnixNativeWindow *window) {
	if (window == NULL) return;
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		if (window->state.wayland.xdg_toplevel == NULL) return;
		if (window->maximized) xdg_toplevel_unset_maximized(window->state.wayland.xdg_toplevel);
		else xdg_toplevel_set_maximized(window->state.wayland.xdg_toplevel);
		wl_surface_commit(window->state.wayland.surface);
		wl_display_flush(window->state.wayland.display);
		return;
	}
	if (window->backend != ONIX_BACKEND_X11) return;
	XEvent event = {0};
	event.xclient.type = ClientMessage;
	event.xclient.window = window->state.x11.window;
	event.xclient.message_type = XInternAtom(window->state.x11.display, "_NET_WM_STATE", False);
	event.xclient.format = 32;
	event.xclient.data.l[0] = 2;
	event.xclient.data.l[1] = XInternAtom(window->state.x11.display, "_NET_WM_STATE_MAXIMIZED_HORZ", False);
	event.xclient.data.l[2] = XInternAtom(window->state.x11.display, "_NET_WM_STATE_MAXIMIZED_VERT", False);
	event.xclient.data.l[3] = 1;
	XSendEvent(window->state.x11.display, DefaultRootWindow(window->state.x11.display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
	XFlush(window->state.x11.display);
}

void onix_window_toggle_fullscreen(OnixNativeWindow *window) {
	if (window == NULL) return;
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		if (window->state.wayland.xdg_toplevel == NULL) return;
		window->fullscreen = !window->fullscreen;
		if (window->fullscreen) xdg_toplevel_set_fullscreen(window->state.wayland.xdg_toplevel, NULL);
		else xdg_toplevel_unset_fullscreen(window->state.wayland.xdg_toplevel);
		wl_surface_commit(window->state.wayland.surface);
		wl_display_flush(window->state.wayland.display);
		return;
	}
	if (window->backend != ONIX_BACKEND_X11) return;
	XEvent event = {0};
	event.xclient.type = ClientMessage;
	event.xclient.window = window->state.x11.window;
	event.xclient.message_type = XInternAtom(window->state.x11.display, "_NET_WM_STATE", False);
	event.xclient.format = 32;
	event.xclient.data.l[0] = window->fullscreen ? 0 : 1;
	event.xclient.data.l[1] = window->state.x11.fullscreen;
	event.xclient.data.l[3] = 1;
	XSendEvent(window->state.x11.display, DefaultRootWindow(window->state.x11.display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
	XFlush(window->state.x11.display);
	window->fullscreen = !window->fullscreen;
}

void onix_window_start_drag(OnixNativeWindow *window) {
	if (window == NULL || window->backend != ONIX_BACKEND_X11) return;
	Window root, child;
	int root_x, root_y, win_x, win_y;
	unsigned int mask;
	if (!XQueryPointer(window->state.x11.display, window->state.x11.window, &root, &child, &root_x, &root_y, &win_x, &win_y, &mask)) return;
	XEvent event = {0};
	event.xclient.type = ClientMessage;
	event.xclient.window = window->state.x11.window;
	event.xclient.message_type = XInternAtom(window->state.x11.display, "_NET_WM_MOVERESIZE", False);
	event.xclient.format = 32;
	event.xclient.data.l[0] = root_x;
	event.xclient.data.l[1] = root_y;
	event.xclient.data.l[2] = 8;
	event.xclient.data.l[3] = Button1;
	XUngrabPointer(window->state.x11.display, CurrentTime);
	XSendEvent(window->state.x11.display, root, False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
	XFlush(window->state.x11.display);
}

void onix_window_set_text_input_config(OnixNativeWindow *window, int active, int multiline, int kind) {
	if (window == NULL) {
		return;
	}
	window->text_input_active = active != 0;
	window->text_input_multiline = multiline != 0;
	window->text_input_kind = kind;
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		onix_wayland_apply_text_input_config(window);
		return;
	}
	if (window->backend == ONIX_BACKEND_X11 && window->state.x11.ic != NULL && window->text_input_active && window->state.x11.focused) {
		XSetICFocus(window->state.x11.ic);
	} else if (window->backend == ONIX_BACKEND_X11 && window->state.x11.ic != NULL) {
		XUnsetICFocus(window->state.x11.ic);
	}
}

void onix_window_poll_events(OnixNativeWindow *window) {
	if (window == NULL || window->should_close) {
		return;
	}
	if (window->backend == ONIX_BACKEND_X11) {
		onix_poll_x11(window);
		return;
	}
	if (window->backend == ONIX_BACKEND_WAYLAND) {
		onix_poll_wayland(window);
	}
}

void onix_window_wait_events(OnixNativeWindow *window, int wake_fd, int timeout_ms) {
	if (window == NULL || window->should_close || window->queue.head != window->queue.tail) return;
	struct pollfd fds[2] = {{.fd = -1, .events = POLLIN}, {.fd = wake_fd, .events = POLLIN}};
	struct wl_display *display = NULL;
	if (window->backend == ONIX_BACKEND_X11) {
		if (XPending(window->state.x11.display) != 0) return;
		fds[0].fd = ConnectionNumber(window->state.x11.display);
	} else if (window->backend == ONIX_BACKEND_WAYLAND) {
		display = window->state.wayland.display;
		if (wl_display_prepare_read(display) != 0) return;
		wl_display_flush(display);
		fds[0].fd = wl_display_get_fd(display);
	}
	int result = poll(fds, 2, timeout_ms);
	if (display != NULL) {
		if (result > 0 && (fds[0].revents & POLLIN) != 0) {
			if (wl_display_read_events(display) < 0) window->should_close = 1;
		} else {
			wl_display_cancel_read(display);
		}
	}
	if ((fds[0].revents & (POLLERR | POLLHUP | POLLNVAL)) != 0) window->should_close = 1;
}

int onix_window_pop_event(OnixNativeWindow *window, OnixEvent *event) {
	if (window == NULL || event == NULL || window->queue.head == window->queue.tail) {
		return 0;
	}
	*event = window->queue.items[window->queue.head];
	window->queue.head = (window->queue.head + 1) % ONIX_EVENT_QUEUE_CAPACITY;
	return 1;
}
