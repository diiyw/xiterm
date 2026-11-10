package terminal

import (
	"strings"
	"unsafe"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Bound retained history independently of window width and the line limit.
// Only visible rows are expanded for rendering; searches read packed cells.
const historyByteLimit = 32 << 20

type historyCell struct {
	end, width, style uint32
}

type historyStyle struct {
	style uv.Style
	link  uv.Link
}

type historyLine struct {
	text   string
	cells  []historyCell
	styles []historyStyle
}

func packHistoryLine(line uv.Line) historyLine {
	packed := historyLine{cells: make([]historyCell, len(line))}
	var content strings.Builder
	textLen, styleCount := 0, 0
	var previous historyStyle
	for i := range line {
		textLen += len(line[i].Content)
		next := historyStyle{line[i].Style, line[i].Link}
		if next != previous && next != (historyStyle{}) {
			styleCount++
		}
		previous = next
	}
	content.Grow(textLen)
	if styleCount > 0 {
		packed.styles = make([]historyStyle, 0, styleCount)
	}
	var last historyStyle
	var style uint32
	for i := range line {
		c := &line[i]
		next := historyStyle{c.Style, c.Link}
		if next != last {
			// Style zero is implicit, avoiding any style allocation for plain logs.
			if next == (historyStyle{}) {
				style = 0
			} else {
				packed.styles = append(packed.styles, next)
				style = uint32(len(packed.styles))
			}
			last = next
		}
		content.WriteString(c.Content)
		packed.cells[i] = historyCell{uint32(content.Len()), uint32(c.Width), style}
	}
	// The exact byte count is known, so the builder never grows or needs a clone.
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
	c := l.cells[x]
	var start uint32
	if x > 0 {
		start = l.cells[x-1].end
	}
	cell := uv.Cell{Content: l.text[start:c.end], Width: int(c.width)}
	if c.style != 0 {
		style := l.styles[c.style-1]
		cell.Style, cell.Link = style.style, style.link
	}
	return cell
}

type scrollback struct {
	lines       []historyLine
	start, size int
	bytes       int
	viewStart   int
	viewEnd     int
	decoded     map[int]uv.Line // Ring indices, allocated only inside the viewport.
}

func (h *scrollback) clear() {
	clear(h.lines)
	h.decoded = nil
	h.viewStart, h.viewEnd = 0, 0
	h.start, h.size, h.bytes = 0, 0, 0
}

func (h *scrollback) drop() {
	h.bytes -= h.lines[h.start].bytes()
	h.lines[h.start] = historyLine{}
	delete(h.decoded, h.start)
	h.start = (h.start + 1) % len(h.lines)
	h.size--
}

func (h *scrollback) push(line uv.Line) {
	if len(h.lines) == 0 {
		return
	}
	packed := packHistoryLine(line)
	n := packed.bytes()
	for h.size > 0 && (h.size == len(h.lines) || h.bytes+n > historyByteLimit) {
		h.drop()
	}
	// A single pathological row must not exceed the entire history budget.
	if n > historyByteLimit {
		return
	}
	index := (h.start + h.size) % len(h.lines)
	h.lines[index] = packed
	delete(h.decoded, index)
	h.size++
	h.bytes += n
}

func (h *scrollback) cellValue(x, row int) (uv.Cell, bool) {
	if x < 0 || row < 0 || row >= h.size {
		return uv.Cell{}, false
	}
	index := (h.start + row) % len(h.lines)
	line := &h.lines[index]
	if x >= len(line.cells) {
		return uv.Cell{}, false
	}
	return line.cell(x), true
}

func (h *scrollback) cell(x, row int) *uv.Cell {
	if row < h.viewStart || row >= h.viewEnd {
		// Non-rendering callers can read one cell without expanding its row.
		cell, ok := h.cellValue(x, row)
		if !ok {
			return nil
		}
		return &cell
	}
	index := (h.start + row) % len(h.lines)
	line := &h.lines[index]
	if x < 0 || x >= len(line.cells) {
		return nil
	}
	decoded, ok := h.decoded[index]
	if !ok {
		decoded = make(uv.Line, len(line.cells))
		for i := range line.cells {
			decoded[i] = line.cell(i)
		}
		if h.decoded == nil {
			h.decoded = make(map[int]uv.Line)
		}
		h.decoded[index] = decoded
	}
	return &decoded[x]
}

// SetViewport releases rows outside the visible terminal area. History stays
// packed until Cell is called to draw it; searching and copying never populate
// this cache. Passing zero rows releases the cache when a tab is hidden or the
// pane returns to live output.
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
	s.history = &scrollback{lines: make([]historyLine, maxLines)}
	s.Term.SetScrollbackSize(max(1, maxLines))
	// Observe the parsed sequence, including sequences split across PTY reads.
	// Returning false lets VT perform its usual screen erase afterwards.
	s.Term.RegisterCsiHandler('J', func(params ansi.Params) bool {
		if n, _, _ := params.Param(0, 0); n == 3 && !s.Term.IsAltScreen() {
			s.history.clear()
			s.Term.ClearScrollback()
			s.releaseClearedHistory()
		}
		return false
	})
}

func (s *Session) collectHistory() {
	if s.history == nil {
		return
	}
	for _, line := range s.Term.Scrollback().Lines() {
		s.history.push(line)
	}
	s.Term.ClearScrollback()
	s.releaseClearedHistory()
}
