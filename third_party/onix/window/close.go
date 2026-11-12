package window

type closeRequestSupport struct {
	handler func() bool
	close   func()
}

func (s *closeRequestSupport) OnCloseRequested(handler func() bool) {
	s.handler = handler
}

func (s *closeRequestSupport) RequestClose() {
	if s.close != nil {
		s.close()
	}
}

func (s *closeRequestSupport) acceptCloseRequest() bool {
	return s.handler == nil || s.handler()
}
