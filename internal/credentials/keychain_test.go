package credentials

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
)

// fakeSecurityStore replaces the security command with a script that keeps items as files: `-i` stores the
// add-generic-password line it reads from stdin, and find-generic-password prints a stored item.
func fakeSecurityStore(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("keychain references are macOS only")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "security")
	content := `#!/bin/sh
dir=$(dirname "$0")
if [ "$1" = "-i" ]; then
  read -r line
  printf '%s\n' "$line" > "$dir/last-command"
  eval "set -- ${line#add-generic-password }"   # -U -s SERVICE -a ACCOUNT -w TOKEN
  [ "$3" = broken ] && exit 0                    # simulates a write that silently fails
  printf '%s\n' "$7" > "$dir/$3.$5"
  exit 0
fi
# find-generic-password -s SERVICE -a ACCOUNT -w
[ -f "$dir/$3.$5" ] && { cat "$dir/$3.$5"; exit 0; }
echo "security: The specified item could not be found in the keychain." >&2
exit 44
`
	if err := os.WriteFile(script, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	previous := securityCommand
	securityCommand = script
	t.Cleanup(func() { securityCommand = previous })
	return dir
}

func TestStoreRoundTrip(t *testing.T) {
	dir := fakeSecurityStore(t)
	token := `tok"en\with=specials`
	if err := Store("keychain:alpr/work", token); err != nil {
		t.Fatal(err)
	}
	if got, err := Resolve("keychain:alpr/work"); err != nil || got != token {
		t.Errorf("read back %q, %v", got, err)
	}

	// The token must reach security on stdin only; the arguments are just "-i".
	command, err := os.ReadFile(filepath.Join(dir, "last-command"))
	if err != nil || string(command) != `add-generic-password -U -s "alpr" -a "work" -w "tok\"en\\with=specials"`+"\n" {
		t.Errorf("command on stdin: %q, %v", command, err)
	}
}

func TestStoreDetectsFailedWrite(t *testing.T) {
	fakeSecurityStore(t)
	err := Store("keychain:broken/work", "token")
	if code := errorCode(t, err); code != domain.CodeInternal {
		t.Errorf("code %s", code)
	}
}

func TestStoreRejects(t *testing.T) {
	fakeSecurityStore(t)
	tests := []struct {
		ref, token string
		want       domain.Code
	}{
		{"env:GITHUB_TOKEN", "token", domain.CodeUsage},
		{"keychain:alpr/work", "", domain.CodeUsage},
		{"keychain:alpr/work", "two\nlines", domain.CodeUsage},
		{"not-a-reference", "token", domain.CodeConfigInvalid},
	}
	for _, tt := range tests {
		err := Store(tt.ref, tt.token)
		if code := errorCode(t, err); code != tt.want {
			t.Errorf("%s / %q: code %s, want %s", tt.ref, tt.token, code, tt.want)
		}
	}
}

func TestQuote(t *testing.T) {
	if got := quote(`a"b\c d$e`); got != `"a\"b\\c d$e"` {
		t.Errorf("got %s", got)
	}
}
