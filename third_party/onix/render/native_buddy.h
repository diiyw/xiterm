#ifndef ONIX_NATIVE_BUDDY_H
#define ONIX_NATIVE_BUDDY_H

#include <stdint.h>
#include <stdlib.h>

// A native block owns its metadata once. Leases carry tree indices, so neither
// allocation nor coalescing needs a temporary list or per-mesh heap allocation.
typedef struct {
	uint8_t *states;
	uint64_t size, minimum;
} OnixNativeBuddy;

static inline int onix_buddy_init(OnixNativeBuddy *buddy, uint64_t size, uint64_t minimum) {
	if (size < minimum || size / minimum > SIZE_MAX / 2) return 0;
	uint8_t *states = calloc((size_t)(size / minimum) * 2, 1);
	if (!states) return 0;
	*buddy = (OnixNativeBuddy){.states = states, .size = size, .minimum = minimum};
	return 1;
}

static inline uint64_t onix_buddy_take_node(OnixNativeBuddy *buddy, uint64_t node, uint64_t span, uint64_t required, uint64_t base, uint64_t *offset) {
	if (buddy->states[node] == 2 || span < required) return 0;
	if (span / 2 < required || span == buddy->minimum) {
		if (buddy->states[node]) return 0;
		buddy->states[node] = 2;
		*offset = base;
		return node;
	}
	uint64_t result = onix_buddy_take_node(buddy, node * 2, span / 2, required, base, offset);
	if (!result) result = onix_buddy_take_node(buddy, node * 2 + 1, span / 2, required, base + span / 2, offset);
	if (result) buddy->states[node] = buddy->states[node * 2] == 2 && buddy->states[node * 2 + 1] == 2 ? 2 : 1;
	return result;
}

static inline uint64_t onix_buddy_take(OnixNativeBuddy *buddy, uint64_t size, uint64_t *offset) {
	return onix_buddy_take_node(buddy, 1, buddy->size, size, 0, offset);
}

static inline void onix_buddy_release(OnixNativeBuddy *buddy, uint64_t node) {
	buddy->states[node] = 0;
	while (node > 1) {
		node /= 2;
		uint8_t left = buddy->states[node * 2], right = buddy->states[node * 2 + 1];
		buddy->states[node] = left == 0 && right == 0 ? 0 : left == 2 && right == 2 ? 2 : 1;
	}
}

#endif
