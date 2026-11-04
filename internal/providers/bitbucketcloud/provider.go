package bitbucketcloud

import (
	"context"

	"github.com/lehtaku/alpr/internal/domain"
)

// Provider implements providers.PrProvider for Bitbucket Cloud.
type Provider struct {
	client *client
}

// New returns a Bitbucket Cloud provider; an empty baseURL means DefaultBaseURL.
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

// PullRequest returns pull request id of workspace/slug with its reviewers and comment threads.
func (p *Provider) PullRequest(ctx context.Context, workspace, slug, id string) (domain.PullRequest, error) {
	pr, err := p.client.pullRequest(ctx, workspace, slug, id)
	if err != nil {
		return domain.PullRequest{}, err
	}
	comments, err := p.client.comments(ctx, workspace, slug, id)
	if err != nil {
		return domain.PullRequest{}, err
	}
	return mapPullRequest(pr, comments), nil
}
