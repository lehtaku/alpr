// Package bitbucketcloud is the Bitbucket Cloud adapter.
package bitbucketcloud

import (
	"context"
	"net/http"

	"github.com/lehtaku/alpr/internal/transport"
)

// DefaultBaseURL is the Bitbucket Cloud REST API root.
const DefaultBaseURL = "https://api.bitbucket.org/2.0"

type client struct {
	http *transport.Client
}

// apiUser is an account object, as returned by GET /user.
type apiUser struct {
	UUID        string `json:"uuid"`
	Nickname    string `json:"nickname"`
	DisplayName string `json:"display_name"`
}

func newClient(baseURL, token string) *client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	// API tokens work as bearer tokens, so no account email is needed.
	return &client{http: &transport.Client{BaseURL: baseURL, Authorization: transport.BearerAuth(token)}}
}

func (c *client) currentUser(ctx context.Context) (apiUser, error) {
	var user apiUser
	_, err := c.http.Do(ctx, http.MethodGet, "user", nil, &user)
	return user, err
}
