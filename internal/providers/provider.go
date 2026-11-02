// Package providers defines the Git hosting interface and builds adapters by name.
package providers

import (
	"context"

	"github.com/lehtaku/alpr/internal/domain"
)

// PrProvider is a Git hosting platform. It gains methods as commands need them.
type PrProvider interface {
	CurrentUser(ctx context.Context) (domain.User, error)
}
