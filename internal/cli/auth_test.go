package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGitHub serves GitHub's GET /user; any token other than good-github gets 401.
func fakeGitHub(t *testing.T) string {
	t.Helper()
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-github" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id": 42, "login": "someone", "name": "Some One"}`))
	}))
	t.Cleanup(github.Close)
	return github.URL
}

// useConfig writes config.toml under a temporary XDG_CONFIG_HOME.
func useConfig(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "alpr"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alpr", "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func authConfig(t *testing.T) {
	githubURL := fakeGitHub(t)
	useConfig(t, `
[profiles.personal]
provider = "github"
base_url = "`+githubURL+`"
match    = ["github.com/someone/*"]
token    = "env:ALPR_TEST_GITHUB"

[profiles.work]
provider = "github"
base_url = "`+githubURL+`"
match    = ["github.com/acme/*"]
token    = "env:ALPR_TEST_WORK"
`)
}

func TestAuthTestAllPass(t *testing.T) {
	authConfig(t)
	t.Setenv("ALPR_TEST_GITHUB", "good-github")
	t.Setenv("ALPR_TEST_WORK", "good-github")

	code, stdout, stderr := runArgs("auth", "test")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{
		"PROFILE   PROVIDER  RESULT",
		"personal  github    ok: Some One (someone)",
		"work      github    ok: Some One (someone)",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestAuthTestJSONWithFailure(t *testing.T) {
	authConfig(t)
	t.Setenv("ALPR_TEST_GITHUB", "good-github")
	t.Setenv("ALPR_TEST_WORK", "bad-github")

	code, stdout, stderr := runArgs("auth", "test", "--json")
	if code != 3 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	var got struct {
		Profiles []map[string]any `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout isn't JSON: %v\n%s", err, stdout)
	}
	if len(got.Profiles) != 2 || got.Profiles[0]["profile"] != "personal" || got.Profiles[1]["profile"] != "work" || len(got.Profiles[0]) != 2 {
		t.Fatalf("profiles: %s", stdout)
	}
	passed, failed := got.Profiles[0]["provider"].(map[string]any), got.Profiles[1]["provider"].(map[string]any)
	if passed["ok"] != true || passed["error"] != nil || passed["user"].(map[string]any)["username"] != "someone" {
		t.Errorf("passing check: %v", passed)
	}
	failedErr, _ := failed["error"].(map[string]any)
	if failed["ok"] != false || failed["user"] != nil || failedErr["code"] != "auth_invalid" || failedErr["scope"] != "work" {
		t.Errorf("failing check: %v", failed)
	}
}

func TestAuthTestMissingToken(t *testing.T) {
	authConfig(t)
	t.Setenv("ALPR_TEST_GITHUB", "")

	code, stdout, _ := runArgs("auth", "test", "--profile", "personal")
	if code != 3 || !strings.Contains(stdout, "failed: environment variable ALPR_TEST_GITHUB is not set or empty") {
		t.Errorf("exit %d, stdout:\n%s", code, stdout)
	}
}

func TestAuthTestErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if code, _, stderr := runArgs("auth", "test"); code != 2 || !strings.Contains(stderr, "config file not found") {
		t.Errorf("no config: exit %d, stderr %q", code, stderr)
	}

	authConfig(t)
	if code, _, stderr := runArgs("auth", "test", "--profile", "nope"); code != 2 || !strings.Contains(stderr, `no profile named "nope"`) {
		t.Errorf("unknown profile: exit %d, stderr %q", code, stderr)
	}
}

func TestAuthSetRejects(t *testing.T) {
	authConfig(t)
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"env reference", []string{"auth", "set", "personal"}, "comes from env:ALPR_TEST_GITHUB"},
		{"unknown profile", []string{"auth", "set", "nope"}, `no profile named "nope"`},
		{"missing profile argument", []string{"auth", "set"}, "accepts 1 arg(s), received 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runWithInput("piped-token\n", tt.args...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, tt.wantMsg) {
				t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
			}
		})
	}
}
