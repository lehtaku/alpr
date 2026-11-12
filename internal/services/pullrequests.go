package services

import (
	"context"
	"slices"
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

// ListAllPullRequests returns at most limit open pull requests, most recently updated first, across every repository
// the profiles' match patterns cover, optionally only those a reviewer is involved in. A failing profile, pattern or
// repository is listed in Errors and doesn't stop the others.
func ListAllPullRequests(ctx context.Context, profiles []*config.Profile, reviewer string, limit int) (PullRequestList, error) {
	if limit < 1 {
		return PullRequestList{}, domain.Errorf(domain.CodeUsage, "--limit must be at least 1")
	}
	list := PullRequestList{PullRequests: []domain.PullRequestSummary{}, Errors: []domain.ErrorItem{}}
	seen := map[string]bool{}
	for _, profile := range profiles {
		token, err := credentials.Resolve(profile.Token)
		if err != nil {
			list.Errors = append(list.Errors, domain.NewErrorItem(profile.Name, err))
			continue
		}
		provider, err := providers.New(profile.Provider, profile.BaseURL, token)
		if err != nil {
			list.Errors = append(list.Errors, domain.NewErrorItem(profile.Name, err))
			continue
		}
		for _, pattern := range profile.Match {
			segments := strings.Split(pattern, "/")
			if domain.IsPattern(segments[1]) {
				list.Errors = append(list.Errors, domain.NewErrorItem(pattern,
					domain.Errorf(domain.CodeUsage, "--all-repos can't list repositories of an owner pattern: %s", pattern)))
				continue
			}
			prs, items, err := provider.PullRequestsIn(ctx, segments[1], segments[2], reviewer, limit)
			if err != nil {
				list.Errors = append(list.Errors, domain.NewErrorItem(pattern, err))
				continue
			}
			list.Errors = append(list.Errors, items...)
			for _, pr := range prs {
				if key := pr.Repo + "#" + pr.ID; !seen[key] {
					seen[key] = true
					list.PullRequests = append(list.PullRequests, pr)
				}
			}
		}
	}
	slices.SortStableFunc(list.PullRequests, func(a, b domain.PullRequestSummary) int {
		return strings.Compare(b.UpdatedAt, a.UpdatedAt)
	})
	list.PullRequests = list.PullRequests[:min(len(list.PullRequests), limit)]
	return list, nil
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
