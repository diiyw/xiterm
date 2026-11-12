//go:build linux && !android

package font

import (
	"os"
	"os/exec"
	"strings"
)

func platformSystemFontFiles() []string {
	// Fontconfig also knows fonts outside the standard directories.
	output, err := exec.Command("fc-list", "--format=%{file}\n").Output()
	if err != nil {
		return nil
	}
	return strings.Split(string(output), "\n")
}

func platformDefaultFont() (systemFontReference, bool) {
	output, err := exec.Command("fc-match", "-f", "%{file}\n%{postscriptname}", "sans-serif").Output()
	if err != nil {
		return systemFontReference{}, false
	}
	path, face, found := strings.Cut(string(output), "\n")
	path = strings.TrimSpace(path)
	face = strings.TrimSpace(face)
	if !found || path == "" || face == "" || !isSupportedFontPath(path) {
		return systemFontReference{}, false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return systemFontReference{}, false
	}
	return systemFontReference{path: path, face: face}, true
}
