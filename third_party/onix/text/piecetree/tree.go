// Package piecetree stores UTF-8 text in an AVL tree of references into an
// immutable original buffer and an append-only edit buffer. Tree is owned by
// one goroutine; String returns immutable snapshots safe to share with workers.
package piecetree

import (
	"strings"
	"unicode/utf8"
)

const pieceBytes = 1024

type piece struct {
	start, size, runes, breaks int
	added                      bool
}
type node struct {
	piece
	left, right, height int
	bytes, chars, lines int
}

// Tree's zero value is an empty document. Do not copy a populated Tree.
// Nodes use arena indices so rotations and free-list reuse do not allocate.
type Tree struct {
	original       string
	snapshot       string
	appendSnapshot strings.Builder
	added          []byte
	nodes          []node
	root, free     int
	// prefix is the unchanged prefix of snapshot after pending edits. The
	// builder only appends: previously published strings are never overwritten.
	prefix int
	cached bool
}

func (t *Tree) Len() int   { return t.bytes(t.root) }
func (t *Tree) Runes() int { return t.chars(t.root) }

// Lines counts LF-delimited lines, including the empty line after a final LF.
// CRLF is preserved verbatim and counts as one line break.
func (t *Tree) Lines() int { return t.lines(t.root) + 1 }
func (t *Tree) bytes(n int) int {
	if n == 0 {
		return 0
	}
	return t.nodes[n].bytes
}
func (t *Tree) chars(n int) int {
	if n == 0 {
		return 0
	}
	return t.nodes[n].chars
}
func (t *Tree) lines(n int) int {
	if n == 0 {
		return 0
	}
	return t.nodes[n].lines
}
func (t *Tree) height(n int) int {
	if n == 0 {
		return 0
	}
	return t.nodes[n].height
}

// Reserve preallocates edit bytes and nodes for allocation-free mutations.
func (t *Tree) Reserve(editBytes, nodes int) {
	if cap(t.added)-len(t.added) < editBytes {
		next := make([]byte, len(t.added), len(t.added)+editBytes)
		copy(next, t.added)
		t.added = next
	}
	if len(t.nodes) == 0 {
		t.nodes = append(t.nodes, node{})
	}
	if cap(t.nodes)-len(t.nodes) < nodes {
		next := make([]node, len(t.nodes), len(t.nodes)+nodes)
		copy(next, t.nodes)
		t.nodes = next
	}
}

// Reset releases the previous document and indexes the original in one pass.
func (t *Tree) Reset(value string) {
	t.original, t.snapshot, t.cached = value, value, true
	t.prefix = len(value)
	t.appendSnapshot.Reset()
	t.added = t.added[:0]
	clear(t.nodes)
	t.nodes = t.nodes[:0]
	t.root, t.free = 0, 0
	if value != "" {
		t.root = t.build(false, 0, len(value))
	}
}

func (t *Tree) decode(added bool, offset int) (rune, int) {
	if added {
		return utf8.DecodeRune(t.added[offset:])
	}
	return utf8.DecodeRuneInString(t.original[offset:])
}
func (t *Tree) measure(added bool, start, end int) piece {
	p := piece{start: start, size: end - start, added: added}
	for offset := start; offset < end; {
		r, size := t.decode(added, offset)
		offset += size
		p.runes++
		if r == '\n' {
			p.breaks++
		}
	}
	return p
}
func (t *Tree) alloc(p piece) int {
	if len(t.nodes) == 0 {
		t.nodes = append(t.nodes, node{})
	}
	n := t.free
	if n != 0 {
		t.free = t.nodes[n].left
	} else {
		n = len(t.nodes)
		t.nodes = append(t.nodes, node{})
	}
	t.nodes[n] = node{piece: p, height: 1, bytes: p.size, chars: p.runes, lines: p.breaks}
	return n
}
func (t *Tree) release(n int) { t.nodes[n] = node{left: t.free}; t.free = n }
func (t *Tree) discard(n int) {
	if n == 0 {
		return
	}
	left, right := t.nodes[n].left, t.nodes[n].right
	t.discard(left)
	t.discard(right)
	t.release(n)
}
func (t *Tree) update(n int) {
	p := &t.nodes[n]
	p.height = max(t.height(p.left), t.height(p.right)) + 1
	p.bytes = t.bytes(p.left) + p.size + t.bytes(p.right)
	p.chars = t.chars(p.left) + p.runes + t.chars(p.right)
	p.lines = t.lines(p.left) + p.breaks + t.lines(p.right)
}
func (t *Tree) rotateLeft(n int) int {
	r := t.nodes[n].right
	t.nodes[n].right = t.nodes[r].left
	t.nodes[r].left = n
	t.update(n)
	t.update(r)
	return r
}
func (t *Tree) rotateRight(n int) int {
	l := t.nodes[n].left
	t.nodes[n].left = t.nodes[l].right
	t.nodes[l].right = n
	t.update(n)
	t.update(l)
	return l
}
func (t *Tree) balance(n int) int {
	t.update(n)
	l, r := t.nodes[n].left, t.nodes[n].right
	if t.height(l)-t.height(r) > 1 {
		if t.height(t.nodes[l].right) > t.height(t.nodes[l].left) {
			t.nodes[n].left = t.rotateLeft(l)
		}
		return t.rotateRight(n)
	}
	if t.height(r)-t.height(l) > 1 {
		if t.height(t.nodes[r].left) > t.height(t.nodes[r].right) {
			t.nodes[n].right = t.rotateRight(r)
		}
		return t.rotateLeft(n)
	}
	return n
}

// join accepts trees of arbitrary heights and a detached pivot.
func (t *Tree) join(l, n, r int) int {
	if t.height(l) > t.height(r)+1 {
		t.nodes[l].right = t.join(t.nodes[l].right, n, r)
		return t.balance(l)
	}
	if t.height(r) > t.height(l)+1 {
		t.nodes[r].left = t.join(l, n, t.nodes[r].left)
		return t.balance(r)
	}
	t.nodes[n].left, t.nodes[n].right = l, r
	t.update(n)
	return n
}
func (t *Tree) popLast(n int) (int, int) {
	if t.nodes[n].right == 0 {
		return t.nodes[n].left, n
	}
	r, last := t.popLast(t.nodes[n].right)
	t.nodes[n].right = r
	return t.balance(n), last
}
func (t *Tree) popFirst(n int) (int, int) {
	if t.nodes[n].left == 0 {
		return t.nodes[n].right, n
	}
	l, first := t.popFirst(t.nodes[n].left)
	t.nodes[n].left = l
	return t.balance(n), first
}
func (t *Tree) concat(l, r int) int {
	if l == 0 {
		return r
	}
	if r == 0 {
		return l
	}
	last, first := l, r
	for t.nodes[last].right != 0 {
		last = t.nodes[last].right
	}
	for t.nodes[first].left != 0 {
		first = t.nodes[first].left
	}
	a, b := t.nodes[last].piece, t.nodes[first].piece
	if a.added == b.added && a.start+a.size == b.start && a.size+b.size <= pieceBytes {
		l, last = t.popLast(l)
		r, first = t.popFirst(r)
		t.nodes[last].piece = piece{a.start, a.size + b.size, a.runes + b.runes, a.breaks + b.breaks, a.added}
		t.release(first)
		return t.join(l, last, r)
	}
	l, last = t.popLast(l)
	return t.join(l, last, r)
}
func (t *Tree) split(n, offset int) (int, int) {
	if n == 0 {
		return 0, 0
	}
	p := t.nodes[n]
	size := t.bytes(p.left)
	if offset < size {
		l, r := t.split(p.left, offset)
		return l, t.join(r, n, p.right)
	}
	if offset > size+p.size {
		l, r := t.split(p.right, offset-size-p.size)
		return t.join(p.left, n, l), r
	}
	if offset == size {
		return p.left, t.join(0, n, p.right)
	}
	if offset == size+p.size {
		return t.join(p.left, n, 0), p.right
	}
	cut := p.start + offset - size
	var a, b piece
	if cut-p.start <= p.size/2 {
		a = t.measure(p.added, p.start, cut)
		b = piece{cut, p.size - a.size, p.runes - a.runes, p.breaks - a.breaks, p.added}
	} else {
		b = t.measure(p.added, cut, p.start+p.size)
		a = piece{p.start, p.size - b.size, p.runes - b.runes, p.breaks - b.breaks, p.added}
	}
	t.nodes[n].piece = a
	right := t.alloc(b)
	return t.join(p.left, n, 0), t.join(0, right, p.right)
}
func (t *Tree) build(added bool, start, end int) int {
	root := 0
	for start < end {
		offset, count, breaks := start, 0, 0
		for offset < end && offset-start < pieceBytes {
			r, size := t.decode(added, offset)
			if offset+size-start > pieceBytes {
				break
			}
			offset += size
			count++
			if r == '\n' {
				breaks++
			}
		}
		n := t.alloc(piece{start, offset - start, count, breaks, added})
		root = t.concat(root, n)
		start = offset
	}
	return root
}

// boundary examines at most four source bytes after locating the piece.
func (t *Tree) boundary(offset int) bool {
	if offset == 0 || offset == t.Len() {
		return true
	}
	n := t.root
	for n != 0 {
		p := t.nodes[n]
		left := t.bytes(p.left)
		if offset < left {
			n = p.left
			continue
		}
		offset -= left
		if offset > p.size {
			offset -= p.size
			n = p.right
			continue
		}
		if offset == 0 || offset == p.size {
			return true
		}
		pos := p.start + offset
		at := func(index int) byte {
			if p.added {
				return t.added[index]
			}
			return t.original[index]
		}
		if utf8.RuneStart(at(pos)) {
			return true
		}
		for previous := pos - 1; previous >= p.start && pos-previous < utf8.UTFMax; previous-- {
			if utf8.RuneStart(at(previous)) {
				_, width := t.decode(p.added, previous)
				return previous+width <= pos
			}
		}
		return true // An isolated invalid UTF-8 byte decodes as one RuneError.
	}
	return true
}

// Replace edits a byte range. Offsets must be at UTF-8 rune boundaries.
// It copies only the inserted text; unchanged document bytes never move.
func (t *Tree) Replace(start, end int, value string) {
	if start < 0 || end < start || end > t.Len() {
		panic("piecetree: invalid range")
	}
	if start == end && value == "" {
		return
	}
	if !t.boundary(start) || (end != start && !t.boundary(end)) {
		panic("piecetree: range splits a rune")
	}
	// Append before splitting, so offsets remain valid even when the arena grows.
	addedStart := len(t.added)
	t.added = append(t.added, value...)
	left, rest := t.split(t.root, start)
	deleted, right := t.split(rest, end-start)
	t.discard(deleted)
	middle := t.build(true, addedStart, len(t.added))
	t.root = t.concat(t.concat(left, middle), right)
	t.prefix = min(t.prefix, start)
	t.cached = false
}

// ByteOffset maps a clamped rune offset to bytes in O(log(pieces)+pieceBytes).
func (t *Tree) ByteOffset(index int) int {
	index = max(0, min(index, t.Runes()))
	n, offset := t.root, 0
	for n != 0 {
		p := t.nodes[n]
		count := t.chars(p.left)
		if index < count {
			n = p.left
			continue
		}
		offset += t.bytes(p.left)
		index -= count
		if index <= p.runes {
			if p.runes == p.size {
				return offset + index
			}
			pos := p.start
			for range index {
				_, size := t.decode(p.added, pos)
				pos += size
			}
			return offset + pos - p.start
		}
		index -= p.runes
		offset += p.size
		n = p.right
	}
	return offset
}

// RuneOffset maps a clamped byte offset to a rune offset, rounding down inside a rune.
func (t *Tree) RuneOffset(offset int) int {
	offset = max(0, min(offset, t.Len()))
	n, count := t.root, 0
	for n != 0 {
		p := t.nodes[n]
		size := t.bytes(p.left)
		if offset < size {
			n = p.left
			continue
		}
		count += t.chars(p.left)
		offset -= size
		if offset <= p.size {
			if p.runes == p.size {
				return count + offset
			}
			pos := p.start
			for pos < p.start+offset {
				_, width := t.decode(p.added, pos)
				if pos+width > p.start+offset {
					break
				}
				pos += width
				count++
			}
			return count
		}
		offset -= p.size
		count += p.runes
		n = p.right
	}
	return count
}

// LineStart returns the byte offset of a clamped zero-based line.
func (t *Tree) LineStart(line int) int {
	if line <= 0 {
		return 0
	}
	if line >= t.Lines() {
		return t.Len()
	}
	n, offset := t.root, 0
	for n != 0 {
		p := t.nodes[n]
		count := t.lines(p.left)
		if line <= count {
			n = p.left
			continue
		}
		line -= count
		offset += t.bytes(p.left)
		if line <= p.breaks {
			for pos := p.start; pos < p.start+p.size; {
				r, width := t.decode(p.added, pos)
				pos += width
				if r == '\n' {
					line--
					if line == 0 {
						return offset + pos - p.start
					}
				}
			}
		}
		line -= p.breaks
		offset += p.size
		n = p.right
	}
	return offset
}

// Position returns zero-based LF line and rune column for a byte offset.
func (t *Tree) Position(offset int) (line, column int) {
	offset = max(0, min(offset, t.Len()))
	n, base := t.root, 0
	for n != 0 {
		p := t.nodes[n]
		size := t.bytes(p.left)
		if offset < size {
			n = p.left
			continue
		}
		line += t.lines(p.left)
		base += size
		offset -= size
		if offset <= p.size {
			for pos := p.start; pos < p.start+offset; {
				r, width := t.decode(p.added, pos)
				if pos+width > p.start+offset {
					break
				}
				pos += width
				if r == '\n' {
					line++
				}
			}
			break
		}
		line += p.breaks
		offset -= p.size
		base += p.size
		n = p.right
	}
	column = t.RuneOffset(base+offset) - t.RuneOffset(t.LineStart(line))
	return
}
func (t *Tree) write(b *strings.Builder, n, start, end int) {
	if n == 0 || start >= end {
		return
	}
	p := t.nodes[n]
	left := t.bytes(p.left)
	if start < left {
		t.write(b, p.left, start, min(end, left))
	}
	lo, hi := max(0, start-left), min(p.size, end-left)
	if lo < hi {
		if p.added {
			b.Write(t.added[p.start+lo : p.start+hi])
		} else {
			b.WriteString(t.original[p.start+lo : p.start+hi])
		}
	}
	if end > left+p.size {
		t.write(b, p.right, max(0, start-left-p.size), end-left-p.size)
	}
}

// Slice returns an owned string for the byte range.
func (t *Tree) Slice(start, end int) string {
	if start < 0 || end < start || end > t.Len() {
		panic("piecetree: invalid range")
	}
	var b strings.Builder
	b.Grow(end - start)
	t.write(&b, t.root, start, end)
	return b.String()
}

// String materializes at most one immutable snapshot per changed version.
func (t *Tree) String() string {
	if !t.cached {
		size := t.Len()
		switch {
		case size <= t.prefix:
			// Removing a suffix can share the old immutable backing store.
			t.snapshot = t.snapshot[:size]
		case t.prefix == len(t.snapshot):
			// A bounded reserve amortizes continuous tail typing. Never rewind
			// a builder after deletion: older readers can still own its suffix.
			if t.appendSnapshot.Len() != len(t.snapshot) || t.appendSnapshot.String() != t.snapshot || t.appendSnapshot.Cap()-t.appendSnapshot.Len() < size-len(t.snapshot) {
				// Reserve a bounded amount instead of doubling a large document.
				t.appendSnapshot.Reset()
				t.appendSnapshot.Grow(size + min(max(size/8, 256), 256<<10))
				t.appendSnapshot.WriteString(t.snapshot)
			}
			t.write(&t.appendSnapshot, t.root, len(t.snapshot), size)
			t.snapshot = t.appendSnapshot.String()
		default:
			t.appendSnapshot.Reset()
			t.snapshot = t.Slice(0, size)
		}
		t.prefix = size
		t.cached = true
	}
	return t.snapshot
}

// Matches checks the cached snapshot without materializing a new version.
func (t *Tree) Matches(value string) bool { return t.cached && t.snapshot == value }

// Compact reclaims deleted bytes and unused arena nodes. Copy directly from
// the tree: String may still share a large backing store after tail deletion.
// Previously published snapshots remain immutable.
func (t *Tree) Compact() {
	value := t.Slice(0, t.Len())
	*t = Tree{}
	t.Reset(value)
}

// RetainedEditBytes reports append-buffer usage, including deleted bytes.
func (t *Tree) RetainedEditBytes() int { return len(t.added) }

// ApplySnapshot applies a known change from the current text to snapshot and
// caches the supplied immutable snapshot without copying it. The caller must
// supply a complete changed range; text outside that range must be unchanged.
func (t *Tree) ApplySnapshot(snapshot string, start, oldEnd, newEnd int) {
	if start < 0 || start > newEnd || newEnd > len(snapshot) || t.Len()-oldEnd != len(snapshot)-newEnd {
		panic("piecetree: invalid snapshot range")
	}
	t.Replace(start, oldEnd, snapshot[start:newEnd])
	t.snapshot, t.cached = snapshot, true
	t.prefix = len(snapshot)
	t.appendSnapshot.Reset()
}

// RetainedBytes includes the original buffer and discarded edit bytes.
func (t *Tree) RetainedBytes() int { return len(t.original) + len(t.added) }
