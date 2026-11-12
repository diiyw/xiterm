package spine

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type atlasRegion struct {
	name                string
	x, y, width, height int
	originalWidth       int
	originalHeight      int
	offsetX, offsetY    int
	rotate              bool
}

type atlasPage struct {
	name                 string
	minFilter, magFilter string
}

func (p atlasPage) mipmapped() bool {
	return strings.HasPrefix(strings.ToLower(p.minFilter), "mipmap")
}

func parseAtlas(data []byte) (atlasPage, map[string]atlasRegion, error) {
	lines := make([]string, 0, 64)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return atlasPage{}, nil, err
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	if len(lines) == 0 {
		return atlasPage{}, nil, fmt.Errorf("spine: atlas is empty")
	}
	page := atlasPage{name: lines[0]}
	i := 1
	for i < len(lines) && lines[i] != "" {
		key, values, ok := atlasFields(lines[i])
		if !ok {
			break
		}
		if key == "filter" {
			if len(values) > 0 {
				page.minFilter = values[0]
			}
			if len(values) > 1 {
				page.magFilter = values[1]
			}
		}
		i++
	}
	regions := make(map[string]atlasRegion)
	for i < len(lines) {
		line := lines[i]
		if line == "" || strings.Contains(line, ":") {
			i++
			continue
		}
		region := atlasRegion{name: line}
		i++
		for i < len(lines) && lines[i] != "" && strings.Contains(lines[i], ":") {
			key, values, _ := atlasFields(lines[i])
			switch key {
			case "rotate":
				region.rotate = values[0] == "true" || values[0] == "90"
			case "xy":
				region.x, region.y = atlasInt(values, 0), atlasInt(values, 1)
			case "size":
				region.width, region.height = atlasInt(values, 0), atlasInt(values, 1)
			case "orig":
				region.originalWidth, region.originalHeight = atlasInt(values, 0), atlasInt(values, 1)
			case "offset":
				region.offsetX, region.offsetY = atlasInt(values, 0), atlasInt(values, 1)
			}
			i++
		}
		if region.width <= 0 || region.height <= 0 {
			return atlasPage{}, nil, fmt.Errorf("spine: atlas region %q has invalid size", region.name)
		}
		if region.originalWidth <= 0 {
			region.originalWidth = region.width
		}
		if region.originalHeight <= 0 {
			region.originalHeight = region.height
		}
		regions[region.name] = region
	}
	if len(regions) == 0 {
		return atlasPage{}, nil, fmt.Errorf("spine: atlas has no regions")
	}
	return page, regions, nil
}

func atlasFields(line string) (string, []string, bool) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", nil, false
	}
	values := strings.Split(strings.TrimSpace(value), ",")
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	return strings.TrimSpace(key), values, true
}

func atlasInt(values []string, index int) int {
	if index >= len(values) {
		return 0
	}
	value, _ := strconv.Atoi(values[index])
	return value
}
