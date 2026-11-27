package terminal

import (
	"bytes"
	"encoding/base64"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func (s *Session) handleClipboard(data []byte) bool {
	// A full parser buffer may contain a truncated control string.
	if s.WriteClipboard == nil || len(data) == cap(data) {
		return true
	}
	params, ok := bytes.CutPrefix(data, []byte("52;"))
	if !ok {
		return true
	}
	targets, encoded, ok := bytes.Cut(params, []byte(";"))
	if !ok || len(bytes.Trim(targets, "cpqs01234567")) != 0 {
		return true
	}
	if len(targets) > 0 && !bytes.ContainsRune(targets, ansi.SystemClipboard) {
		return true
	}
	// An OSC 52 query must never send the desktop clipboard back to the PTY.
	if bytes.Equal(encoded, []byte("?")) {
		return true
	}
	decoded, err := base64.StdEncoding.AppendDecode(nil, encoded)
	if err == nil && utf8.Valid(decoded) && bytes.IndexByte(decoded, 0) < 0 {
		s.WriteClipboard(string(decoded))
	}
	return true
}
