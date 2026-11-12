package font

import "os"

func platformDefaultFont() (systemFontReference, bool) {
	for _, config := range [...]string{"/system/etc/font_fallback.xml", "/system/etc/fonts.xml"} {
		file, err := os.Open(config)
		if err != nil {
			continue
		}
		reference, ok := androidFontReference(file, fontSearchDirs())
		_ = file.Close()
		if ok {
			return reference, true
		}
	}
	return systemFontReference{}, false
}
