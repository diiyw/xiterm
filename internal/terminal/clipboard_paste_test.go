package terminal

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func TestClipboardPasteEvent(t *testing.T) {
	s := &Session{
		Term:             vt.NewEmulator(80, 24),
		mouseModes:       make(map[ansi.Mode]bool),
		clipboardReplies: make(chan clipboardReply, 1),
	}
	defer s.Term.Close()
	s.bindCallbacks()
	reader := bufio.NewReader(s.Term)

	readReply := func(action func(), delimiter byte, count int) string {
		t.Helper()
		result := make(chan string, 1)
		go func() {
			var reply strings.Builder
			for range count {
				part, err := reader.ReadString(delimiter)
				reply.WriteString(part)
				if err != nil {
					break
				}
			}
			result <- reply.String()
		}()
		action()
		select {
		case reply := <-result:
			return reply
		case <-time.After(time.Second):
			t.Fatal("terminal reply did not complete")
			return ""
		}
	}
	write := func(sequence string) func() { return func() { _, _ = s.Term.WriteString(sequence) } }
	if got := readReply(write("\x1b[?5522$p"), 'y', 1); got != "\x1b[?5522;2$y" {
		t.Fatalf("mode query returned %q", got)
	}
	write("\x1b[?5522h\x1b[?2004h")()
	if !s.ClipboardPasteEvents() {
		t.Fatal("paste event mode was not enabled")
	}

	image := bytes.Repeat([]byte{0x89, 'P', 'N', 'G', 0}, 2000)
	offerImage := func() { s.OfferClipboardPaste("text", true, image) }
	got := readReply(offerImage, '\\', 3)
	mimeList := base64.StdEncoding.EncodeToString([]byte("text/plain image/png"))
	if !strings.Contains(got, ":mime=Lg==;"+mimeList+"\x1b\\") {
		t.Fatalf("paste notification returned %q", got)
	}
	_, token, _ := strings.Cut(got, ":pw=")
	token, _, _ = strings.Cut(token, "\x1b\\")
	if token == "" {
		t.Fatal("paste notification omitted its single-use token")
	}
	request := ";aW1hZ2UvcG5n\x1b\\"
	modeRequest := "\x1b]5522;type=read:id=screen-1"
	denied := "\x1b]5522;type=read:status=EPERM:id=screen-1\x1b\\"
	if got = readReply(write(modeRequest+request), '\\', 1); got != denied {
		t.Fatalf("unprompted clipboard request returned %q", got)
	}
	authorized := modeRequest + ":pw=" + token + ":name=UGFzdGUgZXZlbnQ=" + request
	write(authorized)()
	select {
	case reply := <-s.clipboardReplies:
		var wire bytes.Buffer
		if err := reply.write(&wire); err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(strings.TrimSuffix(wire.String(), "\x1b\\"), "\x1b\\")
		if len(parts) != 5 || parts[0] != "\x1b]5522;type=read:status=OK:id=screen-1" ||
			parts[4] != "\x1b]5522;type=read:status=DONE:id=screen-1" {
			t.Fatalf("expected start, three data packets and end, got %d packets", len(parts))
		}
		var received []byte
		for _, part := range parts[1:4] {
			payload, ok := strings.CutPrefix(part, "\x1b]5522;type=read:status=DATA:id=screen-1:mime=aW1hZ2UvcG5n;")
			chunk, err := base64.StdEncoding.DecodeString(payload)
			if !ok || err != nil || len(chunk) > 4096 {
				t.Fatalf("invalid clipboard data packet of %d bytes", len(part))
			}
			received = append(received, chunk...)
		}
		if !bytes.Equal(received, image) {
			t.Fatal("image paste changed clipboard bytes")
		}
	default:
		t.Fatal("authorized clipboard request was not queued")
	}
	if got = readReply(write(authorized), '\\', 1); got != denied {
		t.Fatalf("replayed clipboard token returned %q", got)
	}
	_ = readReply(offerImage, '\\', 3)
	write("\x1b" + "c")()
	if s.ClipboardPasteEvents() || !s.ClipboardPasteDeadline().IsZero() {
		t.Fatal("terminal reset kept clipboard paste mode or its offer")
	}
}
