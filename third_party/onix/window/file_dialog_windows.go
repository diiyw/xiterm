//go:build windows && cgo

package window

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/ebitengine/purego"
	xwindows "golang.org/x/sys/windows"
)

const (
	ofNFileMustExist    = 0x00001000
	ofNPathMustExist    = 0x00000800
	ofNAllowMultiSelect = 0x00000200
	ofNExplorer         = 0x00080000
	ofNNoChangeDir      = 0x00000008
	fnErrBufferTooSmall = 0x00003003

	// IFileOpenDialog / IShellItem vtable slots (see the IFileDialog layout).
	vtblRelease        = 2
	vtblShow           = 3
	vtblSetOptions     = 9
	vtblSetFolder      = 12
	vtblSetTitle       = 17
	vtblGetResult      = 20
	vtblGetDisplayName = 5

	// FOS_* dialog options.
	fosNoChangeDir     = 0x00000008
	fosPickFolders     = 0x00000020
	fosForceFileSystem = 0x00000040

	sigdnFileSystemPath = 0x80058000

	comInitApartmentThreaded = 0x00000002
	clsctxInprocServer       = 0x00000001
	hresultCancelled         = uintptr(0x800704C7) // HRESULT_FROM_WIN32(ERROR_CANCELLED)
	sOK                      = 0
)

// guid mirrors the Win32 GUID/CLSID/IID layout used by the COM entry points.
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]uint8
}

var (
	clsidFileOpenDialog = guid{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]uint8{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = guid{0xD57C7288, 0xD4AD, 0x4768, [8]uint8{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidIShellItem       = guid{0x43826D1E, 0xE718, 0x42EE, [8]uint8{0xBC, 0x55, 0xA2, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

type openFileName struct {
	StructSize      uint32
	Owner           hwnd
	Instance        hinst
	Filter          *uint16
	CustomFilter    *uint16
	MaxCustomFilter uint32
	FilterIndex     uint32
	File            *uint16
	MaxFile         uint32
	FileTitle       *uint16
	MaxFileTitle    uint32
	InitialDir      *uint16
	Title           *uint16
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefExtension    *uint16
	CustData        uintptr
	Hook            uintptr
	TemplateName    *uint16
	Reserved        uintptr
	FlagsEx         uint32
}

var (
	getOpenFileName             func(*openFileName) bool
	commDlgExtendedError        func() uint32
	coTaskMemFree               func(unsafe.Pointer)
	coInitializeEx              func(uintptr, uint32) int32
	coUninitialize              func()
	coCreateInstance            func(*guid, unsafe.Pointer, uint32, *guid, *unsafe.Pointer) int32
	shCreateItemFromParsingName func(*uint16, unsafe.Pointer, *guid, *unsafe.Pointer) int32
)

var loadWindowsFileDialog = sync.OnceFunc(func() {
	comdlg32 := xwindows.NewLazySystemDLL("comdlg32.dll").Handle()
	purego.RegisterLibFunc(&getOpenFileName, comdlg32, "GetOpenFileNameW")
	purego.RegisterLibFunc(&commDlgExtendedError, comdlg32, "CommDlgExtendedError")
})

var loadWindowsFolderDialog = sync.OnceFunc(func() {
	ole32 := xwindows.NewLazySystemDLL("ole32.dll").Handle()
	purego.RegisterLibFunc(&coTaskMemFree, ole32, "CoTaskMemFree")
	purego.RegisterLibFunc(&coInitializeEx, ole32, "CoInitializeEx")
	purego.RegisterLibFunc(&coUninitialize, ole32, "CoUninitialize")
	purego.RegisterLibFunc(&coCreateInstance, ole32, "CoCreateInstance")
	purego.RegisterLibFunc(&shCreateItemFromParsingName, shell32, "SHCreateItemFromParsingName")
})

func openNativeFileDialog(context *Context, options FileDialogOptions) ([]string, error) {
	owner := hwnd(0)
	if context != nil {
		owner = context.window
	}
	if options.Mode == FileDialogFolder {
		return openWindowsFolderDialog(owner, options)
	}
	return openWindowsFileDialog(owner, options)
}

func openWindowsFileDialog(owner hwnd, options FileDialogOptions) ([]string, error) {
	loadWindowsFileDialog()
	filter := windowsDialogFilter(options.Filters)
	filterPtr := &filter[0]
	initialDir := utf16PtrOrNil(options.DefaultPath)
	title := mustUTF16Ptr(options.Title)
	flags := uint32(ofNExplorer | ofNPathMustExist | ofNFileMustExist | ofNNoChangeDir)
	if options.Mode == FileDialogMultipleFiles {
		flags |= ofNAllowMultiSelect
	}
	for capacity := 32768; capacity <= 1<<20; capacity *= 2 {
		pathBuffer := make([]uint16, capacity)
		name := openFileName{
			StructSize: uint32(unsafe.Sizeof(openFileName{})),
			Owner:      owner,
			Filter:     filterPtr,
			File:       &pathBuffer[0],
			MaxFile:    uint32(len(pathBuffer)),
			InitialDir: initialDir,
			Title:      title,
			Flags:      flags,
		}
		if getOpenFileName(&name) {
			return parseWindowsFileDialogPaths(pathBuffer), nil
		}
		errorCode := commDlgExtendedError()
		// A zero extended error is the documented cancellation result.
		if errorCode == 0 {
			return nil, nil
		}
		if errorCode != fnErrBufferTooSmall {
			return nil, ErrFileDialogUnavailable
		}
	}
	return nil, ErrFileDialogUnavailable
}

func openWindowsFolderDialog(owner hwnd, options FileDialogOptions) ([]string, error) {
	loadWindowsFolderDialog()
	// COM apartments are thread-local. Keep the dialog and its COM objects on
	// the same OS thread, including when callers invoke this from a worker.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Both S_OK and S_FALSE are successful initializations and require one
	// matching CoUninitialize. A negative HRESULT means the caller's existing
	// apartment mode must be preserved.
	if result := coInitializeEx(0, comInitApartmentThreaded); result >= 0 {
		defer coUninitialize()
	}
	var dialog unsafe.Pointer
	if coCreateInstance(&clsidFileOpenDialog, nil, clsctxInprocServer, &iidIFileOpenDialog, &dialog) != sOK || dialog == nil {
		return nil, ErrFileDialogUnavailable
	}
	defer comCall(comSlot(dialog, vtblRelease), uintptr(dialog))

	if comCall(comSlot(dialog, vtblSetOptions), uintptr(dialog), uintptr(fosPickFolders|fosForceFileSystem|fosNoChangeDir)) != sOK {
		return nil, ErrFileDialogUnavailable
	}
	if options.Title != "" {
		title := mustUTF16Ptr(options.Title)
		result := comCall(comSlot(dialog, vtblSetTitle), uintptr(dialog), uintptr(unsafe.Pointer(title)))
		runtime.KeepAlive(title)
		if result != sOK {
			return nil, ErrFileDialogUnavailable
		}
	}
	if options.DefaultPath != "" {
		initial := mustUTF16Ptr(options.DefaultPath)
		var item unsafe.Pointer
		if shCreateItemFromParsingName(initial, nil, &iidIShellItem, &item) == sOK && item != nil {
			result := comCall(comSlot(dialog, vtblSetFolder), uintptr(dialog), uintptr(item))
			comCall(comSlot(item, vtblRelease), uintptr(item))
			runtime.KeepAlive(initial)
			if result != sOK {
				return nil, ErrFileDialogUnavailable
			}
		} else {
			runtime.KeepAlive(initial)
		}
	}
	if result := comCall(comSlot(dialog, vtblShow), uintptr(dialog), uintptr(owner)); result != sOK {
		if result == hresultCancelled {
			return nil, nil
		}
		return nil, ErrFileDialogUnavailable
	}
	var item unsafe.Pointer
	if comCall(comSlot(dialog, vtblGetResult), uintptr(dialog), uintptr(unsafe.Pointer(&item))) != sOK || item == nil {
		return nil, nil
	}
	defer comCall(comSlot(item, vtblRelease), uintptr(item))
	var name *uint16
	if comCall(comSlot(item, vtblGetDisplayName), uintptr(item), uintptr(sigdnFileSystemPath), uintptr(unsafe.Pointer(&name))) != sOK || name == nil {
		return nil, ErrFileDialogUnavailable
	}
	defer coTaskMemFree(unsafe.Pointer(name))
	return []string{xwindows.UTF16PtrToString(name)}, nil
}

func comSlot(object unsafe.Pointer, slot int) uintptr {
	vtable := *(*unsafe.Pointer)(object)
	return (*[64]uintptr)(vtable)[slot]
}

//go:uintptrescapes
func comCall(procedure uintptr, arguments ...uintptr) uintptr {
	result, _, _ := syscall.SyscallN(procedure, arguments...)
	return result
}

func windowsDialogFilter(filters []FileDialogFilter) []uint16 {
	result := make([]uint16, 0, 16)
	for _, filter := range filters {
		result = appendWindowsUTF16(result, filter.Name)
		patterns := make([]string, 0, len(filter.Extensions))
		for _, extension := range filter.Extensions {
			patterns = append(patterns, fileDialogGlob(extension))
		}
		result = appendWindowsUTF16(result, strings.Join(patterns, ";"))
	}
	result = appendWindowsUTF16(result, "All files")
	result = appendWindowsUTF16(result, "*.*")
	result = append(result, 0)
	return result
}

func appendWindowsUTF16(dst []uint16, value string) []uint16 {
	dst = append(dst, utf16.Encode([]rune(value))...)
	return append(dst, 0)
}

func parseWindowsFileDialogPaths(buffer []uint16) []string {
	parts := make([]string, 0, 2)
	start := 0
	for index, value := range buffer {
		if value != 0 {
			continue
		}
		if index == start {
			break
		}
		parts = append(parts, string(utf16.Decode(buffer[start:index])))
		start = index + 1
	}
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		return parts
	}
	paths := make([]string, 0, len(parts)-1)
	for _, name := range parts[1:] {
		paths = append(paths, filepath.Join(parts[0], name))
	}
	return paths
}

func windowsUTF16String(buffer []uint16) string {
	for index, value := range buffer {
		if value == 0 {
			return string(utf16.Decode(buffer[:index]))
		}
	}
	return string(utf16.Decode(buffer))
}

func utf16PtrOrNil(value string) *uint16 {
	if value == "" {
		return nil
	}
	return mustUTF16Ptr(value)
}
