package bitbucketcloud

import (
	"context"
	"regexp"

	"github.com/lehtaku/alpr/internal/domain"
)

var accountUUID = regexp.MustCompile(`^\{[0-9A-Fa-f-]+\}$`)

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

// PullRequests returns at most limit open pull requests of workspace/slug, most recently updated first. A reviewer
// ("@me" or an account UUID) keeps those where the user is a requested reviewer; Bitbucket can't filter on
// participants, so pull requests the user reviewed without being requested are left out.
func (p *Provider) PullRequests(ctx context.Context, workspace, slug, reviewer string, limit int) ([]domain.PullRequestSummary, error) {
	reviewerUUID := ""
	if reviewer != "" {
		uuid, err := p.accountUUID(ctx, reviewer)
		if err != nil {
			return nil, err
		}
		reviewerUUID = uuid
	}
	prs, err := p.client.openPullRequests(ctx, workspace, slug, reviewerUUID, limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]domain.PullRequestSummary, 0, len(prs))
	for _, pr := range prs {
		summaries = append(summaries, mapSummary(pr))
	}
	return summaries, nil
}

// accountUUID resolves "@me" to the token's account UUID and checks that anything else is an account UUID.
func (p *Provider) accountUUID(ctx context.Context, user string) (string, error) {
	if user == "@me" {
		me, err := p.client.currentUser(ctx)
		return me.UUID, err
	}
	if !accountUUID.MatchString(user) {
		return "", domain.Errorf(domain.CodeUsage, "a Bitbucket user is @me or an account UUID such as {0a1b2c3d-…}, not %q", user)
	}
	return user, nil
}
