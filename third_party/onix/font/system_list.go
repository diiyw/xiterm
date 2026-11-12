package font

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"sync"
)

// SystemFont identifies one installed face, including its index in a collection.
// Listing faces reads metadata only; Load parses the selected face on demand.
type SystemFont struct {
	Family    string
	Path      string
	Index     int
	Weight    int
	Italic    bool
	Monospace bool
}

var systemFonts = sync.OnceValues(func() ([]SystemFont, error) {
	return listSystemFonts(fontSearchDirs(), platformSystemFontFiles())
})

// SystemFonts returns the process-cached installed font list. Callers must not
// modify the returned slice. Fonts installed later become available on restart.
func SystemFonts() ([]SystemFont, error) { return systemFonts() }

func (f SystemFont) Load() (*Face, error) {
	backing, err := loadFontFile(f.Path)
	if err != nil {
		return nil, err
	}
	face, err := ParseIndex(backing.data, f.Index)
	if err != nil {
		_ = backing.close()
		return nil, err
	}
	face.attachBacking(backing)
	return face, nil
}

func listSystemFonts(dirs, files []string) ([]SystemFont, error) {
	var fonts []SystemFont
	seen := make(map[string]bool)
	read := func(path string) {
		path = filepath.Clean(path)
		if seen[path] || !isSupportedFontPath(path) {
			return
		}
		seen[path] = true
		backing, err := loadFontFile(path)
		if err != nil {
			return
		}
		defer backing.close()
		data, count := backing.data, 1
		if len(data) >= 12 && string(data[:4]) == "ttcf" {
			count = int(binary.BigEndian.Uint32(data[8:12]))
			if count > (len(data)-12)/4 {
				return
			}
		}
		for index := range count {
			sfnt, err := parseSFNT(data, index)
			if err != nil {
				continue
			}
			name, err := sfnt.table("name")
			if err != nil {
				continue
			}
			family := systemFontFamily(name)
			if family == "" {
				continue
			}
			f := SystemFont{Family: family, Path: path, Index: index, Weight: 400}
			if os2, err := sfnt.table("OS/2"); err == nil {
				if len(os2) >= 6 {
					f.Weight = int(binary.BigEndian.Uint16(os2[4:6]))
				}
				if len(os2) >= 64 {
					f.Italic = binary.BigEndian.Uint16(os2[62:64])&0x201 != 0
				}
			}
			if post, err := sfnt.table("post"); err == nil && len(post) >= 16 {
				f.Monospace = binary.BigEndian.Uint32(post[12:16]) != 0
			}
			fonts = append(fonts, f)
		}
	}
	for _, path := range files {
		read(path)
	}
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				read(path)
			}
			return nil
		})
	}
	sort.Slice(fonts, func(i, j int) bool {
		a, b := fonts[i], fonts[j]
		if a.Family != b.Family {
			return a.Family < b.Family
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Index < b.Index
	})
	if len(fonts) == 0 {
		return nil, errors.New("no installed system fonts found")
	}
	return fonts, nil
}

func systemFontFamily(data []byte) string {
	if len(data) < 6 {
		return ""
	}
	count, offset := int(binary.BigEndian.Uint16(data[2:4])), int(binary.BigEndian.Uint16(data[4:6]))
	if 6+count*12 > len(data) || offset > len(data) {
		return ""
	}
	var family string
	best := -1
	for i := range count {
		r := data[6+i*12 : 18+i*12]
		platform, language, id := binary.BigEndian.Uint16(r), binary.BigEndian.Uint16(r[4:6]), binary.BigEndian.Uint16(r[6:8])
		if id != 1 && id != 16 {
			continue
		}
		start, length := offset+int(binary.BigEndian.Uint16(r[10:12])), int(binary.BigEndian.Uint16(r[8:10]))
		if start+length > len(data) {
			continue
		}
		score := 0
		if id == 16 {
			score += 4
		}
		if platform == 0 || platform == 3 {
			score++
		}
		if language == 0 || language == 0x409 {
			score += 2
		}
		if score > best {
			if value := decodeFontName(data[start:start+length], platform); value != "" {
				family, best = value, score
			}
		}
	}
	return family
}
