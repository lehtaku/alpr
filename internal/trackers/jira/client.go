// Package jira is the Jira Cloud adapter.
package jira

import (
	"context"
	"net/http"

	"github.com/lehtaku/alpr/internal/transport"
)

type client struct {
	http *transport.Client
}

// apiUser is a user object, as returned by GET /rest/api/3/myself.
type apiUser struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
}

// newClient talks to a Jira site; classic API tokens use Basic auth with the account email.
func newClient(siteURL, username, token string) *client {
	return &client{http: &transport.Client{BaseURL: siteURL, Authorization: transport.BasicAuth(username, token)}}
}

func (c *client) currentUser(ctx context.Context) (apiUser, error) {
	var user apiUser
	_, err := c.http.Do(ctx, http.MethodGet, "rest/api/3/myself", nil, &user)
	return user, err
}
