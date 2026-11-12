//go:build darwin || ios

package font

import (
	"bytes"
	"unsafe"

	"github.com/ebitengine/purego"
)

const coreTextSystemUIFont = 2
const coreFoundationUTF8 = 0x08000100

func platformDefaultFont() (systemFontReference, bool) {
	coreText, err := purego.Dlopen("/System/Library/Frameworks/CoreText.framework/CoreText", purego.RTLD_LAZY)
	if err != nil {
		return systemFontReference{}, false
	}
	defer purego.Dlclose(coreText)
	coreFoundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY)
	if err != nil {
		return systemFontReference{}, false
	}
	defer purego.Dlclose(coreFoundation)

	var createUIFont func(int32, float64, uintptr) uintptr
	var copyAttribute func(uintptr, uintptr) uintptr
	var copyPostScriptName func(uintptr) uintptr
	var getFileSystemRepresentation func(uintptr, byte, *byte, int64) byte
	var getCString func(uintptr, *byte, int64, uint32) byte
	var release func(uintptr)
	purego.RegisterLibFunc(&createUIFont, coreText, "CTFontCreateUIFontForLanguage")
	purego.RegisterLibFunc(&copyAttribute, coreText, "CTFontCopyAttribute")
	purego.RegisterLibFunc(&copyPostScriptName, coreText, "CTFontCopyPostScriptName")
	purego.RegisterLibFunc(&getFileSystemRepresentation, coreFoundation, "CFURLGetFileSystemRepresentation")
	purego.RegisterLibFunc(&getCString, coreFoundation, "CFStringGetCString")
	purego.RegisterLibFunc(&release, coreFoundation, "CFRelease")

	font := createUIFont(coreTextSystemUIFont, 0, 0)
	if font == 0 {
		return systemFontReference{}, false
	}
	defer release(font)
	attributeSymbol, err := purego.Dlsym(coreText, "kCTFontURLAttribute")
	if err != nil {
		return systemFontReference{}, false
	}
	fontURL := copyAttribute(font, *(*uintptr)(unsafe.Pointer(attributeSymbol)))
	if fontURL == 0 {
		return systemFontReference{}, false
	}
	defer release(fontURL)

	pathBuffer := make([]byte, 4096)
	if getFileSystemRepresentation(fontURL, 1, &pathBuffer[0], int64(len(pathBuffer))) == 0 {
		return systemFontReference{}, false
	}
	if end := bytes.IndexByte(pathBuffer, 0); end >= 0 {
		pathBuffer = pathBuffer[:end]
	}
	nameRef := copyPostScriptName(font)
	if nameRef == 0 {
		return systemFontReference{}, false
	}
	defer release(nameRef)
	nameBuffer := make([]byte, 512)
	if getCString(nameRef, &nameBuffer[0], int64(len(nameBuffer)), coreFoundationUTF8) == 0 {
		return systemFontReference{}, false
	}
	if end := bytes.IndexByte(nameBuffer, 0); end >= 0 {
		nameBuffer = nameBuffer[:end]
	}
	path := string(pathBuffer)
	face := string(nameBuffer)
	if path == "" || face == "" || !isSupportedFontPath(path) || !fileExists(path) {
		return systemFontReference{}, false
	}
	return systemFontReference{path: path, face: face}, true
}
