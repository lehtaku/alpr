// Package providers defines the Git hosting interface and builds adapters by name.
package providers

import (
	"context"

	"github.com/lehtaku/alpr/internal/domain"
)

// PrProvider is a Git hosting platform. It gains methods as commands need them.
type PrProvider interface {
	CurrentUser(ctx context.Context) (domain.User, error)
	// PullRequest returns pull request id of owner/repo with its reviewers and comment threads.
	PullRequest(ctx context.Context, owner, repo, id string) (domain.PullRequest, error)
	// PullRequests returns at most limit open pull requests of owner/repo, most recently updated first. A non-empty
	// reviewer ("@me" or a platform user ID) keeps those the user is asked to review or has reviewed.
	PullRequests(ctx context.Context, owner, repo, reviewer string, limit int) ([]domain.PullRequestSummary, error)
}
