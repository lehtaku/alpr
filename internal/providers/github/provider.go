package github

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/lehtaku/alpr/internal/domain"
)

// login keeps a reviewer from adding qualifiers to the search query.
var login = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

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

// PullRequests returns at most limit open pull requests of owner/repo, most recently updated first. A reviewer
// ("@me" or a login) keeps those the user is asked to review, directly or through a team, or has reviewed.
func (p *Provider) PullRequests(ctx context.Context, owner, repo, reviewer string, limit int) ([]domain.PullRequestSummary, error) {
	query := fmt.Sprintf("repo:%s/%s is:pr is:open sort:updated-desc", owner, repo)
	if reviewer != "" {
		if reviewer != "@me" && !login.MatchString(reviewer) {
			return nil, domain.Errorf(domain.CodeUsage, "a GitHub user is @me or a login, not %q", reviewer)
		}
		query += " review-involves:" + reviewer
	}
	prs, err := p.client.searchPullRequests(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]domain.PullRequestSummary, 0, len(prs))
	for _, pr := range prs {
		summaries = append(summaries, mapSearchPullRequest(pr))
	}
	return summaries, nil
}
