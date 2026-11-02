// Package cli defines the alpr commands: parse arguments, call one service, render the result.
package cli

import (
	"errors"
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

// Execute runs alpr with the process arguments and returns the exit code.
func Execute() int {
	return run(os.Args[1:], os.Stdout, os.Stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	var flags globalFlags
	root := newRootCommand(&flags)
	root.SetArgs(args)
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
	return root
}

// reportError writes err to stderr and returns its exit code.
func reportError(stderr io.Writer, err error, asJSON bool) int {
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
