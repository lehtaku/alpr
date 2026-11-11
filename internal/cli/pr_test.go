package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// prConfig serves Bitbucket pull request 42 of ws/repo with one inline thread, and writes a config with a
// Bitbucket profile for ws/*.
func prConfig(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prPath = "/2.0/repositories/ws/repo/pullrequests/42"
		switch {
		case r.Header.Get("Authorization") != "Bearer good-bitbucket":
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == prPath:
			_, _ = w.Write([]byte(`{
  "id": 42, "title": "Add parser", "summary": {"raw": "Adds the parser.\n\nSee PROJ-1."}, "state": "OPEN", "draft": false,
  "author": {"uuid": "{a}", "nickname": "author", "display_name": "Author Person"},
  "source": {"branch": {"name": "feature/parser"}, "commit": {"hash": "abc123def456"}},
  "destination": {"branch": {"name": "main"}, "repository": {"full_name": "ws/repo"}},
  "created_on": "2026-10-30T08:00:00+00:00", "updated_on": "2026-10-31T09:15:00+00:00",
  "links": {"html": {"href": "https://bitbucket.org/ws/repo/pull-requests/42"}},
  "reviewers": [{"uuid": "{ann}", "nickname": "ann", "display_name": "Ann"}, {"uuid": "{bob}", "nickname": "bob", "display_name": "Bob"}],
  "participants": [{"user": {"uuid": "{ann}", "nickname": "ann", "display_name": "Ann"}, "role": "REVIEWER", "approved": true, "state": "approved"}]
}`))
		case r.URL.Path == prPath+"/comments":
			_, _ = w.Write([]byte(`{"values": [
  {"id": 1, "inline": {"path": "src/a.go", "to": 12, "start_to": 10}, "content": {"raw": "Rename this.\nIt's unclear."},
   "user": {"uuid": "{ann}", "display_name": "Ann"}, "created_on": "2026-10-31T10:00:00+00:00", "updated_on": "2026-10-31T10:00:00+00:00"},
  {"id": 2, "parent": {"id": 1}, "content": {"raw": "Done."}, "user": {"uuid": "{a}", "display_name": "Author Person"},
   "created_on": "2026-10-31T10:02:00+00:00", "updated_on": "2026-10-31T10:02:00+00:00"}
]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"type": "error", "error": {"message": "Resource not found"}}`))
		}
	}))
	t.Cleanup(server.Close)
	useConfig(t, `
[profiles.work]
provider = "bitbucket-cloud"
base_url = "`+server.URL+`/2.0"
match    = ["bitbucket.org/ws/*"]
token    = "env:ALPR_TEST_BITBUCKET"
`)
	t.Setenv("ALPR_TEST_BITBUCKET", "good-bitbucket")
}

func TestPRShowText(t *testing.T) {
	prConfig(t)
	code, stdout, stderr := runArgs("pr", "show", "42", "--repo", "bitbucket.org/ws/repo")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want := `Add parser
https://bitbucket.org/ws/repo/pull-requests/42

State:      open
Author:     Author Person
Branches:   feature/parser -> main
Reviewers:  Ann (approved), Bob

Adds the parser.

See PROJ-1.

src/a.go:10-12 (new side), unresolved
  Ann, 2026-10-31T10:00:00Z:
    Rename this.
    It's unclear.
  Author Person, 2026-10-31T10:02:00Z:
    Done.
`
	if stdout != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestPRShowJSON(t *testing.T) {
	prConfig(t)
	code, stdout, stderr := runArgs("pr", "show", "42", "--repo", "bitbucket.org/ws/repo", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout isn't JSON: %v\n%s", err, stdout)
	}
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if strings.Join(keys, " ") != "author created_at description draft head_commit id repo reviewers source_branch state target_branch threads title updated_at url" {
		t.Errorf("keys %v", keys)
	}
	thread := got["threads"].([]any)[0].(map[string]any)
	if thread["resolved"] != false || thread["outdated"] != nil || len(thread["comments"].([]any)) != 2 {
		t.Errorf("thread %v", thread)
	}
}

func TestPRShowDetectsOriginRemote(t *testing.T) {
	prConfig(t)
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@bitbucket.org:ws/repo.git"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(dir)

	code, stdout, stderr := runArgs("pr", "show", "42", "--json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"repo": "bitbucket.org/ws/repo"`) {
		t.Errorf("exit %d, stderr %q, stdout:\n%s", code, stderr, stdout)
	}
}

func TestPRShowErrors(t *testing.T) {
	prConfig(t)
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantMsg  string
	}{
		{"not a number", []string{"pr", "show", "abc", "--repo", "bitbucket.org/ws/repo"}, 2, `"abc" is not a pull request number`},
		{"bad --repo", []string{"pr", "show", "42", "--repo", "ws/repo"}, 2, "is not of the form host/owner/repo"},
		{"no matching profile", []string{"pr", "show", "42", "--repo", "bitbucket.org/other/repo"}, 2, "no profile matches"},
		{"unknown pull request", []string{"pr", "show", "7", "--repo", "bitbucket.org/ws/repo"}, 4, "Resource not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runArgs(tt.args...)
			if code != tt.wantCode || stdout != "" || !strings.Contains(stderr, tt.wantMsg) {
				t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
			}
		})
	}
}
