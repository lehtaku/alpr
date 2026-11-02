package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
)

func errorCode(t *testing.T, err error) domain.Code {
	t.Helper()
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) {
		t.Fatalf("want *domain.Error, got %T: %v", err, err)
	}
	return domainErr.Code
}

func TestParseReference(t *testing.T) {
	tests := []struct {
		raw  string
		want Reference
	}{
		{"keychain:alpr/work", Reference{Service: "alpr", Account: "work"}},
		{"keychain:alpr/work/jira", Reference{Service: "alpr", Account: "work/jira"}},
		{"env:GITHUB_TOKEN", Reference{EnvName: "GITHUB_TOKEN"}},
	}
	for _, tt := range tests {
		got, err := ParseReference(tt.raw)
		if err != nil || got != tt.want {
			t.Errorf("%s: got %+v, %v", tt.raw, got, err)
		}
	}
}

func TestParseReferenceInvalid(t *testing.T) {
	for _, raw := range []string{"ghp_secretTokenValue", "keychain:alpr", "keychain:/work", "keychain:alpr/", "env:", "env:1BAD", "env:HAS-DASH", ""} {
		_, err := ParseReference(raw)
		if code := errorCode(t, err); code != domain.CodeConfigInvalid {
			t.Errorf("%q: code %s", raw, code)
		}
	}
}

func TestParseReferenceDoesNotEchoToken(t *testing.T) {
	_, err := ParseReference("ghp_secretTokenValue")
	if err == nil || strings.Contains(err.Error(), "secretTokenValue") {
		t.Errorf("the error repeats the token: %v", err)
	}
}

func TestResolveEnv(t *testing.T) {
	t.Setenv("ALPR_TEST_TOKEN", "secret-token")
	if got, err := Resolve("env:ALPR_TEST_TOKEN"); err != nil || got != "secret-token" {
		t.Errorf("got %q, %v", got, err)
	}

	t.Setenv("ALPR_TEST_TOKEN", "")
	_, err := Resolve("env:ALPR_TEST_TOKEN")
	if code := errorCode(t, err); code != domain.CodeAuthMissing {
		t.Errorf("code %s", code)
	}
}

// fakeSecurity replaces the security command with a script that knows one item: service "alpr", account "work".
func fakeSecurity(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("keychain references are macOS only")
	}
	script := filepath.Join(t.TempDir(), "security")
	content := `#!/bin/sh
# Called as: find-generic-password -s SERVICE -a ACCOUNT -w
if [ "$3" = alpr ] && [ "$5" = work ]; then echo "secret-token"; exit 0; fi
if [ "$3" = alpr ] && [ "$5" = locked ]; then echo "security: User interaction is not allowed." >&2; exit 36; fi
echo "security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain." >&2
exit 44
`
	if err := os.WriteFile(script, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	previous := securityCommand
	securityCommand = script
	t.Cleanup(func() { securityCommand = previous })
}

func TestResolveKeychain(t *testing.T) {
	fakeSecurity(t)

	if got, err := Resolve("keychain:alpr/work"); err != nil || got != "secret-token" {
		t.Errorf("got %q, %v", got, err)
	}

	tests := []struct {
		raw, wantMsg string
	}{
		{"keychain:alpr/missing", "store one with `alpr auth set <profile>`"},
		{"keychain:alpr/locked", "User interaction is not allowed"},
	}
	for _, tt := range tests {
		_, err := Resolve(tt.raw)
		if code := errorCode(t, err); code != domain.CodeAuthMissing || !strings.Contains(err.Error(), tt.wantMsg) {
			t.Errorf("%s: %s: %v", tt.raw, code, err)
		}
	}
}
