package config

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/lehtaku/alpr/internal/credentials"
	"github.com/lehtaku/alpr/internal/domain"
)

const (
	ProviderBitbucketCloud = "bitbucket-cloud"
	ProviderGitHub         = "github"
	TrackerJira            = "jira"
	TrackerGitHubIssues    = "github-issues"
)

// Config is a parsed and validated config.toml.
type Config struct {
	Profiles map[string]*Profile `toml:"profiles"`
}

// Profile ties a set of repositories to a provider, its token and an optional tracker.
type Profile struct {
	Name     string   `toml:"-"`
	Provider string   `toml:"provider"`
	Match    []string `toml:"match"`
	Token    string   `toml:"token"`
	BaseURL  string   `toml:"base_url"`
	Tracker  *Tracker `toml:"tracker"`
}

// Tracker is a profile's ticket tracker.
type Tracker struct {
	Type     string `toml:"type"`
	BaseURL  string `toml:"base_url"`
	Username string `toml:"username"`
	Token    string `toml:"token"`
}

var profileName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Load reads and validates the config file.
func Load(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &domain.Error{
			Code:    domain.CodeConfigNotFound,
			Message: "config file not found: " + file,
			Details: map[string]any{"path": file},
		}
	}
	if err != nil {
		return nil, invalid(file, "%v", err)
	}
	return parse(file, data)
}

func parse(file string, data []byte) (*Config, error) {
	var cfg Config
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return nil, invalid(file, "%v", err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		return nil, invalid(file, "unknown keys: %s", strings.Join(keys, ", "))
	}
	if len(cfg.Profiles) == 0 {
		return nil, invalid(file, "no profiles defined")
	}

	for _, name := range cfg.names() {
		profile := cfg.Profiles[name]
		profile.Name = name
		if err := profile.validate(); err != nil {
			return nil, invalid(file, "profile %q: %v", name, err)
		}
	}
	return &cfg, nil
}

func (c *Config) names() []string {
	return slices.Sorted(maps.Keys(c.Profiles))
}

func (p *Profile) validate() error {
	if !profileName.MatchString(p.Name) {
		return errors.New("the name may only contain letters, digits, - and _")
	}
	if p.Provider != ProviderBitbucketCloud && p.Provider != ProviderGitHub {
		return fmt.Errorf("provider must be %q or %q", ProviderBitbucketCloud, ProviderGitHub)
	}
	if len(p.Match) == 0 {
		return errors.New("match must list at least one pattern")
	}
	for _, pattern := range p.Match {
		if err := validatePattern(pattern); err != nil {
			return err
		}
	}
	if p.Token == "" {
		return errors.New("token is required")
	}
	if _, err := credentials.ParseReference(p.Token); err != nil {
		return err
	}
	if p.BaseURL != "" {
		if err := validateURL("base_url", p.BaseURL); err != nil {
			return err
		}
	}
	if p.Tracker == nil {
		return nil
	}
	if p.Tracker.Type == TrackerGitHubIssues && p.Provider != ProviderGitHub {
		return fmt.Errorf("a %q tracker needs provider %q", TrackerGitHubIssues, ProviderGitHub)
	}
	return p.Tracker.validate()
}

func (t *Tracker) validate() error {
	switch t.Type {
	case TrackerJira:
		if t.BaseURL == "" || t.Username == "" || t.Token == "" {
			return errors.New("a jira tracker needs base_url, username and token")
		}
		if _, err := credentials.ParseReference(t.Token); err != nil {
			return fmt.Errorf("tracker %v", err)
		}
		return validateURL("tracker.base_url", t.BaseURL)
	case TrackerGitHubIssues:
		if t.BaseURL != "" || t.Username != "" || t.Token != "" {
			return errors.New("a github-issues tracker takes no base_url, username or token; it uses the profile's token")
		}
		return nil
	default:
		return fmt.Errorf("tracker.type must be %q or %q", TrackerJira, TrackerGitHubIssues)
	}
}

func validatePattern(pattern string) error {
	segments := strings.Split(pattern, "/")
	if len(segments) != 3 || slices.Contains(segments, "") {
		return fmt.Errorf("match pattern %q must have the form host/owner/repo", pattern)
	}
	if _, err := path.Match(pattern, pattern); err != nil {
		return fmt.Errorf("match pattern %q: %v", pattern, err)
	}
	return nil
}

func validateURL(key, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("%s must be an http(s) URL, got %q", key, raw)
	}
	return nil
}

func invalid(file, format string, args ...any) *domain.Error {
	return &domain.Error{
		Code:    domain.CodeConfigInvalid,
		Message: file + ": " + fmt.Sprintf(format, args...),
		Details: map[string]any{"path": file},
	}
}
