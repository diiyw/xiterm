//go:build !js

package render

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
