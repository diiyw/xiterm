package terminal

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func TestOSC52WritesClipboard(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(80, 24)}
	defer s.Term.Close()
	s.bindCallbacks()
	var copied string
	var writes int
	s.WriteClipboard = func(text string) {
		copied = text
		writes++
	}

	text := strings.Repeat("复制 🖇\n", 8000)
	sequence := ansi.SetSystemClipboard(text)
	split := len(sequence) / 2
	_, _ = s.Term.WriteString(sequence[:split])
	if writes != 0 {
		t.Fatal("incomplete OSC 52 wrote to the clipboard")
	}
	_, _ = s.Term.WriteString(sequence[split:len(sequence)-1] + "\x1b\\")
	if writes != 1 || copied != text {
		t.Fatalf("split OSC 52: %d writes, copied %d of %d bytes", writes, len(copied), len(text))
	}

	_, _ = s.Term.WriteString("\x1b]52;;eA==\a")
	if writes != 2 || copied != "x" {
		t.Fatal("default clipboard target did not write")
	}
	_, _ = s.Term.WriteString("\x1b]52;pc;eQ==\a")
	if writes != 3 || copied != "y" {
		t.Fatal("combined clipboard targets did not write")
	}
	_, _ = s.Term.WriteString(ansi.ResetSystemClipboard)
	if writes != 4 || copied != "" {
		t.Fatal("empty OSC 52 did not clear the clipboard")
	}
}

func TestOSC52IgnoresUnsupportedRequests(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(80, 24)}
	defer s.Term.Close()
	s.bindCallbacks()
	s.WriteClipboard = func(text string) { t.Errorf("unexpected clipboard write of %d bytes", len(text)) }

	for _, sequence := range []string{
		ansi.RequestSystemClipboard,
		ansi.SetPrimaryClipboard("primary"),
		"\x1b]52;c\a",
		"\x1b]52;c;invalid!\a",
		"\x1b]52;c;eA==\x18",
		ansi.SetSystemClipboard("\x00"),
		ansi.SetSystemClipboard("\xff"),
		"\x1b]52;;" + strings.Repeat("eHh4", 1<<20) + "\a",
	} {
		_, _ = s.Term.WriteString(sequence)
	}
}
