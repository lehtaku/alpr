// Package github is the GitHub adapter.
package github

import (
	"context"
	"net/http"

	"github.com/lehtaku/alpr/internal/transport"
)

// DefaultBaseURL is the GitHub REST API root.
const DefaultBaseURL = "https://api.github.com"

type client struct {
	http *transport.Client
}

// apiUser is a user object, as returned by GET /user.
type apiUser struct {
	ID    int64   `json:"id"`
	Login string  `json:"login"`
	Name  *string `json:"name"`
}

func newClient(baseURL, token string) *client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &client{http: &transport.Client{
		BaseURL:       baseURL,
		Authorization: transport.BearerAuth(token),
		Headers:       map[string]string{"Accept": "application/vnd.github+json"},
	}}
}

func (c *client) currentUser(ctx context.Context) (apiUser, error) {
	var user apiUser
	_, err := c.http.Do(ctx, http.MethodGet, "user", nil, &user)
	return user, err
}
