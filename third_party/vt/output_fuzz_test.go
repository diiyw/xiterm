package vt

import (
	"io"
	"testing"
)

func FuzzTerminalOutput(f *testing.F) {
	for _, input := range []string{
		"hello\r\n中文 👩‍💻 e\u0301 \ue0b0",
		"\x00\x07\x08\x09\x0a\x0d\x7f\x80\xff\xc0\xaf\xed\xa0\x80",
		"\x1b[31mred\x1b[0m\x1b[2J\x1b[H",
		"\x1b[?69h\x1b[2;4s\x1b[2;3r\x1b[4L\x1b[4M",
		"\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\",
		"\x1b[38:2::255:0:0mX\x1b[48;2;0;255;0mY",
		"\x1bP$qm\x1b\\\x1b[6n\x1b[c",
	} {
		f.Add([]byte(input), uint8(10), uint8(4), uint8(0))
	}
	f.Fuzz(func(t *testing.T, data []byte, cols, rows, chunk uint8) {
		if len(data) > 4096 {
			t.Skip()
		}
		term := NewEmulator(1+int(cols)%80, 1+int(rows)%24)
		done := make(chan struct{})
		go func() {
			_, _ = io.Copy(io.Discard, term)
			close(done)
		}()
		defer func() {
			_ = term.Close()
			<-done
		}()
		for len(data) > 0 {
			n := min(len(data), 1+int(chunk))
			_, _ = term.Write(data[:n])
			data = data[n:]
		}
		_ = term.String()
	})
}
