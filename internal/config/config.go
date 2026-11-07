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

type Config struct {
	Profiles       []Profile `json:"profiles"`
	DefaultProfile string    `json:"default_profile"`
	FontPath       string    `json:"font_path,omitempty"`
	FontSize       int       `json:"font_size"`
	Scrollback     int       `json:"scrollback"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
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
	return Config{Profiles: []Profile{{Name: "Default", Shell: shell, Args: args, Directory: home}}, DefaultProfile: "Default", FontSize: 15, Scrollback: 10000, Width: 1200, Height: 780}
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
