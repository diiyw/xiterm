//go:build linux && cgo && onix_drm

package window

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

type directSignalSupport struct {
	channel chan os.Signal
	pending *atomic.Bool
	stop    chan struct{}
}

func newDirectSignalSupport(wake func()) directSignalSupport {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	pending, stop := &atomic.Bool{}, make(chan struct{})
	go func() {
		for {
			select {
			case <-channel:
				pending.Store(true)
				wake()
			case <-stop:
				return
			}
		}
	}()
	return directSignalSupport{channel: channel, pending: pending, stop: stop}
}

func (s *directSignalSupport) requested() bool {
	return s.pending != nil && s.pending.Swap(false)
}

func (s *directSignalSupport) destroy() {
	if s.channel != nil {
		signal.Stop(s.channel)
		close(s.stop)
		s.channel = nil
	}
}
