package piecetree

import "unicode/utf8"

// Cursor reads a byte range in order, looking up the tree only when crossing a
// piece. Copies are independent and keep their immutable snapshot alive.
type Cursor struct {
	value       Snapshot
	chunk       string
	offset, end int
}

func (s Snapshot) Cursor(start, end int) Cursor {
	s.check(start, end)
	cursor := Cursor{value: s, offset: start, end: end}
	cursor.refill()
	return cursor
}

func (c *Cursor) Offset() int { return c.offset }

// Byte returns the next byte, or zero at the end of the range.
func (c *Cursor) Byte() byte {
	if len(c.chunk) == 0 {
		return 0
	}
	return c.chunk[0]
}

// DecodeRune peeks at the next rune. A zero size marks the end of the range.
func (c *Cursor) DecodeRune() (rune, int) {
	if len(c.chunk) > 0 && c.chunk[0] >= utf8.RuneSelf && !utf8.FullRuneInString(c.chunk) && len(c.chunk) < c.end-c.offset {
		// Editing invalid UTF-8 can join a formerly incomplete sequence across
		// pieces. Match decoding the complete range without materializing it.
		var bytes [utf8.UTFMax]byte
		count := copy(bytes[:], c.chunk)
		for count < len(bytes) && c.offset+count < c.end {
			bytes[count] = c.value.ByteAt(c.offset + count)
			count++
		}
		return utf8.DecodeRune(bytes[:count])
	}
	return utf8.DecodeRuneInString(c.chunk)
}

func (c *Cursor) Advance(bytes int) {
	if bytes < 0 || bytes > c.end-c.offset {
		panic("piecetree: invalid cursor advance")
	}
	c.offset += bytes
	if bytes < len(c.chunk) {
		c.chunk = c.chunk[bytes:]
	} else {
		c.refill()
	}
}

func (c *Cursor) refill() {
	c.chunk = ""
	if c.offset < c.end {
		c.chunk = c.value.contiguous(c.offset)
		c.chunk = c.chunk[:min(len(c.chunk), c.end-c.offset)]
	}
}

// EqualRange compares two byte ranges without joining pieces into strings.
func (s Snapshot) EqualRange(start, end int, other Snapshot, otherStart, otherEnd int) bool {
	s.check(start, end)
	other.check(otherStart, otherEnd)
	if end-start != otherEnd-otherStart {
		return false
	}
	if s == other && start == otherStart {
		return true
	}
	for start < end {
		a, b := s.contiguous(start), other.contiguous(otherStart)
		count := min(len(a), len(b), end-start)
		if a[:count] != b[:count] {
			return false
		}
		start, otherStart = start+count, otherStart+count
	}
	return true
}
