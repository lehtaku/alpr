package domain

import "testing"

func TestIsPattern(t *testing.T) {
	for name, want := range map[string]bool{"repo": false, "my.repo_2": false, "*": true, "team-*": true, "re?o": true, "[ab]c": true} {
		if got := IsPattern(name); got != want {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}
}

func TestMatchName(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"*", "anything", true},
		{"team-*", "Team-API", true},
		{"team-*", "other", false},
		{"Repo", "repo", true},
		{"[", "x", false},
	}
	for _, tt := range tests {
		if got := MatchName(tt.pattern, tt.name); got != tt.want {
			t.Errorf("%s ~ %s: %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}
