package reuse

// MapRetention tracks the largest live population because Go maps expose no
// capacity. Rebuilding after sustained underuse releases historical buckets.
type MapRetention struct {
	retention Retention
	capacity  int
}

func (r *MapRetention) Record(length int) { r.capacity = max(r.capacity, length) }

func (r *MapRetention) Trim[K comparable, V any](values map[K]V, minimum int) map[K]V {
	if values == nil {
		*r = MapRetention{}
		return nil
	}
	r.Record(len(values))
	target := r.retention.TrimCapacity(r.capacity, len(values), minimum)
	if target == 0 {
		return values
	}
	result := make(map[K]V, max(0, target))
	for key, value := range values {
		result[key] = value
	}
	r.capacity = max(0, target)
	return result
}

// Compact is used once a containing owner's retention window has expired.
func Compact[T any](values []T, minimum int) []T {
	if cap(values) <= minimum || len(values) > cap(values)/4 {
		return values
	}
	if len(values) == 0 {
		return nil
	}
	result := make([]T, len(values), max(minimum, len(values)*2))
	copy(result, values)
	return result
}
