package config

import (
	"fmt"
	"path"
	"strings"

	"github.com/lehtaku/alpr/internal/domain"
)

// Select returns the named profile or, when name is empty, the only profile whose patterns cover repo (host/owner/repo).
func (c *Config) Select(name, repo string) (*Profile, error) {
	if name != "" {
		if profile, ok := c.Profiles[name]; ok {
			return profile, nil
		}
		return nil, &domain.Error{
			Code:    domain.CodeProfileNotFound,
			Message: fmt.Sprintf("no profile named %q; profiles: %s", name, strings.Join(c.names(), ", ")),
			Details: map[string]any{"profile": name, "profiles": c.names()},
		}
	}

	var matches []string
	for _, candidate := range c.names() {
		if c.Profiles[candidate].Matches(repo) {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 1:
		return c.Profiles[matches[0]], nil
	case 0:
		return nil, &domain.Error{
			Code:    domain.CodeProfileNotFound,
			Message: fmt.Sprintf("no profile matches %s; add a match pattern or pass --profile", repo),
			Details: map[string]any{"repo": repo},
		}
	default:
		return nil, &domain.Error{
			Code:    domain.CodeProfileAmbiguous,
			Message: fmt.Sprintf("several profiles match %s (%s); pass --profile", repo, strings.Join(matches, ", ")),
			Details: map[string]any{"repo": repo, "profiles": matches},
		}
	}
}

// Matches reports whether one of the profile's patterns covers repo (host/owner/repo), ignoring case.
func (p *Profile) Matches(repo string) bool {
	repo = strings.ToLower(repo)
	for _, pattern := range p.Match {
		// Patterns are validated on load, so Match can't fail here.
		if ok, _ := path.Match(strings.ToLower(pattern), repo); ok {
			return true
		}
	}
	return false
}
