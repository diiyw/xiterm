package window

// mouseCaptureSupport keeps captured motion separate from absolute UI positions.
type mouseCaptureSupport struct {
	applyMouseCapture func(bool) bool
	mouseCaptured     bool
	mouseDeltaX       float64
	mouseDeltaY       float64
}

// SetMouseCaptured hides and locks the mouse for unlimited relative movement.
// It returns whether the platform accepted the request. Activation may be
// asynchronous; MouseCaptured reports the actual state. Call on the window thread.
func (s *mouseCaptureSupport) SetMouseCaptured(captured bool) bool {
	return s.applyMouseCapture != nil && s.applyMouseCapture(captured)
}

// MouseCaptured reports whether the platform currently owns the mouse lock.
func (s *mouseCaptureSupport) MouseCaptured() bool {
	return s.mouseCaptured
}

// TakeMouseDelta consumes motion accumulated since the last call, in native
// window coordinates. Absolute cursor movement and capture transitions are excluded.
func (s *mouseCaptureSupport) TakeMouseDelta() (float64, float64) {
	x, y := s.mouseDeltaX, s.mouseDeltaY
	s.mouseDeltaX, s.mouseDeltaY = 0, 0
	return x, y
}

func (s *mouseCaptureSupport) updateMouseCapture(captured bool) {
	if s.mouseCaptured != captured {
		s.mouseDeltaX, s.mouseDeltaY = 0, 0
		s.mouseCaptured = captured
	}
}

func (s *mouseCaptureSupport) addMouseDelta(x, y float64) {
	if s.mouseCaptured {
		s.mouseDeltaX += x
		s.mouseDeltaY += y
	}
}
