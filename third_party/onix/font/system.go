package font

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	defaultFontOnce sync.Once
	defaultFont     systemFontReference
	defaultFontErr  error

	defaultCFFFontPathOnce sync.Once
	defaultCFFFontPath     string
	defaultCFFFontPathErr  error
)

type systemFontReference struct {
	path  string
	face  string
	index int
}

func LoadDefault() (*Face, error) {
	reference, err := defaultFontReference()
	if err != nil {
		return nil, err
	}
	backing, err := loadFontFile(reference.path)
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	index := reference.index
	if reference.face != "" {
		index, err = CollectionIndex(backing.data, reference.face)
		if err != nil {
			_ = backing.close()
			return nil, fmt.Errorf("find font face %q: %w", reference.face, err)
		}
	}
	face, err := ParseIndex(backing.data, index)
	if err != nil {
		_ = backing.close()
		return nil, fmt.Errorf("parse font: %w", err)
	}
	face.attachBacking(backing)
	return face, nil
}

func DefaultFontPath() (string, error) {
	reference, err := defaultFontReference()
	if err != nil {
		return "", err
	}
	return reference.path, nil
}

func defaultFontReference() (systemFontReference, error) {
	defaultFontOnce.Do(func() {
		defaultFont, defaultFontErr = findDefaultFont()
	})
	return defaultFont, defaultFontErr
}

func DefaultCFFFontPath() (string, error) {
	defaultCFFFontPathOnce.Do(func() {
		defaultCFFFontPath, defaultCFFFontPathErr = findDefaultCFFFontPath()
	})
	if defaultCFFFontPathErr != nil {
		return "", defaultCFFFontPathErr
	}
	return defaultCFFFontPath, nil
}

func findDefaultFont() (systemFontReference, error) {
	if reference, ok := platformDefaultFont(); ok {
		return reference, nil
	}

	if path := fontConfigMatch(defaultFontPattern()); path != "" {
		return systemFontReference{path: path}, nil
	}

	if path := firstExistingPath(defaultFontCandidates()...); path != "" {
		return systemFontReference{path: path}, nil
	}

	if path := findFontByBaseName(defaultFontBaseNames(), func(path string) bool {
		return isSupportedFontPath(path)
	}); path != "" {
		return systemFontReference{path: path}, nil
	}

	return systemFontReference{}, errors.New("no local default font found")
}

func defaultFontPattern() string {
	for _, value := range [...]string{os.Getenv("LC_ALL"), os.Getenv("LC_CTYPE"), os.Getenv("LANG")} {
		locale := strings.TrimSpace(value)
		if index := strings.IndexByte(locale, '.'); index >= 0 {
			locale = locale[:index]
		}
		if index := strings.IndexByte(locale, '@'); index >= 0 {
			locale = locale[:index]
		}
		locale = strings.TrimSpace(strings.ReplaceAll(locale, "_", "-"))
		if locale == "" || strings.EqualFold(locale, "c") || strings.EqualFold(locale, "posix") {
			continue
		}
		return "sans-serif:lang=" + strings.ToLower(locale)
	}
	return "sans-serif"
}

func findDefaultCFFFontPath() (string, error) {
	if path := firstExistingPath(defaultCFFFontCandidates()...); path != "" {
		if isCFFFont(path) {
			return path, nil
		}
	}

	path := findFirstFont(func(path string) bool {
		if strings.ToLower(filepath.Ext(path)) != ".otf" {
			return false
		}
		return isCFFFont(path)
	})
	if path != "" {
		return path, nil
	}

	return "", errors.New("no local CFF .otf font found")
}

func isCFFFont(path string) bool {
	face, err := Load(path)
	if err != nil {
		return false
	}
	defer face.Close()
	return face.CFF != nil
}

func fontConfigMatch(pattern string) string {
	if _, err := exec.LookPath("fc-match"); err != nil {
		return ""
	}
	output, err := exec.Command("fc-match", "-f", "%{file}\n", pattern).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		path := strings.TrimSpace(line)
		if path == "" || !isSupportedFontPath(path) || !fileExists(path) {
			continue
		}
		return path
	}
	return ""
}

func findFontByBaseName(baseNames []string, accept func(path string) bool) string {
	wanted := map[string]struct{}{}
	for _, name := range baseNames {
		wanted[strings.ToLower(name)] = struct{}{}
	}
	return findFirstFont(func(path string) bool {
		if _, ok := wanted[strings.ToLower(filepath.Base(path))]; !ok {
			return false
		}
		return accept(path)
	})
}

func findFirstFont(accept func(path string) bool) string {
	var found string
	stop := errors.New("font found")
	for _, dir := range fontSearchDirs() {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if accept(path) {
				found = path
				return stop
			}
			return nil
		})
		if err != nil && !errors.Is(err, stop) {
			continue
		}
		if found != "" {
			return found
		}
	}
	return ""
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if fileExists(path) {
			return path
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isSupportedFontPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	default:
		return false
	}
}

func fontSearchDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "android":
		return []string{"/system/fonts", "/product/fonts", "/vendor/fonts", "/data/fonts"}
	case "ios":
		return []string{"/System/Library/Fonts", "/Library/Fonts"}
	case "windows":
		root := strings.TrimSpace(os.Getenv("WINDIR"))
		if root == "" {
			root = `C:\Windows`
		}
		local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if local != "" {
			local = filepath.Join(local, "Microsoft", "Windows", "Fonts")
		}
		return compactStrings(
			local,
			filepath.Join(root, "Fonts"),
		)
	case "darwin":
		return compactStrings(
			"/System/Library/Fonts",
			"/Library/Fonts",
			filepath.Join(home, "Library", "Fonts"),
		)
	default:
		return compactStrings(
			"/usr/share/fonts",
			"/usr/local/share/fonts",
			filepath.Join(home, ".local", "share", "fonts"),
			filepath.Join(home, ".fonts"),
		)
	}
}

func defaultFontCandidates() []string {
	switch runtime.GOOS {
	case "android":
		return []string{"/system/fonts/Roboto-Regular.ttf", "/system/fonts/RobotoStatic-Regular.ttf", "/system/fonts/NotoSans-Regular.ttf"}
	case "ios":
		return nil // CoreText resolves the actual system font and collection face.
	case "windows":
		root := strings.TrimSpace(os.Getenv("WINDIR"))
		if root == "" {
			root = `C:\Windows`
		}
		return []string{
			filepath.Join(root, "Fonts", "segoeui.ttf"),
			filepath.Join(root, "Fonts", "arial.ttf"),
			filepath.Join(root, "Fonts", "tahoma.ttf"),
		}
	case "darwin":
		return []string{
			"/System/Library/Fonts/SFNS.ttf",
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/Library/Fonts/Arial.ttf",
		}
	default:
		return []string{
			"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		}
	}
}

func defaultFontBaseNames() []string {
	return []string{
		"Roboto-Regular.ttf",
		"RobotoStatic-Regular.ttf",
		"NotoSans-Regular.ttf",
		"DejaVuSans.ttf",
		"LiberationSans-Regular.ttf",
		"Arial.ttf",
		"segoeui.ttf",
		"tahoma.ttf",
		"SFNS.ttf",
	}
}

func defaultCFFFontCandidates() []string {
	switch runtime.GOOS {
	case "android", "ios":
		return nil
	case "windows":
		root := strings.TrimSpace(os.Getenv("WINDIR"))
		if root == "" {
			root = `C:\Windows`
		}
		return []string{
			filepath.Join(root, "Fonts", "gabriola.ttf"),
		}
	case "darwin":
		return []string{
			"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
		}
	default:
		return []string{
			"/usr/share/fonts/opentype/urw-base35/NimbusSans-Regular.otf",
			"/usr/share/fonts/opentype/urw-base35/NimbusRoman-Regular.otf",
			"/usr/share/fonts/opentype/urw-base35/URWGothic-Book.otf",
		}
	}
}

func compactStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}
