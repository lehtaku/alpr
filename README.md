# alpr

A command-line tool for routine code-review work across Git hosting platforms and ticket trackers: list and inspect pull requests, post review comments, clear a "changes requested" status once comments are handled, and read or comment on tickets. It's meant for the terminal, cron jobs and AI assistants alike: every command has `--json` output and stable exit codes, and every write command has `--dry-run`.

Supported platforms: Bitbucket Cloud and GitHub for pull requests; Jira Cloud and GitHub Issues for tickets.

**Status:** early development. Working so far: `auth set`, `auth test`, and `pr show` on Bitbucket Cloud. The planned interface is in [docs/spec.md](docs/spec.md); see [Not implemented yet](#not-implemented-yet).

## Install

Requires Go 1.27 or later.

```sh
go install github.com/lehtaku/alpr/cmd/alpr@latest
```

From a clone of this repository:

```sh
go install ./cmd/alpr
```

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`), or in `$GOBIN` when it's set; that directory must be on your `PATH`. Check the installation with:

```sh
alpr --version
```

## Quick start

1. [Create an API token](#tokens) for each platform you use.
2. [Write the config file](#configuration) with one profile per account.
3. Store the tokens in the macOS Keychain: `alpr auth set <profile>`.
4. Check that they work: `alpr auth test`.
5. Inside a clone of a repository: `alpr pr show <id>`.

## Tokens

alpr needs one token per platform account. Tokens expire on the date you pick when creating them; `alpr auth test` reports an expired token as `auth_invalid`.

### Bitbucket Cloud

Create an API token with scopes at [Atlassian account → Security → API tokens](https://id.atlassian.com/manage-profile/security/api-tokens): choose **Create API token with scopes**, then the **Bitbucket** app. Bitbucket app passwords no longer work.

| Scope | Used by |
|---|---|
| `read:user:bitbucket` | `auth test`, and `@me` in later commands |
| `read:pullrequest:bitbucket` | `pr show`; later `pr list`, `pr comment` and the diff-line check |
| `write:pullrequest:bitbucket` | later `pr unblock` (removing your change request) |
| `read:repository:bitbucket` | later `pr list --all-repos` and `sync` (listing a workspace's repositories) |

The scopes per endpoint come from Bitbucket's OpenAPI description. For the commands that work today, the first two are enough.

### Jira Cloud

Create a classic API token at the same Atlassian page with **Create API token** (without scopes). It acts with your account's permissions. The config's `username` is your Atlassian account email. Scoped Jira tokens aren't supported.

### GitHub

Create a [fine-grained personal access token](https://github.com/settings/personal-access-tokens/new) with access to the repositories you review, and these repository permissions:

- **Pull requests:** read and write
- **Issues:** read and write
- **Metadata:** read (added automatically)

A fine-grained token covers the repositories of one owner. For repositories of several owners, use one profile per owner, or a classic token with the `repo` scope.

> TODO: confirm the minimum GitHub permissions when the GitHub commands are implemented. `auth test` works with any valid token.

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

| Key | Required | Meaning |
|---|---|---|
| `provider` | yes | `bitbucket-cloud` or `github` |
| `match` | yes | `host/owner/repo` patterns of the repositories this profile covers; `*` matches within one path segment |
| `token` | yes | where the token is: `keychain:<service>/<account>` or `env:<NAME>` |
| `base_url` | no | API root; leave it out for bitbucket.org and github.com. For GitHub Enterprise Server, e.g. `https://github.example.com/api/v3` |
| `tracker.type` | with a tracker | `jira` or `github-issues` (only with provider `github`; uses the profile's token) |
| `tracker.base_url`, `tracker.username`, `tracker.token` | for `jira` | your Jira site, your Atlassian account email, and the token reference |

- Profile names may contain letters, digits, `-` and `_`. Unknown keys are errors, so typos don't pass silently.
- alpr picks the profile whose `match` patterns cover the repository (`--repo`, or the `origin` remote of the current directory), or the one named with `--profile`. No match, or more than one, is an error.
- Patterns match the host as written in the remote. If you use an SSH host alias (`git@bitbucket-work:<workspace>/<repo>.git`), add a pattern with that alias, e.g. `bitbucket-work/<workspace>/*`, or pass `--repo`.
- Tokens are never stored in the file: `keychain:<service>/<account>` reads the macOS Keychain, and `env:<NAME>` reads an environment variable.
- Bitbucket has no workspace-wide "pull requests I'm reviewing" query, so alpr will check each repository a pattern covers. Prefer narrow patterns, such as `bitbucket.org/<workspace>/<repo>` or `bitbucket.org/<workspace>/team-*`.

The full reference is in [docs/spec.md](docs/spec.md) §7–8.

## Commands

### Global flags

| Flag | Meaning |
|---|---|
| `--json` | machine-readable output (see [Output and exit codes](#output-and-exit-codes)) |
| `--profile <name>` | use this profile instead of matching the repository |
| `--repo <host>/<owner>/<repo>` | act on this repository instead of the current directory's `origin` remote |
| `-h`, `--help` | help for any command |
| `--version` | print the version |

### `alpr auth set <profile> [--tracker]`

Stores a profile's token in the Keychain item its `token` reference names. `--tracker` stores the Jira tracker token instead.

```sh
alpr auth set work              # prompts for the token without echoing it
alpr auth set work --tracker
pbpaste | alpr auth set work    # or pipe it in
```

```
Stored the provider token of profile "work" in keychain:alpr/work.
```

The token is never passed as a process argument or printed. Running it again replaces the stored token. Profiles whose token comes from `env:<NAME>` can't be set this way; set the variable instead. To check that an item exists without showing the token:

```sh
security find-generic-password -s alpr -a work
```

### `alpr auth test`

Asks each platform who the token belongs to, for every profile or only `--profile`. Exits with 0 when every check passes.

```
PROFILE  TARGET    TYPE             RESULT
work     provider  bitbucket-cloud  ok: Your Name (your-nickname)
work     tracker   jira             ok: Your Name
```

A failed check shows `failed: <reason>` and sets the exit code of the first failure.

### `alpr pr show <id>`

Shows a pull request: title, URL, state, author, branches, reviewers and their status, the description, and every comment thread with its location and resolved state.

```sh
alpr pr show 42                                   # inside a clone of the repository
alpr pr show 42 --repo bitbucket.org/<workspace>/<repo>
alpr pr show 42 --json
```

```
Add parser
https://bitbucket.org/<workspace>/<repo>/pull-requests/42

State:      open
Author:     Author Person
Branches:   feature/parser -> main
Reviewers:  Ann (approved), Bob

Adds the parser.

src/a.go:10-12 (new side), unresolved
  Ann, 2026-10-31T10:00:00Z:
    Rename this.
  Author Person, 2026-10-31T10:02:00Z:
    Done.
```

- Times are UTC. "new side" is the pull request's version of the file, "old side" the target branch's.
- Deleted comments and your unpublished draft comments aren't shown.
- On Bitbucket, `head_commit` is an abbreviated hash and `outdated` is always `null`.

> TODO: `pr show` on GitHub (exits with `unsupported` for now).

### Not implemented yet

These are specified in [docs/spec.md](docs/spec.md) §5 and will be documented here as they land:

- [ ] `pr show` on GitHub
- [ ] `pr list [--reviewer <user>] [--all-repos] [--limit <n>]`: open pull requests, optionally only where you're a reviewer
- [ ] `sync`: copy your review queue into a local SQLite database
- [ ] `ticket show <key>`: Jira or GitHub Issues ticket with comments
- [ ] `pr comment <id>`: inline or general comment, with `--dry-run`
- [ ] `ticket comment <key>`: comment on a ticket, with `--dry-run`
- [ ] `pr unblock <id>`: clear your "changes requested" status once your comments are handled

## Output and exit codes

- stdout carries only the result; errors go to stderr as one line, `alpr: <message>`.
- With `--json`, stdout carries exactly one JSON document, and an error is one JSON object on stderr: `{"error": {"code": "...", "message": "...", "details": {}}}`. The JSON fields are a stable contract ([docs/spec.md](docs/spec.md) §4); the text output may change.

| Exit | Meaning | Error codes |
|---|---|---|
| 0 | success | |
| 1 | unexpected internal error | `internal` |
| 2 | bad input: flags, arguments, config, profile or repository | `usage`, `config_not_found`, `config_invalid`, `profile_not_found`, `profile_ambiguous`, `repo_not_detected` |
| 3 | authentication or permission | `auth_missing`, `auth_invalid`, `permission_denied` |
| 4 | not found | `not_found` |
| 5 | inline comment target not in the diff | `line_not_in_diff`, `file_not_in_diff` |
| 6 | not supported by the platform | `unsupported` |
| 7 | API or network failure | `api_error`, `network_error`, `rate_limited` |

## Troubleshooting

| Message | Fix |
|---|---|
| `config file not found: …` | create `~/.config/alpr/config.toml` ([Configuration](#configuration)) |
| `no token in the Keychain for keychain:…` | run `alpr auth set <profile>` (with `--tracker` for a Jira token) |
| `environment variable … is not set or empty` | export the variable the `env:` reference names |
| `… HTTP 401 …` (`auth_invalid`) | the token is wrong or expired; create a new one and run `alpr auth set` again |
| `… HTTP 403 …` (`permission_denied`) | the token lacks a scope or permission ([Tokens](#tokens)) |
| `no profile matches …` | add a `match` pattern for the repository, or pass `--profile` |
| `can't read the origin remote …` | run alpr inside a git repository, or pass `--repo host/owner/repo` |

## Shell completion

`alpr completion <shell>` prints a completion script for `bash`, `zsh`, `fish` or `powershell`. For zsh, add this to `~/.zshrc`:

```sh
source <(alpr completion zsh)
```

## Development

```sh
go build ./...
go test ./...
go vet ./...
golangci-lint run
```

## License

MIT; see [LICENSE](LICENSE).
