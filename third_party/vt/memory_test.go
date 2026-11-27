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
	e := NewEmulator(80, 24)
	defer e.Close()
	e.Resize(160, 3)
	e.WriteString("\x1bc")
	e.TrimBuffers()
	if e.scrs[1].buf != nil {
		t.Fatal("resize, reset or trim allocated the unused alternate grid")
	}
	e.WriteString("main\r\n\x1b[?1049h")
	if !e.IsAltScreen() || e.Width() != 160 || e.Height() != 3 {
		t.Fatal("first alternate screen did not inherit the current grid size")
	}
	e.WriteString(strings.Repeat(strings.Repeat("x", 140)+"\r\n", 12000))
	if e.scrs[1].scrollback != nil {
		t.Fatal("alternate screen retains an inaccessible history")
	}
	e.WriteString("\x1b[?1049l")
	if !strings.Contains(e.String(), "main") {
		t.Fatal("alternate screen damaged main-screen content")
	}
	e.WriteString("\x1b[?1047h")
	if !e.IsAltScreen() || strings.TrimSpace(e.String()) != "" {
		t.Fatal("reopened alternate screen retained the previous contents")
	}
}

func TestTrimResizedScreens(t *testing.T) {
	e := NewEmulator(80, 24)
	defer e.Close()
	e.WriteString("主\x1b[?1049h副")
	e.Resize(20, 6)
	e.TrimBuffers()
	for i, want := range []string{"主", "副"} {
		screen := &e.scrs[i]
		if cap(screen.buf.Lines) != 6 || cap(screen.buf.Lines[0]) != 20 {
			t.Fatalf("screen %d retained the large grid", i)
		}
		cell := screen.CellAt(0, 0)
		if cell.Content != want || cell.Width != 2 || screen.CellAt(1, 0).Width != 0 {
			t.Fatalf("screen %d lost its wide cell", i)
		}
	}
	e.WriteString("\x1b[?1049l")
	e.Resize(80, 24)
	e.WriteString("\x1b[24;80HX")
	if e.CellAt(79, 23).Content != "X" || e.CellAt(0, 0).Content != "主" {
		t.Fatal("restored main screen did not survive regrowth")
	}
}

func TestTrimSmallShrinkClearsHiddenCellsAndRows(t *testing.T) {
	e := NewEmulator(10, 4)
	defer e.Close()
	e.WriteString("\x1b[1;10HX\x1b[4;1HY")
	lines := e.scr.buf.Lines
	e.Resize(9, 3)
	e.TrimBuffers()
	if &lines[0][0] != e.CellAt(0, 0) || !lines[0][:10][9].IsZero() || lines[3] != nil {
		t.Fatal("small shrink must reuse the visible grid and clear its discarded references")
	}
}

func TestASCIIRepaintAllocations(t *testing.T) {
	e := NewEmulator(96, 1)
	defer e.Close()
	line := []byte{'\r'}
	for c := byte(' '); c <= '~'; c++ {
		line = append(line, c)
	}
	if allocs := testing.AllocsPerRun(100, func() { e.Write(line) }); allocs != 0 {
		t.Fatalf("ASCII repaint allocated %.0f times", allocs)
	}
	for i, c := range line[1:] {
		if cell := e.CellAt(i, 0); cell.Content != string(c) || cell.Width != 1 {
			t.Fatalf("printable ASCII %q rendered as %+v", c, cell)
		}
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
