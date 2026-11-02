// Package config locates and validates config.toml and selects the profile to use.
package config

import (
	"os"
	"path/filepath"

	"github.com/lehtaku/alpr/internal/domain"
)

// FilePath returns $XDG_CONFIG_HOME/alpr/config.toml, or ~/.config/alpr/config.toml.
// os.UserConfigDir isn't used because it returns ~/Library/Application Support on macOS.
func FilePath() (string, error) {
	dir, err := xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "alpr", "config.toml"), nil
}

// xdgDir returns $env when it holds an absolute path, as the XDG spec requires, otherwise ~/fallback.
func xdgDir(env, fallback string) (string, error) {
	if dir := os.Getenv(env); filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", domain.Errorf(domain.CodeConfigNotFound, "can't locate the home directory: %v", err)
	}
	return filepath.Join(home, fallback), nil
}
