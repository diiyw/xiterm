package terminal

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"io"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

const (
	clipboardPasteMode      = ansi.DECMode(5522)
	clipboardChunkSize      = 4096
	maxClipboardRequestSize = 16 << 10
)

type clipboardOffer struct {
	password string
	expires  time.Time
	items    []clipboardItem
}

type clipboardItem struct {
	mime string
	data []byte
}

type clipboardReply struct {
	metadata string
	items    []clipboardItem
}

func (s *Session) ClipboardPasteEvents() bool { return s.pasteEvents }

func (s *Session) ClipboardPasteDeadline() time.Time { return s.clipboardOffer.expires }

// OfferClipboardPaste captures the clipboard belonging to this paste action.
// Requests may only retrieve that snapshot, once, from the same session.
func (s *Session) OfferClipboardPaste(text string, hasText bool, image []byte) {
	if !s.pasteEvents || s.Exited || s.TransferActive() {
		return
	}
	offer := clipboardOffer{
		password: base64.StdEncoding.EncodeToString([]byte(rand.Text())),
		expires:  time.Now().Add(10 * time.Second),
	}
	if hasText {
		offer.items = append(offer.items, clipboardItem{mime: "text/plain", data: []byte(text)})
	}
	if len(image) != 0 {
		offer.items = append(offer.items, clipboardItem{mime: "image/png", data: image})
	}
	s.clipboardOffer = offer
	s.sendClipboardListing(":pw=" + offer.password)
}

func (s *Session) sendClipboardListing(metadata string) {
	var formats []string
	for _, item := range s.clipboardOffer.items {
		formats = append(formats, item.mime)
	}
	listing := clipboardReply{
		metadata: metadata,
		items:    []clipboardItem{{mime: ".", data: []byte(strings.Join(formats, " "))}},
	}
	var packets strings.Builder
	_ = listing.write(&packets)
	s.Term.SendText(packets.String())
}

func (s *Session) handleClipboardPasteRequest(data []byte) bool {
	params, ok := bytes.CutPrefix(data, []byte("5522;"))
	if !ok || len(data) > maxClipboardRequestSize {
		return true
	}
	metadata, payload, _ := strings.Cut(string(params), ";")
	var kind, password, name, location, id string
	for field := range strings.SplitSeq(metadata, ":") {
		key, value, _ := strings.Cut(field, "=")
		switch key {
		case "type":
			kind = value
		case "pw":
			password = value
		case "name":
			name = value
		case "loc":
			location = value
		case "id":
			id = strings.Map(clipboardIDRune, value)
		}
	}
	responseID := ""
	if id != "" {
		responseID = ":id=" + id
	}
	switch kind {
	case "write":
		s.Term.SendText("\x1b]5522;type=write:status=EPERM" + responseID + "\x1b\\")
		return true
	case "read":
	default:
		return true
	}

	request, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return true
	}
	offer := s.clipboardOffer
	switch {
	case location != "":
		s.Term.SendText(clipboardPacket("ENOSYS", responseID))
	case string(request) == ".":
		if time.Now().Before(offer.expires) {
			s.sendClipboardListing(responseID)
		} else {
			s.Term.SendText(clipboardPacket("OK", responseID) + clipboardPacket("DATA", responseID+":mime=Lg==;") + clipboardPacket("DONE", responseID))
		}
	case password == "" || password != offer.password || !time.Now().Before(offer.expires) || !validClipboardName(name):
		s.Term.SendText(clipboardPacket("EPERM", responseID))
	default:
		var items []clipboardItem
		for _, mime := range strings.Fields(string(request)) {
			for _, item := range offer.items {
				if item.mime == mime && !slices.ContainsFunc(items, func(sent clipboardItem) bool { return sent.mime == mime }) {
					items = append(items, item)
				}
			}
		}
		select {
		case s.clipboardReplies <- clipboardReply{metadata: responseID, items: items}:
			s.clipboardOffer = clipboardOffer{}
		default:
			s.Term.SendText(clipboardPacket("EBUSY", responseID))
		}
	}
	return true
}

func validClipboardName(encoded string) bool {
	name, err := base64.StdEncoding.DecodeString(encoded)
	return err == nil && utf8.Valid(name) && strings.TrimSpace(string(name)) != ""
}

func clipboardIDRune(r rune) rune {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_+.", r) {
		return r
	}
	return -1
}

func clipboardPacket(status, metadata string) string {
	return "\x1b]5522;type=read:status=" + status + metadata + "\x1b\\"
}

func (reply clipboardReply) write(w io.Writer) error {
	if err := writeAll(w, []byte(clipboardPacket("OK", reply.metadata))); err != nil {
		return err
	}
	for _, item := range reply.items {
		prefix := reply.metadata + ":mime=" + base64.StdEncoding.EncodeToString([]byte(item.mime)) + ";"
		data := item.data
		for {
			n := min(clipboardChunkSize, len(data))
			packet := clipboardPacket("DATA", prefix+base64.StdEncoding.EncodeToString(data[:n]))
			if err := writeAll(w, []byte(packet)); err != nil {
				return err
			}
			data = data[n:]
			if len(data) == 0 {
				break
			}
		}
	}
	return writeAll(w, []byte(clipboardPacket("DONE", reply.metadata)))
}
