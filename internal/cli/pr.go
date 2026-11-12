package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/lehtaku/alpr/internal/domain"
	"github.com/lehtaku/alpr/internal/gitinfo"
	"github.com/lehtaku/alpr/internal/services"
)

func newPRCommand(flags *globalFlags) *cobra.Command {
	pr := &cobra.Command{Use: "pr", Short: "Work with pull requests"}
	pr.AddCommand(newPRListCommand(flags), newPRShowCommand(flags))
	return pr
}

func newPRListCommand(flags *globalFlags) *cobra.Command {
	var reviewer string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open pull requests, most recently updated first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := currentRepo(flags)
			if err != nil {
				return err
			}
			list, err := services.ListPullRequests(cmd.Context(), cfg, flags.profile, repo, reviewer, limit)
			if err != nil {
				return err
			}
			if flags.json {
				return writeJSON(cmd.OutOrStdout(), list)
			}
			return writePullRequestList(cmd.OutOrStdout(), list.PullRequests)
		},
	}
	cmd.Flags().StringVar(&reviewer, "reviewer", "", "only pull requests this user is asked to review or has reviewed: @me, a GitHub login or a Bitbucket account UUID")
	cmd.Flags().IntVar(&limit, "limit", 50, "the most pull requests to list")
	return cmd
}

func newPRShowCommand(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a pull request with its reviewers and comment threads",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := currentRepo(flags)
			if err != nil {
				return err
			}
			pr, err := services.ShowPullRequest(cmd.Context(), cfg, flags.profile, repo, args[0])
			if err != nil {
				return err
			}
			if flags.json {
				return writeJSON(cmd.OutOrStdout(), pr)
			}
			return writePullRequest(cmd.OutOrStdout(), pr)
		},
	}
}

// currentRepo returns --repo, or host/owner/repo of the current directory's origin remote.
func currentRepo(flags *globalFlags) (string, error) {
	if flags.repo != "" {
		return gitinfo.ParseRepo(flags.repo)
	}
	return gitinfo.OriginRepo(".")
}

func writePullRequestList(w io.Writer, prs []domain.PullRequestSummary) error {
	if len(prs) == 0 {
		_, err := io.WriteString(w, "No open pull requests.\n")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "REPO\tID\tTITLE\tAUTHOR\tUPDATED")
	for _, pr := range prs {
		title := pr.Title
		if pr.Draft {
			title = "[draft] " + title
		}
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", pr.Repo, pr.ID, title, pr.Author.DisplayName, pr.UpdatedAt)
	}
	return table.Flush()
}

func writePullRequest(w io.Writer, pr domain.PullRequest) error {
	var b strings.Builder
	state := string(pr.State)
	if pr.Draft {
		state += " (draft)"
	}
	fmt.Fprintf(&b, "%s\n%s\n\n", pr.Title, pr.URL)
	fmt.Fprintf(&b, "State:      %s\n", state)
	fmt.Fprintf(&b, "Author:     %s\n", pr.Author.DisplayName)
	fmt.Fprintf(&b, "Branches:   %s -> %s\n", pr.SourceBranch, pr.TargetBranch)
	fmt.Fprintf(&b, "Reviewers:  %s\n", reviewerList(pr.Reviewers))
	if pr.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", strings.TrimRight(pr.Description, "\n"))
	}
	for _, thread := range pr.Threads {
		fmt.Fprintf(&b, "\n%s\n", threadHeading(thread))
		for _, comment := range thread.Comments {
			fmt.Fprintf(&b, "  %s, %s:\n%s\n", comment.Author.DisplayName, comment.CreatedAt, indent(comment.Body, "    "))
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func reviewerList(reviewers []domain.Reviewer) string {
	if len(reviewers) == 0 {
		return "none"
	}
	names := make([]string, 0, len(reviewers))
	for _, r := range reviewers {
		switch r.Status {
		case domain.ReviewApproved:
			names = append(names, r.User.DisplayName+" (approved)")
		case domain.ReviewChangesRequested:
			names = append(names, r.User.DisplayName+" (changes requested)")
		default:
			names = append(names, r.User.DisplayName)
		}
	}
	return strings.Join(names, ", ")
}

// threadHeading is e.g. "src/app.go:118-120 (new side), unresolved" or "General comment".
func threadHeading(thread domain.Thread) string {
	heading := "General comment"
	if loc := thread.Location; loc != nil {
		heading = loc.Path
		if loc.Line != nil {
			if loc.StartLine != nil && *loc.StartLine != *loc.Line {
				heading += fmt.Sprintf(":%d-%d", *loc.StartLine, *loc.Line)
			} else {
				heading += fmt.Sprintf(":%d", *loc.Line)
			}
		}
		heading += fmt.Sprintf(" (%s side)", loc.Side)
	}
	if thread.Resolved != nil {
		if *thread.Resolved {
			heading += ", resolved"
		} else {
			heading += ", unresolved"
		}
	}
	if thread.Outdated != nil && *thread.Outdated {
		heading += ", outdated"
	}
	return heading
}

func indent(text, prefix string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
