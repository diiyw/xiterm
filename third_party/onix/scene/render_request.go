package scene

import "sync/atomic"

type RenderRequest struct {
	wake      func()
	requested atomic.Bool
}

func NewRenderRequest(wake func()) *RenderRequest { return &RenderRequest{wake: wake} }

func (r *RenderRequest) Request() {
	if r != nil {
		r.requested.Store(true)
		if r.wake != nil {
			r.wake()
		}
	}
}

func (r *RenderRequest) Take() bool {
	if r == nil {
		return false
	}
	return r.requested.Swap(false)
}

func (r *RenderRequest) Pending() bool { return r != nil && r.requested.Load() }
