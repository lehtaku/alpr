package domain_test

import (
	"testing"
	"time"

	"github.com/lehtaku/alpr/internal/domain"
)

func TestFormatTime(t *testing.T) {
	in := time.Date(2026, 10, 31, 11, 15, 0, 999_000_000, time.FixedZone("EET", 2*60*60))
	if got := domain.FormatTime(in); got != "2026-10-31T09:15:00Z" {
		t.Errorf("got %s", got)
	}
}
