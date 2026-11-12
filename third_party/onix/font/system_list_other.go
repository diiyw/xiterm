//go:build (!linux || android) && !windows

package font

func platformSystemFontFiles() []string { return nil }
