#ifndef ONIX_NATIVE_SLOTS_H
#define ONIX_NATIVE_SLOTS_H

#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	uint32_t count, capacity, free;
} OnixNativeSlots;

// Entries carry free_next themselves. Acquisition only prepares storage; insert
// commits the slot after resource creation succeeds. Vulkan recycles after its
// frame fence, while Metal retains retired objects separately from their slots.
#define ONIX_NATIVE_SLOTS(prefix, type, initial_capacity, limit) \
static int prefix##_acquire(type **entries, OnixNativeSlots *slots, uint32_t *index) { \
	if (slots->free != 0) { *index = slots->free - 1; return 1; } \
	if (slots->count == (limit)) return 0; \
	if (slots->count == slots->capacity) { \
		uint32_t capacity = slots->capacity == 0 ? (initial_capacity) \
			: slots->capacity > (limit) / 2 ? (limit) : slots->capacity * 2; \
		if ((uint64_t)capacity * sizeof(type) > SIZE_MAX) return 0; \
		type *grown = realloc(*entries, (size_t)capacity * sizeof(type)); \
		if (grown == NULL) return 0; \
		memset(grown + slots->capacity, 0, (size_t)(capacity - slots->capacity) * sizeof(type)); \
		*entries = grown; slots->capacity = capacity; \
	} \
	*index = slots->count; return 1; \
} \
static void prefix##_insert(type *entries, OnixNativeSlots *slots, uint32_t index, type value) { \
	if (index == slots->count) slots->count++; \
	else slots->free = entries[index].free_next; \
	entries[index] = value; \
} \
static void prefix##_recycle(type *entries, OnixNativeSlots *slots, uint32_t index) { \
	entries[index] = (type){.free_next = slots->free}; \
	slots->free = index + 1; \
}

#endif
