package gitinfo

import (
	"errors"
	"os/exec"
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

func TestParseRemoteURL(t *testing.T) {
	tests := map[string]string{
		"https://github.com/owner/repo.git":                 "github.com/owner/repo",
		"https://github.com/owner/repo":                     "github.com/owner/repo",
		"https://github.com/owner/repo/":                    "github.com/owner/repo",
		"https://someone@bitbucket.org/workspace/repo.git":  "bitbucket.org/workspace/repo",
		"https://user:secret@github.example.com/owner/repo": "github.example.com/owner/repo",
		"http://git.example.com:8080/owner/repo.git":        "git.example.com/owner/repo",
		"ssh://git@github.com/owner/repo.git":               "github.com/owner/repo",
		"ssh://git@bitbucket.org:22/workspace/repo.git":     "bitbucket.org/workspace/repo",
		"git@github.com:owner/repo.git":                     "github.com/owner/repo",
		"git@bitbucket-work:workspace/repo.git":             "bitbucket-work/workspace/repo",
		"github.com:owner/repo":                             "github.com/owner/repo",
	}
	for raw, want := range tests {
		if got, err := ParseRemoteURL(raw); err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", raw, got, err, want)
		}
	}
}

func TestParseRemoteURLInvalid(t *testing.T) {
	for _, raw := range []string{
		"https://user:secret@github.com/owner",
		"https://gitlab.com/group/subgroup/repo.git",
		"file:///srv/git/repo.git",
		"/srv/git/repo.git",
		"",
	} {
		_, err := ParseRemoteURL(raw)
		if code := errorCode(t, err); code != domain.CodeRepoNotDetected {
			t.Errorf("%q: code %s", raw, code)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("%q: the error leaks credentials: %v", raw, err)
		}
	}
}

func TestParseRepo(t *testing.T) {
	if got, err := ParseRepo("github.com/owner/repo"); err != nil || got != "github.com/owner/repo" {
		t.Errorf("got %q, %v", got, err)
	}
	for _, s := range []string{"owner/repo", "github.com/owner/repo/extra", "github.com//repo", ""} {
		_, err := ParseRepo(s)
		if code := errorCode(t, err); code != domain.CodeUsage {
			t.Errorf("%q: code %s", s, code)
		}
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestOriginRepo(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--quiet")

	_, err := OriginRepo(dir)
	if code := errorCode(t, err); code != domain.CodeRepoNotDetected {
		t.Errorf("without origin: code %s", code)
	}

	git(t, dir, "remote", "add", "origin", "git@github.com:owner/repo.git")
	if got, err := OriginRepo(dir); err != nil || got != "github.com/owner/repo" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestOriginRepoOutsideGit(t *testing.T) {
	_, err := OriginRepo(t.TempDir())
	if code := errorCode(t, err); code != domain.CodeRepoNotDetected {
		t.Errorf("code %s", code)
	}
}
