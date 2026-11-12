package scene

import (
	"iter"
	"sort"

	"github.com/onix-engine/onix/text/piecetree"
)

// TextSelectionRects2D visits the selected visual rows in a vertical viewport.
// Selection offsets are runes; bottom <= top disables viewport culling. It
// searches cached line ranges and measures only the selected visible rows.
func TextSelectionRects2D[V piecetree.Value](text *Text2D, inputValue V, start, end int, top, bottom float32) iter.Seq[TextRangeRect2D] {
	return func(yield func(TextRangeRect2D) bool) {
		if text == nil || start == end || !text.ensureMeasuredLayout() || text.face == nil {
			return
		}
		value := piecetree.Text(inputValue)
		if start > end {
			start, end = end, start
		}
		count := value.Runes()
		start, end = max(0, min(start, count)), max(0, min(end, count))
		if start == end {
			return
		}
		startByte, endByte := text.caretByteIndex(value, start), text.caretByteIndex(value, end)
		first := sort.Search(len(text.lines), func(i int) bool { return text.lines[i].end > startByte })
		cull := bottom > top
		if cull {
			first = max(first, sort.Search(len(text.lines), func(i int) bool { return text.lines[i].y+text.lines[i].height > top }))
		}
		matching := value == text.Text()
		for _, line := range text.lines[first:] {
			if line.start >= endByte || cull && line.y >= bottom {
				break
			}
			lo, hi := max(startByte, line.start), min(endByte, line.end)
			if lo >= hi || hi > value.Len() {
				continue
			}
			if matching && lo == line.start && hi == line.end {
				if !yield(TextRangeRect2D{X: line.x, Y: line.y, Width: max(line.advance, 1), Height: line.height}) {
					return
				}
				continue
			}
			advance := textAdvance2D{previous: -1}
			left := float32(0)
			cursor := value.Cursor(line.start, hi)
			objectIndex := sort.Search(len(text.inlineObjectLayouts), func(i int) bool { return text.inlineObjectLayouts[i].object.End > line.start })
			for cursor.Offset() < hi {
				position := cursor.Offset()
				if position == lo {
					left = advance.width
				}
				character, size := cursor.DecodeRune()
				var object *TextInlineObject2D
				if objectIndex < len(text.inlineObjectLayouts) && text.inlineObjectLayouts[objectIndex].object.Start == position {
					object = &text.inlineObjectLayouts[objectIndex].object
					if lo >= position && lo < object.End {
						left = advance.width
					}
					size = min(object.End, hi) - position
					objectIndex++
				}
				advance.add(text.face, character, object, text.LetterSpacing, text.TabWidth)
				cursor.Advance(size)
			}
			if !yield(TextRangeRect2D{X: line.x + left, Y: line.y, Width: max(advance.width-left, 1), Height: line.height}) {
				return
			}
		}
	}
}
