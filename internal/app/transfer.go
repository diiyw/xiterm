package app

import (
	"os"
	"path/filepath"

	"github.com/diiyw/xiterm/internal/terminal"
	"github.com/onix-engine/onix/window"
)

// Native dialogs run on the UI thread. The PTY reader continues buffering the
// handshake while the user chooses; the helper starts only after selection.
func (a *App) handleTransfer(s *terminal.Session) {
	if s.TransferNotice != "" {
		a.notify(s.TransferNotice)
		s.TransferNotice = ""
	}
	upload, pending := s.TakeTransferRequest()
	if !pending {
		return
	}
	home, _ := os.UserHomeDir()
	options := window.FileDialogOptions{Title: "ZMODEM: choose files to upload", Mode: window.FileDialogMultipleFiles, DefaultPath: home}
	if !upload {
		options.Title = "ZMODEM: choose download folder"
		options.Mode = window.FileDialogFolder
		if downloads := filepath.Join(home, "Downloads"); isDirectory(downloads) {
			options.DefaultPath = downloads
		}
	}
	paths, err := a.win.OpenFileDialog(options)
	if err != nil {
		s.CancelTransfer()
		a.notify("ZMODEM file chooser: " + err.Error())
		return
	}
	if err := s.SelectTransfer(paths); err != nil {
		s.CancelTransfer()
		a.notify(err.Error())
	}
	a.dirty = true
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
