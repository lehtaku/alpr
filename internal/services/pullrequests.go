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
