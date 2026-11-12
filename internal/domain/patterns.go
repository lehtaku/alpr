package domain

import (
	"path"
	"strings"
)

// IsPattern reports whether a repository name holds path.Match wildcards.
func IsPattern(name string) bool {
	return strings.ContainsAny(name, `*?[\`)
}

// MatchName reports whether name matches the path.Match pattern, ignoring case.
func MatchName(pattern, name string) bool {
	ok, _ := path.Match(strings.ToLower(pattern), strings.ToLower(name))
	return ok
}
