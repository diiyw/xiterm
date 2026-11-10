package terminal

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

func TestFindIntoUnicodeReuseAndOwnership(t *testing.T) {
	s := compactSession(t, 30, 2, 100)
	feedSession(s, "İ界ABC\r\nİ界abc\r\nlast")
	want := []Match{{Row: 0, Col: 1, Width: 3}, {Row: 1, Col: 1, Width: 3}}
	owned := s.Find("界A")
	var dst []Match
	queries := []string{"界A", "界a", "İ界A", "i界a", "absent", ""}
	for _, query := range queries {
		dst = s.FindInto(dst, query)
		switch query {
		case "界A", "界a":
			if !slices.Equal(dst, want) {
				t.Fatalf("FindInto(%q) = %+v, want %+v", query, dst, want)
			}
		case "İ界A", "i界a":
			expected := []Match{{Row: 0, Col: 0, Width: 4}, {Row: 1, Col: 0, Width: 4}}
			if !slices.Equal(dst, expected) {
				t.Fatalf("FindInto(%q) = %+v, want %+v", query, dst, expected)
			}
		default:
			if len(dst) != 0 {
				t.Fatalf("FindInto(%q) = %+v, want no matches", query, dst)
			}
		}
	}
	if !slices.Equal(owned, want) {
		t.Fatal("search scratch overwrote an independently owned Find result")
	}
	if allocs := testing.AllocsPerRun(20, func() {
		for _, query := range queries {
			dst = s.FindInto(dst, query)
		}
	}); allocs != 0 {
		t.Fatalf("warmed Unicode search allocated %.0f times", allocs)
	}
	if s.history.decoded != nil {
		t.Fatal("search expanded packed history")
	}
}

func TestOutputBlockStaysOwnedAcrossPolls(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(120, 24), output: make(chan []byte, 1), outputPool: newBytePool(outputBlockSize)}
	defer s.Term.Close()
	block := s.outputPool.get()
	for i := range block {
		block[i] = 'x'
	}
	s.output <- block
	if !s.Poll() || len(s.pendingOutput) == 0 {
		t.Fatal("large output did not remain pending")
	}
	if len(s.outputPool.free) != 0 || &s.pendingBuffer[0] != &block[0] {
		t.Fatal("partially consumed output block was recycled")
	}
	for s.PendingOutput() {
		s.Poll()
	}
	if s.pendingBuffer != nil || len(s.outputPool.free) != 1 {
		t.Fatal("completed output block was not recycled")
	}
	reused := s.outputPool.get()
	if len(reused) != outputBlockSize || &reused[0] != &block[0] {
		t.Fatal("pool lost the original block after reslicing")
	}
	copy(reused, "\x1b[HZ")
	s.output <- reused[:4]
	s.Poll()
	if s.Term.CellAt(0, 0).Content != "Z" {
		t.Fatal("recycled output was not parsed")
	}
}

func TestBytePoolAllocationsAndBound(t *testing.T) {
	p := newBytePool(outputBlockSize)
	if allocs := testing.AllocsPerRun(100, func() {
		block := p.get()
		p.put(block[:7])
	}); allocs != 0 {
		t.Fatalf("warmed buffer transfer allocated %.0f times", allocs)
	}
	for range cap(p.free) + 10 {
		p.put(make([]byte, outputBlockSize))
	}
	if len(p.free) != ioQueueSize+2 {
		t.Fatal("pool did not enforce its retention bound")
	}
	p.clear()
	if len(p.free) != 0 {
		t.Fatal("pool retained blocks after clearing")
	}
}

func BenchmarkFindHistoryInto(b *testing.B) {
	s := compactSession(b, 120, 24, 10000)
	feedSession(s, strings.Repeat("Terminal OUTPUT with İ中文 text and a SEARCHABLE MARKER\r\n", 10024))
	matches := s.FindInto(nil, "MaRkEr")
	b.ReportAllocs()
	for b.Loop() {
		matches = s.FindInto(matches, "MaRkEr")
	}
}

func BenchmarkIOBlockReuse(b *testing.B) {
	p := newBytePool(outputBlockSize)
	b.ReportAllocs()
	for b.Loop() {
		block := p.get()
		p.put(block[:7])
	}
}

func BenchmarkPackHistoryLine(b *testing.B) {
	for _, styled := range []bool{false, true} {
		b.Run(fmt.Sprint("styled=", styled), func(b *testing.B) {
			line := make(uv.Line, 140)
			for i := range line {
				line[i] = uv.Cell{Content: "x", Width: 1}
				if styled {
					line[i].Style.Attrs = uv.AttrBold
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				_ = packHistoryLine(line)
			}
		})
	}
}
