// Package gitinfo reads repository identity from git.
package gitinfo

import (
	"net/url"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/lehtaku/alpr/internal/domain"
)

// scpRemote matches the scp-like SSH form: [user@]host:owner/repo.
var scpRemote = regexp.MustCompile(`^(?:[^@/]+@)?([^:/]+):(.+)$`)

// OriginRepo returns host/owner/repo for the origin remote of the git repository in dir.
func OriginRepo(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		reason := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			reason = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", domain.Errorf(domain.CodeRepoNotDetected,
			"can't read the origin remote (%s); run alpr inside a git repository or pass --repo host/owner/repo", reason)
	}
	return ParseRemoteURL(strings.TrimSpace(string(out)))
}

// ParseRemoteURL turns an HTTPS, ssh:// or scp-like git remote URL into host/owner/repo.
// Errors don't repeat the URL, because HTTPS remotes can embed credentials.
func ParseRemoteURL(raw string) (string, error) {
	var host, repoPath string
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err == nil && slices.Contains([]string{"https", "http", "ssh"}, u.Scheme) {
			host, repoPath = u.Hostname(), u.Path
		}
	} else if m := scpRemote.FindStringSubmatch(raw); m != nil {
		host, repoPath = m[1], m[2]
	}

	repoPath = strings.TrimSuffix(strings.Trim(repoPath, "/"), ".git")
	if repo, err := ParseRepo(host + "/" + repoPath); host != "" && err == nil {
		return repo, nil
	}
	return "", domain.Errorf(domain.CodeRepoNotDetected,
		"the origin remote isn't a host/owner/repo URL; pass --repo host/owner/repo")
}

// ParseRepo checks that s has the form host/owner/repo.
func ParseRepo(s string) (string, error) {
	segments := strings.Split(s, "/")
	if len(segments) != 3 || slices.Contains(segments, "") {
		return "", domain.Errorf(domain.CodeUsage, "%q is not of the form host/owner/repo", s)
	}
	return s, nil
}
