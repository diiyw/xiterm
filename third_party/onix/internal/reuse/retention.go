package reuse

// Retention releases historical peaks after sustained underuse. Record folds
// multiple batches into one observation; Trim observes once per completed frame.
type Retention struct {
	used, peak, frames int
}

const RetentionFrames = 30

func (r *Retention) Record(length int) {
	r.used = max(r.used, length)
}

// Trim preserves live values and keeps twice the recent peak, with a small
// capacity floor. Empty, unused storage is released entirely.
func (r *Retention) Trim[T any](values []T, minimum int) []T {
	target := r.TrimCapacity(cap(values), len(values), minimum)
	if target < 0 {
		return nil
	}
	if target == 0 {
		return values
	}
	result := make([]T, len(values), target)
	copy(result, values)
	return result
}

// TrimCapacity returns zero while storage stays warm, -1 when idle storage can
// be released, or a smaller capacity after sustained underuse.
func (r *Retention) TrimCapacity(capacity, length, minimum int) int {
	used := max(length, r.used)
	r.used = 0
	if capacity <= minimum || used > capacity/4 {
		r.peak, r.frames = 0, 0
		return 0
	}
	r.peak = max(r.peak, used)
	r.frames++
	if r.frames < RetentionFrames {
		return 0
	}
	target := max(minimum, r.peak*2)
	idle := r.peak == 0
	*r = Retention{}
	if idle {
		return -1
	}
	return target
}
