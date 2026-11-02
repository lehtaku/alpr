// Package credentials resolves token references from config.toml to tokens.
package credentials

import (
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/lehtaku/alpr/internal/domain"
)

// Reference is a parsed token reference: keychain:<service>/<account> or env:<NAME>.
type Reference struct {
	Service string // set for keychain references
	Account string // set for keychain references
	EnvName string // set for env references
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseReference parses a token reference. Errors never repeat the value, in case it's a token pasted in by mistake.
func ParseReference(raw string) (Reference, error) {
	if rest, ok := strings.CutPrefix(raw, "keychain:"); ok {
		service, account, found := strings.Cut(rest, "/")
		if found && service != "" && account != "" {
			return Reference{Service: service, Account: account}, nil
		}
	}
	if name, ok := strings.CutPrefix(raw, "env:"); ok && envName.MatchString(name) {
		return Reference{EnvName: name}, nil
	}
	return Reference{}, domain.Errorf(domain.CodeConfigInvalid,
		"token must be a reference, keychain:<service>/<account> or env:<NAME>, not the token itself")
}

// Resolve returns the token that the reference points to.
func Resolve(raw string) (string, error) {
	ref, err := ParseReference(raw)
	if err != nil {
		return "", err
	}
	if ref.EnvName == "" {
		return readKeychain(ref.Service, ref.Account)
	}
	token := os.Getenv(ref.EnvName)
	if token == "" {
		return "", domain.Errorf(domain.CodeAuthMissing, "environment variable %s is not set or empty", ref.EnvName)
	}
	return token, nil
}

// Store saves token in the Keychain item that a keychain: reference points to.
func Store(raw, token string) error {
	ref, err := ParseReference(raw)
	if err != nil {
		return err
	}
	if ref.EnvName != "" {
		return domain.Errorf(domain.CodeUsage, "this token comes from env:%s; set that environment variable instead", ref.EnvName)
	}
	if token == "" || strings.ContainsFunc(token, unicode.IsControl) {
		return domain.Errorf(domain.CodeUsage, "the token is empty or contains line breaks or control characters")
	}
	return writeKeychain(ref.Service, ref.Account, token)
}
