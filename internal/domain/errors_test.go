package domain_test

import (
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
)

func TestCodeExitCode(t *testing.T) {
	tests := map[domain.Code]int{
		domain.CodeInternal:         1,
		domain.CodeUsage:            2,
		domain.CodeConfigNotFound:   2,
		domain.CodeConfigInvalid:    2,
		domain.CodeProfileNotFound:  2,
		domain.CodeProfileAmbiguous: 2,
		domain.CodeRepoNotDetected:  2,
		domain.CodeAuthMissing:      3,
		domain.CodeAuthInvalid:      3,
		domain.CodePermissionDenied: 3,
		domain.CodeNotFound:         4,
		domain.CodeLineNotInDiff:    5,
		domain.CodeFileNotInDiff:    5,
		domain.CodeUnsupported:      6,
		domain.CodeAPIError:         7,
		domain.CodeNetworkError:     7,
		domain.CodeRateLimited:      7,
		domain.Code("unknown_code"): 1,
	}
	for code, want := range tests {
		if got := code.ExitCode(); got != want {
			t.Errorf("%s: exit code %d, want %d", code, got, want)
		}
	}
}

func TestErrorf(t *testing.T) {
	err := domain.Errorf(domain.CodeNotFound, "pull request %d not found", 42)
	if err.Error() != "pull request 42 not found" || err.Code != domain.CodeNotFound {
		t.Errorf("got %q (%s)", err.Error(), err.Code)
	}
}
