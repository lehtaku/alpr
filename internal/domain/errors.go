// Package domain holds platform-neutral models, errors and rules. It imports nothing from the project.
package domain

import "fmt"

// Code is a stable error code from the spec (§6).
type Code string

const (
	CodeInternal         Code = "internal"
	CodeUsage            Code = "usage"
	CodeConfigNotFound   Code = "config_not_found"
	CodeConfigInvalid    Code = "config_invalid"
	CodeProfileNotFound  Code = "profile_not_found"
	CodeProfileAmbiguous Code = "profile_ambiguous"
	CodeRepoNotDetected  Code = "repo_not_detected"
	CodeAuthMissing      Code = "auth_missing"
	CodeAuthInvalid      Code = "auth_invalid"
	CodePermissionDenied Code = "permission_denied"
	CodeNotFound         Code = "not_found"
	CodeLineNotInDiff    Code = "line_not_in_diff"
	CodeFileNotInDiff    Code = "file_not_in_diff"
	CodeUnsupported      Code = "unsupported"
	CodeAPIError         Code = "api_error"
	CodeNetworkError     Code = "network_error"
	CodeRateLimited      Code = "rate_limited"
)

var exitCodes = map[Code]int{
	CodeInternal:         1,
	CodeUsage:            2,
	CodeConfigNotFound:   2,
	CodeConfigInvalid:    2,
	CodeProfileNotFound:  2,
	CodeProfileAmbiguous: 2,
	CodeRepoNotDetected:  2,
	CodeAuthMissing:      3,
	CodeAuthInvalid:      3,
	CodePermissionDenied: 3,
	CodeNotFound:         4,
	CodeLineNotInDiff:    5,
	CodeFileNotInDiff:    5,
	CodeUnsupported:      6,
	CodeAPIError:         7,
	CodeNetworkError:     7,
	CodeRateLimited:      7,
}

// ExitCode returns the process exit code for c; unknown codes count as internal errors.
func (c Code) ExitCode() int {
	if n, ok := exitCodes[c]; ok {
		return n
	}
	return 1
}

// Error is an error with a stable code, reported to users and scripts.
type Error struct {
	Code    Code
	Message string
	Details map[string]any // extra fields for --json output; nil means none
}

// Errorf returns an Error with a formatted message.
func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func (e *Error) Error() string {
	return e.Message
}
