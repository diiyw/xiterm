package render

const bufferTrimDelay = 30

type bufferRetention struct {
	peak   int
	frames int
}

// trimCapacity observes one completed frame, retaining twice the largest demand
// in a sustained low-load window. Zero means the current capacity stays warm.
func (r *bufferRetention) trimCapacity(capacity, used, minimum int) int {
	if capacity <= minimum || used > capacity/4 {
		*r = bufferRetention{}
		return 0
	}
	r.peak = max(r.peak, used)
	r.frames++
	if r.frames < bufferTrimDelay {
		return 0
	}
	target := max(minimum, r.peak*2)
	*r = bufferRetention{}
	return target
}

func (retention *bufferRetention) trimUploadBuffer[T any](buffer []T, used, minimum int) []T {
	idle := retention.peak == 0 && used == 0
	if capacity := retention.trimCapacity(cap(buffer), used, minimum); capacity != 0 {
		if idle {
			return nil
		}
		return make([]T, 0, capacity)
	}
	return buffer[:0]
}

// scratchRetention measures the largest batch between frame starts, including
// batches overwritten or flushed before the frame ends. Scratch is consumed
// synchronously and never owns resources referenced by submitted GPU work.
type scratchRetention struct {
	retention bufferRetention
	used      int
}

func (retention *scratchRetention) resetScratch[T any](buffer []T) []T {
	retention.used = max(retention.used, len(buffer))
	return buffer[:0]
}

func (retention *scratchRetention) trimScratch[T any](buffer []T, minimum int) []T {
	used := max(retention.used, len(buffer))
	retention.used = 0
	return (&retention.retention).trimUploadBuffer(buffer, used, minimum)
}
