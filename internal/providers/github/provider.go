package github

import (
	"context"
	"strconv"

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

// PullRequest returns pull request id of owner/repo with its reviewers and comment threads.
func (p *Provider) PullRequest(ctx context.Context, owner, repo, id string) (domain.PullRequest, error) {
	number, err := strconv.Atoi(id)
	if err != nil {
		return domain.PullRequest{}, domain.Errorf(domain.CodeUsage, "%q is not a pull request number", id)
	}
	pr, err := p.client.pullRequest(ctx, owner, repo, number)
	if err != nil {
		return domain.PullRequest{}, err
	}
	reviews, err := p.client.reviews(ctx, owner, repo, number)
	if err != nil {
		return domain.PullRequest{}, err
	}
	comments, err := p.client.issueComments(ctx, owner, repo, number)
	if err != nil {
		return domain.PullRequest{}, err
	}
	threads, err := p.client.reviewThreads(ctx, owner, repo, number)
	if err != nil {
		return domain.PullRequest{}, err
	}
	return mapPullRequest(pr, reviews, comments, threads), nil
}
