#ifndef ONIX_NATIVE_BUFFER_H
#define ONIX_NATIVE_BUFFER_H

#include <stdint.h>
#include <stdlib.h>

#define ONIX_BUFFER_TRIM_DELAY 30u

typedef struct {
	uint64_t instances;
	uint64_t staging;
	uint64_t readback;
	uint64_t scratch;
} OnixBufferMemory;

typedef struct {
	uint64_t peak;
	uint32_t frames;
} OnixBufferRetention;

// Observe one completed frame. Keep twice the recent demand after sustained
// underuse, and reset the window whenever demand exceeds a quarter of capacity.
static inline uint64_t onix_buffer_trim_capacity(OnixBufferRetention *retention, uint64_t capacity, uint64_t used, uint64_t minimum) {
	if (capacity <= minimum || used > capacity / 4) {
		*retention = (OnixBufferRetention){0};
		return 0;
	}
	if (used > retention->peak) retention->peak = used;
	if (++retention->frames < ONIX_BUFFER_TRIM_DELAY) return 0;
	uint64_t target = retention->peak * 2;
	*retention = (OnixBufferRetention){0};
	return target > minimum ? target : minimum;
}

// Shrink metadata without moving live entries relative to one another. A failed
// realloc leaves both the allocation and its recorded capacity unchanged.
// Minimum is a nonzero power of two, matching the arrays' growth granularity.
static inline void *onix_buffer_trim_array(void *entries, uint32_t *capacity, uint32_t used, size_t stride, uint32_t minimum, OnixBufferRetention *retention) {
	uint64_t target = onix_buffer_trim_capacity(retention, *capacity, used, minimum);
	if (!target) return entries;
	uint32_t smaller = minimum;
	while (smaller < target) smaller *= 2;
	void *replacement = realloc(entries, (size_t)smaller * stride);
	if (!replacement) return entries;
	*capacity = smaller;
	return replacement;
}

// Small frames start with a small block. Additional blocks grow geometrically
// up to the normal bulk-upload size, without inflating the first allocation.
static inline uint64_t onix_buffer_block_capacity(uint64_t required, uint64_t minimum, uint64_t previous, uint64_t bulk) {
	// Large uploads round to the bulk granularity instead of doubling their
	// entire payload; a 9 MiB staging request needs 12 MiB, not 16 MiB.
	if (required > bulk) {
		if (required > UINT64_MAX - (bulk - 1)) return 0;
		return (required + bulk - 1) & ~(bulk - 1);
	}
	uint64_t capacity = previous >= bulk ? bulk : previous > minimum ? previous : minimum;
	if (previous && capacity < bulk) capacity *= 2;
	while (capacity < required && capacity <= UINT64_MAX / 2) capacity *= 2;
	return capacity >= required ? capacity : 0;
}

#endif
