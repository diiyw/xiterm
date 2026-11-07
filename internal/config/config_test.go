package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMissingConfigUsesDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestConfigRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	c := Default()
	c.FontSize = 18
	c.Profiles = append(c.Profiles, Profile{Name: "Work", Shell: "/bin/sh", Args: []string{"-l"}, Env: []string{"WORK=1"}})
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.FontSize != 18 || len(got.Profiles) != 2 || got.Profiles[1].Env[0] != "WORK=1" {
		t.Fatalf("round trip: %+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("private config mode: %v", info.Mode())
	}
	c.DefaultProfile = "missing"
	if err = Save(path, c); err == nil {
		t.Fatal("accepted unknown default profile")
	}
}
func TestRejectMalformedAndInvalid(t *testing.T) {
	for _, input := range []string{`{`, `{"font_size":0}`, `{"scrollback":-1}`, `{"profiles":[]}`, `{"width":10}`, `{"profiles":[{"name":"a","shell":"/bin/sh"},{"name":"a","shell":"/bin/sh"}]}`} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
