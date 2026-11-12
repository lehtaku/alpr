package github

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lehtaku/alpr/internal/domain"
)

// login and repoName keep users and repositories from adding qualifiers to search queries.
var (
	login    = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
	repoName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
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

// PullRequests returns at most limit open pull requests of owner/repo, most recently updated first. A reviewer
// ("@me" or a login) keeps those the user is asked to review, directly or through a team, or has reviewed.
func (p *Provider) PullRequests(ctx context.Context, owner, repo, reviewer string, limit int) ([]domain.PullRequestSummary, error) {
	return p.search(ctx, fmt.Sprintf("repo:%s/%s", owner, repo), reviewer, limit, nil)
}

// PullRequestsIn returns at most limit open pull requests across the owner's repositories whose names match
// repoPattern, most recently updated first. It runs one search over the owner's repositories and filters the results
// locally when repoPattern is a partial pattern such as "team-*", so it never reports per-repository errors.
func (p *Provider) PullRequestsIn(ctx context.Context, owner, repoPattern, reviewer string, limit int) ([]domain.PullRequestSummary, []domain.ErrorItem, error) {
	if !login.MatchString(owner) {
		return nil, nil, domain.Errorf(domain.CodeConfigInvalid, "%q is not a GitHub owner", owner)
	}
	var prs []domain.PullRequestSummary
	var err error
	switch {
	case !domain.IsPattern(repoPattern):
		if !repoName.MatchString(repoPattern) {
			return nil, nil, domain.Errorf(domain.CodeConfigInvalid, "%q is not a GitHub repository name", repoPattern)
		}
		prs, err = p.search(ctx, fmt.Sprintf("repo:%s/%s", owner, repoPattern), reviewer, limit, nil)
	case repoPattern == "*":
		prs, err = p.search(ctx, "user:"+owner, reviewer, limit, nil)
	default:
		prs, err = p.search(ctx, "user:"+owner, reviewer, limit, func(pr apiSearchPullRequest) bool {
			_, name, _ := strings.Cut(pr.Repository.NameWithOwner, "/")
			return domain.MatchName(repoPattern, name)
		})
	}
	return prs, []domain.ErrorItem{}, err
}

// search returns at most limit open pull requests within scope (a repo: or user: qualifier), most recently updated
// first, optionally only those a reviewer is involved in.
func (p *Provider) search(ctx context.Context, scope, reviewer string, limit int, keep func(apiSearchPullRequest) bool) ([]domain.PullRequestSummary, error) {
	query := scope + " is:pr is:open sort:updated-desc"
	if reviewer != "" {
		if reviewer != "@me" && !login.MatchString(reviewer) {
			return nil, domain.Errorf(domain.CodeUsage, "a GitHub user is @me or a login, not %q", reviewer)
		}
		query += " review-involves:" + reviewer
	}
	prs, err := p.client.searchPullRequests(ctx, query, limit, keep)
	if err != nil {
		return nil, err
	}
	summaries := make([]domain.PullRequestSummary, 0, len(prs))
	for _, pr := range prs {
		summaries = append(summaries, mapSearchPullRequest(pr))
	}
	return summaries, nil
}
