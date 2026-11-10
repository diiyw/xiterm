package terminal

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

func compactSession(t testing.TB, cols, rows, lines int) *Session {
	t.Helper()
	s := &Session{Term: vt.NewEmulator(cols, rows), output: make(chan []byte, 1)}
	s.initScrollback(lines)
	t.Cleanup(func() { _ = s.Term.Close() })
	return s
}

func feedSession(s *Session, value string) {
	s.output <- []byte(value)
	for s.PendingOutput() {
		s.Poll()
	}
}

func TestCompactHistoryMatchesVT(t *testing.T) {
	for _, chunk := range []int{1, 7, 32768} {
		t.Run(fmt.Sprint(chunk), func(t *testing.T) {
			s := compactSession(t, 40, 3, 12)
			reference := vt.NewEmulator(40, 3)
			defer reference.Close()
			reference.SetScrollbackSize(12)
			payload := strings.Repeat("plain\r\n\x1b[31;1m红色 e\u0301\x1b[0m\r\n\x1b[48;2;10;20;30mBG\x1b[0m\r\n\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\\r\n\r\n", 10)
			// Include a clear followed by enough new history in the same write
			// to hide a reset from a before/after length comparison.
			payload += "\x1b[3J" + strings.Repeat("new 世界\r\n", 30)
			payload += "\x1b[?1049hALT\x1b[3Jalternate\x1b[?1049lmain\r\n"
			for len(payload) > 0 {
				n := min(chunk, len(payload))
				part := payload[:n]
				payload = payload[n:]
				feedSession(s, part)
				_, _ = reference.Write([]byte(part))
				compareHistory(t, s, reference)
			}
		})
	}
}

func compareHistory(t *testing.T, s *Session, reference *vt.Emulator) {
	t.Helper()
	history := reference.ScrollbackLen()
	if reference.IsAltScreen() {
		history = 0
	}
	if s.HistoryLen() != history {
		t.Fatalf("history length = %d, want %d", s.HistoryLen(), history)
	}
	for row := 0; row < history+reference.Height(); row++ {
		for x := 0; x < reference.Width(); x++ {
			want := reference.CellAt(x, row-history)
			if row < history {
				want = reference.ScrollbackCellAt(x, row)
			}
			got := s.Cell(x, row)
			if got == nil && want == nil {
				continue
			}
			if got == nil || !got.Equal(want) {
				t.Fatalf("cell (%d,%d) = %+v, want %+v", x, row, got, want)
			}
		}
	}
}

func TestCompactHistorySearchSelectionAndResize(t *testing.T) {
	s := compactSession(t, 30, 3, 100)
	feedSession(s, "hello 世界\r\nHELLO again\r\nthird\r\nlast")
	if got := s.Find("世界"); len(got) != 1 || got[0] != (Match{Row: 0, Col: 6, Width: 4}) {
		t.Fatalf("wide search = %+v", got)
	}
	if got := s.Selection(Position{0, 0}, Position{1, 10}); got != "hello 世界\nHELLO again" {
		t.Fatalf("selection = %q", got)
	}
	s.Term.Resize(12, 3)
	if got := s.Line(0); got != "hello 世界" {
		t.Fatalf("history after resize = %q", got)
	}
	s.ClearHistory()
	if s.HistoryLen() != 0 || s.history.bytes != 0 {
		t.Fatal("clear retained history")
	}
	if len(s.history.decoded) != 0 {
		t.Fatal("clear retained decoded cells")
	}
}

func TestCompactHistoryZeroDisablesHistory(t *testing.T) {
	s := compactSession(t, 30, 3, 0)
	feedSession(s, strings.Repeat("no history\r\n", 100))
	if s.HistoryLen() != 0 {
		t.Fatalf("scrollback=0 retained %d lines", s.HistoryLen())
	}
}

func TestHistoryViewportIsLazyAndReleasesOffscreenRows(t *testing.T) {
	s := compactSession(t, 30, 100, 200)
	for i := range 300 {
		feedSession(s, fmt.Sprintf("row %03d 世界\r\n", i))
	}
	if len(s.history.decoded) != 0 {
		t.Fatal("output eagerly expanded history")
	}
	s.SetViewport(0, 80)
	if len(s.history.decoded) != 0 {
		t.Fatal("setting viewport eagerly expanded history")
	}
	for row := range 80 {
		if c := s.Cell(0, row); c == nil {
			t.Fatalf("missing visible row %d", row)
		}
	}
	if len(s.history.decoded) != 80 {
		t.Fatalf("visible rows cached = %d, want 80", len(s.history.decoded))
	}
	// Overlap stays cached while rows that scroll off screen are released.
	retained := s.Cell(0, 20)
	s.SetViewport(10, 80)
	if len(s.history.decoded) != 70 || s.Cell(0, 20) != retained {
		t.Fatal("scroll failed to retain only overlapping visible rows")
	}
	if c := s.Cell(0, 100); c == nil || len(s.history.decoded) != 70 {
		t.Fatal("offscreen cell read expanded a whole row")
	}
	_ = s.Find("世界")
	_ = s.Selection(Position{0, 0}, Position{s.HistoryLen() - 1, 29})
	_ = s.Line(0)
	if len(s.history.decoded) != 70 || s.Cell(0, 20) != retained {
		t.Fatal("search/copy displaced the viewport cache")
	}
	s.SetViewport(10, 5)
	if len(s.history.decoded) != 5 {
		t.Fatal("shrinking viewport retained rows")
	}
	s.SetViewport(s.HistoryLen(), s.Term.Height())
	if s.history.decoded != nil {
		t.Fatal("returning to live output retained expanded history")
	}
	s.SetViewport(0, 10)
	_ = s.Cell(0, 0)
	s.SetViewport(0, 0)
	if s.history.decoded != nil {
		t.Fatal("hiding pane retained expanded history")
	}
	s.SetViewport(0, 10)
	_ = s.Cell(0, 0)
	feedSession(s, "\x1b[?1049hALT")
	s.SetViewport(0, 10)
	if s.history.decoded != nil {
		t.Fatal("alternate screen retained expanded history")
	}
}

func TestHistoryViewportSurvivesRingWrap(t *testing.T) {
	s := compactSession(t, 30, 3, 10)
	reference := vt.NewEmulator(30, 3)
	defer reference.Close()
	reference.SetScrollbackSize(10)
	for i := range 100 {
		line := fmt.Sprintf("row %03d\r\n", i)
		feedSession(s, line)
		fmt.Fprint(reference, line)
		s.SetViewport(0, 3)
		compareHistory(t, s, reference)
		if len(s.history.decoded) > 3 {
			t.Fatal("ring wrap retained cells outside viewport")
		}
	}
}

func TestHistoryScanAllocationsDoNotScaleWithRows(t *testing.T) {
	s := compactSession(t, 80, 3, 1000)
	for range 10 {
		feedSession(s, strings.Repeat("plain log output\r\n", 100))
	}
	searchAllocs := testing.AllocsPerRun(3, func() { _ = s.Find("absent marker") })
	copyAllocs := testing.AllocsPerRun(3, func() { _ = s.Selection(Position{0, 0}, Position{999, 79}) })
	if searchAllocs > 10 || copyAllocs > 40 {
		t.Fatalf("history scan allocated per row: search=%.0f copy=%.0f", searchAllocs, copyAllocs)
	}
	if s.history.decoded != nil {
		t.Fatal("scan created viewport cache")
	}
}

func TestCompactHistoryMemoryPlateaus(t *testing.T) {
	s := compactSession(t, 160, 3, 10000)
	batch := strings.Repeat(strings.Repeat("x", 140)+"\r\n", 500)
	for range 22 {
		feedSession(s, batch)
	}
	fullBytes := s.history.bytes
	for range 40 {
		feedSession(s, batch)
	}
	if s.HistoryLen() != 10000 || s.history.bytes != fullBytes {
		t.Fatalf("history grew after filling: lines=%d bytes=%d -> %d", s.HistoryLen(), fullBytes, s.history.bytes)
	}
	unpacked := 10000 * 140 * int(unsafe.Sizeof(uv.Cell{}))
	if fullBytes > unpacked/4 {
		t.Fatalf("packed history uses %d bytes, unpacked %d", fullBytes, unpacked)
	}
	if s.Term.ScrollbackLen() != 0 {
		t.Fatal("VT retained a second copy of history")
	}
	t.Logf("10000 × 140 cells: packed=%.1f MiB, unpacked=%.1f MiB", float64(fullBytes)/(1<<20), float64(unpacked)/(1<<20))
}

func TestCompactHistoryByteBudgetAndCacheEviction(t *testing.T) {
	h := &scrollback{lines: make([]historyLine, 1000)}
	// Long links exercise the byte budget without relying on the line limit.
	line := uv.Line{{Content: "x", Width: 1, Link: uv.Link{URL: strings.Repeat("u", 1<<20)}}}
	for range 100 {
		h.push(line)
		_ = h.cell(0, h.size-1)
		if h.bytes > historyByteLimit {
			t.Fatalf("history exceeds byte budget: %d", h.bytes)
		}
	}
	if h.size >= len(h.lines) || h.start == 0 {
		t.Fatal("byte limit did not evict old rows")
	}
	h.clear()
	for _, line := range h.lines {
		if line.text != "" || line.cells != nil || line.styles != nil {
			t.Fatal("clear retained packed data")
		}
	}
}

func BenchmarkContinuousOutput(b *testing.B) {
	for _, compact := range []bool{false, true} {
		b.Run(fmt.Sprint("compact=", compact), func(b *testing.B) {
			s := &Session{Term: vt.NewEmulator(160, 40), output: make(chan []byte, 1)}
			defer s.Term.Close()
			if compact {
				s.initScrollback(10000)
			}
			batch := []byte(strings.Repeat(strings.Repeat("x", 140)+"\r\n", 100))
			for range 110 {
				s.output <- batch
				for s.PendingOutput() {
					s.Poll()
				}
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(batch)))
			b.ResetTimer()
			for b.Loop() {
				s.output <- batch
				for s.PendingOutput() {
					s.Poll()
				}
			}
		})
	}
}

func BenchmarkCompactHistoryRead(b *testing.B) {
	s := compactSession(b, 120, 24, 10000)
	batch := strings.Repeat("terminal output with 中文 text and a searchable marker\r\n", 200)
	for range 51 {
		feedSession(s, batch)
	}
	for _, operation := range []string{"search", "copy"} {
		b.Run(operation, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if operation == "search" {
					_ = s.Find("marker")
				} else {
					_ = s.Selection(Position{0, 0}, Position{s.HistoryLen() - 1, 119})
				}
			}
		})
	}
}
