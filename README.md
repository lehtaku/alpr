# alpr

A command-line tool for routine code-review work across Git hosting platforms and ticket trackers: list and inspect pull requests, post review comments, clear a "changes requested" status once comments are handled, and read or comment on tickets. It's meant for the terminal, cron jobs and AI assistants alike: every command has `--json` output and stable exit codes, and every write command has `--dry-run`.

Supported platforms: Bitbucket Cloud and GitHub for pull requests; Jira Cloud and GitHub Issues for tickets.

**Status:** early development; no commands work yet. The planned interface is in [docs/spec.md](docs/spec.md).

## Install

Requires Go 1.27 or later.

```sh
go install github.com/lehtaku/alpr/cmd/alpr@latest
```

## Configuration

alpr reads `~/.config/alpr/config.toml` (or `$XDG_CONFIG_HOME/alpr/config.toml`). Each profile ties a set of repositories to a platform, a token and optionally a ticket tracker:

```toml
[profiles.work]
provider = "bitbucket-cloud"
match    = ["bitbucket.org/<workspace>/*"]
token    = "keychain:alpr/work"

[profiles.work.tracker]
type     = "jira"
base_url = "https://<site>.atlassian.net"
username = "you@example.com"
token    = "keychain:alpr/work-jira"

[profiles.personal]
provider = "github"
match    = ["github.com/<owner>/*"]
token    = "env:GITHUB_TOKEN"

[profiles.personal.tracker]
type = "github-issues"
```

- alpr picks the profile whose `match` patterns cover the repository's `origin` remote (`host/owner/repo`, where `*` matches one path segment), or the one named with `--profile`.
- Tokens are never stored in the file: `keychain:<service>/<account>` reads the macOS Keychain, and `env:<NAME>` reads an environment variable.
- Bitbucket has no workspace-wide "pull requests I'm reviewing" query, so alpr checks each repository a pattern covers. Prefer narrow patterns, such as `bitbucket.org/<workspace>/<repo>` or `bitbucket.org/<workspace>/team-*`.

The full reference is in [docs/spec.md](docs/spec.md) §7–8.

## Development

```sh
go build ./...
go test ./...
go vet ./...
golangci-lint run
```

## License

MIT; see [LICENSE](LICENSE).
