//go:build windows

package font

import (
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	spiGetNonClientMetrics = 0x0029
	lfFaceSize             = 32
	windowsFontsRegistry   = `Software\Microsoft\Windows NT\CurrentVersion\Fonts`
)

type windowsLogFont struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         byte
	Underline      byte
	StrikeOut      byte
	CharSet        byte
	OutPrecision   byte
	ClipPrecision  byte
	Quality        byte
	PitchAndFamily byte
	FaceName       [lfFaceSize]uint16
}

type windowsNonClientMetrics struct {
	Size               uint32
	BorderWidth        int32
	ScrollWidth        int32
	ScrollHeight       int32
	CaptionWidth       int32
	CaptionHeight      int32
	CaptionFont        windowsLogFont
	SmallCaptionWidth  int32
	SmallCaptionHeight int32
	SmallCaptionFont   windowsLogFont
	MenuWidth          int32
	MenuHeight         int32
	MenuFont           windowsLogFont
	StatusFont         windowsLogFont
	MessageFont        windowsLogFont
	PaddedBorderWidth  int32
}

var systemParametersInfoW = windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW")

func platformSystemFontFiles() []string {
	var paths []string
	for _, root := range [...]registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, windowsFontsRegistry, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, _ := key.ReadValueNames(-1)
		for _, name := range names {
			value, kind, err := key.GetStringValue(name)
			if err != nil {
				continue
			}
			if kind == registry.EXPAND_SZ {
				value, err = registry.ExpandString(value)
				if err != nil {
					continue
				}
			}
			if path := windowsResolveFontPath(value); path != "" {
				paths = append(paths, path)
			}
		}
		_ = key.Close()
	}
	return paths
}

func platformDefaultFont() (systemFontReference, bool) {
	face := windowsSystemUIFont()
	if face == "" {
		return systemFontReference{}, false
	}
	path := windowsRegisteredFontPath(face)
	if path == "" {
		return systemFontReference{}, false
	}
	return systemFontReference{path: path, face: face}, true
}

func windowsSystemUIFont() string {
	metrics := windowsNonClientMetrics{Size: uint32(unsafe.Sizeof(windowsNonClientMetrics{}))}
	result, _, _ := systemParametersInfoW.Call(
		spiGetNonClientMetrics,
		uintptr(metrics.Size),
		uintptr(unsafe.Pointer(&metrics)),
		0,
	)
	if result == 0 {
		return ""
	}
	return strings.TrimSpace(windows.UTF16ToString(metrics.MessageFont.FaceName[:]))
}

func windowsRegisteredFontPath(face string) string {
	for _, root := range [...]registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, windowsFontsRegistry, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		path := windowsFontPathFromRegistry(key, face)
		_ = key.Close()
		if path != "" {
			return path
		}
	}
	return ""
}

func windowsFontPathFromRegistry(key registry.Key, face string) string {
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return ""
	}
	var regularPath string
	for _, name := range names {
		match := windowsFontNameMatch(name, face)
		if match == 0 {
			continue
		}
		value, valueType, err := key.GetStringValue(name)
		if err != nil {
			continue
		}
		if valueType == registry.EXPAND_SZ {
			value, err = registry.ExpandString(value)
			if err != nil {
				continue
			}
		}
		path := windowsResolveFontPath(value)
		if path == "" {
			continue
		}
		if match == 2 {
			return path
		}
		if regularPath == "" {
			regularPath = path
		}
	}
	return regularPath
}

func windowsFontNameMatch(registryName, face string) int {
	name := strings.TrimSpace(registryName)
	if index := strings.LastIndexByte(name, '('); index >= 0 && strings.HasSuffix(name, ")") {
		name = strings.TrimSpace(name[:index])
	}
	regularFace := face + " Regular"
	for family := range strings.SplitSeq(name, "&") {
		family = strings.TrimSpace(family)
		if strings.EqualFold(family, face) {
			return 2
		}
		if strings.EqualFold(family, regularFace) {
			return 1
		}
	}
	return 0
}

func windowsResolveFontPath(value string) string {
	value = strings.Trim(strings.TrimSpace(value), `"`)
	if value == "" || !isSupportedFontPath(value) {
		return ""
	}
	if filepath.IsAbs(value) {
		if fileExists(value) {
			return filepath.Clean(value)
		}
		return ""
	}
	for _, directory := range fontSearchDirs() {
		path := filepath.Join(directory, value)
		if fileExists(path) {
			return path
		}
	}
	return ""
}
