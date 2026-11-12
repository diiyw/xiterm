package terminal

import (
	"bytes"
	"sort"
)

// SearchIndex stores one cumulative match count per physical row. Even a
// single-character query matching every cell uses O(rows) retained memory.
// Individual matches are decoded only for navigation and the visible viewport.
type SearchIndex struct {
	ends   []int
	needle []byte
}

func (index *SearchIndex) Count() int {
	if len(index.ends) == 0 {
		return 0
	}
	return index.ends[len(index.ends)-1]
}

func (index *SearchIndex) Clear() {
	index.ends, index.needle = nil, nil
}

func (s *Session) IndexSearch(index *SearchIndex, query string) {
	if query == "" {
		index.Clear()
		return
	}
	index.needle = appendLower(index.needle[:0], query)
	rows := s.HistoryLen() + s.Term.Height()
	if cap(index.ends) < rows || cap(index.ends) > max(1024, rows*4) {
		index.ends = make([]int, rows)
	} else {
		index.ends = index.ends[:rows]
	}
	total := 0
	for row := range rows {
		line, _ := s.searchRow(row)
		for offset := 0; offset < len(line); {
			i := bytes.Index(line[offset:], index.needle)
			if i < 0 {
				break
			}
			total++
			offset += i + len(index.needle)
		}
		index.ends[row] = total
	}
}

func (index *SearchIndex) Match(s *Session, position int) (Match, bool) {
	if position < 0 || position >= index.Count() {
		return Match{}, false
	}
	row := sort.Search(len(index.ends), func(row int) bool { return index.ends[row] > position })
	if row > 0 {
		position -= index.ends[row-1]
	}
	line, columns := s.searchRow(row)
	for offset := 0; offset < len(line); {
		i := bytes.Index(line[offset:], index.needle)
		if i < 0 {
			break
		}
		start := offset + i
		if position == 0 {
			return s.searchMatch(row, start, start+len(index.needle)-1, columns), true
		}
		position--
		offset = start + len(index.needle)
	}
	return Match{}, false
}

func (index *SearchIndex) VisibleInto(s *Session, dst []Match, start, rows int) []Match {
	dst = dst[:0]
	for row := max(0, start); row < min(start+rows, len(index.ends)); row++ {
		previous := 0
		if row > 0 {
			previous = index.ends[row-1]
		}
		if index.ends[row] == previous {
			continue
		}
		line, columns := s.searchRow(row)
		for offset := 0; offset < len(line); {
			i := bytes.Index(line[offset:], index.needle)
			if i < 0 {
				break
			}
			first := offset + i
			last := first + len(index.needle) - 1
			dst = append(dst, s.searchMatch(row, first, last, columns))
			offset = last + 1
		}
	}
	if cap(dst) > max(1024, len(dst)*4) {
		trimmed := make([]Match, len(dst))
		copy(trimmed, dst)
		return trimmed
	}
	return dst
}

func (s *Session) searchRow(row int) ([]byte, []int) {
	line, columns := s.findLine[:0], s.findColumns[:0]
	for x := 0; x < s.Term.Width(); x++ {
		c, ok := s.cellValue(x, row)
		value := " "
		if ok {
			if c.Width == 0 {
				continue
			}
			value = c.Content
		}
		start := len(line)
		line = appendLower(line, value)
		for range len(line) - start {
			columns = append(columns, x)
		}
	}
	s.findLine, s.findColumns = line, columns
	return line, columns
}

func (s *Session) searchMatch(row, first, last int, columns []int) Match {
	col, end := columns[first], columns[last]
	width := 1
	if cell, ok := s.cellValue(end, row); ok {
		width = max(1, cell.Width)
	}
	return Match{Row: row, Col: col, Width: end - col + width}
}
