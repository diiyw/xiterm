package piecetree

import (
	"errors"
	"io"
	"iter"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

// Snapshot is an immutable, comparable text value. Copies share unchanged
// pieces; neither editing nor reading a range requires a whole-document copy.
// String explicitly materializes a version, once, and is safe for worker use.
// The zero value is empty. Snapshot and its nodes must never be pooled: readers
// may retain any published version independently of the editing buffer.
type Snapshot struct {
	state *snapshotState
	flat  string
}
type snapshotState struct {
	root         *snapshotNode
	flat         string
	once         sync.Once
	materialized atomic.Bool
}
type snapshotNode struct {
	left, right                  *snapshotNode
	text                         string
	origin                       string
	offset                       int
	bytes, runes, breaks, height int
}

// FromString wraps immutable text without allocating or indexing it.
func FromString(value string) Snapshot { return Snapshot{flat: value} }
func (s Snapshot) Len() int {
	if s.state == nil {
		return len(s.flat)
	}
	return s.state.root.bytes
}
func (s Snapshot) String() string {
	if s.state == nil {
		return s.flat
	}
	s.state.once.Do(func() {
		var b strings.Builder
		b.Grow(s.Len())
		for part := range s.Chunks(0, s.Len()) {
			b.WriteString(part)
		}
		s.state.flat = b.String()
		s.state.materialized.Store(true)
	})
	return s.state.flat
}
func (s Snapshot) Materialized() bool { return s.state == nil || s.state.materialized.Load() }
func snapshotHeight(n *snapshotNode) int {
	if n == nil {
		return 0
	}
	return n.height
}
func snapshotLeaf(value string) *snapshotNode {
	if value == "" {
		return nil
	}
	return snapshotLeafRange(value, 0, len(value))
}
func snapshotBranch(l, r *snapshotNode) *snapshotNode {
	if l == nil {
		return r
	}
	if r == nil {
		return l
	}
	n := &snapshotNode{left: l, right: r, bytes: l.bytes + r.bytes, runes: l.runes + r.runes, breaks: l.breaks + r.breaks, height: max(l.height, r.height) + 1}
	if l.origin != "" && l.origin == r.origin && l.offset+l.bytes == r.offset {
		n.origin, n.offset = l.origin, l.offset
	}
	return n
}
func snapshotBalance(l, r *snapshotNode) *snapshotNode {
	if snapshotHeight(l) > snapshotHeight(r)+1 {
		if snapshotHeight(l.left) < snapshotHeight(l.right) {
			return snapshotBranch(snapshotBranch(l.left, l.right.left), snapshotBranch(l.right.right, r))
		}
		return snapshotBranch(l.left, snapshotBranch(l.right, r))
	}
	if snapshotHeight(r) > snapshotHeight(l)+1 {
		if snapshotHeight(r.right) < snapshotHeight(r.left) {
			return snapshotBranch(snapshotBranch(l, r.left.left), snapshotBranch(r.left.right, r.right))
		}
		return snapshotBranch(snapshotBranch(l, r.left), r.right)
	}
	return snapshotBranch(l, r)
}
func snapshotJoin(l, r *snapshotNode) *snapshotNode {
	if l == nil {
		return r
	}
	if r == nil {
		return l
	}
	if l.left == nil && r.left == nil && l.bytes+r.bytes <= pieceBytes {
		return snapshotLeaf(l.text + r.text)
	}
	if l.height > r.height+1 {
		return snapshotBalance(l.left, snapshotJoin(l.right, r))
	}
	if r.height > l.height+1 {
		return snapshotBalance(snapshotJoin(l, r.left), r.right)
	}
	return snapshotBranch(l, r)
}
func snapshotSplit(n *snapshotNode, offset int) (*snapshotNode, *snapshotNode) {
	if offset <= 0 {
		return nil, n
	}
	if offset >= n.bytes {
		return n, nil
	}
	if n.left == nil {
		return snapshotLeafRange(n.origin, n.offset, n.offset+offset), snapshotLeafRange(n.origin, n.offset+offset, n.offset+n.bytes)
	}
	left := n.left.bytes
	if offset < left {
		a, b := snapshotSplit(n.left, offset)
		return a, snapshotJoin(b, n.right)
	}
	if offset == left {
		return n.left, n.right
	}
	a, b := snapshotSplit(n.right, offset-left)
	return snapshotJoin(n.left, a), b
}
func snapshotBuild(value string) *snapshotNode {
	if len(value) <= pieceBytes {
		return snapshotLeaf(value)
	}
	// One arena owns the original immutable index. Published nodes are never
	// modified; edits allocate only their replacement paths.
	// Count only split boundaries first, not text contents. Exact sizing avoids
	// retaining an extra power-of-two arena for documents near a size boundary.
	arena := make([]snapshotNode, snapshotNodeCount(value, 0, len(value)))
	used := 0
	var build func(int, int) *snapshotNode
	build = func(start, end int) *snapshotNode {
		n := &arena[used]
		used++
		n.bytes = end - start
		n.origin, n.offset = value, start
		if end-start <= pieceBytes {
			n.text = value[start:end]
			n.runes = utf8.RuneCountInString(n.text)
			n.breaks = strings.Count(n.text, "\n")
			n.height = 1
			return n
		}
		cut := snapshotCut(value, start, end)
		n.left, n.right = build(start, cut), build(cut, end)
		n.runes = n.left.runes + n.right.runes
		n.breaks = n.left.breaks + n.right.breaks
		n.height = max(n.left.height, n.right.height) + 1
		return n
	}
	return build(0, len(value))
}

func snapshotCut(value string, start, end int) int {
	cut := start + (end-start)/2
	for !FromString(value).boundary(cut) {
		cut++
	}
	return cut
}

func snapshotNodeCount(value string, start, end int) int {
	if end-start <= pieceBytes {
		return 1
	}
	cut := snapshotCut(value, start, end)
	return 1 + snapshotNodeCount(value, start, cut) + snapshotNodeCount(value, cut, end)
}
func snapshotLeafRange(origin string, start, end int) *snapshotNode {
	if start == end {
		return nil
	}
	value := origin[start:end]
	return &snapshotNode{text: value, origin: origin, offset: start, bytes: end - start, runes: utf8.RuneCountInString(value), breaks: strings.Count(value, "\n"), height: 1}
}

func (s Snapshot) root() *snapshotNode {
	if s.state != nil {
		return s.state.root
	}
	return snapshotBuild(s.flat)
}
func (s Snapshot) check(start, end int) {
	if start < 0 || end < start || end > s.Len() {
		panic("piecetree: invalid snapshot range")
	}
}

// Replace returns a new version, owning the inserted bytes. Callers provide
// UTF-8 byte boundaries, as with Tree.Replace.
func (s Snapshot) Replace(start, end int, value string) Snapshot {
	s.check(start, end)
	if start == end && value == "" {
		return s
	}
	if !s.boundary(start) || end != start && !s.boundary(end) {
		panic("piecetree: range splits a rune")
	}
	if end-start == len(value) && s.rangeEqual(start, end, value) {
		return s
	}
	root := s.root()
	var l, r *snapshotNode
	if root != nil {
		l, r = snapshotSplit(root, start)
		if r != nil {
			_, r = snapshotSplit(r, end-start)
		}
	}
	root = snapshotJoin(snapshotJoin(l, snapshotBuild(strings.Clone(value))), r)
	if root == nil {
		return Snapshot{}
	}
	return Snapshot{state: &snapshotState{root: root}}
}

func (s Snapshot) boundary(offset int) bool {
	if offset == 0 || offset == s.Len() || utf8.RuneStart(s.ByteAt(offset)) {
		return true
	}
	for previous := offset - 1; previous >= 0 && offset-previous < utf8.UTFMax; previous-- {
		if utf8.RuneStart(s.ByteAt(previous)) {
			_, width := s.DecodeRune(previous)
			return previous+width <= offset
		}
	}
	return true // Isolated invalid bytes each decode as one RuneError.
}
func snapshotChunks(n *snapshotNode, start, end int, yield func(string) bool) bool {
	if n == nil || start >= end {
		return true
	}
	if n.left == nil {
		return yield(n.text[start:end])
	}
	left := n.left.bytes
	if start < left && !snapshotChunks(n.left, start, min(end, left), yield) {
		return false
	}
	return end <= left || snapshotChunks(n.right, max(0, start-left), end-left, yield)
}

// Chunks visits immutable pieces of a byte range without allocating.
func (s Snapshot) Chunks(start, end int) iter.Seq[string] {
	s.check(start, end)
	return func(yield func(string) bool) {
		if start == end {
			return
		}
		if s.state == nil {
			yield(s.flat[start:end])
			return
		}
		snapshotChunks(s.state.root, start, end, yield)
	}
}

// Slice shares a contiguous piece, copying only when the range crosses pieces.
func (s Snapshot) Slice(start, end int) string {
	s.check(start, end)
	if s.state == nil {
		return s.flat[start:end]
	}
	n := s.state.root
	lo, hi := start, end
	if n.origin != "" {
		return n.origin[n.offset+lo : n.offset+hi]
	}
	for n.left != nil {
		if n.origin != "" {
			return n.origin[n.offset+lo : n.offset+hi]
		}
		left := n.left.bytes
		if hi <= left {
			n = n.left
		} else if lo >= left {
			lo -= left
			hi -= left
			n = n.right
		} else {
			break
		}
	}
	if n.left == nil {
		return n.text[lo:hi]
	}
	var b strings.Builder
	b.Grow(end - start)
	snapshotChunks(n, lo, hi, func(v string) bool { b.WriteString(v); return true })
	return b.String()
}
func (s Snapshot) ByteAt(index int) byte {
	if index < 0 || index >= s.Len() {
		panic("piecetree: invalid byte index")
	}
	if s.state == nil {
		return s.flat[index]
	}
	n := s.state.root
	for n.left != nil {
		if index < n.left.bytes {
			n = n.left
		} else {
			index -= n.left.bytes
			n = n.right
		}
	}
	return n.text[index]
}
func (s Snapshot) Runes() int {
	if s.state == nil {
		return utf8.RuneCountInString(s.flat)
	}
	return s.state.root.runes
}
func (s Snapshot) Lines() int {
	if s.state == nil {
		return strings.Count(s.flat, "\n") + 1
	}
	return s.state.root.breaks + 1
}
func (s Snapshot) ByteOffset(index int) int {
	index = max(0, index)
	offset := 0
	value := s.flat
	if s.state != nil {
		n := s.state.root
		for n.left != nil {
			if index < n.left.runes {
				n = n.left
			} else {
				index -= n.left.runes
				offset += n.left.bytes
				n = n.right
			}
		}
		value = n.text
	}
	for pos := range value {
		if index == 0 {
			return offset + pos
		}
		index--
	}
	return offset + len(value)
}
func (s Snapshot) RuneOffset(offset int) int {
	offset = max(0, min(offset, s.Len()))
	count := 0
	value := s.flat
	if s.state != nil {
		n := s.state.root
		for n.left != nil {
			if offset < n.left.bytes {
				n = n.left
			} else {
				offset -= n.left.bytes
				count += n.left.runes
				n = n.right
			}
		}
		value = n.text
	}
	for pos := 0; pos < offset; {
		_, size := utf8.DecodeRuneInString(value[pos:])
		if pos+size > offset {
			break
		}
		pos += size
		count++
	}
	return count
}
func (s Snapshot) LineStart(line int) int {
	if line <= 0 {
		return 0
	}
	offset := 0
	value := s.flat
	if s.state != nil {
		n := s.state.root
		if line > n.breaks {
			return n.bytes
		}
		for n.left != nil {
			if line <= n.left.breaks {
				n = n.left
			} else {
				line -= n.left.breaks
				offset += n.left.bytes
				n = n.right
			}
		}
		value = n.text
	}
	for i := 0; i < len(value); i++ {
		if value[i] == '\n' {
			line--
			if line == 0 {
				return offset + i + 1
			}
		}
	}
	return offset + len(value)
}
func (s Snapshot) Position(offset int) (line, column int) {
	offset = max(0, min(offset, s.Len()))
	local := offset
	value := s.flat
	if s.state != nil {
		n := s.state.root
		for n.left != nil {
			if local < n.left.bytes {
				n = n.left
			} else {
				local -= n.left.bytes
				line += n.left.breaks
				n = n.right
			}
		}
		value = n.text
	}
	line += strings.Count(value[:local], "\n")
	column = s.RuneOffset(offset) - s.RuneOffset(s.LineStart(line))
	return
}

func (s Snapshot) IndexByte(start int, b byte) int {
	for part := range s.Chunks(start, s.Len()) {
		if i := strings.IndexByte(part, b); i >= 0 {
			return start + i
		}
		start += len(part)
	}
	return -1
}
func (s Snapshot) ReadAt(p []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, errors.New("piecetree: negative read offset")
	}
	if len(p) == 0 {
		return 0, nil
	}
	if offset >= int64(s.Len()) {
		return 0, io.EOF
	}
	n := 0
	for part := range s.Chunks(int(offset), int(offset)+min(s.Len()-int(offset), len(p))) {
		n += copy(p[n:], part)
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
func (s Snapshot) Equal(other Snapshot) bool {
	if s == other {
		return true
	}
	if s.Len() != other.Len() {
		return false
	}
	start, _, _ := s.ChangedRange(other)
	return start == s.Len()
}

// ChangedRange finds a single encompassing edit, preserving rune boundaries.
// Chunk comparison skips unchanged text without materializing either version.
func (s Snapshot) ChangedRange(other Snapshot) (start, oldEnd, newEnd int) {
	if s == other {
		return s.Len(), s.Len(), s.Len()
	}
	limit := min(s.Len(), other.Len())
	for start < limit {
		a := s.contiguous(start)
		b := other.contiguous(start)
		n := min(len(a), len(b))
		if a[:n] == b[:n] {
			start += n
			continue
		}
		for i := 0; i < n && a[i] == b[i]; i++ {
			start++
		}
		break
	}
	for start > 0 && start < limit && (!utf8.RuneStart(s.ByteAt(start)) || !utf8.RuneStart(other.ByteAt(start))) {
		start--
	}
	if s.splitsCRLF(start) || other.splitsCRLF(start) {
		start--
	}
	oldEnd, newEnd = s.Len(), other.Len()
	for oldEnd > start && newEnd > start {
		a, b := s.contiguousBefore(oldEnd), other.contiguousBefore(newEnd)
		n := min(len(a), len(b), oldEnd-start, newEnd-start)
		if a[len(a)-n:] == b[len(b)-n:] {
			oldEnd -= n
			newEnd -= n
			continue
		}
		for i := 1; i <= n && a[len(a)-i] == b[len(b)-i]; i++ {
			oldEnd--
			newEnd--
		}
		break
	}
	for oldEnd < s.Len() && newEnd < other.Len() && (!utf8.RuneStart(s.ByteAt(oldEnd)) || !utf8.RuneStart(other.ByteAt(newEnd))) {
		oldEnd++
		newEnd++
	}
	if s.splitsCRLF(oldEnd) || other.splitsCRLF(newEnd) {
		oldEnd++
		newEnd++
	}
	return
}
func (s Snapshot) contiguous(start int) string {
	if s.state == nil {
		return s.flat[start:]
	}
	n := s.state.root
	for n.left != nil {
		if start < n.left.bytes {
			n = n.left
		} else {
			start -= n.left.bytes
			n = n.right
		}
	}
	return n.text[start:]
}

// Value permits the same geometry helpers to accept labels and document text.
type Value interface{ string | Snapshot }

func Text[V Value](value V) Snapshot {
	switch v := any(value).(type) {
	case string:
		return FromString(v)
	case Snapshot:
		return v
	}
	panic("unreachable")
}
func (s Snapshot) TrimSpaceEmpty() bool {
	for p := range s.Chunks(0, s.Len()) {
		if strings.TrimSpace(p) != "" {
			return false
		}
	}
	return true
}
func (s Snapshot) DecodeRune(offset int) (rune, int) {
	p := s.contiguous(offset)
	return utf8.DecodeRuneInString(p)
}

func (s Snapshot) contiguousBefore(end int) string {
	if s.state == nil {
		return s.flat[:end]
	}
	n := s.state.root
	for n.left != nil {
		if end <= n.left.bytes {
			n = n.left
		} else {
			end -= n.left.bytes
			n = n.right
		}
	}
	return n.text[:end]
}
func (s Snapshot) rangeEqual(start, end int, value string) bool {
	if end-start != len(value) {
		return false
	}
	for part := range s.Chunks(start, end) {
		if part != value[:len(part)] {
			return false
		}
		value = value[len(part):]
	}
	return true
}

func (s Snapshot) splitsCRLF(offset int) bool {
	return offset > 0 && offset < s.Len() && s.ByteAt(offset-1) == '\r' && s.ByteAt(offset) == '\n'
}
