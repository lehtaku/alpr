// Package cli defines the alpr commands: parse arguments, call one service, render the result.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"slices"

	"github.com/spf13/cobra"

	"github.com/lehtaku/alpr/internal/domain"
)

type globalFlags struct {
	json    bool
	profile string
	repo    string
}

// exitStatus ends a command with an exit code when its output already describes the failure.
type exitStatus int

func (s exitStatus) Error() string {
	return fmt.Sprintf("exit status %d", int(s))
}

// Execute runs alpr with the process arguments and returns the exit code.
func Execute() int {
	return run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var flags globalFlags
	root := newRootCommand(&flags)
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	if err == nil {
		return 0
	}
	// Flag parsing can fail before --json is read, so the raw arguments count too.
	return reportError(stderr, err, flags.json || slices.Contains(args, "--json"))
}

func newRootCommand(flags *globalFlags) *cobra.Command {
	root := &cobra.Command{
		Use:           "alpr",
		Short:         "Routine code-review work across Git hosts and ticket trackers",
		Version:       version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	persistent := root.PersistentFlags()
	persistent.BoolVar(&flags.json, "json", false, "machine-readable JSON output")
	persistent.StringVar(&flags.profile, "profile", "", "use this profile instead of matching the repository")
	persistent.StringVar(&flags.repo, "repo", "", "act on `host/owner/repo` instead of the current directory's origin remote")

	root.AddCommand(newAuthCommand(flags), newPRCommand(flags))
	return root
}

// reportError writes err to stderr and returns its exit code.
func reportError(stderr io.Writer, err error, asJSON bool) int {
	var status exitStatus
	if errors.As(err, &status) {
		return int(status)
	}
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) {
		// Commands return *domain.Error, so any other error comes from cobra's argument parsing.
		domainErr = &domain.Error{Code: domain.CodeUsage, Message: err.Error()}
	}
	writeError(stderr, domainErr, asJSON)
	return domainErr.Code.ExitCode()
}

// version is the module version set by `go install …@<version>`; local builds report "(devel)".
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "(devel)"
}
