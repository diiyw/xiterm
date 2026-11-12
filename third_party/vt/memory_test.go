package vt

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestControlStringBufferGrowsWithinLimitAndTrimsAfterCompletion(t *testing.T) {
	e := NewEmulator(20, 3)
	defer e.Close()
	if cap(e.parser.Data()) != initialParserDataSize {
		t.Fatal("emulator allocated its maximum parser buffer at startup")
	}
	payload := strings.Repeat("x", 180000)
	e.WriteString("\x1b]2;" + payload[:90000])
	e.TrimBuffers()
	e.WriteString(payload[90000:] + "\x1b\\")
	if e.title != payload {
		t.Fatal("growing or trimming a split OSC changed its data")
	}
	e.TrimBuffers()
	if cap(e.parser.Data()) != initialParserDataSize {
		t.Fatal("completed large OSC retained its buffer")
	}
	e.WriteString("\x1b]2;next\a")
	if e.title != "next" {
		t.Fatal("trimmed parser lost its handlers")
	}
	e.WriteString("\x1b]2;" + strings.Repeat("y", maxParserDataSize+100) + "\a")
	if cap(e.parser.Data()) > maxParserDataSize || len(e.title) > maxParserDataSize {
		t.Fatal("control string bypassed the parser memory limit")
	}
}

func TestAlternateScreenDoesNotRetainInaccessibleHistory(t *testing.T) {
	e := NewEmulator(160, 3)
	defer e.Close()
	e.WriteString("main\r\n\x1b[?1049h")
	e.WriteString(strings.Repeat(strings.Repeat("x", 140)+"\r\n", 12000))
	if e.scrs[1].scrollback != nil {
		t.Fatal("alternate screen retains an inaccessible history")
	}
	e.WriteString("\x1b[?1049l")
	if !strings.Contains(e.String(), "main") {
		t.Fatal("alternate screen damaged main-screen content")
	}
}

func TestScrollbackSinkBorrowsRowsWithoutAllocations(t *testing.T) {
	sb := NewScrollback(100)
	line := uv.Line{{Content: "x", Width: 1}, uv.EmptyCell}
	calls := 0
	sb.SetSink(func(row uv.Line) {
		calls++
		if len(row) != 1 || &row[0] != &line[0] {
			t.Fatal("sink did not receive a trimmed borrowed row")
		}
	})
	if allocs := testing.AllocsPerRun(100, func() { sb.Push(line) }); allocs != 0 {
		t.Fatalf("scroll sink allocated %.0f times", allocs)
	}
	if calls == 0 || sb.Len() != 0 || sb.lines != nil {
		t.Fatal("sink retained a second copy")
	}
}
