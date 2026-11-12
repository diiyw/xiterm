package window

import (
	"path/filepath"
	"strings"
)

// normalizeDroppedPath converts a desktop file URI to the local path format.
// Browser drops are already names and are returned unchanged.
func normalizeDroppedPath(value string) string {
	value = strings.TrimSuffix(value, "\r")
	if value == "" || strings.Trim(value, "\r\n") == "" {
		return ""
	}
	if !strings.HasPrefix(value, "file:") {
		if strings.Contains(value, "://") {
			return ""
		}
		return value
	}
	path, ok := localFileURIPath(value)
	if !ok {
		return ""
	}
	return filepath.FromSlash(path)
}

func localFileURIPath(value string) (string, bool) {
	if !strings.HasPrefix(value, "file:") {
		return "", false
	}
	rest := value[len("file:"):]
	path := rest
	if strings.HasPrefix(rest, "//") {
		rest = rest[2:]
		slash := strings.IndexByte(rest, '/')
		if slash < 0 {
			return "", false
		}
		host := rest[:slash]
		if host != "" && host != "localhost" {
			return "", false
		}
		path = rest[slash:]
	} else if rest == "" {
		return "", false
	}
	if cut := strings.IndexAny(path, "?#"); cut >= 0 {
		path = path[:cut]
	}
	return unescapeFileURIPath(path)
}

func unescapeFileURIPath(value string) (string, bool) {
	if !strings.ContainsRune(value, '%') {
		return value, value != ""
	}
	result := make([]byte, 0, len(value))
	for index := 0; index < len(value); index++ {
		if value[index] != '%' {
			result = append(result, value[index])
			continue
		}
		if index+2 >= len(value) {
			return "", false
		}
		hi, okHigh := hexDigit(value[index+1])
		lo, okLow := hexDigit(value[index+2])
		if !okHigh || !okLow {
			return "", false
		}
		result = append(result, hi<<4|lo)
		index += 2
	}
	return string(result), len(result) > 0
}

func hexDigit(value byte) (byte, bool) {
	switch {
	case '0' <= value && value <= '9':
		return value - '0', true
	case 'a' <= value && value <= 'f':
		return value - 'a' + 10, true
	case 'A' <= value && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}
