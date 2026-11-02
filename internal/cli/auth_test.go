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

// fakeAPIs serves GitHub's GET /user and Jira's GET /rest/api/3/myself; any other token gets 401.
func fakeAPIs(t *testing.T) (githubURL, jiraURL string) {
	t.Helper()
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-github" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id": 42, "login": "someone", "name": "Some One"}`))
	}))
	jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, token, _ := r.BasicAuth(); token != "good-jira" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"accountId": "5b10", "displayName": "Jira User"}`))
	}))
	t.Cleanup(github.Close)
	t.Cleanup(jira.Close)
	return github.URL, jira.URL
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
	githubURL, jiraURL := fakeAPIs(t)
	useConfig(t, `
[profiles.personal]
provider = "github"
base_url = "`+githubURL+`"
match    = ["github.com/someone/*"]
token    = "env:ALPR_TEST_GITHUB"

[profiles.personal.tracker]
type = "github-issues"

[profiles.work]
provider = "github"
base_url = "`+githubURL+`"
match    = ["github.com/acme/*"]
token    = "env:ALPR_TEST_GITHUB"

[profiles.work.tracker]
type     = "jira"
base_url = "`+jiraURL+`"
username = "you@example.com"
token    = "env:ALPR_TEST_JIRA"
`)
}

func TestAuthTestAllPass(t *testing.T) {
	authConfig(t)
	t.Setenv("ALPR_TEST_GITHUB", "good-github")
	t.Setenv("ALPR_TEST_JIRA", "good-jira")

	code, stdout, stderr := runArgs("auth", "test")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{
		"personal  provider  github         ok: Some One (someone)",
		"personal  tracker   github-issues  ok: Some One (someone)",
		"work      tracker   jira           ok: Jira User",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestAuthTestJSONWithFailure(t *testing.T) {
	authConfig(t)
	t.Setenv("ALPR_TEST_GITHUB", "good-github")
	t.Setenv("ALPR_TEST_JIRA", "bad-jira")

	code, stdout, stderr := runArgs("auth", "test", "--json", "--profile", "work")
	if code != 3 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	var got struct {
		Profiles []struct {
			Profile  string         `json:"profile"`
			Provider map[string]any `json:"provider"`
			Tracker  map[string]any `json:"tracker"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout isn't JSON: %v\n%s", err, stdout)
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Profile != "work" {
		t.Fatalf("profiles: %s", stdout)
	}
	provider, tracker := got.Profiles[0].Provider, got.Profiles[0].Tracker
	if provider["ok"] != true || provider["error"] != nil || provider["user"].(map[string]any)["username"] != "someone" {
		t.Errorf("provider: %v", provider)
	}
	trackerErr, _ := tracker["error"].(map[string]any)
	if tracker["ok"] != false || tracker["user"] != nil || trackerErr["code"] != "auth_invalid" || trackerErr["scope"] != "work" {
		t.Errorf("tracker: %v", tracker)
	}
}

func TestAuthTestMissingToken(t *testing.T) {
	authConfig(t)
	t.Setenv("ALPR_TEST_GITHUB", "")
	t.Setenv("ALPR_TEST_JIRA", "good-jira")

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
		{"github-issues tracker", []string{"auth", "set", "personal", "--tracker"}, "has no tracker with its own token"},
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
