// Package githubissues is the GitHub Issues adapter. It shares the GitHub provider's API access and token.
package githubissues

import (
	"context"

	"github.com/lehtaku/alpr/internal/domain"
	"github.com/lehtaku/alpr/internal/providers/github"
)

// Tracker implements trackers.TicketTracker for GitHub Issues.
type Tracker struct {
	github *github.Provider
}

// New returns a GitHub Issues tracker; an empty baseURL means github.DefaultBaseURL.
func New(baseURL, token string) *Tracker {
	return &Tracker{github: github.New(baseURL, token)}
}

// CurrentUser returns the user the token belongs to.
func (t *Tracker) CurrentUser(ctx context.Context) (domain.User, error) {
	return t.github.CurrentUser(ctx)
}
