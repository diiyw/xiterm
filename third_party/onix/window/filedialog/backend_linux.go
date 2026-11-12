//go:build linux && cgo && !android && !ios && !js && !wasm && !onix_drm

package filedialog

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/onix-engine/onix/window"
	"github.com/godbus/dbus/v5"
)

const (
	portalName      = "org.freedesktop.portal.Desktop"
	portalPath      = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalInterface = "org.freedesktop.portal.FileChooser"
)

type portalFilterRule struct {
	Pattern string
	Kind    uint32
}

type portalFilter struct {
	Name  string
	Rules []portalFilterRule
}

type portalResponse struct {
	Results map[string]dbus.Variant
	Code    uint32
}

func open(context *window.Context, options window.FileDialogOptions) ([]string, error) {
	bus, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("%w: connect to desktop portal: %v", window.ErrFileDialogUnavailable, err)
	}
	defer bus.Close()
	signals := make(chan *dbus.Signal, 1)
	bus.Signal(signals)
	defer bus.RemoveSignal(signals)
	if err := bus.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.portal.Request"),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return nil, fmt.Errorf("%w: subscribe to desktop portal: %v", window.ErrFileDialogUnavailable, err)
	}

	portalOptions := make(map[string]dbus.Variant, 5)
	portalOptions["modal"] = dbus.MakeVariant(true)
	if options.Mode == window.FileDialogFolder {
		portalOptions["directory"] = dbus.MakeVariant(true)
	} else {
		portalOptions["multiple"] = dbus.MakeVariant(options.Mode == window.FileDialogMultipleFiles)
	}
	if options.DefaultPath != "" {
		if path, pathErr := portalFileDialogPath(options.DefaultPath); pathErr == nil {
			portalOptions["current_folder"] = dbus.MakeVariant(path)
		}
	}
	if len(options.Filters) > 0 && options.Mode != window.FileDialogFolder {
		filters := make([]portalFilter, 0, len(options.Filters))
		for _, filter := range options.Filters {
			rules := make([]portalFilterRule, 0, len(filter.Extensions))
			for _, extension := range filter.Extensions {
				rules = append(rules, portalFilterRule{Kind: 0, Pattern: fileDialogGlob(extension)})
			}
			filters = append(filters, portalFilter{Name: filter.Name, Rules: rules})
		}
		portalOptions["filters"] = dbus.MakeVariant(filters)
	}

	portal := bus.Object(portalName, portalPath)
	call := portal.Call(portalInterface+".OpenFile", 0, portalParent(context), options.Title, portalOptions)
	if call.Err != nil {
		return nil, fmt.Errorf("%w: open desktop portal: %v", window.ErrFileDialogUnavailable, call.Err)
	}
	var requestPath dbus.ObjectPath
	if err := call.Store(&requestPath); err != nil {
		return nil, fmt.Errorf("%w: read desktop portal request: %v", window.ErrFileDialogUnavailable, err)
	}

	for signal := range signals {
		if signal == nil || signal.Path != requestPath || len(signal.Body) != 2 {
			continue
		}
		var response portalResponse
		if err := dbus.Store(signal.Body, &response.Code, &response.Results); err != nil {
			return nil, fmt.Errorf("%w: decode desktop portal response: %v", window.ErrFileDialogUnavailable, err)
		}
		return response.paths()
	}
	return nil, nil
}

func (response portalResponse) paths() ([]string, error) {
	switch response.Code {
	case 1, 2:
		return nil, nil
	case 0:
	default:
		return nil, fmt.Errorf("%w: desktop portal response code %d", window.ErrFileDialogUnavailable, response.Code)
	}
	value, ok := response.Results["uris"]
	if !ok {
		return nil, nil
	}
	uris, ok := value.Value().([]string)
	if !ok {
		return nil, fmt.Errorf("%w: desktop portal returned invalid paths", window.ErrFileDialogUnavailable)
	}
	paths := make([]string, 0, len(uris))
	for _, rawURI := range uris {
		if path, ok := fileDialogPathFromURI(rawURI); ok {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func portalParent(context *window.Context) string {
	if context == nil {
		return ""
	}
	windowID := context.NativeX11Window()
	if windowID == 0 {
		return ""
	}
	return fmt.Sprintf("x11:%x", uint64(windowID))
}

func portalFileDialogPath(path string) ([]byte, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	path = filepath.ToSlash(absolute)
	result := make([]byte, len(path)+1)
	copy(result, path)
	return result, nil
}

func fileDialogPathFromURI(rawURI string) (string, bool) {
	parsed, err := url.Parse(rawURI)
	if err != nil || parsed.Scheme != "file" || parsed.Path == "" || parsed.Host != "" && parsed.Host != "localhost" {
		return "", false
	}
	return filepath.FromSlash(parsed.Path), true
}

func fileDialogGlob(extension string) string {
	return "*." + normalizedFileDialogExtension(extension)
}

func normalizedFileDialogExtension(extension string) string {
	if len(extension) >= 2 && extension[:2] == "*." {
		extension = extension[2:]
	}
	if len(extension) > 0 && extension[0] == '.' {
		extension = extension[1:]
	}
	return extension
}
