package config

import (
	"errors"
	"slices"
	"testing"

	"github.com/lehtaku/alpr/internal/domain"
)

const selectConfig = `
[profiles.work]
provider = "bitbucket-cloud"
match    = ["bitbucket.org/acme/*"]
token    = "env:A"

[profiles.alias]
provider = "bitbucket-cloud"
match    = ["bitbucket-work/acme/*"]
token    = "env:B"

[profiles.personal]
provider = "github"
match    = ["github.com/someone/*"]
token    = "env:C"

[profiles.shared]
provider = "github"
match    = ["github.com/someone/shared", "github.com/other/team-*"]
token    = "env:D"
`

func TestSelect(t *testing.T) {
	cfg, err := Load(writeConfig(t, selectConfig))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, profile, repo string
		want                string
		wantCode            domain.Code
	}{
		{"by name", "personal", "", "personal", ""},
		{"by name wins over match", "work", "github.com/someone/x", "work", ""},
		{"unknown name", "nope", "", "", domain.CodeProfileNotFound},
		{"match", "", "bitbucket.org/acme/api", "work", ""},
		{"match ignores case", "", "Bitbucket.org/ACME/Api", "work", ""},
		{"ssh host alias", "", "bitbucket-work/acme/api", "alias", ""},
		{"prefix pattern", "", "github.com/other/team-tools", "shared", ""},
		{"no match", "", "gitlab.com/x/y", "", domain.CodeProfileNotFound},
		{"star stays within a segment", "", "bitbucket.org/acme", "", domain.CodeProfileNotFound},
		{"ambiguous", "", "github.com/someone/shared", "", domain.CodeProfileAmbiguous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.Select(tt.profile, tt.repo)
			if tt.wantCode != "" {
				if code := errorCode(t, err); code != tt.wantCode {
					t.Errorf("code %s, want %s", code, tt.wantCode)
				}
				return
			}
			if err != nil || got.Name != tt.want {
				t.Errorf("got %v, %v; want %s", got, err, tt.want)
			}
		})
	}
}

func TestSelectAmbiguousListsProfiles(t *testing.T) {
	cfg, err := Load(writeConfig(t, selectConfig))
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.Select("", "github.com/someone/shared")
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || !slices.Equal(domainErr.Details["profiles"].([]string), []string{"personal", "shared"}) {
		t.Errorf("details: %v", err)
	}
}
