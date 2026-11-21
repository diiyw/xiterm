package terminal

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// WebLink describes the inclusive columns occupied by a browser link.
type WebLink struct {
	URL             string
	Row, Start, End int
}

var webURLPattern = regexp.MustCompile("(?i)https?://[^\\s<>\"'`，。；！？（）【】「」]+")

// IsWebURL limits browser actions, including OSC 8 targets, to HTTP(S).
func IsWebURL(value string) bool {
	if strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false
	}
	u, err := url.Parse(value)
	return err == nil && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")) && u.Hostname() != "" && u.Opaque == ""
}

// WebLinkAt resolves explicit OSC 8 links before recognizing plain URLs. It
// reads terminal cells so ANSI styling and wide characters do not shift hits.
func (s *Session) WebLinkAt(pos Position) WebLink {
	width := s.Term.Width()
	if pos.Col < 0 || pos.Col >= width || pos.Row < 0 || pos.Row >= s.HistoryLen()+s.Term.Height() {
		return WebLink{}
	}
	col := pos.Col
	cell, ok := s.cellValue(col, pos.Row)
	for ok && cell.Width == 0 && col > 0 {
		col--
		cell, ok = s.cellValue(col, pos.Row)
	}
	if ok && cell.Link.URL != "" {
		if !IsWebURL(cell.Link.URL) {
			return WebLink{}
		}
		link := WebLink{URL: cell.Link.URL, Row: pos.Row, Start: col, End: col + max(1, cell.Width) - 1}
		for x := col - 1; x >= 0; x-- {
			c, exists := s.cellValue(x, pos.Row)
			if !exists || c.Width > 0 && c.Link != cell.Link {
				break
			}
			if c.Width > 0 {
				link.Start = x
			}
		}
		for x := link.End + 1; x < width; x++ {
			c, exists := s.cellValue(x, pos.Row)
			if !exists || c.Width > 0 && c.Link != cell.Link {
				break
			}
			link.End = x
		}
		return link
	}
	var line strings.Builder
	// Each byte maps to its owning terminal column, including Unicode cells.
	columns := make([]int, 0, width)
	for x := 0; x < width; x++ {
		c, exists := s.cellValue(x, pos.Row)
		content := " "
		if exists {
			if c.Width == 0 {
				continue
			}
			content = c.Content
		}
		line.WriteString(content)
		for range len(content) {
			columns = append(columns, x)
		}
	}
	text := line.String()
	for _, span := range webURLPattern.FindAllStringIndex(text, -1) {
		value := trimWebURL(text[span[0]:span[1]])
		if !IsWebURL(value) {
			continue
		}
		start, end := columns[span[0]], columns[span[0]+len(value)-1]
		last, _ := s.cellValue(end, pos.Row)
		end += max(1, last.Width) - 1
		if pos.Col >= start && pos.Col <= end {
			return WebLink{URL: value, Row: pos.Row, Start: start, End: end}
		}
	}
	return WebLink{}
}

func trimWebURL(value string) string {
	for len(value) > 0 {
		last := value[len(value)-1]
		if strings.ContainsRune(".,;:!?", rune(last)) {
			value = value[:len(value)-1]
			continue
		}
		open := byte(0)
		switch last {
		case ')':
			open = '('
		case ']':
			open = '['
		case '}':
			open = '{'
		}
		if open != 0 && strings.Count(value, string(last)) > strings.Count(value, string(open)) {
			value = value[:len(value)-1]
			continue
		}
		break
	}
	return value
}
