package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lehtaku/alpr/internal/config"
	"github.com/lehtaku/alpr/internal/domain"
	"github.com/lehtaku/alpr/internal/services"
)

// maxTokenInput caps how much piped input auth set reads.
const maxTokenInput = 64 << 10

func newAuthCommand(flags *globalFlags) *cobra.Command {
	auth := &cobra.Command{Use: "auth", Short: "Store and check API tokens"}
	auth.AddCommand(newAuthSetCommand(flags), newAuthTestCommand(flags))
	return auth
}

func newAuthSetCommand(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set <profile>",
		Short: "Store a profile's token in the macOS Keychain",
		Long:  "Store a profile's token in the macOS Keychain item its token reference names.\nThe token is read from a hidden prompt, or from stdin when it's piped in.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			stored, err := services.SetToken(cfg, args[0], func(label string) (string, error) {
				return readToken(cmd, label)
			})
			if err != nil {
				return err
			}
			if flags.json {
				return writeJSON(cmd.OutOrStdout(), stored)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Stored the token of profile %q in %s.\n", stored.Profile, stored.TokenRef)
			return err
		},
	}
}

// readToken prompts without echo on a terminal, and otherwise reads the token piped to stdin.
func readToken(cmd *cobra.Command, label string) (string, error) {
	in := cmd.InOrStdin()
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Token for %s: ", label)
		data, err := term.ReadPassword(int(file.Fd()))
		_, _ = fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return "", domain.Errorf(domain.CodeUsage, "can't read the token: %v", err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	data, err := io.ReadAll(io.LimitReader(in, maxTokenInput))
	if err != nil {
		return "", domain.Errorf(domain.CodeUsage, "can't read the token from stdin: %v", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func newAuthTestCommand(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Check that each profile's token works",
		Long:  "Ask each platform who the token belongs to, for all profiles or only --profile.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			profiles, err := selectedOrAll(cfg, flags.profile)
			if err != nil {
				return err
			}

			checks := services.CheckProfiles(cmd.Context(), profiles)
			if flags.json {
				err = writeJSON(cmd.OutOrStdout(), map[string]any{"profiles": checks})
			} else {
				err = writeChecks(cmd.OutOrStdout(), checks)
			}
			if err != nil {
				return err
			}
			return firstFailure(checks)
		},
	}
}

func writeChecks(w io.Writer, checks []services.ProfileCheck) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "PROFILE\tPROVIDER\tRESULT")
	for _, profile := range checks {
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\n", profile.Profile, profile.Provider.Type, checkResult(&profile.Provider))
	}
	return table.Flush()
}

func checkResult(check *services.Check) string {
	if !check.OK {
		return "failed: " + check.Error.Message
	}
	if check.User.Username != nil && *check.User.Username != check.User.DisplayName {
		return fmt.Sprintf("ok: %s (%s)", check.User.DisplayName, *check.User.Username)
	}
	return "ok: " + check.User.DisplayName
}

// firstFailure returns the exit status of the first failed check, which the output already describes.
func firstFailure(checks []services.ProfileCheck) error {
	for _, profile := range checks {
		if check := profile.Provider; !check.OK {
			return exitStatus(check.Error.Code.ExitCode())
		}
	}
	return nil
}

func loadConfig() (*config.Config, error) {
	file, err := config.FilePath()
	if err != nil {
		return nil, err
	}
	return config.Load(file)
}

// selectedOrAll returns the --profile profile, or every profile in name order.
func selectedOrAll(cfg *config.Config, name string) ([]*config.Profile, error) {
	if name != "" {
		profile, err := cfg.Select(name, "")
		if err != nil {
			return nil, err
		}
		return []*config.Profile{profile}, nil
	}
	return cfg.All(), nil
}
