package terminal

import (
	"fmt"
	"io"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func TestHomeEndHonorCursorMode(t *testing.T) {
	term := vt.NewEmulator(40, 5)
	defer term.Close()
	s := &Session{Term: term, mouseModes: make(map[ansi.Mode]bool)}
	s.bindCallbacks()
	for _, mode := range []string{"\x1b[?1h", "\x1b[?1l", "\x1b[?1h", "\x1bc"} {
		fmt.Fprint(term, mode)
		prefix := "\x1b["
		if mode == "\x1b[?1h" {
			prefix = "\x1bO"
		}
		for key, final := range map[rune]string{uv.KeyHome: "H", uv.KeyEnd: "F"} {
			got := make(chan string, 1)
			go func() {
				data := make([]byte, 3)
				_, _ = io.ReadFull(term, data)
				got <- string(data)
			}()
			s.SendKey(uv.KeyPressEvent{Code: key})
			select {
			case output := <-got:
				if output != prefix+final {
					t.Fatalf("mode %q: got %q, want %q", mode, output, prefix+final)
				}
			case <-time.After(time.Second):
				t.Fatal("navigation key was not sent")
			}
		}
	}
}
