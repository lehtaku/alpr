package credentials

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"

	"github.com/lehtaku/alpr/internal/domain"
)

// securityCommand is the macOS Keychain CLI; tests point it at a fake.
var securityCommand = "/usr/bin/security"

// itemNotFound is the exit code of `security find-generic-password` when no item matches.
const itemNotFound = 44

func readKeychain(service, account string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", domain.Errorf(domain.CodeConfigInvalid, "keychain: token references need macOS; use env:<NAME> instead")
	}

	cmd := exec.Command(securityCommand, "find-generic-password", "-s", service, "-a", account, "-w")
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr) && exitErr.ExitCode() == itemNotFound:
		return "", domain.Errorf(domain.CodeAuthMissing,
			"no token in the Keychain for keychain:%s/%s; store one with `alpr auth set <profile>`", service, account)
	case errors.As(err, &exitErr):
		return "", domain.Errorf(domain.CodeAuthMissing,
			"can't read keychain:%s/%s from the Keychain: %s", service, account, strings.TrimSpace(string(exitErr.Stderr)))
	case err != nil:
		return "", domain.Errorf(domain.CodeAuthMissing, "can't run %s: %v", securityCommand, err)
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", domain.Errorf(domain.CodeAuthMissing, "the Keychain item keychain:%s/%s is empty", service, account)
	}
	return token, nil
}
