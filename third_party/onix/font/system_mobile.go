package font

import (
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
)

// Read system configuration once; filenames and collection indices may vary
// by vendor. Prefer the regular sans-serif face rather than a guessed filename.
func androidFontReference(reader io.Reader, directories []string) (systemFontReference, bool) {
	decoder := xml.NewDecoder(reader)
	var best systemFontReference
	bestScore := int(^uint(0) >> 1)
	familyCount, familyRank := 0, -1
	for {
		token, err := decoder.Token()
		if err != nil {
			return best, err == io.EOF && best.path != ""
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "family":
				familyRank = -1
				if familyCount == 0 {
					familyRank = 1
				}
				familyCount++
				for _, attr := range element.Attr {
					if attr.Name.Local == "name" && attr.Value == "sans-serif" {
						familyRank = 0
					}
				}
				if familyRank < 0 {
					if err := decoder.Skip(); err != nil {
						return systemFontReference{}, false
					}
				}
			case "font":
				if familyRank < 0 {
					continue
				}
				var entry struct {
					File   string `xml:",chardata"`
					Style  string `xml:"style,attr"`
					Weight int    `xml:"weight,attr"`
					Index  int    `xml:"index,attr"`
				}
				if err := decoder.DecodeElement(&entry, &element); err != nil {
					return systemFontReference{}, false
				}
				if entry.Style == "italic" || entry.Index < 0 || entry.Weight < 0 || entry.Weight > 1000 {
					continue
				}
				if entry.Weight == 0 {
					entry.Weight = 400
				}
				score := familyRank*1000 + max(entry.Weight-400, 400-entry.Weight)
				if score >= bestScore {
					continue
				}
				name := strings.TrimSpace(entry.File)
				path := ""
				if filepath.IsAbs(name) {
					if fileExists(name) {
						path = name
					}
				} else {
					for _, directory := range directories {
						candidate := filepath.Join(directory, name)
						if fileExists(candidate) {
							path = candidate
							break
						}
					}
				}
				if !isSupportedFontPath(path) {
					continue
				}
				best, bestScore = systemFontReference{path: path, index: entry.Index}, score
				if score == 0 {
					return best, true
				}
			}
		case xml.EndElement:
			if element.Name.Local == "family" {
				familyRank = -1
			}
		}
	}
}
