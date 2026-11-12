package window

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
)

// FileDialogMode controls which kind of entries a native file dialog accepts.
type FileDialogMode uint8

const (
	// FileDialogFiles selects one file.
	FileDialogFiles FileDialogMode = iota
	// FileDialogFolder selects one folder.
	FileDialogFolder
	// FileDialogMultipleFiles selects one or more files.
	FileDialogMultipleFiles
)

// FileDialogFilter describes one group in the native dialog's file type menu.
// Extensions may be written with or without a leading dot (for example,
// both "png" and ".png" are accepted).
type FileDialogFilter struct {
	Name       string
	Extensions []string
}

// FileDialogOptions configures OpenFileDialog.
type FileDialogOptions struct {
	Title       string
	DefaultPath string
	Filters     []FileDialogFilter
	// Mode defaults to FileDialogFiles.
	Mode FileDialogMode
}

var (
	// ErrFileDialogUnavailable indicates that the current target has no
	// synchronous native file dialog integration (for example, a browser or
	// mobile build).
	ErrFileDialogUnavailable = errors.New("window: native file dialog is unavailable")
	// ErrFileDialogInvalidOptions indicates an invalid mode or filter.
	ErrFileDialogInvalidOptions = errors.New("window: invalid file dialog options")
)

// FileDialogBackend is the platform implementation selected by the host
// project. Keeping it outside this package lets projects that do not use file
// dialogs avoid linking desktop portal dependencies.
type FileDialogBackend func(*Context, FileDialogOptions) ([]string, error)

var fileDialogBackend FileDialogBackend

// RegisterFileDialogBackend installs the optional native file dialog backend.
// Applications should call this once during startup, before opening a dialog.
func RegisterFileDialogBackend(backend FileDialogBackend) {
	fileDialogBackend = backend
}

var fileDialogMu sync.Mutex

// OpenFileDialog opens a modal native file dialog without an explicit owner.
// An empty result means the user cancelled the dialog.
func OpenFileDialog(options FileDialogOptions) ([]string, error) {
	return openFileDialog(nil, options)
}

// OpenFileDialog opens a modal native file dialog owned by the engine window.
// An empty result means the user cancelled the dialog.
func (c *Context) OpenFileDialog(options FileDialogOptions) ([]string, error) {
	return openFileDialog(c, options)
}

// OpenFile opens a native dialog for one file. An empty path means cancellation.
func OpenFile(options FileDialogOptions) (string, error) {
	options.Mode = FileDialogFiles
	paths, err := OpenFileDialog(options)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// OpenFolder opens a native dialog for one folder. An empty path means cancellation.
func OpenFolder(options FileDialogOptions) (string, error) {
	options.Mode = FileDialogFolder
	paths, err := OpenFileDialog(options)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// OpenFiles opens a native dialog that allows multiple files. An empty result
// means cancellation.
func OpenFiles(options FileDialogOptions) ([]string, error) {
	options.Mode = FileDialogMultipleFiles
	return OpenFileDialog(options)
}

func (c *Context) OpenFile(options FileDialogOptions) (string, error) {
	options.Mode = FileDialogFiles
	paths, err := c.OpenFileDialog(options)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func (c *Context) OpenFolder(options FileDialogOptions) (string, error) {
	options.Mode = FileDialogFolder
	paths, err := c.OpenFileDialog(options)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func (c *Context) OpenFiles(options FileDialogOptions) ([]string, error) {
	options.Mode = FileDialogMultipleFiles
	return c.OpenFileDialog(options)
}

func openFileDialog(context *Context, options FileDialogOptions) ([]string, error) {
	normalized, err := normalizeFileDialogOptions(options)
	if err != nil {
		return nil, err
	}
	fileDialogMu.Lock()
	defer fileDialogMu.Unlock()
	if fileDialogBackend != nil {
		return fileDialogBackend(context, normalized)
	}
	return openNativeFileDialog(context, normalized)
}

func normalizeFileDialogOptions(options FileDialogOptions) (FileDialogOptions, error) {
	if options.Mode > FileDialogMultipleFiles {
		return FileDialogOptions{}, ErrFileDialogInvalidOptions
	}
	if strings.ContainsRune(options.Title, '\x00') || strings.ContainsRune(options.DefaultPath, '\x00') {
		return FileDialogOptions{}, ErrFileDialogInvalidOptions
	}
	options.Title = strings.TrimSpace(options.Title)
	if options.Title == "" {
		switch options.Mode {
		case FileDialogFolder:
			options.Title = "Select a folder"
		case FileDialogMultipleFiles:
			options.Title = "Select files"
		default:
			options.Title = "Select a file"
		}
	}
	if options.DefaultPath != "" {
		options.DefaultPath = filepath.Clean(options.DefaultPath)
	}
	for _, filter := range options.Filters {
		if strings.TrimSpace(filter.Name) == "" || strings.ContainsRune(filter.Name, '\x00') || len(filter.Extensions) == 0 {
			return FileDialogOptions{}, ErrFileDialogInvalidOptions
		}
		for _, extension := range filter.Extensions {
			extension = strings.TrimSpace(extension)
			if extension == "" || strings.ContainsAny(extension, "/\\;\x00") || !validFileDialogExtension(extension) {
				return FileDialogOptions{}, ErrFileDialogInvalidOptions
			}
		}
	}
	return options, nil
}

func validFileDialogExtension(extension string) bool {
	if strings.HasPrefix(extension, "*.") {
		extension = extension[2:]
	}
	if strings.HasPrefix(extension, ".") {
		extension = extension[1:]
	}
	return extension != "" && !strings.ContainsRune(extension, '*')
}

func normalizedFileDialogExtension(extension string) string {
	extension = strings.TrimSpace(extension)
	if strings.HasPrefix(extension, "*.") {
		extension = extension[2:]
	}
	return strings.TrimPrefix(extension, ".")
}

func fileDialogGlob(extension string) string {
	return "*." + normalizedFileDialogExtension(extension)
}
