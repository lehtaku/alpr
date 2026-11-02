package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
)

type errorOutput struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func runArgs(args ...string) (code int, stdout, stderr string) {
	return runWithInput("", args...)
}

func runWithInput(stdin string, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func decodeError(t *testing.T, s string) errorOutput {
	t.Helper()
	var got errorOutput
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, s)
	}
	return got
}

func TestRunVersion(t *testing.T) {
	code, stdout, stderr := runArgs("--version")
	if code != 0 || !strings.HasPrefix(stdout, "alpr version ") || stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRunAcceptsGlobalFlags(t *testing.T) {
	code, _, stderr := runArgs("--json", "--profile", "work", "--repo", "github.com/owner/repo")
	if code != 0 || stderr != "" {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestRunUsageErrorText(t *testing.T) {
	code, stdout, stderr := runArgs("--bogus")
	if code != 2 || stdout != "" || stderr != "alpr: unknown flag: --bogus\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRunUsageErrorJSON(t *testing.T) {
	code, stdout, stderr := runArgs("--json", "--bogus")
	if code != 2 || stdout != "" {
		t.Fatalf("exit %d, stdout %q", code, stdout)
	}
	got := decodeError(t, stderr)
	if got.Error.Code != "usage" || got.Error.Message != "unknown flag: --bogus" || got.Error.Details == nil {
		t.Errorf("unexpected error output: %s", stderr)
	}
}

func TestReportErrorDomainError(t *testing.T) {
	err := domain.Errorf(domain.CodeLineNotInDiff, "src/app.go:120 (new side) is not part of the diff")
	err.Details = map[string]any{"path": "src/app.go", "line": 120}

	var stderr bytes.Buffer
	if code := reportError(&stderr, err, true); code != 5 {
		t.Errorf("exit %d, want 5", code)
	}
	got := decodeError(t, stderr.String())
	if got.Error.Code != "line_not_in_diff" || got.Error.Details["path"] != "src/app.go" {
		t.Errorf("unexpected error output: %s", stderr.String())
	}
}

func TestReportErrorWrappedDomainError(t *testing.T) {
	wrapped := errors.Join(domain.Errorf(domain.CodeAuthInvalid, "token rejected"))

	var stderr bytes.Buffer
	if code := reportError(&stderr, wrapped, false); code != 3 || stderr.String() != "alpr: token rejected\n" {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}
