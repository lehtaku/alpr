package services

import (
	"context"
	"strconv"
	"strings"

	"github.com/lehtaku/alpr/internal/config"
	"github.com/lehtaku/alpr/internal/credentials"
	"github.com/lehtaku/alpr/internal/domain"
	"github.com/lehtaku/alpr/internal/providers"
)

// ShowPullRequest returns pull request id of repo (host/owner/repo), using the named profile or the one matching repo.
func ShowPullRequest(ctx context.Context, cfg *config.Config, profileName, repo, id string) (domain.PullRequest, error) {
	number, err := strconv.Atoi(id)
	if err != nil || number < 1 {
		return domain.PullRequest{}, domain.Errorf(domain.CodeUsage, "%q is not a pull request number", id)
	}
	provider, err := providerFor(cfg, profileName, repo)
	if err != nil {
		return domain.PullRequest{}, err
	}
	segments := strings.Split(repo, "/")
	return provider.PullRequest(ctx, segments[1], segments[2], strconv.Itoa(number))
}

// PullRequestList is the result of ListPullRequests; Errors lists failures that didn't stop the listing.
type PullRequestList struct {
	PullRequests []domain.PullRequestSummary `json:"pull_requests"`
	Errors       []domain.ErrorItem          `json:"errors"`
}

// ListPullRequests returns at most limit open pull requests of repo (host/owner/repo), most recently updated first,
// optionally only those a reviewer is involved in.
func ListPullRequests(ctx context.Context, cfg *config.Config, profileName, repo, reviewer string, limit int) (PullRequestList, error) {
	if limit < 1 {
		return PullRequestList{}, domain.Errorf(domain.CodeUsage, "--limit must be at least 1")
	}
	provider, err := providerFor(cfg, profileName, repo)
	if err != nil {
		return PullRequestList{}, err
	}
	segments := strings.Split(repo, "/")
	prs, err := provider.PullRequests(ctx, segments[1], segments[2], reviewer, limit)
	if err != nil {
		return PullRequestList{}, err
	}
	return PullRequestList{PullRequests: prs, Errors: []domain.ErrorItem{}}, nil
}

func providerFor(cfg *config.Config, profileName, repo string) (providers.PrProvider, error) {
	profile, err := cfg.Select(profileName, repo)
	if err != nil {
		return nil, err
	}
	token, err := credentials.Resolve(profile.Token)
	if err != nil {
		return nil, err
	}
	return providers.New(profile.Provider, profile.BaseURL, token)
}
