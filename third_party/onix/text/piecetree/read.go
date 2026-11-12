package piecetree

import (
	"errors"
	"io"
)

// ReadAt copies bytes from the tree without materializing a string snapshot.
// Like other tree operations, it must not run concurrently with mutations.
func (t *Tree) ReadAt(dst []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, errors.New("piecetree: negative read offset")
	}
	if len(dst) == 0 {
		return 0, nil
	}
	if offset >= int64(t.Len()) {
		return 0, io.EOF
	}
	n := min(len(dst), t.Len()-int(offset))
	t.copyBytes(dst[:n], t.root, int(offset), int(offset)+n)
	if n < len(dst) {
		return n, io.EOF
	}
	return n, nil
}

func (t *Tree) copyBytes(dst []byte, n, start, end int) {
	if n == 0 || start >= end {
		return
	}
	p := t.nodes[n]
	left := t.bytes(p.left)
	if start < left {
		count := min(end, left) - start
		t.copyBytes(dst[:count], p.left, start, min(end, left))
		dst = dst[count:]
	}
	lo, hi := max(0, start-left), min(p.size, end-left)
	if lo < hi {
		if p.added {
			copy(dst, t.added[p.start+lo:p.start+hi])
		} else {
			copy(dst, t.original[p.start+lo:p.start+hi])
		}
		dst = dst[hi-lo:]
	}
	if end > left+p.size {
		t.copyBytes(dst, p.right, max(0, start-left-p.size), end-left-p.size)
	}
}
