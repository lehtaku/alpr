package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
)

const validConfig = `
[profiles.work]
provider = "bitbucket-cloud"
match    = ["bitbucket.org/example-workspace/*"]
token    = "keychain:alpr/work"

[profiles.personal]
provider = "github"
match    = ["github.com/example-owner/*"]
token    = "env:GITHUB_TOKEN"
base_url = "https://github.example.com/api/v3"
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func errorCode(t *testing.T, err error) domain.Code {
	t.Helper()
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) {
		t.Fatalf("want *domain.Error, got %T: %v", err, err)
	}
	return domainErr.Code
}

func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatal(err)
	}

	work := cfg.Profiles["work"]
	if work.Name != "work" || work.Provider != ProviderBitbucketCloud || work.Token != "keychain:alpr/work" {
		t.Errorf("work profile: %+v", work)
	}
	personal := cfg.Profiles["personal"]
	if personal.Name != "personal" || personal.BaseURL != "https://github.example.com/api/v3" {
		t.Errorf("personal profile: %+v", personal)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if code := errorCode(t, err); code != domain.CodeConfigNotFound {
		t.Errorf("code %s", code)
	}
}

func TestLoadInvalid(t *testing.T) {
	const github = "[profiles.p]\nprovider = \"github\"\nmatch = [\"github.com/o/*\"]\ntoken = \"env:T\"\n"
	tests := []struct {
		name, content, wantMsg string
	}{
		{"syntax error", "[profiles.p\n", "toml"},
		{"no profiles", "", "no profiles defined"},
		{"unknown key", github + "usename = \"x\"\n", "unknown keys: profiles.p.usename"},
		{"bad name", strings.Replace(github, "profiles.p", "profiles.\"my profile\"", 1), "may only contain"},
		{"bad provider", strings.Replace(github, `"github"`, `"gitlab"`, 1), "provider must be"},
		{"no match", strings.Replace(github, `["github.com/o/*"]`, "[]", 1), "at least one pattern"},
		{"two-segment pattern", strings.Replace(github, "github.com/o/*", "github.com/*", 1), "host/owner/repo"},
		{"malformed pattern", strings.Replace(github, "github.com/o/*", "github.com/[o/*", 1), "syntax error in pattern"},
		{"no token", strings.Replace(github, "token = \"env:T\"\n", "", 1), "token is required"},
		{"raw token", strings.Replace(github, "env:T", "ghp_pastedToken", 1), "token must be a reference"},
		{"bad base_url", github + "base_url = \"api.github.com\"\n", "base_url must be an http(s) URL"},
		{"tracker table", github + "[profiles.p.tracker]\ntype = \"jira\"\n", "unknown keys: profiles.p.tracker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := writeConfig(t, tt.content)
			_, err := Load(file)
			if code := errorCode(t, err); code != domain.CodeConfigInvalid {
				t.Fatalf("code %s: %v", code, err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) || !strings.HasPrefix(err.Error(), file+": ") {
				t.Errorf("message %q, want it to contain %q", err.Error(), tt.wantMsg)
			}
		})
	}
}
