//go:build windows

package terminal

import (
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/internal/config"
)

func waitWindows(t *testing.T, s *Session, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		s.Poll()
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("ConPTY timed out: screen=%q exited=%v err=%v", s.Term.String(), s.Exited, s.ExitErr)
}
func TestConPTYInputResizeAndExit(t *testing.T) {
	s, err := Start(config.Profile{Name: "Test", Shell: "powershell.exe", Args: []string{"-NoLogo", "-NoProfile", "-Command", "[Console]::WriteLine('READY'); $v=[Console]::ReadLine(); [Console]::WriteLine('INPUT='+$v); exit 7"}, Directory: t.TempDir()}, 80, 24, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	waitWindows(t, s, func() bool { return strings.Contains(s.Term.String(), "READY") })
	if err = s.Resize(97, 31); err != nil {
		t.Fatal(err)
	}
	s.SendText("hello")
	s.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitWindows(t, s, func() bool { return s.Exited && strings.Contains(s.Term.String(), "INPUT=hello") })
	if s.ExitErr == nil {
		t.Fatal("nonzero shell exit must be reported")
	}
}
func TestConPTYCloseEndsProcess(t *testing.T) {
	s, err := Start(config.Profile{Name: "Test", Shell: "powershell.exe", Args: []string{"-NoLogo", "-NoProfile", "-Command", "[Console]::WriteLine('READY'); Start-Sleep -Seconds 120"}, Directory: t.TempDir()}, 80, 24, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	waitWindows(t, s, func() bool { return strings.Contains(s.Term.String(), "READY") })
	s.Close()
	s.Close()
	waitWindows(t, s, func() bool { return s.Exited })
}
