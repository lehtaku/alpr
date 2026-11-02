package credentials

import (
	"errors"
	"fmt"
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

// writeKeychain stores token as the generic password for service and account, replacing any earlier one.
// The command goes to `security -i` on stdin, so the token never appears in process arguments.
func writeKeychain(service, account, token string) error {
	if runtime.GOOS != "darwin" {
		return domain.Errorf(domain.CodeUsage, "keychain: token references need macOS; use env:<NAME> instead")
	}

	cmd := exec.Command(securityCommand, "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("add-generic-password -U -s %s -a %s -w %s\n",
		quote(service), quote(account), quote(token)))
	// security's output isn't shown: in interactive mode it could echo the command, token included.
	if err := cmd.Run(); err != nil {
		return domain.Errorf(domain.CodeInternal, "can't write keychain:%s/%s: %v", service, account, err)
	}

	// Interactive mode exits 0 even when a command fails, so read the item back to confirm.
	stored, err := readKeychain(service, account)
	if err != nil || stored != token {
		return domain.Errorf(domain.CodeInternal, "the token wasn't stored in keychain:%s/%s", service, account)
	}
	return nil
}

// quote makes s one argument for `security -i`, which understands double quotes with backslash escapes.
func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
