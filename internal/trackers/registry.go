package trackers

import (
	"github.com/lehtaku/alpr/internal/config"
	"github.com/lehtaku/alpr/internal/domain"
	"github.com/lehtaku/alpr/internal/trackers/githubissues"
	"github.com/lehtaku/alpr/internal/trackers/jira"
)

// New returns the adapter for a tracker type from config.toml. github-issues takes the profile's base URL and token.
func New(kind, baseURL, username, token string) (TicketTracker, error) {
	switch kind {
	case config.TrackerJira:
		return jira.New(baseURL, username, token), nil
	case config.TrackerGitHubIssues:
		return githubissues.New(baseURL, token), nil
	}
	return nil, domain.Errorf(domain.CodeConfigInvalid, "unknown tracker type %q", kind)
}
