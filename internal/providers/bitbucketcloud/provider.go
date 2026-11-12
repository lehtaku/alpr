package bitbucketcloud

import (
	"context"
	"regexp"
	"slices"
	"sync"

	"github.com/lehtaku/alpr/internal/domain"
)

var accountUUID = regexp.MustCompile(`^\{[0-9A-Fa-f-]+\}$`)

// maxParallel caps the repository listings PullRequestsIn runs at once.
const maxParallel = 8

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
	reviewerUUID, err := p.reviewerUUID(ctx, reviewer)
	if err != nil {
		return nil, err
	}
	return p.openPullRequests(ctx, workspace, slug, reviewerUUID, limit)
}

// PullRequestsIn returns open pull requests, at most limit per repository, across the workspace's repositories whose
// slugs match repoPattern. It lists the workspace's repositories first, unless repoPattern names a single repository,
// and then queries maxParallel repositories at a time.
func (p *Provider) PullRequestsIn(ctx context.Context, workspace, repoPattern, reviewer string, limit int) ([]domain.PullRequestSummary, []domain.ErrorItem, error) {
	reviewerUUID, err := p.reviewerUUID(ctx, reviewer)
	if err != nil {
		return nil, nil, err
	}
	slugs := []string{repoPattern}
	if domain.IsPattern(repoPattern) {
		all, err := p.client.repositorySlugs(ctx, workspace)
		if err != nil {
			return nil, nil, err
		}
		slugs = slices.DeleteFunc(all, func(slug string) bool { return !domain.MatchName(repoPattern, slug) })
	}

	results := make([][]domain.PullRequestSummary, len(slugs))
	errs := make([]error, len(slugs))
	running := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i, slug := range slugs {
		wg.Go(func() {
			running <- struct{}{}
			defer func() { <-running }()
			results[i], errs[i] = p.openPullRequests(ctx, workspace, slug, reviewerUUID, limit)
		})
	}
	wg.Wait()

	summaries := []domain.PullRequestSummary{}
	items := []domain.ErrorItem{}
	for i, slug := range slugs {
		if errs[i] != nil {
			items = append(items, domain.NewErrorItem(host+"/"+workspace+"/"+slug, errs[i]))
		}
		summaries = append(summaries, results[i]...)
	}
	return summaries, items, nil
}

func (p *Provider) openPullRequests(ctx context.Context, workspace, slug, reviewerUUID string, limit int) ([]domain.PullRequestSummary, error) {
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

// reviewerUUID resolves "@me" to the token's account UUID and checks that anything else is an account UUID; no
// reviewer gives "".
func (p *Provider) reviewerUUID(ctx context.Context, user string) (string, error) {
	if user == "" {
		return "", nil
	}
	if user == "@me" {
		me, err := p.client.currentUser(ctx)
		return me.UUID, err
	}
	if !accountUUID.MatchString(user) {
		return "", domain.Errorf(domain.CodeUsage, "a Bitbucket user is @me or an account UUID such as {0a1b2c3d-…}, not %q", user)
	}
	return user, nil
}
