// Package terminal connects a native PTY to a VT emulator. The UI goroutine
// owns Emulator; the reader and writer goroutines only move bytes.
package terminal

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/diiyw/xiterm/internal/config"
)

// process owns the platform PTY and the entire shell lifetime.
type process interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	Wait() error
}

type Session struct {
	MouseTracking     bool
	mouseModes        map[ansi.Mode]bool
	Term              *vt.Emulator
	Profile           config.Profile
	Title             string
	Directory         string
	ExitErr           error
	IOErr             error
	Exited            bool
	Revision          uint64
	inputCursor       uv.Position
	inputCursorSet    bool
	applicationCursor bool
	process           process
	output            chan []byte
	pendingOutput     []byte
	pendingBuffer     []byte
	outputPool        bytePool
	finished          chan error
	stop              chan struct{}
	collectorDone     chan struct{}
	writerDone        chan struct{}
	readerDone        chan struct{}
	closeOnce         sync.Once
	writeMu           sync.Mutex
	wake              func()
	transfer          *fileTransfer
	TransferNotice    string
	zprefix           [18]byte
	zcount            int
	zdeadline         time.Time
	ztimer            *time.Timer
	history           *scrollback
	findLine          []byte
	findColumns       []int
	findNeedle        []byte
}

func Start(profile config.Profile, cols, rows, scrollback int, wake func()) (*Session, error) {
	f, err := startProcess(profile, cols, rows)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", profile.Shell, err)
	}

	s := &Session{Term: vt.NewEmulator(cols, rows), Profile: profile, Title: profile.Name, Directory: profile.Directory, process: f, output: make(chan []byte, ioQueueSize), finished: make(chan error, 1), stop: make(chan struct{}), collectorDone: make(chan struct{}), writerDone: make(chan struct{}), readerDone: make(chan struct{}), outputPool: newBytePool(outputBlockSize)}
	s.wake = wake
	s.initScrollback(scrollback)
	s.Term.SetDefaultForegroundColor(color.RGBA{R: 216, G: 222, B: 233, A: 255})
	s.Term.SetDefaultBackgroundColor(color.RGBA{R: 22, G: 25, B: 31, A: 255})
	palette := []uint32{0x242933, 0xe06c75, 0x98c379, 0xe5c07b, 0x61afef, 0xc678dd, 0x56b6c2, 0xd8dee9, 0x5c6370, 0xf28b91, 0xb3d994, 0xf2d396, 0x85c1ff, 0xdb9df2, 0x7fced8, 0xffffff}
	for i, rgb := range palette {
		s.Term.SetIndexedColor(i, color.RGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: 255})
	}
	s.mouseModes = make(map[ansi.Mode]bool)
	s.bindCallbacks()
	// Read responses and keyboard encodings even when the child is backpressured.
	input := make(chan []byte, ioQueueSize)
	inputPool := newBytePool(inputBlockSize)
	go func() {
		defer close(input)
		defer close(s.collectorDone)
		for {
			buf := inputPool.get()
			n, err := s.Term.Read(buf)
			if n > 0 {
				select {
				case input <- buf[:n]:
					buf = nil // The writer returns the block after Write completes.
				case <-s.stop:
					inputPool.put(buf)
					return
				}
			}
			inputPool.put(buf)
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer close(s.writerDone)
		defer inputPool.clear()
		failed := false
		for b := range input {
			if !failed {
				err := s.writeProcess(b)
				failed = err != nil
			}
			// Continue draining after an error so the collector can still stop.
			inputPool.put(b)
		}
	}()
	go func() {
		defer close(s.readerDone)
		defer close(s.output)
		for {
			buf := s.outputPool.get()
			n, err := f.Read(buf)
			if n > 0 {
				select {
				case s.output <- buf[:n]:
					buf = nil // Poll owns the whole block, including partial writes.
					if wake != nil {
						wake()
					}
				case <-s.stop:
					// Keep draining during ConPTY shutdown, including on older Windows.
				}
			}
			s.outputPool.put(buf)
			if err != nil {
				return
			}
		}
	}()
	go func() {
		err := f.Wait()
		s.finished <- err
		if wake != nil {
			wake()
		}
	}()
	return s, nil
}

func (s *Session) bindCallbacks() {
	s.Term.SetCallbacks(vt.Callbacks{
		Title:            func(title string) { s.Title = title },
		WorkingDirectory: func(cwd string) { s.Directory = cwd },
		EnableMode: func(mode ansi.Mode) {
			if mode == ansi.ModeCursorKeys {
				s.applicationCursor = true
			}
			s.mouseMode(mode, true)
		},
		DisableMode: func(mode ansi.Mode) {
			if mode == ansi.ModeCursorKeys {
				s.applicationCursor = false
			}
			s.mouseMode(mode, false)
		},
		CursorVisibility: func(bool) {
			// Snapshot the insertion point before a TUI moves its hidden cursor
			// around to repaint. PTY reads can split that repaint across frames.
			s.inputCursor = s.Term.CursorPosition()
			s.inputCursorSet = true
		},
		AltScreen: func(bool) { s.inputCursorSet = false },
	})
}

// InputPosition keeps IME panels at the insertion point during hidden-cursor
// repaints. A visible cursor always supplies the current position.
func (s *Session) InputPosition() uv.Position {
	pos := s.Term.CursorPosition()
	if s.Term.CursorHidden() && s.inputCursorSet {
		pos = s.inputCursor
	}
	pos.X = max(0, min(pos.X, s.Term.Width()-1))
	pos.Y = max(0, min(pos.Y, s.Term.Height()-1))
	return pos
}

// environment replaces inherited entries instead of adding ambiguous duplicates.
func environment(base, extra []string) []string {
	values := map[string]string{}
	keys := []string{}
	for _, entry := range append(append(append([]string{}, base...), extra...), "TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=XiTerm") {
		key, _, ok := strings.Cut(entry, "=")
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(key)
		}
		if !ok {
			continue
		}
		if _, ok = values[key]; !ok {
			keys = append(keys, key)
		}
		values[key] = entry
	}
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

// PendingOutput keeps the event loop awake when a bounded Poll leaves queued
// data behind, even if the reader's wake notifications were coalesced.
func (s *Session) PendingOutput() bool {
	if s.transfer != nil {
		select {
		case <-s.transfer.done:
			return true
		default:
		}
	}
	return len(s.pendingOutput) > 0 || len(s.output) > 0 || len(s.finished) > 0 || s.zcount > 0 && !time.Now().Before(s.zdeadline)
}

const pollBudget = 2 * time.Millisecond
const pollWriteSize = 1024

// Poll yields regularly to the native event loop, even when one PTY read
// contains an expensive screen repaint or thousands of scrolling lines.
func (s *Session) Poll() bool {
	return s.PollUntil(time.Now().Add(pollBudget))
}

// PollUntil also respects a shared deadline when several sessions are busy.
func (s *Session) PollUntil(deadline time.Time) bool {
	if limit := time.Now().Add(pollBudget); limit.Before(deadline) {
		deadline = limit
	}
	changed := false
	if s.pollTransfer() {
		changed = true
	}
	for range 8 {
		if !time.Now().Before(deadline) {
			break
		}
		if len(s.pendingOutput) == 0 {
			select {
			case b, ok := <-s.output:
				if !ok {
					goto done
				}
				s.pendingOutput = b
				s.pendingBuffer = b
			default:
				goto done
			}
		}
		b := s.pendingOutput
		n := outputPrefix(b)
		history := s.Term.ScrollbackLen()
		n = s.routeOutput(b[:n])
		if n == 0 {
			break
		}
		s.pendingOutput = b[n:]
		if len(s.pendingOutput) == 0 {
			s.pendingOutput = nil
			s.outputPool.put(s.pendingBuffer)
			s.pendingBuffer = nil
		}
		s.collectHistory()
		if s.Term.ScrollbackLen() < history {
			s.releaseClearedHistory()
		}
		changed = true
		s.Revision++
	}
done:
	if s.flushZmodemPrefix(false) {
		s.collectHistory()
		changed = true
	}
	// Do not discard the final output when process exit arrives during a
	// partial read. The next poll must finish applying the queued bytes first.
	if len(s.pendingOutput) > 0 || len(s.output) > 0 {
		return changed
	}
	select {
	case err := <-s.finished:
		s.CancelTransfer()
		s.flushZmodemPrefix(true)
		s.Exited = true
		s.ExitErr = err
		changed = true
	default:
	}
	return changed
}

func outputPrefix(b []byte) int {
	if len(b) <= pollWriteSize {
		return len(b)
	}
	// ASCII on both sides is a safe, cheap boundary for ordinary TUI output.
	if b[pollWriteSize-1] < 0x80 && b[pollWriteSize] < 0x80 {
		return pollWriteSize
	}
	// VT flushes a grapheme at the end of each Write. Keep combining marks,
	// emoji and UTF-8 code points together at the boundaries we introduce.
	n := 0
	for n < pollWriteSize {
		cluster, _ := ansi.FirstGraphemeCluster(b[n:], ansi.GraphemeWidth)
		next := n + len(cluster)
		if next == n || next > pollWriteSize && n > 0 {
			break
		}
		n = next
	}
	return max(1, n)
}

func (s *Session) Resize(cols, rows int) error {
	cols = max(2, cols)
	rows = max(1, rows)
	if cols == s.Term.Width() && rows == s.Term.Height() {
		return nil
	}
	if !s.Exited {
		if err := s.process.Resize(cols, rows); err != nil {
			return err
		}
	}
	s.Term.Resize(cols, rows)
	s.Revision++
	return nil
}

func (s *Session) SendText(text string) {
	if s.TransferActive() {
		if strings.ContainsRune(text, '\x03') {
			s.CancelTransfer()
		}
		return
	}
	if !s.Exited {
		s.Term.SendText(text)
	}
}
func (s *Session) SendKey(key uv.KeyPressEvent) {
	if s.TransferActive() {
		if key.Code == 'c' && key.Mod&uv.ModCtrl != 0 || key.Code == uv.KeyEscape {
			s.CancelTransfer()
		}
		return
	}
	if !s.Exited {
		// vt v0.1.0 honors DECCKM for arrows but always encodes Home/End as
		// CSI. xterm applications expect SS3 for these keys in cursor mode.
		if s.applicationCursor && key.Mod == 0 {
			switch key.Code {
			case uv.KeyHome:
				s.Term.SendText("\x1bOH")
				return
			case uv.KeyEnd:
				s.Term.SendText("\x1bOF")
				return
			}
		}
		s.Term.SendKey(key)
	}
}
func (s *Session) Paste(text string) {
	if !s.Exited && !s.TransferActive() {
		s.Term.Paste(text)
	}
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.stop)
		s.CancelTransfer()
		if s.ztimer != nil {
			s.ztimer.Stop()
		}
		_ = s.process.Close()
		if s.transfer != nil {
			<-s.transfer.done
			s.transfer = nil
		}
		_ = s.Term.InputPipe().(io.Closer).Close()
		<-s.collectorDone
		<-s.writerDone
		<-s.readerDone
		_ = s.Term.Close()
		s.pendingOutput, s.pendingBuffer = nil, nil
		for range s.output {
		}
		s.outputPool.clear()
	})
}

func (s *Session) HistoryLen() int {
	if s.Term.IsAltScreen() {
		return 0
	}
	n := s.Term.ScrollbackLen()
	if s.history != nil {
		n += s.history.size
	}
	return n
}

// ClearHistory also drops the line references retained by vt's backing slice.
// ClearScrollback alone only resets its length, keeping old cells alive.
func (s *Session) ClearHistory() {
	if s.history != nil {
		s.history.clear()
	}
	s.Term.ClearScrollback()
	s.releaseClearedHistory()
	s.Revision++
}

func (s *Session) releaseClearedHistory() {
	if history := s.Term.Scrollback(); history != nil {
		lines := history.Lines()
		clear(lines[len(lines):cap(lines)])
	}
}
func (s *Session) Cell(x, row int) *uv.Cell {
	history := s.HistoryLen()
	if row < history {
		if s.history != nil {
			if row < s.history.size {
				return s.history.cell(x, row)
			}
			row -= s.history.size
		}
		return s.Term.ScrollbackCellAt(x, row)
	}
	return s.Term.CellAt(x, row-history)
}

// cellValue reads packed history without allocating a decoded row or touching
// the viewport cache. Search and copy can scan any amount of history this way.
func (s *Session) cellValue(x, row int) (uv.Cell, bool) {
	if s.history != nil && !s.Term.IsAltScreen() && row < s.history.size {
		return s.history.cellValue(x, row)
	}
	if c := s.Cell(x, row); c != nil {
		return *c, true
	}
	return uv.Cell{}, false
}

func (s *Session) Line(row int) string {
	var b strings.Builder
	for x := 0; x < s.Term.Width(); x++ {
		c, ok := s.cellValue(x, row)
		if !ok {
			b.WriteByte(' ')
		} else if c.Width > 0 {
			b.WriteString(c.Content)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

type Position struct{ Row, Col int }
type Match struct{ Row, Col, Width int }

func (s *Session) Selection(a, b Position) string {
	if a.Row > b.Row || a.Row == b.Row && a.Col > b.Col {
		a, b = b, a
	}
	var out strings.Builder
	line := make([]byte, 0, s.Term.Width())
	for row := a.Row; row <= b.Row; row++ {
		left, right := 0, s.Term.Width()-1
		if row == a.Row {
			left = a.Col
		}
		if row == b.Row {
			right = b.Col
		}
		line = line[:0]
		for x := left; x <= right; x++ {
			c, ok := s.cellValue(x, row)
			if !ok {
				line = append(line, ' ')
			} else if c.Width > 0 {
				line = append(line, c.Content...)
			}
		}
		out.Write(bytes.TrimRight(line, " "))
		if row != b.Row {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// Find maps matching UTF-8 byte ranges back to terminal columns, including wide cells.
func (s *Session) Find(query string) []Match {
	return s.FindInto(nil, query)
}

// FindInto reuses dst and the session's row scratch. The caller owns the
// returned matches; subsequent searches only overwrite them if passed as dst.
// Like the emulator, search is owned by the UI goroutine.
func (s *Session) FindInto(dst []Match, query string) []Match {
	matches := dst[:0]
	if query == "" {
		return matches
	}
	s.findNeedle = appendLower(s.findNeedle[:0], query)
	needle := s.findNeedle
	line, columns := s.findLine[:0], s.findColumns[:0]
	cols, rows := s.Term.Width(), s.HistoryLen()+s.Term.Height()
	for row := 0; row < rows; row++ {
		line = line[:0]
		columns = columns[:0]
		for x := 0; x < cols; x++ {
			c, ok := s.cellValue(x, row)
			value := " "
			if ok {
				if c.Width == 0 {
					continue
				}
				value = c.Content
			}
			start := len(line)
			line = appendLower(line, value)
			for range len(line) - start {
				columns = append(columns, x)
			}
		}
		for offset := 0; offset < len(line); {
			i := bytes.Index(line[offset:], needle)
			if i < 0 {
				break
			}
			start := offset + i
			end := start + len(needle) - 1
			col := columns[start]
			last := columns[end]
			width := 1
			if cell, ok := s.cellValue(last, row); ok {
				width = max(1, cell.Width)
			}
			matches = append(matches, Match{Row: row, Col: col, Width: last - col + width})
			offset = end + 1
		}
	}
	s.findLine, s.findColumns = line[:0], columns[:0]
	return matches
}

func appendLower(dst []byte, value string) []byte {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= utf8.RuneSelf {
			for _, r := range value[i:] {
				dst = utf8.AppendRune(dst, unicode.ToLower(r))
			}
			return dst
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		dst = append(dst, c)
	}
	return dst
}

func (s *Session) mouseMode(mode ansi.Mode, enabled bool) {
	switch mode {
	case ansi.ModeMouseX10, ansi.ModeMouseNormal, ansi.ModeMouseButtonEvent, ansi.ModeMouseAnyEvent:
		s.mouseModes[mode] = enabled
	}
	s.MouseTracking = false
	for _, active := range s.mouseModes {
		s.MouseTracking = s.MouseTracking || active
	}
}
func (s *Session) SendMouse(event uv.MouseEvent) {
	if !s.Exited && !s.TransferActive() {
		s.Term.SendMouse(event)
	}
}

// ResolveColor applies the terminal palette before converting ANSI colors to RGB.
func (s *Session) ResolveColor(c color.Color) color.Color {
	switch value := c.(type) {
	case ansi.BasicColor:
		return s.Term.IndexedColor(int(value))
	case ansi.IndexedColor:
		return s.Term.IndexedColor(int(value))
	}
	return c
}
