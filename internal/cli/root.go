// Package cli defines the alpr commands.
package cli

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Execute runs the root command and returns the process exit code.
func Execute() int {
	root := &cobra.Command{
		Use:           "alpr",
		Short:         "Routine code-review work across Git hosts and ticket trackers",
		Version:       version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "alpr:", err)
		return 2 // only usage errors exist so far
	}
	return 0
}

// version is the module version set by `go install …@<version>`; local builds report "(devel)".
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "(devel)"
}
