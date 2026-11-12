//go:build linux && cgo && onix_drm

#include "drm_input.h"

#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <linux/input.h>
#include <poll.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <unistd.h>

#define ONIX_DRM_INPUT_QUEUE_CAPACITY 256
#define ONIX_DRM_INPUT_DEVICE_CAPACITY 64
#define ONIX_BITS_PER_LONG (sizeof(unsigned long) * 8)
#define ONIX_BIT_WORD(bit) ((bit) / ONIX_BITS_PER_LONG)
#define ONIX_BIT_MASK(bit) (1ul << ((bit) % ONIX_BITS_PER_LONG))
#define ONIX_BIT_COUNT(maximum) (ONIX_BIT_WORD(maximum) + 1)

typedef enum {
	ONIX_DRM_DEVICE_NONE = 0,
	ONIX_DRM_DEVICE_MOUSE = 1,
	ONIX_DRM_DEVICE_TOUCH = 2,
} OnixDRMDeviceKind;

typedef struct {
	int fd;
	int kind;
	int absolute;
	int abs_x_code;
	int abs_y_code;
	struct input_absinfo abs_x;
	struct input_absinfo abs_y;
	double x;
	double y;
	double wheel_x;
	double wheel_y;
	int motion_dirty;
	int touch_down;
	int touch_pending;
	int button_pending[3];
} OnixDRMPointerDevice;

struct OnixDRMInput {
	int width;
	int height;
	int keyboard_fds[ONIX_DRM_INPUT_DEVICE_CAPACITY];
	int keyboard_count;
	OnixDRMPointerDevice pointers[ONIX_DRM_INPUT_DEVICE_CAPACITY];
	int pointer_count;
	double mouse_x;
	double mouse_y;
	OnixDRMInputEvent queue[ONIX_DRM_INPUT_QUEUE_CAPACITY];
	int head;
	int tail;
};

static int onix_drm_test_bit(const unsigned long *bits, int bit) {
	return (bits[ONIX_BIT_WORD(bit)] & ONIX_BIT_MASK(bit)) != 0;
}

static void onix_drm_pointer_reset(OnixDRMPointerDevice *device, int width, int height) {
	memset(device, 0, sizeof(*device));
	device->fd = -1;
	device->x = (double)width * 0.5;
	device->y = (double)height * 0.5;
	device->touch_pending = -1;
	for (int index = 0; index < 3; index++) {
		device->button_pending[index] = -1;
	}
}

static void onix_drm_input_push(OnixDRMInput *input, int kind, int code, double x, double y) {
	int next = (input->tail + 1) % ONIX_DRM_INPUT_QUEUE_CAPACITY;
	if (next == input->head) {
		return;
	}
	input->queue[input->tail] = (OnixDRMInputEvent){.kind = kind, .code = code, .x = x, .y = y};
	input->tail = next;
}

static int onix_drm_keycode(int code) {
	switch (code) {
	case KEY_0: return 48;
	case KEY_1: return 49;
	case KEY_2: return 50;
	case KEY_3: return 51;
	case KEY_4: return 52;
	case KEY_5: return 53;
	case KEY_6: return 54;
	case KEY_7: return 55;
	case KEY_8: return 56;
	case KEY_9: return 57;
	case KEY_A: return 65;
	case KEY_B: return 66;
	case KEY_C: return 67;
	case KEY_D: return 68;
	case KEY_E: return 69;
	case KEY_F: return 70;
	case KEY_G: return 71;
	case KEY_H: return 72;
	case KEY_I: return 73;
	case KEY_J: return 74;
	case KEY_K: return 75;
	case KEY_L: return 76;
	case KEY_M: return 77;
	case KEY_N: return 78;
	case KEY_O: return 79;
	case KEY_P: return 80;
	case KEY_Q: return 81;
	case KEY_R: return 82;
	case KEY_S: return 83;
	case KEY_T: return 84;
	case KEY_U: return 85;
	case KEY_V: return 86;
	case KEY_W: return 87;
	case KEY_X: return 88;
	case KEY_Y: return 89;
	case KEY_Z: return 90;
	case KEY_SPACE: return 32;
	case KEY_APOSTROPHE: return 39;
	case KEY_COMMA: return 44;
	case KEY_MINUS: return 45;
	case KEY_DOT: return 46;
	case KEY_SLASH: return 47;
	case KEY_SEMICOLON: return 59;
	case KEY_EQUAL: return 61;
	case KEY_LEFTBRACE: return 91;
	case KEY_BACKSLASH: return 92;
	case KEY_RIGHTBRACE: return 93;
	case KEY_GRAVE: return 96;
	case KEY_ESC: return 256;
	case KEY_ENTER: return 257;
	case KEY_TAB: return 258;
	case KEY_BACKSPACE: return 259;
	case KEY_INSERT: return 260;
	case KEY_DELETE: return 261;
	case KEY_RIGHT: return 262;
	case KEY_LEFT: return 263;
	case KEY_DOWN: return 264;
	case KEY_UP: return 265;
	case KEY_PAGEUP: return 266;
	case KEY_PAGEDOWN: return 267;
	case KEY_HOME: return 268;
	case KEY_END: return 269;
	case KEY_CAPSLOCK: return 280;
	case KEY_SCROLLLOCK: return 281;
	case KEY_NUMLOCK: return 282;
	case KEY_SYSRQ: return 283;
	case KEY_PAUSE: return 284;
	case KEY_F1: return 290;
	case KEY_F2: return 291;
	case KEY_F3: return 292;
	case KEY_F4: return 293;
	case KEY_F5: return 294;
	case KEY_F6: return 295;
	case KEY_F7: return 296;
	case KEY_F8: return 297;
	case KEY_F9: return 298;
	case KEY_F10: return 299;
	case KEY_F11: return 300;
	case KEY_F12: return 301;
	case KEY_KP0: return 320;
	case KEY_KP1: return 321;
	case KEY_KP2: return 322;
	case KEY_KP3: return 323;
	case KEY_KP4: return 324;
	case KEY_KP5: return 325;
	case KEY_KP6: return 326;
	case KEY_KP7: return 327;
	case KEY_KP8: return 328;
	case KEY_KP9: return 329;
	case KEY_KPDOT: return 330;
	case KEY_KPSLASH: return 331;
	case KEY_KPASTERISK: return 332;
	case KEY_KPMINUS: return 333;
	case KEY_KPPLUS: return 334;
	case KEY_KPENTER: return 335;
	case KEY_KPEQUAL: return 336;
	case KEY_LEFTSHIFT: return 340;
	case KEY_LEFTCTRL: return 341;
	case KEY_LEFTALT: return 342;
	case KEY_LEFTMETA: return 343;
	case KEY_RIGHTSHIFT: return 344;
	case KEY_RIGHTCTRL: return 345;
	case KEY_RIGHTALT: return 346;
	case KEY_RIGHTMETA: return 347;
	case KEY_MENU: return 348;
	default: return 0;
	}
}

static int onix_drm_mouse_button(int code) {
	switch (code) {
	case BTN_LEFT: return 0;
	case BTN_RIGHT: return 1;
	case BTN_MIDDLE: return 2;
	default: return -1;
	}
}

static double onix_drm_clamp(double value, double maximum) {
	if (value < 0) return 0;
	if (value > maximum) return maximum;
	return value;
}

static double onix_drm_absolute_position(int value, const struct input_absinfo *axis, int size) {
	int range = axis->maximum - axis->minimum;
	if (range <= 0 || size <= 1) return 0;
	return onix_drm_clamp((double)(value - axis->minimum) * (double)(size - 1) / (double)range, (double)(size - 1));
}

static int onix_drm_pointer_candidate(int fd, OnixDRMPointerDevice *candidate, int width, int height) {
	unsigned long event_bits[ONIX_BIT_COUNT(EV_MAX)] = {0};
	unsigned long key_bits[ONIX_BIT_COUNT(KEY_MAX)] = {0};
	unsigned long relative_bits[ONIX_BIT_COUNT(REL_MAX)] = {0};
	unsigned long absolute_bits[ONIX_BIT_COUNT(ABS_MAX)] = {0};
	if (ioctl(fd, EVIOCGBIT(0, sizeof(event_bits)), event_bits) < 0 || !onix_drm_test_bit(event_bits, EV_KEY)) {
		return 0;
	}
	if (ioctl(fd, EVIOCGBIT(EV_KEY, sizeof(key_bits)), key_bits) < 0) {
		return 0;
	}
	int has_relative = 0;
	if (onix_drm_test_bit(event_bits, EV_REL) && ioctl(fd, EVIOCGBIT(EV_REL, sizeof(relative_bits)), relative_bits) >= 0) {
		has_relative = onix_drm_test_bit(relative_bits, REL_X) && onix_drm_test_bit(relative_bits, REL_Y);
	}
	int abs_x = ABS_X;
	int abs_y = ABS_Y;
	int has_absolute = 0;
	if (onix_drm_test_bit(event_bits, EV_ABS) && ioctl(fd, EVIOCGBIT(EV_ABS, sizeof(absolute_bits)), absolute_bits) >= 0) {
		has_absolute = onix_drm_test_bit(absolute_bits, ABS_X) && onix_drm_test_bit(absolute_bits, ABS_Y);
		if (!has_absolute && onix_drm_test_bit(absolute_bits, ABS_MT_POSITION_X) && onix_drm_test_bit(absolute_bits, ABS_MT_POSITION_Y)) {
			abs_x = ABS_MT_POSITION_X;
			abs_y = ABS_MT_POSITION_Y;
			has_absolute = 1;
		}
	}
	int kind = ONIX_DRM_DEVICE_NONE;
	if (onix_drm_test_bit(key_bits, BTN_LEFT) && (has_relative || has_absolute)) {
		kind = ONIX_DRM_DEVICE_MOUSE;
	} else if (onix_drm_test_bit(key_bits, BTN_TOUCH) && has_absolute) {
		kind = ONIX_DRM_DEVICE_TOUCH;
	}
	if (kind == ONIX_DRM_DEVICE_NONE) {
		return 0;
	}
	onix_drm_pointer_reset(candidate, width, height);
	candidate->fd = fd;
	candidate->kind = kind;
	candidate->absolute = has_absolute;
	candidate->abs_x_code = abs_x;
	candidate->abs_y_code = abs_y;
	if (has_absolute) {
		if (ioctl(fd, EVIOCGABS(abs_x), &candidate->abs_x) < 0 || ioctl(fd, EVIOCGABS(abs_y), &candidate->abs_y) < 0) {
			return 0;
		}
		candidate->x = onix_drm_absolute_position(candidate->abs_x.value, &candidate->abs_x, width);
		candidate->y = onix_drm_absolute_position(candidate->abs_y.value, &candidate->abs_y, height);
	}
	return has_absolute ? 2 : 1;
}

static int onix_drm_keyboard_candidate(int fd) {
	unsigned long event_bits[ONIX_BIT_COUNT(EV_MAX)] = {0};
	unsigned long key_bits[ONIX_BIT_COUNT(KEY_MAX)] = {0};
	return ioctl(fd, EVIOCGBIT(0, sizeof(event_bits)), event_bits) >= 0 && onix_drm_test_bit(event_bits, EV_KEY) &&
		ioctl(fd, EVIOCGBIT(EV_KEY, sizeof(key_bits)), key_bits) >= 0 &&
		onix_drm_test_bit(key_bits, KEY_A) && onix_drm_test_bit(key_bits, KEY_Z) && onix_drm_test_bit(key_bits, KEY_ENTER);
}

static int onix_drm_event_device_name(const char *name) {
	if (name == NULL || strncmp(name, "event", 5) != 0 || name[5] == '\0') {
		return 0;
	}
	for (const char *cursor = name + 5; *cursor != '\0'; cursor++) {
		if (*cursor < '0' || *cursor > '9') {
			return 0;
		}
	}
	return 1;
}

static void onix_drm_input_add_device(OnixDRMInput *input, int fd, int width, int height) {
	if (input->keyboard_count < ONIX_DRM_INPUT_DEVICE_CAPACITY && onix_drm_keyboard_candidate(fd)) {
		int keyboard_fd = fcntl(fd, F_DUPFD_CLOEXEC, 0);
		if (keyboard_fd >= 0) {
			input->keyboard_fds[input->keyboard_count++] = keyboard_fd;
		}
	}
	if (input->pointer_count >= ONIX_DRM_INPUT_DEVICE_CAPACITY) {
		close(fd);
		return;
	}
	OnixDRMPointerDevice candidate;
	if (onix_drm_pointer_candidate(fd, &candidate, width, height) > 0) {
		input->pointers[input->pointer_count++] = candidate;
		return;
	}
	close(fd);
}

OnixDRMInput *onix_drm_input_create(int width, int height) {
	if (width <= 0 || height <= 0) {
		return NULL;
	}
	OnixDRMInput *input = calloc(1, sizeof(*input));
	if (input == NULL) {
		return NULL;
	}
	input->width = width;
	input->height = height;
	input->mouse_x = (double)width * 0.5;
	input->mouse_y = (double)height * 0.5;
	DIR *directory = opendir("/dev/input");
	if (directory == NULL) {
		return input;
	}
	int directory_fd = dirfd(directory);
	struct dirent *entry;
	while ((entry = readdir(directory)) != NULL) {
		if (!onix_drm_event_device_name(entry->d_name)) {
			continue;
		}
		int fd = openat(directory_fd, entry->d_name, O_RDONLY | O_NONBLOCK | O_CLOEXEC | O_NOFOLLOW);
		if (fd >= 0) {
			onix_drm_input_add_device(input, fd, width, height);
		}
	}
	closedir(directory);
	return input;
}

static void onix_drm_flush_pointer(OnixDRMInput *input, OnixDRMPointerDevice *device) {
	if (device->kind == ONIX_DRM_DEVICE_TOUCH) {
		if (device->touch_pending == 1) {
			device->touch_down = 1;
			onix_drm_input_push(input, ONIX_DRM_INPUT_TOUCH_START, 0, device->x, device->y);
		} else if (device->touch_pending == 0) {
			device->touch_down = 0;
			onix_drm_input_push(input, ONIX_DRM_INPUT_TOUCH_END, 0, device->x, device->y);
		} else if (device->motion_dirty && device->touch_down) {
			onix_drm_input_push(input, ONIX_DRM_INPUT_TOUCH_MOVE, 0, device->x, device->y);
		}
		device->touch_pending = -1;
		device->motion_dirty = 0;
		return;
	}
	if (device->motion_dirty) {
		input->mouse_x = device->x;
		input->mouse_y = device->y;
	}
	if (device->motion_dirty) {
		onix_drm_input_push(input, ONIX_DRM_INPUT_MOUSE_MOVE, 0, device->x, device->y);
		device->motion_dirty = 0;
	}
	for (int button = 0; button < 3; button++) {
		if (device->button_pending[button] >= 0) {
			onix_drm_input_push(input, device->button_pending[button] ? ONIX_DRM_INPUT_MOUSE_DOWN : ONIX_DRM_INPUT_MOUSE_UP, button, device->x, device->y);
			device->button_pending[button] = -1;
		}
	}
	if (device->wheel_x != 0 || device->wheel_y != 0) {
		onix_drm_input_push(input, ONIX_DRM_INPUT_MOUSE_WHEEL, 0, device->wheel_x, device->wheel_y);
		device->wheel_x = 0;
		device->wheel_y = 0;
	}
}

static void onix_drm_handle_pointer_event(OnixDRMInput *input, OnixDRMPointerDevice *device, const struct input_event *event) {
	if (event->type == EV_REL) {
		switch (event->code) {
		case REL_X:
			device->x = onix_drm_clamp(device->x + event->value, input->width - 1);
			device->motion_dirty = 1;
			break;
		case REL_Y:
			device->y = onix_drm_clamp(device->y + event->value, input->height - 1);
			device->motion_dirty = 1;
			break;
		case REL_HWHEEL:
			device->wheel_x += event->value;
			break;
		case REL_WHEEL:
			device->wheel_y += event->value;
			break;
		}
	} else if (event->type == EV_ABS) {
		if (event->code == device->abs_x_code) {
			device->x = onix_drm_absolute_position(event->value, &device->abs_x, input->width);
			device->motion_dirty = 1;
		} else if (event->code == device->abs_y_code) {
			device->y = onix_drm_absolute_position(event->value, &device->abs_y, input->height);
			device->motion_dirty = 1;
		}
	} else if (event->type == EV_KEY) {
		if (device->kind == ONIX_DRM_DEVICE_TOUCH && event->code == BTN_TOUCH) {
			device->touch_pending = event->value != 0;
		} else {
			int button = onix_drm_mouse_button(event->code);
			if (button >= 0) device->button_pending[button] = event->value != 0;
		}
	} else if (event->type == EV_SYN && event->code == SYN_REPORT) {
		onix_drm_flush_pointer(input, device);
	}
}

static void onix_drm_poll_keyboard(OnixDRMInput *input, int fd) {
	struct input_event events[32];
	for (;;) {
		ssize_t size = read(fd, events, sizeof(events));
		if (size <= 0) return;
		int count = (int)(size / (ssize_t)sizeof(events[0]));
		for (int index = 0; index < count; index++) {
			if (events[index].type != EV_KEY || events[index].value < 0 || events[index].value > 2) continue;
			int code = onix_drm_keycode(events[index].code);
			if (code != 0) onix_drm_input_push(input, events[index].value == 0 ? ONIX_DRM_INPUT_KEY_UP : ONIX_DRM_INPUT_KEY_DOWN, code, 0, 0);
		}
	}
}

static void onix_drm_poll_pointer(OnixDRMInput *input, OnixDRMPointerDevice *device) {
	struct input_event events[32];
	if (device->kind == ONIX_DRM_DEVICE_MOUSE && !device->absolute) {
		device->x = input->mouse_x;
		device->y = input->mouse_y;
	}
	for (;;) {
		ssize_t size = read(device->fd, events, sizeof(events));
		if (size <= 0) return;
		int count = (int)(size / (ssize_t)sizeof(events[0]));
		for (int index = 0; index < count; index++) {
			onix_drm_handle_pointer_event(input, device, &events[index]);
		}
	}
}

void onix_drm_input_poll(OnixDRMInput *input) {
	if (input == NULL) return;
	for (int index = 0; index < input->keyboard_count; index++) {
		onix_drm_poll_keyboard(input, input->keyboard_fds[index]);
	}
	for (int index = 0; index < input->pointer_count; index++) {
		onix_drm_poll_pointer(input, &input->pointers[index]);
	}
}

void onix_drm_input_wait(OnixDRMInput *input, int wake_fd, int timeout_ms) {
	if (input == NULL || input->head != input->tail) return;
	struct pollfd fds[ONIX_DRM_INPUT_DEVICE_CAPACITY * 2 + 1];
	int count = 0;
	fds[count++] = (struct pollfd){.fd = wake_fd, .events = POLLIN};
	for (int index = 0; index < input->keyboard_count; index++) fds[count++] = (struct pollfd){.fd = input->keyboard_fds[index], .events = POLLIN};
	for (int index = 0; index < input->pointer_count; index++) fds[count++] = (struct pollfd){.fd = input->pointers[index].fd, .events = POLLIN};
	poll(fds, count, timeout_ms);
}

int onix_drm_input_pop(OnixDRMInput *input, OnixDRMInputEvent *event) {
	if (input == NULL || event == NULL || input->head == input->tail) return 0;
	*event = input->queue[input->head];
	input->head = (input->head + 1) % ONIX_DRM_INPUT_QUEUE_CAPACITY;
	return 1;
}

int onix_drm_input_has_keyboard(OnixDRMInput *input) {
	return input != NULL && input->keyboard_count > 0;
}

int onix_drm_input_has_pointer(OnixDRMInput *input) {
	return input != NULL && input->pointer_count > 0;
}

void onix_drm_input_destroy(OnixDRMInput *input) {
	if (input == NULL) return;
	for (int index = 0; index < input->keyboard_count; index++) {
		close(input->keyboard_fds[index]);
	}
	for (int index = 0; index < input->pointer_count; index++) {
		if (input->pointers[index].fd >= 0) close(input->pointers[index].fd);
	}
	free(input);
}
