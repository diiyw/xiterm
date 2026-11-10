package terminal

import (
	"bytes"
	"time"
)

func (s *Session) writeProcess(b []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeAll(s.process, b)
}

func (s *Session) pollTransfer() bool {
	t := s.transfer
	if t == nil {
		return false
	}
	select {
	case <-t.done:
		s.TransferNotice = "ZMODEM transfer completed"
		if t.err != nil {
			s.TransferNotice = t.err.Error()
		}
		s.transfer = nil
		return true
	default:
		return false
	}
}

// The detector retains only a possible header, including across PTY reads.
// Ordinary output takes the same allocation-free path as before.
func (s *Session) routeOutput(b []byte) int {
	if t := s.transfer; t != nil {
		if !t.remoteDone {
			return t.feed(b)
		}
		// Retain the shell suffix until the helper exits. It may already
		// contain the next command's handshake (e.g. sz a; sz b).
		return 0
	}
	if s.zcount == 0 {
		i := bytes.IndexByte(b, '*')
		if i < 0 {
			_, _ = s.Term.Write(b)
			return len(b)
		}
		_, _ = s.Term.Write(b[:i])
		s.zprefix[0], s.zcount = '*', 1
		return i + 1
	}
	n := 0
	for n < len(b) && s.zcount < len(s.zprefix) {
		s.zprefix[s.zcount] = b[n]
		s.zcount++
		n++
		if !possibleZmodemHeader(s.zprefix[:s.zcount]) {
			break
		}
	}
	if s.zcount == len(s.zprefix) {
		if upload, valid := zmodemInit(s.zprefix[:]); valid {
			s.transfer = newFileTransfer(upload, s.writeProcess, s.wake)
			_ = s.transfer.feed(s.zprefix[:])
			s.zcount = 0
			return n
		}
	}
	if !possibleZmodemHeader(s.zprefix[:s.zcount]) || s.zcount == len(s.zprefix) {
		// Retain an overlapping prefix (for example *** followed by ZDLE).
		cut := 1
		for cut < s.zcount && !possibleZmodemHeader(s.zprefix[cut:s.zcount]) {
			cut++
		}
		_, _ = s.Term.Write(s.zprefix[:cut])
		s.zcount = copy(s.zprefix[:], s.zprefix[cut:s.zcount])
	}
	return n
}

func possibleZmodemHeader(b []byte) bool {
	const prefix = "**\x18B0"
	for i, c := range b {
		if i < len(prefix) {
			if c != prefix[i] {
				return false
			}
		} else if i == 5 {
			if c != '0' && c != '1' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func (s *Session) flushZmodemPrefix(force bool) bool {
	if s.zcount == 0 {
		s.zdeadline = time.Time{}
		if s.ztimer != nil {
			s.ztimer.Stop()
		}
		return false
	}
	if !force && s.zdeadline.IsZero() {
		s.zdeadline = time.Now().Add(100 * time.Millisecond)
		if s.wake != nil {
			if s.ztimer == nil {
				s.ztimer = time.AfterFunc(100*time.Millisecond, s.wake)
			} else {
				s.ztimer.Reset(100 * time.Millisecond)
			}
		}
		return false
	}
	if force || !time.Now().Before(s.zdeadline) {
		_, _ = s.Term.Write(s.zprefix[:s.zcount])
		s.zcount = 0
		s.zdeadline = time.Time{}
		s.Revision++
		return true
	}
	return false
}
