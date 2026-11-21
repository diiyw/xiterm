// Package config loads the user's terminal profiles without executing configuration code.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Profile struct {
	Name      string   `json:"name"`
	Shell     string   `json:"shell"`
	Args      []string `json:"args"`
	Directory string   `json:"directory"`
	Env       []string `json:"env,omitempty"`
}

type Theme struct {
	Name               string
	Background         uint32
	Chrome             uint32
	Surface            uint32
	Border             uint32
	Foreground         uint32
	Muted              uint32
	Accent             uint32
	Selection          uint32
	SearchHighlight    uint32
	TerminalForeground uint32
	TerminalBackground uint32
	TerminalPalette    [16]uint32
}

type Config struct {
	Profiles       []Profile `json:"profiles"`
	DefaultProfile string    `json:"default_profile"`
	FontPath       string    `json:"font_path,omitempty"`
	FontSize       int       `json:"font_size"`
	Scrollback     int       `json:"scrollback"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	Theme          string    `json:"theme,omitempty"`
}

const DefaultTheme = "light"

func ThemeNames() []string {
	return []string{"light", "dark", "solarized"}
}

func ThemeByName(name string) Theme {
	switch name {
	case "dark":
		const darkBackground = 0x303030
		return Theme{
			Name:               "dark",
			Background:         darkBackground,
			Chrome:             0x383838,
			Surface:            0x424242,
			Border:             0x545454,
			Foreground:         0xd8dee9,
			Muted:              0xa8b0be,
			Accent:             0x98c379,
			Selection:          0x344b65,
			SearchHighlight:    0x66552e,
			TerminalForeground: 0xd8dee9,
			TerminalBackground: darkBackground,
			TerminalPalette: [16]uint32{
				0x242933, 0xe06c75, 0x98c379, 0xe5c07b, 0x61afef, 0xc678dd, 0x56b6c2, 0xd8dee9,
				0x5c6370, 0xf28b91, 0xb3d994, 0xf2d396, 0x85c1ff, 0xdb9df2, 0x7fced8, 0xffffff,
			},
		}
	case "solarized":
		return Theme{Name: "solarized", Background: 0xfdf6e3, Chrome: 0xeee8d5, Surface: 0xe6dfc9, Border: 0xccc4ad, Foreground: 0x586e75, Muted: 0x839496, Accent: 0x268bd2, Selection: 0xc9e4e8, SearchHighlight: 0xf1d879, TerminalForeground: 0x586e75, TerminalBackground: 0xfdf6e3, TerminalPalette: [16]uint32{0x073642, 0xdc322f, 0x859900, 0xb58900, 0x268bd2, 0xd33682, 0x2aa198, 0xeee8d5, 0x002b36, 0xcb4b16, 0x586e75, 0x657b83, 0x839496, 0x6c71c4, 0x93a1a1, 0xfdf6e3}}
	default:
		return Theme{Name: DefaultTheme, Background: 0xf5f1e8, Chrome: 0xebe5d8, Surface: 0xe2dacb, Border: 0xc8bda9, Foreground: 0x2d3035, Muted: 0x72706a, Accent: 0x2f7d68, Selection: 0xc7ded7, SearchHighlight: 0xf0d99a, TerminalForeground: 0x2d3035, TerminalBackground: 0xf5f1e8, TerminalPalette: [16]uint32{0x5b5f66, 0xb54747, 0x287d5f, 0xa16d16, 0x356fa8, 0x8751a1, 0x247f85, 0x454a52, 0x8c9096, 0xd05b5b, 0x4eaa83, 0xc18b2d, 0x578ac4, 0xa875bb, 0x4b9da2, 0x2d3035}}
	}
}

func Default() Config {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	args := []string{"-l"}
	if runtime.GOOS == "windows" {
		shell = os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		args = []string{"/Q"}
	}
	home, _ := os.UserHomeDir()
	return Config{Profiles: []Profile{{Name: "Default", Shell: shell, Args: args, Directory: home}}, DefaultProfile: "Default", FontSize: 15, Scrollback: 10000, Width: 1440, Height: 900, Theme: DefaultTheme}
}

func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "xiterm", "config.json"), nil
}

// Load overlays optional values on defaults. Malformed configuration is an error.
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.Theme != "" && !contains(ThemeNames(), c.Theme) {
		return fmt.Errorf("unknown theme %q", c.Theme)
	}
	if c.FontSize < 10 || c.FontSize > 36 {
		return fmt.Errorf("font_size must be between 10 and 36")
	}
	if c.Scrollback < 0 || c.Scrollback > 100000 {
		return fmt.Errorf("scrollback must be between 0 and 100000")
	}
	if c.Width < 480 || c.Height < 320 || c.Width > 7680 || c.Height > 4320 {
		return fmt.Errorf("window size must be between 480x320 and 7680x4320")
	}
	if len(c.Profiles) == 0 {
		return fmt.Errorf("at least one profile is required")
	}
	names := map[string]bool{}
	for _, p := range c.Profiles {
		if p.Name == "" || p.Shell == "" {
			return fmt.Errorf("profiles require name and shell")
		}
		if names[p.Name] {
			return fmt.Errorf("duplicate profile %q", p.Name)
		}
		names[p.Name] = true
	}
	if !names[c.DefaultProfile] {
		return fmt.Errorf("default_profile %q does not exist", c.DefaultProfile)
	}
	return nil
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func (c Config) Profile() Profile {
	for _, p := range c.Profiles {
		if p.Name == c.DefaultProfile {
			return p
		}
	}
	return c.Profiles[0]
}

// Save uses a private temporary file and atomic rename to avoid partial configuration files.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
