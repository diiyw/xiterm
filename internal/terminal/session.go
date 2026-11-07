// Package terminal connects a native PTY to a VT emulator. The UI goroutine
// owns Emulator; the reader and writer goroutines only move bytes.
package terminal

import (
	"fmt"
	"image/color"
	"io"
	"runtime"
	"strings"
	"sync"

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
	MouseTracking bool
	mouseModes    map[ansi.Mode]bool
	Term          *vt.Emulator
	Profile       config.Profile
	Title         string
	Directory     string
	ExitErr       error
	IOErr         error
	Exited        bool
	Revision      uint64
	process       process
	output        chan []byte
	finished      chan error
	stop          chan struct{}
	collectorDone chan struct{}
	closeOnce     sync.Once
}

func Start(profile config.Profile, cols, rows, scrollback int, wake func()) (*Session, error) {
	f, err := startProcess(profile, cols, rows)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", profile.Shell, err)
	}

	s := &Session{Term: vt.NewEmulator(cols, rows), Profile: profile, Title: profile.Name, Directory: profile.Directory, process: f, output: make(chan []byte, 64), finished: make(chan error, 1), stop: make(chan struct{}), collectorDone: make(chan struct{})}
	s.Term.SetScrollbackSize(scrollback)
	s.Term.SetDefaultForegroundColor(color.RGBA{R: 216, G: 222, B: 233, A: 255})
	s.Term.SetDefaultBackgroundColor(color.RGBA{R: 22, G: 25, B: 31, A: 255})
	palette := []uint32{0x242933, 0xe06c75, 0x98c379, 0xe5c07b, 0x61afef, 0xc678dd, 0x56b6c2, 0xd8dee9, 0x5c6370, 0xf28b91, 0xb3d994, 0xf2d396, 0x85c1ff, 0xdb9df2, 0x7fced8, 0xffffff}
	for i, rgb := range palette {
		s.Term.SetIndexedColor(i, color.RGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: 255})
	}
	s.mouseModes = make(map[ansi.Mode]bool)
	s.Term.SetCallbacks(vt.Callbacks{Title: func(title string) { s.Title = title }, WorkingDirectory: func(cwd string) { s.Directory = cwd }, EnableMode: func(mode ansi.Mode) { s.mouseMode(mode, true) }, DisableMode: func(mode ansi.Mode) { s.mouseMode(mode, false) }})
	// Read responses and keyboard encodings even when the child is backpressured.
	input := make(chan []byte, 64)
	go func() {
		defer close(input)
		defer close(s.collectorDone)
		buf := make([]byte, 32768)
		for {
			n, err := s.Term.Read(buf)
			if n > 0 {
				b := append([]byte(nil), buf[:n]...)
				select {
				case input <- b:
				case <-s.stop:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		for b := range input {
			if _, err := f.Write(b); err != nil {
				return
			}
		}
	}()
	go func() {
		defer close(s.output)
		buf := make([]byte, 32768)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				b := append([]byte(nil), buf[:n]...)
				select {
				case s.output <- b:
					if wake != nil {
						wake()
					}
				case <-s.stop:
					// Keep draining during ConPTY shutdown, including on older Windows.
				}
			}
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

// Poll applies a bounded amount of output so busy sessions cannot starve input.
func (s *Session) Poll() bool {
	changed := false
	for range 8 {
		select {
		case b, ok := <-s.output:
			if !ok {
				goto done
			}
			_, _ = s.Term.Write(b)
			changed = true
			s.Revision++
		default:
			goto done
		}
	}
done:
	select {
	case err := <-s.finished:
		s.Exited = true
		s.ExitErr = err
		changed = true
	default:
	}
	return changed
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
	if !s.Exited {
		s.Term.SendText(text)
	}
}
func (s *Session) SendKey(key uv.KeyPressEvent) {
	if !s.Exited {
		s.Term.SendKey(key)
	}
}
func (s *Session) Paste(text string) {
	if !s.Exited {
		s.Term.Paste(text)
	}
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.stop)
		_ = s.process.Close()
		_ = s.Term.InputPipe().(io.Closer).Close()
		<-s.collectorDone
		_ = s.Term.Close()
	})
}

func (s *Session) HistoryLen() int {
	if s.Term.IsAltScreen() {
		return 0
	}
	return s.Term.ScrollbackLen()
}
func (s *Session) Cell(x, row int) *uv.Cell {
	history := s.HistoryLen()
	if row < history {
		return s.Term.ScrollbackCellAt(x, row)
	}
	return s.Term.CellAt(x, row-history)
}
func (s *Session) Line(row int) string {
	var b strings.Builder
	for x := 0; x < s.Term.Width(); x++ {
		c := s.Cell(x, row)
		if c == nil {
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
	for row := a.Row; row <= b.Row; row++ {
		left, right := 0, s.Term.Width()-1
		if row == a.Row {
			left = a.Col
		}
		if row == b.Row {
			right = b.Col
		}
		var line strings.Builder
		for x := left; x <= right; x++ {
			c := s.Cell(x, row)
			if c == nil {
				line.WriteByte(' ')
			} else if c.Width > 0 {
				line.WriteString(c.Content)
			}
		}
		out.WriteString(strings.TrimRight(line.String(), " "))
		if row != b.Row {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// Find maps matching UTF-8 byte ranges back to terminal columns, including wide cells.
func (s *Session) Find(query string) []Match {
	if query == "" {
		return nil
	}
	query = strings.ToLower(query)
	var matches []Match
	for row := 0; row < s.HistoryLen()+s.Term.Height(); row++ {
		var b strings.Builder
		columns := []int{}
		for x := 0; x < s.Term.Width(); x++ {
			c := s.Cell(x, row)
			value := " "
			if c != nil {
				if c.Width == 0 {
					continue
				}
				value = c.Content
			}
			value = strings.ToLower(value)
			b.WriteString(value)
			for range len(value) {
				columns = append(columns, x)
			}
		}
		line := b.String()
		for offset := 0; offset < len(line); {
			i := strings.Index(line[offset:], query)
			if i < 0 {
				break
			}
			start := offset + i
			end := start + len(query) - 1
			col := columns[start]
			last := columns[end]
			width := 1
			if cell := s.Cell(last, row); cell != nil {
				width = max(1, cell.Width)
			}
			matches = append(matches, Match{Row: row, Col: col, Width: last - col + width})
			offset = end + 1
		}
	}
	return matches
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
	if !s.Exited {
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
