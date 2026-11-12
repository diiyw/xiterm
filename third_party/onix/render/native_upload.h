#ifndef ONIX_NATIVE_UPLOAD_H
#define ONIX_NATIVE_UPLOAD_H

#include "native_buffer.h"

#include <stdint.h>
#include <stdlib.h>
#include <string.h>

// Cache only native-owned snapshots. In particular, never retain a cgo input
// pointer: callers may overwrite their scratch slices immediately after a draw.
typedef struct {
	uint64_t resource, offset, hash;
	size_t size;
	const void *data;
} OnixNativeUpload;

typedef struct {
	OnixNativeUpload *entries;
	uint32_t *buckets;
	uint32_t count, capacity;
	OnixBufferRetention retention;
} OnixNativeUploadCache;

static inline uint64_t onix_upload_hash(const void *data, size_t size) {
	const uint8_t *bytes = data;
	uint64_t hash = UINT64_C(14695981039346656037) ^ size;
	while (size >= 8) {
		uint64_t word;
		memcpy(&word, bytes, 8);
		hash = (hash ^ word) * UINT64_C(1099511628211);
		bytes += 8;
		size -= 8;
	}
	while (size--) hash = (hash ^ *bytes++) * UINT64_C(1099511628211);
	hash ^= hash >> 33;
	hash *= UINT64_C(0xff51afd7ed558ccd);
	return hash ^ (hash >> 33);
}

static inline int onix_upload_cache_resize(OnixNativeUploadCache *cache, uint32_t capacity) {
	if (capacity < cache->count || capacity > UINT32_MAX / 2 ||
		(uint64_t)capacity * sizeof(*cache->entries) > SIZE_MAX || (uint64_t)capacity * 2 * sizeof(*cache->buckets) > SIZE_MAX) return 0;
	uint32_t *buckets = calloc((size_t)capacity * 2, sizeof(*buckets));
	if (!buckets) return 0;
	OnixNativeUpload *entries = realloc(cache->entries, (size_t)capacity * sizeof(*entries));
	if (!entries) { free(buckets); return 0; }
	for (uint32_t index = 0; index < cache->count; index++) {
		uint32_t bucket = (uint32_t)entries[index].hash & (capacity * 2 - 1);
		while (buckets[bucket]) bucket = (bucket + 1) & (capacity * 2 - 1);
		buckets[bucket] = index + 1;
	}
	free(cache->buckets);
	cache->entries = entries;
	cache->buckets = buckets;
	cache->capacity = capacity;
	return 1;
}

// Only reset after the frame using these snapshots has completed. Resize at
// the boundary, where entries are dead and no source pointers need copying.
static inline void onix_upload_cache_reset(OnixNativeUploadCache *cache) {
	uint64_t target = onix_buffer_trim_capacity(&cache->retention, cache->capacity, cache->count, 16);
	cache->count = 0;
	if (target) {
		uint32_t capacity = 16;
		while (capacity < target) capacity *= 2;
		if (onix_upload_cache_resize(cache, capacity)) return;
	}
	if (cache->capacity) memset(cache->buckets, 0, (size_t)cache->capacity * 2 * sizeof(*cache->buckets));
}

static inline int onix_upload_cache_find(const OnixNativeUploadCache *cache, const void *data, size_t size, uint64_t hash, size_t alignment, OnixNativeUpload *upload) {
	if (!cache->capacity) return 0;
	uint32_t mask = cache->capacity * 2 - 1, bucket = (uint32_t)hash & mask;
	while (cache->buckets[bucket]) {
		const OnixNativeUpload *entry = &cache->entries[cache->buckets[bucket] - 1];
		if (entry->hash == hash && entry->size == size && (entry->offset & (alignment - 1)) == 0 && memcmp(entry->data, data, size) == 0) {
			*upload = *entry;
			return 1;
		}
		bucket = (bucket + 1) & mask;
	}
	return 0;
}

static inline int onix_upload_cache_insert(OnixNativeUploadCache *cache, OnixNativeUpload upload) {
	if (cache->count == cache->capacity) {
		uint32_t capacity = cache->capacity ? cache->capacity * 2 : 16;
		if (capacity < cache->capacity || !onix_upload_cache_resize(cache, capacity)) return 0;
	}
	uint32_t mask = cache->capacity * 2 - 1, bucket = (uint32_t)upload.hash & mask;
	while (cache->buckets[bucket]) bucket = (bucket + 1) & mask;
	cache->entries[cache->count] = upload;
	cache->buckets[bucket] = ++cache->count;
	return 1;
}

static inline uint64_t onix_upload_cache_bytes(const OnixNativeUploadCache *cache) {
	return (uint64_t)cache->capacity * (sizeof(*cache->entries) + 2 * sizeof(*cache->buckets));
}

static inline void onix_upload_cache_destroy(OnixNativeUploadCache *cache) {
	free(cache->entries);
	free(cache->buckets);
	*cache = (OnixNativeUploadCache){0};
}

#endif
