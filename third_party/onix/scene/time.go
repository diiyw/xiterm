package scene

import "time"

type Time struct {
	last    time.Time
	Delta   time.Duration
	Elapsed time.Duration
	Frame   uint64
}

func NewTime() *Time {
	return &Time{}
}

func (t *Time) Advance(now time.Time) {
	if t == nil {
		return
	}
	if t.last.IsZero() {
		t.last = now
		t.Delta = 0
		return
	}
	t.Delta = now.Sub(t.last)
	t.Elapsed += t.Delta
	t.last = now
	t.Frame++
}

func (t *Time) Set(delta time.Duration, elapsed time.Duration, frame uint64) {
	if t == nil {
		return
	}
	if delta < 0 {
		delta = 0
	}
	t.Delta = delta
	t.Elapsed = elapsed
	t.Frame = frame
	t.last = time.Time{}
}

func (t *Time) DeltaSeconds() float32 {
	if t == nil {
		return 0
	}
	return float32(t.Delta.Seconds())
}

func (t *Time) ElapsedSeconds() float32 {
	if t == nil {
		return 0
	}
	return float32(t.Elapsed.Seconds())
}

type FixedTime struct {
	Timestep time.Duration
	Overstep time.Duration
	Delta    time.Duration
	Elapsed  time.Duration
	Frame    uint64
}

func NewFixedTime(timestep time.Duration) *FixedTime {
	f := &FixedTime{}
	f.SetTimestep(timestep)
	return f
}

func (t *FixedTime) SetTimestep(timestep time.Duration) {
	if t == nil || timestep <= 0 {
		return
	}
	t.Timestep = timestep
}

func (t *FixedTime) Accumulate(delta time.Duration, maxDelta time.Duration) {
	if t == nil || t.Timestep <= 0 {
		return
	}
	if delta < 0 {
		delta = 0
	}
	if maxDelta > 0 && delta > maxDelta {
		delta = maxDelta
	}
	t.Overstep += delta
}

func (t *FixedTime) Expend() bool {
	if t == nil || t.Timestep <= 0 || t.Overstep < t.Timestep {
		return false
	}
	t.Overstep -= t.Timestep
	t.Delta = t.Timestep
	t.Elapsed += t.Timestep
	t.Frame++
	return true
}

func (t *FixedTime) DeltaSeconds() float32 {
	if t == nil {
		return 0
	}
	return float32(t.Delta.Seconds())
}

func (t *FixedTime) ElapsedSeconds() float32 {
	if t == nil {
		return 0
	}
	return float32(t.Elapsed.Seconds())
}

func (t *FixedTime) OverstepFraction() float32 {
	if t == nil || t.Timestep <= 0 {
		return 0
	}
	return float32(t.Overstep.Seconds() / t.Timestep.Seconds())
}
