package app

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/onix-engine/onix/font"
)

func systemUIFontFace() (*font.Face, error) {
	// Fontconfig's generic sans-serif can differ from the user's desktop font.
	// Read the active desktop's setting first, then let Fontconfig resolve it.
	if description := desktopUIFont(); description != "" {
		pattern := uiFontPattern(description)
		output := fontCommand("fc-match", "-f", "%{file}\n%{index}", pattern)
		path, index, ok := strings.Cut(output, "\n")
		if i, err := strconv.Atoi(strings.TrimSpace(index)); ok && err == nil {
			if face, err := (font.SystemFont{Path: path, Index: i}).Load(); err == nil {
				return face, nil
			}
		}
	}
	return font.LoadDefault()
}

func fontCommand(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func desktopUIFont() string {
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP") + ":" + os.Getenv("XDG_SESSION_DESKTOP"))
	switch {
	case strings.Contains(desktop, "kde"):
		for _, command := range []string{"kreadconfig6", "kreadconfig5"} {
			if value := fontCommand(command, "--file", "kdeglobals", "--group", "General", "--key", "font"); value != "" {
				family, _, _ := strings.Cut(value, ",")
				return family
			}
		}
	case strings.Contains(desktop, "xfce"):
		return fontCommand("xfconf-query", "-c", "xsettings", "-p", "/Gtk/FontName")
	default:
		schema := "org.gnome.desktop.interface"
		if strings.Contains(desktop, "cinnamon") {
			schema = "org.cinnamon.desktop.interface"
		} else if strings.Contains(desktop, "mate") {
			schema = "org.mate.interface"
		} else if !strings.Contains(desktop, "gnome") && !strings.Contains(desktop, "unity") && !strings.Contains(desktop, "pantheon") && !strings.Contains(desktop, "budgie") {
			return ""
		}
		return strings.Trim(fontCommand("gsettings", "get", schema, "font-name"), "'\"")
	}
	return ""
}

func uiFontPattern(description string) string {
	parts := strings.Fields(description)
	if len(parts) > 1 {
		if _, err := strconv.ParseFloat(strings.TrimSuffix(parts[len(parts)-1], "px"), 64); err == nil {
			parts = parts[:len(parts)-1]
		}
	}
	style := ""
	for len(parts) > 1 {
		switch strings.ToLower(parts[len(parts)-1]) {
		case "bold", "semibold", "light", "medium", "regular", "italic", "oblique":
			style = parts[len(parts)-1] + " " + style
			parts = parts[:len(parts)-1]
		default:
			if style != "" {
				return strings.Join(parts, " ") + ":style=" + strings.TrimSpace(style)
			}
			return strings.Join(parts, " ")
		}
	}
	pattern := strings.Join(parts, " ")
	if style != "" {
		pattern += ":style=" + strings.TrimSpace(style)
	}
	return pattern
}
