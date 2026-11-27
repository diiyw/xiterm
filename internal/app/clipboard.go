package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.design/x/clipboard"
)

func (a *App) paste() {
	if a.overlay != "" {
		return
	}
	value, ok := a.win.ClipboardText()
	if a.search {
		if !ok {
			a.notify("Clipboard has no text to search")
			return
		}
		a.query += value
		a.updateSearch(true)
		return
	}
	p := a.pane()
	if p == nil || p.Session.Exited || p.Session.TransferActive() {
		return
	}
	if p.Session.ClipboardPasteEvents() {
		image, _ := readClipboardImage()
		p.Session.OfferClipboardPaste(value, ok, image)
		p.Scroll = 0
		p.Selected = false
		return
	}
	if !ok {
		image, err := readClipboardImage()
		var path string
		if err == nil {
			path, err = a.saveClipboardImage(image)
		}
		if err != nil {
			if errors.Is(err, clipboard.ErrNoData) || errors.Is(err, clipboard.ErrUnavailable) {
				a.notify("Clipboard is unavailable")
			} else {
				a.notify("Could not paste clipboard image: " + err.Error())
			}
			return
		}
		value = quotePastedPath(path)
		shell := strings.ToLower(filepath.Base(p.Session.Profile.Shell))
		if runtime.GOOS == "windows" && (shell == "cmd" || shell == "cmd.exe") {
			value = "\"" + path + "\""
		}
	}
	p.Session.Paste(value)
	p.Scroll = 0
	p.Selected = false
}

func readClipboardImage() ([]byte, error) {
	if err := clipboard.Init(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return clipboard.Read(ctx, clipboard.FmtImage)
}

func (a *App) saveClipboardImage(image []byte) (string, error) {
	if a.pastedImageDir == "" {
		root, err := filepath.Abs(os.TempDir())
		if err != nil {
			return "", err
		}
		dir, err := os.MkdirTemp(root, "xiterm-paste-")
		if err != nil {
			return "", err
		}
		a.pastedImageDir = dir
	}
	file, err := os.CreateTemp(a.pastedImageDir, "image-*.png")
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(image)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func quotePastedPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}
