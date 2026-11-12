package window

import (
	"sync"
	"sync/atomic"
	"time"
)

type loopControl struct {
	wake         func()
	idleDeadline func() (time.Time, bool)
	wakeMutex    sync.Mutex
	frameRequest atomic.Bool
}

func (c *loopControl) RequestFrame()          { c.frameRequest.Store(true); c.Wake() }
func (c *loopControl) FrameRequested() bool   { return c.frameRequest.Load() }
func (c *loopControl) TakeFrameRequest() bool { return c.frameRequest.Swap(false) }

// SetIdleDeadline installs the loop-thread scheduler's next idle deadline.
// A zero deadline with idle=true means wait until a platform event or Wake.
func (c *loopControl) SetIdleDeadline(deadline func() (time.Time, bool)) { c.idleDeadline = deadline }

// Wake interrupts native idle waits and is safe to call from other goroutines.
func (c *loopControl) Wake() {
	c.wakeMutex.Lock()
	if c.wake != nil {
		c.wake()
	}
	c.wakeMutex.Unlock()
}

func (c *loopControl) idleWait() (time.Duration, bool) {
	if c.idleDeadline == nil {
		return 0, false
	}
	deadline, idle := c.idleDeadline()
	if !idle {
		return 0, false
	}
	if deadline.IsZero() {
		return -1, true
	}
	return max(time.Until(deadline), 0), true
}

func idleTimeoutMillis(wait time.Duration) int {
	if wait < 0 {
		return -1
	}
	milliseconds := wait / time.Millisecond
	if wait%time.Millisecond != 0 {
		milliseconds++
	}
	return int(min(milliseconds, time.Duration(2147483647)))
}
