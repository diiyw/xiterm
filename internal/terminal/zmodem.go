package terminal

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const zmodemCancel = "\x18\x18\x18\x18\x18\x18\x18\x18\x08\x08\x08\x08\x08\x08\x08\x08"
const zmodemFinish = "**\x18B0800000000022d\r\n"

// fileTransfer owns the local lrzsz helper. Only Poll touches the routing state;
// the worker moves raw bytes without accessing the emulator or window.
type fileTransfer struct {
	upload, prompted, remoteDone bool
	finishMatch, overAndOut      int
	cancelMatch                  int
	program                      string
	ctx                          context.Context
	cancel                       context.CancelFunc
	paths                        chan []string
	input                        chan []byte
	pool                         bytePool
	done                         chan struct{}
	err                          error // published by closing done
	wake                         func()
	write                        func([]byte) error
	timer                        *time.Timer
	timerMu                      sync.Mutex
}

func newFileTransfer(upload bool, write func([]byte) error, wake func()) *fileTransfer {
	ctx, cancel := context.WithCancel(context.Background())
	t := &fileTransfer{upload: upload, ctx: ctx, cancel: cancel, paths: make(chan []string, 1), input: make(chan []byte, ioQueueSize), pool: newBytePool(inputBlockSize), done: make(chan struct{}), write: write, wake: wake}
	name := "rz"
	if upload {
		name = "sz"
	}
	var err error
	if runtime.GOOS == "windows" {
		err = errors.New("ZMODEM requires a binary-transparent PTY; Windows ConPTY is not supported")
	} else if t.program, err = exec.LookPath(name); err != nil {
		err = fmt.Errorf("ZMODEM requires local lrzsz (%s); install lrzsz and restart XiTerm", name)
	}
	go t.run(err)
	return t
}

func (t *fileTransfer) touch() {
	t.timerMu.Lock()
	if t.timer != nil {
		t.timer.Reset(30 * time.Second)
	}
	t.timerMu.Unlock()
}

// feed never waits for the helper. Backpressure stays in the bounded PTY queue
// so a slow transfer cannot stall keyboard/window handling or another pane.
func (t *fileTransfer) feed(b []byte) int {
	b = b[:min(len(b), inputBlockSize)]
	if t.ctx.Err() != nil {
		return len(b)
	}
	match, oo, cancels, ended := t.finishMatch, t.overAndOut, t.cancelMatch, false
	n := len(b)
	for i, c := range b {
		if c == 0x18 {
			cancels++
		} else {
			cancels = 0
		}
		if cancels == 5 {
			t.remoteDone = true
			t.cancel()
			return i + 1
		}
		if match == len(zmodemFinish) {
			if c == 'O' {
				oo++
			} else {
				oo = 0
			}
			if oo == 2 {
				n, ended = i+1, true
				break
			}
		} else {
			// lrzsz sets the parity bit on the hex header's LF on Unix.
			if c == zmodemFinish[match] || match == len(zmodemFinish)-1 && c == 0x8a {
				match++
			} else if c == '*' {
				if match == 1 || match == 2 {
					match = 2
				} else {
					match = 1
				}
			} else {
				match = 0
			}
			if match == len(zmodemFinish) && t.upload {
				n, ended = i+1, true
				break
			}
		}
	}
	buf := t.pool.get()
	copy(buf, b[:n])
	select {
	case t.input <- buf[:n]:
		t.finishMatch, t.overAndOut, t.cancelMatch, t.remoteDone = match, oo, cancels, ended
		t.touch()
		return n
	default:
		t.pool.put(buf)
		return 0
	}
}

type transferWriter struct{ t *fileTransfer }

func (w transferWriter) Write(b []byte) (int, error) {
	if err := w.t.ctx.Err(); err != nil {
		return 0, err
	}
	w.t.touch()
	if err := w.t.write(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// Keep a bounded diagnostic tail; lrzsz can emit progress for arbitrarily large
// batches. Helper diagnostics never enter the VT parser as escape sequences.
type transferDiagnostic struct{ tail []byte }

func (d *transferDiagnostic) Write(b []byte) (int, error) {
	n := len(b)
	if len(b) > 2048 {
		b = b[len(b)-2048:]
	}
	d.tail = append(d.tail, b...)
	if len(d.tail) > 2048 {
		copy(d.tail, d.tail[len(d.tail)-2048:])
		d.tail = d.tail[:2048]
	}
	return n, nil
}

func (t *fileTransfer) run(startErr error) {
	defer func() {
		t.cancel()
		t.timerMu.Lock()
		if t.timer != nil {
			t.timer.Stop()
			t.timer = nil
		}
		t.timerMu.Unlock()
		for {
			select {
			case b := <-t.input:
				t.pool.put(b)
			default:
				t.pool.clear()
				close(t.done)
				if t.wake != nil {
					t.wake()
				}
				return
			}
		}
	}()
	if startErr != nil {
		t.err = startErr
		_ = t.write([]byte(zmodemCancel))
		return
	}
	var paths []string
	select {
	case paths = <-t.paths:
	case <-t.ctx.Done():
		t.err = context.Canceled
		_ = t.write([]byte(zmodemCancel))
		return
	}
	args := []string{"-b", "-e", "-q"}
	dir := ""
	if t.upload {
		args = append(args, "--")
		args = append(args, paths...)
	} else {
		// Restricted mode rejects remote paths and commands. Rename conflicts
		// rather than allowing a sender to overwrite an existing local file.
		args = append(args, "-R", "-E")
		dir = paths[0]
	}
	cmd := exec.CommandContext(t.ctx, t.program, args...)
	cmd.Dir = dir
	cmd.Stdout = transferWriter{t}
	diagnostic := &transferDiagnostic{}
	cmd.Stderr = diagnostic
	stdin, err := cmd.StdinPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		if stdin != nil {
			_ = stdin.Close()
		}
		t.err = fmt.Errorf("start ZMODEM: %w", err)
		_ = t.write([]byte(zmodemCancel))
		return
	}
	t.timerMu.Lock()
	t.timer = time.AfterFunc(30*time.Second, t.cancel)
	t.timerMu.Unlock()
	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)
		defer stdin.Close()
		for {
			select {
			case b := <-t.input:
				err := writeAll(stdin, b)
				t.pool.put(b)
				if err != nil {
					return
				}
			case <-t.ctx.Done():
				return
			}
		}
	}()
	err = cmd.Wait()
	wasCanceled := t.ctx.Err() != nil
	t.cancel()
	_ = stdin.Close()
	<-pumpDone
	if wasCanceled {
		t.err = errors.New("ZMODEM cancelled or timed out")
	} else if err != nil {
		message := strings.TrimSpace(strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return ' '
			}
			return r
		}, string(diagnostic.tail)))
		t.err = fmt.Errorf("ZMODEM failed: %w (%s)", err, message)
	}
	if t.err != nil {
		_ = t.write([]byte(zmodemCancel))
	}
}

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

func zmodemInit(b []byte) (upload, valid bool) {
	if len(b) != 18 || !bytes.HasPrefix(b, []byte("**\x18B")) {
		return false, false
	}
	var frame [7]byte
	if _, err := hex.Decode(frame[:], b[4:]); err != nil || frame[0] > 1 {
		return false, false
	}
	var crc uint16
	for _, c := range frame {
		crc ^= uint16(c) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return frame[0] == 1, crc == 0
}

// TakeTransferRequest reports each file chooser request once, on the UI thread.
func (s *Session) TakeTransferRequest() (upload, pending bool) {
	if t := s.transfer; t != nil && !t.prompted && t.program != "" {
		t.prompted = true
		return t.upload, true
	}
	return false, false
}

func (s *Session) SelectTransfer(paths []string) error {
	t := s.transfer
	if t == nil {
		return errors.New("no pending ZMODEM transfer")
	}
	if len(paths) == 0 {
		t.cancel()
		return nil
	}
	if !t.upload && len(paths) != 1 {
		t.cancel()
		return errors.New("select one download directory")
	}
	resolved := make([]string, len(paths))
	for i, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.cancel()
			return err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			t.cancel()
			return err
		}
		if t.upload && !info.Mode().IsRegular() || !t.upload && !info.IsDir() {
			t.cancel()
			return fmt.Errorf("invalid ZMODEM path: %s", path)
		}
		resolved[i] = absolute
	}
	select {
	case t.paths <- resolved:
		return nil
	case <-t.ctx.Done():
		return errors.New("ZMODEM transfer already ended")
	default:
		return errors.New("ZMODEM selection already submitted")
	}
}

func (s *Session) CancelTransfer() {
	if s.transfer != nil {
		s.transfer.cancel()
	}
}

func (s *Session) TransferActive() bool { return s.transfer != nil }

func (s *Session) TransferStatus() string {
	if t := s.transfer; t != nil {
		if t.upload {
			return "ZMODEM upload · Ctrl+C to cancel"
		}
		return "ZMODEM download · Ctrl+C to cancel"
	}
	return ""
}
