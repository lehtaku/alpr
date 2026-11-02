// Package services holds the use cases that commands call.
package services

import (
	"context"
	"fmt"

	"github.com/lehtaku/alpr/internal/config"
	"github.com/lehtaku/alpr/internal/credentials"
	"github.com/lehtaku/alpr/internal/domain"
	"github.com/lehtaku/alpr/internal/providers"
	"github.com/lehtaku/alpr/internal/trackers"
)

// Check is the result of asking a platform who a token belongs to.
type Check struct {
	Type  string            `json:"type"`
	OK    bool              `json:"ok"`
	User  *domain.User      `json:"user"`
	Error *domain.ErrorItem `json:"error"`
}

// ProfileCheck holds the token checks of one profile; Tracker is nil when the profile has none.
type ProfileCheck struct {
	Profile  string `json:"profile"`
	Provider Check  `json:"provider"`
	Tracker  *Check `json:"tracker"`
}

// TokenStored describes a token saved by SetToken.
type TokenStored struct {
	Profile  string `json:"profile"`
	Target   string `json:"target"` // "provider" or "tracker"
	TokenRef string `json:"token_ref"`
}

type userLookup interface {
	CurrentUser(ctx context.Context) (domain.User, error)
}

// CheckProfiles asks each profile's provider and tracker who its token belongs to.
func CheckProfiles(ctx context.Context, profiles []*config.Profile) []ProfileCheck {
	checks := make([]ProfileCheck, 0, len(profiles))
	for _, profile := range profiles {
		token, tokenErr := credentials.Resolve(profile.Token)
		check := ProfileCheck{Profile: profile.Name, Provider: checkUser(ctx, profile.Name, profile.Provider, tokenErr,
			func() (userLookup, error) { return providers.New(profile.Provider, profile.BaseURL, token) })}

		if tracker := profile.Tracker; tracker != nil {
			var trackerCheck Check
			if tracker.Type == config.TrackerGitHubIssues {
				trackerCheck = checkUser(ctx, profile.Name, tracker.Type, tokenErr, func() (userLookup, error) {
					return trackers.New(tracker.Type, profile.BaseURL, "", token)
				})
			} else {
				trackerToken, err := credentials.Resolve(tracker.Token)
				trackerCheck = checkUser(ctx, profile.Name, tracker.Type, err, func() (userLookup, error) {
					return trackers.New(tracker.Type, tracker.BaseURL, tracker.Username, trackerToken)
				})
			}
			check.Tracker = &trackerCheck
		}
		checks = append(checks, check)
	}
	return checks
}

func checkUser(ctx context.Context, scope, kind string, tokenErr error, adapter func() (userLookup, error)) Check {
	fail := func(err error) Check {
		item := domain.NewErrorItem(scope, err)
		return Check{Type: kind, Error: &item}
	}
	if tokenErr != nil {
		return fail(tokenErr)
	}
	lookup, err := adapter()
	if err != nil {
		return fail(err)
	}
	user, err := lookup.CurrentUser(ctx)
	if err != nil {
		return fail(err)
	}
	return Check{Type: kind, OK: true, User: &user}
}

// SetToken stores the token of a profile's provider, or with tracker its tracker, in the Keychain item the config
// names. read gets a label for the prompt and is called only once the profile and its reference check out.
func SetToken(cfg *config.Config, name string, tracker bool, read func(label string) (string, error)) (TokenStored, error) {
	profile, err := cfg.Select(name, "")
	if err != nil {
		return TokenStored{}, err
	}

	stored := TokenStored{Profile: profile.Name, Target: "provider", TokenRef: profile.Token}
	label := fmt.Sprintf("%s (%s)", profile.Name, profile.Provider)
	if tracker {
		if profile.Tracker == nil || profile.Tracker.Token == "" {
			return TokenStored{}, domain.Errorf(domain.CodeUsage,
				"profile %q has no tracker with its own token; a github-issues tracker uses the profile's token", profile.Name)
		}
		stored.Target, stored.TokenRef = "tracker", profile.Tracker.Token
		label = fmt.Sprintf("%s (%s tracker)", profile.Name, profile.Tracker.Type)
	}

	ref, err := credentials.ParseReference(stored.TokenRef)
	if err != nil {
		return TokenStored{}, err
	}
	if ref.EnvName != "" {
		return TokenStored{}, domain.Errorf(domain.CodeUsage,
			"the %s token of profile %q comes from env:%s; set that environment variable instead", stored.Target, profile.Name, ref.EnvName)
	}

	token, err := read(label)
	if err != nil {
		return TokenStored{}, err
	}
	if err := credentials.Store(stored.TokenRef, token); err != nil {
		return TokenStored{}, err
	}
	return stored, nil
}
