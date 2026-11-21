//go:build !windows

package app

import (
	"os/exec"
	"runtime"
)

func openBrowser(value string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	// Pass the URL as one argument, without a shell interpreting its contents.
	return exec.Command(name, value).Run()
}
