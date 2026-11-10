//go:build linux || darwin

package terminal

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/diiyw/xiterm/internal/config"
)

func requireLrzsz(t *testing.T) (rz, sz string) {
	t.Helper()
	var err error
	rz, err = exec.LookPath("rz")
	if err != nil {
		t.Skip("install lrzsz to run real ZMODEM integration tests")
	}
	sz, err = exec.LookPath("sz")
	if err != nil {
		t.Skip("install lrzsz to run real ZMODEM integration tests")
	}
	return rz, sz
}

func TestRealZmodemTransfers(t *testing.T) {
	rz, sz := requireLrzsz(t)
	for _, upload := range []bool{false, true} {
		name := "download"
		if upload {
			name = "upload"
		}
		t.Run(name, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			// Exercise all byte values, Unicode, option-like names and batches.
			payload := bytes.Repeat(append([]byte{0, 1, 17, 19, 24, 127, 128, 255}, []byte(zmodemFinish)...), 20000)
			for _, name := range []string{"payload 中文.bin", "--second.bin"} {
				if err := os.WriteFile(filepath.Join(source, name), payload, 0600); err != nil {
					t.Fatal(err)
				}
			}
			// Download conflict handling must preserve the existing file.
			if !upload {
				if err := os.WriteFile(filepath.Join(destination, "payload 中文.bin"), []byte("existing"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			tool := sz
			command := `"$TRANSFER_TOOL" -b -e -- "$SOURCE_ONE" "$SOURCE_TWO"`
			if upload {
				tool = rz
				command = `"$TRANSFER_TOOL" -b -e`
			}
			s, err := Start(config.Profile{Name: "ZMODEM", Shell: "/bin/sh", Directory: destination,
				Args: []string{"-c", command + `; result=$?; printf '\r\nTRANSFER_EXIT=%s\r\n' "$result"; read line; printf 'AFTER=%s\r\n' "$line"`},
				Env:  []string{"TRANSFER_TOOL=" + tool, "SOURCE_ONE=" + filepath.Join(source, "payload 中文.bin"), "SOURCE_TWO=" + filepath.Join(source, "--second.bin")}}, 100, 12, 100, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if s.transfer != nil {
					t.Logf("transfer state: remoteDone=%v finish=%d oo=%d queued=%d err=%v", s.transfer.remoteDone, s.transfer.finishMatch, s.transfer.overAndOut, len(s.transfer.input), s.transfer.ctx.Err())
				}
				s.Close()
			})
			waitFor(t, s, func() bool { return s.TransferActive() })
			gotUpload, pending := s.TakeTransferRequest()
			if !pending || gotUpload != upload {
				t.Fatalf("request upload=%v pending=%v", gotUpload, pending)
			}
			if _, pending := s.TakeTransferRequest(); pending {
				t.Fatal("file chooser requested twice")
			}
			paths := []string{destination}
			if upload {
				paths = []string{filepath.Join(source, "payload 中文.bin"), filepath.Join(source, "--second.bin")}
			}
			if err := s.SelectTransfer(paths); err != nil {
				t.Fatal(err)
			}
			// Pasted/key input during a transfer must not corrupt the protocol.
			s.Paste("must not reach the helper")
			s.SendText("ignored")
			s.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
			waitFor(t, s, func() bool {
				return !s.TransferActive() && strings.Contains(s.Term.String(), "TRANSFER_EXIT=")
			})
			if s.TransferNotice != "ZMODEM transfer completed" || !strings.Contains(s.Term.String(), "TRANSFER_EXIT=0") {
				t.Fatalf("transfer failed: %s; screen=%q", s.TransferNotice, s.Term.String())
			}
			for _, name := range []string{"payload 中文.bin", "--second.bin"} {
				if !upload && name == "payload 中文.bin" {
					existing, _ := os.ReadFile(filepath.Join(destination, name))
					if string(existing) != "existing" {
						t.Fatal("download overwrote an existing file")
					}
					name += ".0"
				}
				got, err := os.ReadFile(filepath.Join(destination, name))
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("file %s: bytes=%d err=%v", name, len(got), err)
				}
			}
			s.SendText("ready\r")
			waitFor(t, s, func() bool { return s.Exited })
			if !strings.Contains(s.Term.String(), "AFTER=ready") {
				t.Fatalf("shell did not recover: %q", s.Term.String())
			}
		})
	}
}

func startZmodemReceiver(t *testing.T) *Session {
	t.Helper()
	rz, _ := requireLrzsz(t)
	s, err := Start(config.Profile{Name: "Cancel", Shell: "/bin/sh", Directory: t.TempDir(),
		Args: []string{"-c", `"$TRANSFER_TOOL" -b -e; printf '\r\nRECOVERED\r\n'; read line; printf 'AFTER=%s\r\n' "$line"`}, Env: []string{"TRANSFER_TOOL=" + rz}}, 80, 12, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestRealZmodemCancelAndClose(t *testing.T) {
	for _, mode := range []string{"chooser", "keyboard", "close"} {
		t.Run(mode, func(t *testing.T) {
			s := startZmodemReceiver(t)
			waitFor(t, s, s.TransferActive)
			switch mode {
			case "chooser":
				if err := s.SelectTransfer(nil); err != nil {
					t.Fatal(err)
				}
			case "keyboard":
				s.SendKey(uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl})
			case "close":
				done := make(chan struct{})
				go func() { s.Close(); close(done) }()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Fatal("close blocked on file selection")
				}
				return
			}
			waitFor(t, s, func() bool { return !s.TransferActive() && strings.Contains(s.Term.String(), "RECOVERED") })
			s.SendText("ok\r")
			waitFor(t, s, func() bool { return s.Exited })
			if !strings.Contains(s.Term.String(), "AFTER=ok") {
				t.Fatalf("cancel did not restore input: %q", s.Term.String())
			}
		})
	}
}

func TestRealZmodemMissingHelper(t *testing.T) {
	s := startZmodemReceiver(t)
	t.Setenv("PATH", t.TempDir())
	waitFor(t, s, func() bool { return strings.Contains(s.TransferNotice, "install lrzsz") })
	waitFor(t, s, func() bool { return strings.Contains(s.Term.String(), "RECOVERED") })
	s.SendText("ok\r")
	waitFor(t, s, func() bool { return s.Exited })
}

func TestRealZmodemConsecutiveDownloads(t *testing.T) {
	_, sz := requireLrzsz(t)
	source, destination := filepath.Join(t.TempDir(), "file.bin"), t.TempDir()
	payload := []byte("two consecutive transfers\x00\xff")
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Start(config.Profile{Name: "Batch", Shell: "/bin/sh", Directory: destination,
		Args: []string{"-c", `"$TRANSFER_TOOL" -b -e -- "$SOURCE"; "$TRANSFER_TOOL" -b -e -- "$SOURCE"; printf '\r\nBATCH_DONE\r\n'; read line`},
		Env:  []string{"TRANSFER_TOOL=" + sz, "SOURCE=" + source}}, 80, 12, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	for range 2 {
		waitFor(t, s, func() bool {
			_, pending := s.TakeTransferRequest()
			return pending
		})
		if err := s.SelectTransfer([]string{destination}); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, s, func() bool { return !s.TransferActive() && strings.Contains(s.Term.String(), "BATCH_DONE") })
	for _, name := range []string{"file.bin", "file.bin.0"} {
		got, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("file %s: %q %v", name, got, err)
		}
	}
}
