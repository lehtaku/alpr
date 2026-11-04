package github

import (
	"context"

	"github.com/lehtaku/alpr/internal/domain"
)

// Provider implements providers.PrProvider for GitHub and GitHub Enterprise Server.
type Provider struct {
	client *client
}

// New returns a GitHub provider; an empty baseURL means DefaultBaseURL.
func New(baseURL, token string) *Provider {
	return &Provider{client: newClient(baseURL, token)}
}

// CurrentUser returns the user the token belongs to.
func (p *Provider) CurrentUser(ctx context.Context) (domain.User, error) {
	user, err := p.client.currentUser(ctx)
	if err != nil {
		return domain.User{}, err
	}
	return mapUser(user), nil
}

// PullRequest isn't implemented for GitHub yet.
func (p *Provider) PullRequest(context.Context, string, string, string) (domain.PullRequest, error) {
	return domain.PullRequest{}, &domain.Error{
		Code:    domain.CodeUnsupported,
		Message: "pr show doesn't support GitHub yet",
		Details: map[string]any{"capability": "pr_show", "platform": "github"},
	}
}
