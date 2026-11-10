package terminal

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

func TestZmodemHeaderValidation(t *testing.T) {
	for _, tc := range []struct {
		header string
		upload bool
		valid  bool
	}{
		{"**\x18B00000000000000", false, true},
		{"**\x18B0100000023be50", true, true},
		{"**\x18B0100000023be51", false, false},
		{"**\x18B0800000000022d", false, false},
		{"**\x18B0100000023be5!", false, false},
		{"**\x18B01", false, false},
	} {
		upload, valid := zmodemInit([]byte(tc.header))
		if valid != tc.valid || valid && upload != tc.upload {
			t.Errorf("header %q: upload=%v valid=%v", tc.header, upload, valid)
		}
	}
}

func TestZmodemDetectorPreservesOrdinaryOutput(t *testing.T) {
	for _, payload := range []string{"plain text", "echo *", "*** hello **there*", "**\x18B0100000023be51", "中文 * text", "*中文", "**👩‍💻", "*e\u0301", "**\x18B05something"} {
		t.Run(payload, func(t *testing.T) {
			for chunk := 1; chunk <= len(payload); chunk++ {
				s := &Session{Term: vt.NewEmulator(100, 2)}
				reference := vt.NewEmulator(100, 2)
				for offset := 0; offset < len(payload); {
					end := min(len(payload), offset+chunk)
					// Compare the same PTY read boundaries: VT's grapheme
					// handling across reads is independent of the detector.
					_, _ = reference.Write([]byte(payload[offset:end]))
					for offset < end {
						offset += s.routeOutput([]byte(payload[offset:end]))
					}
				}
				s.flushZmodemPrefix(true)
				if got, want := s.Term.String(), reference.String(); got != want {
					t.Errorf("chunk %d: got %q want %q", chunk, got, want)
				}
				_ = s.Term.Close()
				_ = reference.Close()
			}
		})
	}
}

func TestZmodemFinishPreservesShellSuffix(t *testing.T) {
	for _, upload := range []bool{false, true} {
		for chunk := 1; chunk <= len(zmodemFinish)+2; chunk++ {
			ctxTransfer := newRoutingTransfer(upload)
			protocol := "some data" + zmodemFinish
			if chunk%2 == 0 {
				protocol = strings.ReplaceAll(protocol, "\n", "\x8a")
			}
			if !upload {
				protocol += "OO"
			}
			payload := []byte(protocol + "SHELL_PROMPT")
			offset := 0
			for offset < len(payload) && !ctxTransfer.remoteDone {
				end := min(len(payload), offset+chunk)
				n := ctxTransfer.feed(payload[offset:end])
				if n == 0 {
					t.Fatal("unexpected backpressure")
				}
				offset += n
			}
			if got := string(payload[offset:]); got != "SHELL_PROMPT" {
				t.Fatalf("upload=%v chunk=%d suffix=%q", upload, chunk, got)
			}
			var forwarded []byte
			for len(ctxTransfer.input) > 0 {
				b := <-ctxTransfer.input
				forwarded = append(forwarded, b...)
			}
			if !bytes.Equal(forwarded, []byte(protocol)) {
				t.Fatalf("protocol changed: %q", forwarded)
			}
			ctxTransfer.cancel()
		}
	}
}

func newRoutingTransfer(upload bool) *fileTransfer {
	// No helper is needed to exercise stream boundaries and backpressure.
	t := &fileTransfer{upload: upload, input: make(chan []byte, ioQueueSize), pool: newBytePool(inputBlockSize)}
	t.ctx, t.cancel = context.WithCancel(context.Background())
	return t
}

func TestZmodemBackpressureDoesNotConsumeBytes(t *testing.T) {
	tf := newRoutingTransfer(false)
	defer tf.cancel()
	for range cap(tf.input) {
		tf.input <- []byte("queued")
	}
	if n := tf.feed([]byte(zmodemFinish)); n != 0 || tf.finishMatch != 0 || tf.remoteDone {
		t.Fatal("backpressure advanced protocol state")
	}
}

func TestZmodemRemoteCancelPreservesShellSuffix(t *testing.T) {
	tf := newRoutingTransfer(false)
	defer tf.cancel()
	if n := tf.feed([]byte("\x18\x18")); n != 2 {
		t.Fatal("split cancellation was not consumed")
	}
	remaining := []byte("\x18\x18\x18PROMPT")
	n := tf.feed(remaining)
	if string(remaining[n:]) != "PROMPT" || !tf.remoteDone || tf.ctx.Err() == nil {
		t.Fatal("remote cancellation swallowed shell output")
	}
}

func TestZmodemFeedBoundsBlocks(t *testing.T) {
	tf := newRoutingTransfer(false)
	defer tf.cancel()
	if n := tf.feed(bytes.Repeat([]byte("x"), inputBlockSize*2)); n != inputBlockSize {
		t.Fatalf("queued %d bytes", n)
	}
}

func TestWriteAllHandlesPartialWrites(t *testing.T) {
	w := &shortTransferWriter{}
	if err := writeAll(w, []byte("payload")); err != nil || w.String() != "payload" {
		t.Fatalf("partial write: %q %v", w.String(), err)
	}
	if err := writeAll(zeroTransferWriter{}, []byte("payload")); err != io.ErrShortWrite {
		t.Fatalf("zero write: %v", err)
	}
}

type shortTransferWriter struct{ bytes.Buffer }

func (w *shortTransferWriter) Write(b []byte) (int, error) { return w.Buffer.Write(b[:1]) }

type zeroTransferWriter struct{}

func (zeroTransferWriter) Write([]byte) (int, error) { return 0, nil }

func TestZmodemPartialHeaderEventuallyFlushes(t *testing.T) {
	s := &Session{Term: vt.NewEmulator(20, 2)}
	defer s.Term.Close()
	s.routeOutput([]byte("*"))
	s.flushZmodemPrefix(false)
	s.zdeadline = time.Now().Add(-time.Millisecond)
	if !s.PendingOutput() {
		t.Fatal("expired header does not wake polling")
	}
	s.Poll()
	if s.Line(0) != "*" || s.zcount != 0 {
		t.Fatal("trailing asterisk was swallowed")
	}
}
