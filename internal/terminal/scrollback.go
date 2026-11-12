package terminal

import (
	"strings"
	"unsafe"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

const historyByteLimit = 32 << 20

// ASCII rows need only their text. Other rows retain byte ends and display
// widths; styles are stored once per run instead of once per cell.
type historyCell struct {
	end, width uint32
}

type historyStyle struct {
	style uv.Style
	link  uv.Link
	end   uint32
}

type historyLine struct {
	text     string
	cells    []historyCell
	styles   []historyStyle
	sequence uint64
	cols     int
}

func packHistoryLine(line uv.Line) historyLine {
	packed := historyLine{cols: len(line)}
	textLen, runs := 0, 1
	ascii, styled := true, false
	var previous historyStyle
	for i := range line {
		c := &line[i]
		textLen += len(c.Content)
		ascii = ascii && len(c.Content) == 1 && c.Content[0] < 0x80 && c.Width == 1
		next := historyStyle{style: c.Style, link: c.Link}
		if i > 0 && next != previous {
			runs++
		}
		styled = styled || next != (historyStyle{})
		previous = next
	}
	if !ascii {
		packed.cells = make([]historyCell, len(line))
	}
	if styled {
		packed.styles = make([]historyStyle, 0, runs)
	}
	var content strings.Builder
	content.Grow(textLen)
	var last historyStyle
	for i := range line {
		c := &line[i]
		if styled {
			next := historyStyle{style: c.Style, link: c.Link}
			if i > 0 && next != last {
				last.end = uint32(i)
				packed.styles = append(packed.styles, last)
			}
			last = next
		}
		content.WriteString(c.Content)
		if !ascii {
			packed.cells[i] = historyCell{uint32(content.Len()), uint32(c.Width)}
		}
	}
	if styled {
		last.end = uint32(len(line))
		packed.styles = append(packed.styles, last)
	}
	packed.text = content.String()
	return packed
}

func (l *historyLine) bytes() int {
	n := int(unsafe.Sizeof(*l)) + len(l.text) + cap(l.cells)*int(unsafe.Sizeof(historyCell{})) + cap(l.styles)*int(unsafe.Sizeof(historyStyle{}))
	for _, style := range l.styles {
		n += len(style.link.URL) + len(style.link.Params)
	}
	return n
}

func (l *historyLine) cell(x int) uv.Cell {
	var cell uv.Cell
	if l.cells == nil {
		cell = uv.Cell{Content: l.text[x : x+1], Width: 1}
	} else {
		c := l.cells[x]
		var start uint32
		if x > 0 {
			start = l.cells[x-1].end
		}
		cell = uv.Cell{Content: l.text[start:c.end], Width: int(c.width)}
	}
	lo, hi := 0, len(l.styles)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if int(l.styles[mid].end) <= x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(l.styles) {
		cell.Style, cell.Link = l.styles[lo].style, l.styles[lo].link
	}
	return cell
}

// HistoryBudget bounds packed history across all sessions in a workspace. Like
// Session's emulator, it is owned by the UI goroutine. Oldest rows are evicted
// first, regardless of which pane produced the latest output.
type HistoryBudget struct {
	histories map[*scrollback]struct{}
	bytes     int
	limit     int
	sequence  uint64
}

func NewHistoryBudget(bytes int) *HistoryBudget {
	return &HistoryBudget{histories: make(map[*scrollback]struct{}), limit: max(0, bytes)}
}

func (b *HistoryBudget) enforce() {
	for b.bytes > b.limit {
		var oldest *scrollback
		for h := range b.histories {
			if h.size > 0 && (oldest == nil || h.lines[h.start].sequence < oldest.lines[oldest.start].sequence) {
				oldest = h
			}
		}
		if oldest == nil {
			return
		}
		oldest.drop()
	}
}

func (s *Session) SetHistoryBudget(b *HistoryBudget) {
	h := s.history
	if h == nil || h.budget == b {
		return
	}
	if h.budget != nil {
		h.budget.bytes -= h.bytes
		delete(h.budget.histories, h)
	}
	h.budget = b
	if b != nil {
		b.histories[h] = struct{}{}
		b.bytes += h.bytes
		for row := range h.size {
			b.sequence++
			h.lines[(h.start+row)%len(h.lines)].sequence = b.sequence
		}
		b.enforce()
	}
}

type scrollback struct {
	revision    *uint64
	budget      *HistoryBudget
	lines       []historyLine
	start, size int
	bytes       int
	maxLines    int
	viewStart   int
	viewEnd     int
	decoded     map[int]uv.Line
}

func (h *scrollback) clear() {
	if h.budget != nil {
		h.budget.bytes -= h.bytes
	}
	h.lines = nil
	h.decoded = nil
	h.viewStart, h.viewEnd = 0, 0
	h.start, h.size, h.bytes = 0, 0, 0
}

func (h *scrollback) drop() {
	n := h.lines[h.start].bytes()
	h.bytes -= n
	if h.budget != nil {
		h.budget.bytes -= n
	}
	h.lines[h.start] = historyLine{}
	delete(h.decoded, h.start)
	h.start = (h.start + 1) % len(h.lines)
	h.size--
	if h.revision != nil {
		(*h.revision)++
	}
}

func (h *scrollback) grow() {
	limit := max(h.maxLines, len(h.lines))
	if h.size < len(h.lines) || len(h.lines) == limit {
		return
	}
	lines := make([]historyLine, min(limit, max(64, len(h.lines)*2)))
	for row := range h.size {
		lines[row] = h.lines[(h.start+row)%len(h.lines)]
	}
	h.lines, h.start, h.decoded = lines, 0, nil
}

func (h *scrollback) push(line uv.Line) {
	h.grow()
	if len(h.lines) == 0 {
		return
	}
	packed := packHistoryLine(line)
	n := packed.bytes()
	for h.size > 0 && (h.size == len(h.lines) || h.bytes+n > historyByteLimit) {
		h.drop()
	}
	if n > historyByteLimit || h.budget != nil && n > h.budget.limit {
		return
	}
	if h.budget != nil {
		h.budget.sequence++
		packed.sequence = h.budget.sequence
		h.budget.bytes += n
	}
	index := (h.start + h.size) % len(h.lines)
	h.lines[index] = packed
	delete(h.decoded, index)
	h.size++
	h.bytes += n
	if h.budget != nil {
		h.budget.enforce()
	}
}

func (h *scrollback) cellValue(x, row int) (uv.Cell, bool) {
	if x < 0 || row < 0 || row >= h.size {
		return uv.Cell{}, false
	}
	line := &h.lines[(h.start+row)%len(h.lines)]
	if x >= line.cols {
		return uv.Cell{}, false
	}
	return line.cell(x), true
}

func (h *scrollback) cell(x, row int) *uv.Cell {
	if row < h.viewStart || row >= h.viewEnd {
		cell, ok := h.cellValue(x, row)
		if !ok {
			return nil
		}
		return &cell
	}
	index := (h.start + row) % len(h.lines)
	line := &h.lines[index]
	if x < 0 || x >= line.cols {
		return nil
	}
	decoded, ok := h.decoded[index]
	if !ok {
		decoded = make(uv.Line, line.cols)
		for i := range decoded {
			decoded[i] = line.cell(i)
		}
		if h.decoded == nil {
			h.decoded = make(map[int]uv.Line)
		}
		h.decoded[index] = decoded
	}
	return &decoded[x]
}

// SetViewport releases rows outside the visible terminal area. Non-rendering
// callers read packed rows directly and never populate the decoded cache.
func (s *Session) SetViewport(start, rows int) {
	h := s.history
	if h == nil {
		return
	}
	h.viewStart = max(0, min(start, h.size))
	h.viewEnd = max(h.viewStart, min(start+max(0, rows), h.size))
	if s.Term.IsAltScreen() || h.viewStart == h.viewEnd {
		h.viewStart, h.viewEnd = 0, 0
		h.decoded = nil
		return
	}
	for index := range h.decoded {
		row := (index - h.start + len(h.lines)) % len(h.lines)
		if row < h.viewStart || row >= h.viewEnd {
			delete(h.decoded, index)
		}
	}
}

func (s *Session) initScrollback(maxLines int) {
	s.history = &scrollback{maxLines: max(0, maxLines), revision: &s.Revision}
	s.Term.SetScrollbackSize(1)
	s.Term.Scrollback().SetSink(s.history.push)
	s.Term.RegisterCsiHandler('J', func(params ansi.Params) bool {
		if n, _, _ := params.Param(0, 0); n == 3 && !s.Term.IsAltScreen() {
			s.history.clear()
			s.Term.ClearScrollback()
			s.releaseClearedHistory()
		}
		return false
	})
}
