package providers

import (
	"github.com/lehtaku/alpr/internal/config"
	"github.com/lehtaku/alpr/internal/domain"
	"github.com/lehtaku/alpr/internal/providers/bitbucketcloud"
	"github.com/lehtaku/alpr/internal/providers/github"
)

// New returns the adapter for a provider name from config.toml; an empty baseURL means the platform default.
func New(name, baseURL, token string) (PrProvider, error) {
	switch name {
	case config.ProviderBitbucketCloud:
		return bitbucketcloud.New(baseURL, token), nil
	case config.ProviderGitHub:
		return github.New(baseURL, token), nil
	}
	return nil, domain.Errorf(domain.CodeConfigInvalid, "unknown provider %q", name)
}
