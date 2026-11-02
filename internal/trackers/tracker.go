// Package trackers defines the ticket tracker interface and builds adapters by name.
package trackers

import (
	"context"

	"github.com/lehtaku/alpr/internal/domain"
)

// TicketTracker is a ticket tracker. It gains methods as commands need them.
type TicketTracker interface {
	CurrentUser(ctx context.Context) (domain.User, error)
}
