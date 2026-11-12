package terminal

import (
	"slices"
	"strings"
	"testing"
	"unsafe"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestPlainHistoryUsesTextWithoutCellMetadata(t *testing.T) {
	s := compactSession(t, 160, 3, 10000)
	for range 22 {
		feedSession(s, strings.Repeat(strings.Repeat("x", 140)+"\r\n", 500))
	}
	if s.history.bytes > 3<<20 {
		t.Fatalf("plain history retained %.1f MiB", float64(s.history.bytes)/(1<<20))
	}
	for _, line := range s.history.lines {
		if line.cells != nil || line.styles != nil {
			t.Fatal("plain ASCII history retained per-cell or style metadata")
		}
	}
}

func TestPackedHistoryPreservesEveryCellAndStyleTransition(t *testing.T) {
	line := uv.Line{{Content: "a", Width: 1}, {Content: "界", Width: 2}, {}, {Content: "e\u0301", Width: 1}, {Content: "b", Width: 1}, {Content: "c", Width: 1}}
	line[1].Style.Attrs = uv.AttrBold
	line[3].Link = uv.Link{URL: "https://example.com", Params: "id=one"}
	line[4].Style.Attrs, line[5].Style.Attrs = uv.AttrBold, uv.AttrBold
	packed := packHistoryLine(line)
	for x := range line {
		got := packed.cell(x)
		if !got.Equal(&line[x]) {
			t.Fatalf("cell %d changed: %+v, want %+v", x, got, line[x])
		}
	}
}

func TestSharedHistoryBudgetEvictsOldestAcrossSessionsAndReleasesClosedHistory(t *testing.T) {
	a, b := compactSession(t, 20, 2, 100), compactSession(t, 20, 2, 100)
	line := packHistoryLine(uv.Line{{Content: "x", Width: 1}})
	budget := NewHistoryBudget(4 * line.bytes())
	a.SetHistoryBudget(budget)
	b.SetHistoryBudget(budget)
	push := func(s *Session, value string) { s.history.push(uv.Line{{Content: value, Width: 1}}) }
	push(a, "a")
	push(b, "b")
	push(a, "c")
	push(b, "d")
	revision := a.Revision
	push(b, "e")
	if a.history.size != 1 || a.history.lines[a.history.start].text != "c" || b.history.size != 3 {
		t.Fatalf("wrong eviction: a=%d b=%d", a.history.size, b.history.size)
	}
	if a.Revision == revision || budget.bytes > budget.limit {
		t.Fatal("global eviction did not invalidate the session or enforce its limit")
	}
	a.history.clear()
	if a.history.lines != nil {
		t.Fatal("cleared history retained its ring allocation")
	}
	a.SetHistoryBudget(nil)
	if len(budget.histories) != 1 || budget.bytes != b.history.bytes {
		t.Fatal("cleared/detached history still consumes global budget")
	}
}

func TestHistoryMetadataGrowsOnDemand(t *testing.T) {
	s := compactSession(t, 20, 2, 100000)
	if len(s.history.lines) != 0 {
		t.Fatal("empty history preallocated its configured maximum")
	}
	feedSession(s, "first\r\nsecond\r\nthird")
	if len(s.history.lines) > 64 || s.HistoryLen() != 1 {
		t.Fatal("initial history allocation was not small")
	}
}

func TestIndexedSearchMatchesCompleteSearchAndBoundsRetainedMemory(t *testing.T) {
	s := compactSession(t, 160, 3, 10000)
	for range 21 {
		feedSession(s, strings.Repeat(strings.Repeat("x", 140)+"\r\n", 500))
	}
	var index SearchIndex
	s.IndexSearch(&index, "x")
	if index.Count() < 1400000 {
		t.Fatalf("lost matches: %d", index.Count())
	}
	if bytes := cap(index.ends) * int(unsafe.Sizeof(int(0))); bytes > 100<<10 {
		t.Fatalf("search index retained %d bytes", bytes)
	}
	for _, position := range []int{0, 139, 140, 500000, index.Count() - 1} {
		got, ok := index.Match(s, position)
		if !ok || got.Col != position%140 || got.Row != position/140 || got.Width != 1 {
			t.Fatalf("match %d = %+v, %v", position, got, ok)
		}
	}
	visible := index.VisibleInto(s, nil, 4000, 20)
	if len(visible) != 2800 || visible[0].Row != 4000 || visible[len(visible)-1].Row != 4019 {
		t.Fatal("viewport search lost matches or retained invisible rows")
	}
	if allocs := testing.AllocsPerRun(3, func() {
		s.IndexSearch(&index, "x")
		visible = index.VisibleInto(s, visible, 4000, 20)
		_, _ = index.Match(s, 500000)
	}); allocs != 0 {
		t.Fatalf("warmed indexed search allocated %.0f times", allocs)
	}
	s.IndexSearch(&index, "")
	if index.ends != nil || index.needle != nil {
		t.Fatal("empty query retained the large index")
	}
}

func TestIndexedUnicodeSearchAndNavigation(t *testing.T) {
	s := compactSession(t, 40, 3, 100)
	feedSession(s, "İ界ABC\r\nİ界abc\r\nx\r\nlast")
	for _, query := range []string{"界a", "i界", "absent", ""} {
		var index SearchIndex
		s.IndexSearch(&index, query)
		want := s.Find(query)
		got := index.VisibleInto(s, nil, 0, s.HistoryLen()+s.Term.Height())
		if index.Count() != len(want) || !slices.Equal(got, want) {
			t.Fatalf("query %q = %+v, want %+v", query, got, want)
		}
		for i := range want {
			match, ok := index.Match(s, i)
			if !ok || match != want[i] {
				t.Fatalf("query %q navigation %d = %+v", query, i, match)
			}
		}
	}
}

func TestIdlePoolTrimPreservesWarmBlocks(t *testing.T) {
	pool := newBytePool(outputBlockSize)
	for range cap(pool.free) {
		pool.put(make([]byte, outputBlockSize))
	}
	pool.trim(2)
	if len(pool.free) != 2 {
		t.Fatalf("idle retained %d blocks", len(pool.free))
	}
	if allocs := testing.AllocsPerRun(100, func() { pool.put(pool.get()) }); allocs != 0 {
		t.Fatal("idle trim lost the reusable working set")
	}
}

func BenchmarkIndexedSearchDense(b *testing.B) {
	s := compactSession(b, 160, 3, 10000)
	for range 21 {
		feedSession(s, strings.Repeat(strings.Repeat("x", 140)+"\r\n", 500))
	}
	var index SearchIndex
	s.IndexSearch(&index, "x")
	for b.Loop() {
		s.IndexSearch(&index, "x")
	}
	b.ReportMetric(float64(cap(index.ends)*int(unsafe.Sizeof(int(0)))), "retained-B")
}
