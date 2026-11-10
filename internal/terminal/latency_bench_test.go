package terminal

import (
	"bytes"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

func BenchmarkPollBusyScreen(b *testing.B) {
	for _, fixture := range []struct {
		name string
		data []byte
	}{
		{"repaint", bytes.Repeat([]byte("\x1b[H\x1b[32mfrequently refreshed terminal input field\x1b[0m"), 600)},
		{"scroll", bytes.Repeat([]byte("terminal output with 中文 and a searchable marker\r\n"), 600)},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			s := &Session{Term: vt.NewEmulator(120, 40), output: make(chan []byte, 8)}
			defer s.Term.Close()
			s.initScrollback(10000)
			b.ReportAllocs()
			b.SetBytes(int64(8 * len(fixture.data)))
			var longest time.Duration
			var polls int
			for b.Loop() {
				for range 8 {
					s.output <- fixture.data
				}
				for s.PendingOutput() {
					started := time.Now()
					s.Poll()
					longest = max(longest, time.Since(started))
					polls++
				}
			}
			b.ReportMetric(float64(longest.Microseconds())/1000, "max-ms/poll")
			b.ReportMetric(float64(polls)/float64(b.N), "polls/op")
		})
	}
}
