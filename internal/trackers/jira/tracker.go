package jira

import (
	"context"

	"github.com/lehtaku/alpr/internal/domain"
)

// Tracker implements trackers.TicketTracker for Jira Cloud.
type Tracker struct {
	client *client
}

// New returns a Jira tracker for a site URL such as https://<site>.atlassian.net.
func New(siteURL, username, token string) *Tracker {
	return &Tracker{client: newClient(siteURL, username, token)}
}

// CurrentUser returns the user the token belongs to.
func (t *Tracker) CurrentUser(ctx context.Context) (domain.User, error) {
	user, err := t.client.currentUser(ctx)
	if err != nil {
		return domain.User{}, err
	}
	return mapUser(user), nil
}
