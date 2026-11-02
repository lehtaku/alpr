package config

import (
	"path/filepath"
	"testing"
)

func TestFilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name, xdg, want string
	}{
		{"unset", "", filepath.Join(home, ".config", "alpr", "config.toml")},
		{"absolute", "/custom/config", "/custom/config/alpr/config.toml"},
		{"relative is ignored", "relative/config", filepath.Join(home, ".config", "alpr", "config.toml")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tt.xdg)
			got, err := FilePath()
			if err != nil || got != tt.want {
				t.Errorf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
