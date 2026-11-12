# alpr specification

**Status:** approved 2026-11-02; scope narrowed to pull requests on 2026-11-11. This is the contract for the CLI: commands and flags, JSON output, exit codes and the config file. The platform facts in §9 were checked against the official API descriptions on 2026-10-31. Design decisions are in §10.

## 1. Scope

| Command | Phase | Purpose |
|---|---|---|
| `auth set`, `auth test` | 1 | store a token; check that tokens work |
| `pr show` | 1 | pull request details, reviewers and comment threads |
| `pr list` | 2 | open pull requests, optionally only where a user is a reviewer |
| `pr comment` | 3 | post a comment on a pull request |
| `pr unblock` | 4 | clear my "changes requested" status when rule B holds (§5.6) |

Platforms: Bitbucket Cloud and GitHub.

Not in v1: draft comments, resolving or reopening threads, approving, file-level comments, GitHub team reviewers, other platforms.

Out of scope: tickets and ticket trackers, a local database or sync, automated reviews, and a UI. alpr only reads and manages pull requests and keeps no state of its own; other tools can build on its `--json` output.

## 2. Conventions

### 2.1 Invocation and global flags

`alpr <command> [<subcommand>] [arguments] [flags]`. Flags may come before or after positional arguments.

| Flag | Meaning |
|---|---|
| `--json` | machine-readable output (§4) |
| `--profile <name>` | use this profile instead of matching the repository (§3) |
| `--repo <host>/<owner>/<repo>` | act on this repository instead of the current directory's `origin` remote |
| `-h`, `--help` | help for any command |
| `--version` | print the version |

### 2.2 Output

- stdout carries only the result: text for people, or exactly one JSON document with `--json`.
- stderr carries errors and warnings. With `--json`, an error is one JSON object on stderr (§6).
- No colors, no progress output, and no prompts except in `auth set`.
- Text output is for people and may change; only `--json` output is a stable contract.

### 2.3 Write commands

- Every command that changes something on a platform accepts `--dry-run`. alpr then does all reads and checks (including the diff-line check), prints the HTTP requests it would send (method, URL and body, never auth headers), and sends nothing.
- Comment text comes from `--body <text>` or `--body-file <path>`, where `-` means stdin. Exactly one is required, and the text must not be empty.

## 3. Repository and profile resolution

1. **Repository:** `--repo` if given; otherwise the `origin` remote of the git repository in the current directory. These remote forms are understood: `https://[user@]host/owner/repo[.git]`, `ssh://git@host[:port]/owner/repo[.git]` and `git@host:owner/repo[.git]`.
2. **Profile:** `--profile` if given; otherwise the one profile whose `match` patterns cover `host/owner/repo`. No match, or more than one, is an error that lists the candidates (exit 2).
3. Patterns match the host as written in the remote. A remote that uses an SSH host alias (`git@bitbucket-work:<workspace>/<repo>.git`) needs a pattern with that alias.
4. `repo` values in output always use the platform's own host (`bitbucket.org`, `github.com`, or the GitHub Enterprise host).
5. `auth test` and `pr list --all-repos` cover all profiles unless `--profile` is given.
6. `@me` means the user the token belongs to, wherever a user is expected.

## 4. JSON output

### 4.1 Rules

- One JSON document per run, UTF-8, `snake_case` keys.
- Every documented field is always present. A value that is unknown, or that the platform doesn't provide, is `null`.
- IDs are strings on every platform: `"42"`, `"{8a1f…}"`.
- Timestamps are UTC, RFC 3339, second precision: `"2026-10-31T09:15:00Z"`.
- Descriptions and comment bodies are Markdown.
- A single result is printed as an object. Lists are wrapped in an object (`{"pull_requests": [...]}`) so they can gain fields later.
- Within a major version, fields are added but never removed, renamed or retyped. Consumers must ignore fields they don't know.

### 4.2 Types

Notation: `T | null` is nullable, `[T]` is an array, `"a" | "b"` is one of the strings.

```
User {
  id:           string          // Bitbucket account UUID, GitHub user ID
  username:     string | null   // Bitbucket nickname, GitHub login
  display_name: string
}

Reviewer {
  user:   User
  status: "approved" | "changes_requested" | "none"
}

Comment {
  id:         string
  author:     User
  body:       string            // Markdown
  created_at: string
  updated_at: string
  url:        string | null
}

Thread {
  id:       string
  kind:     "inline" | "general"
  resolved: boolean | null      // null: this kind of thread can't be resolved on the platform
  outdated: boolean | null      // null: not reported by the platform
  location: Location | null     // null for general threads
  comments: [Comment]           // oldest first; the first comment started the thread
}

Location {
  path:       string
  side:       "new" | "old"     // new = the PR's source version, old = the target version
  line:       integer | null    // last line of the range; null for an outdated GitHub thread with no current line
  start_line: integer | null    // first line of a multi-line comment
}

PullRequestSummary {
  repo:          string         // host/owner/repo
  id:            string
  url:           string
  title:         string
  state:         "open" | "merged" | "closed"
  draft:         boolean
  author:        User
  source_branch: string
  target_branch: string
  head_commit:   string         // as the platform reports it; Bitbucket returns abbreviated hashes
  created_at:    string
  updated_at:    string
}

PullRequest = PullRequestSummary + {
  description: string           // Markdown
  reviewers:   [Reviewer]
  threads:     [Thread]
}

DryRun {
  dry_run:  true
  requests: [{ method: string, url: string, body: object | string | null }]
}

ErrorItem {                     // a failure inside an otherwise successful result
  scope:   string               // the profile or repository it applies to
  code:    string               // error code from §6
  message: string
}
```

Mapping notes:
- **Reviewers:** requested reviewers plus anyone who approved or requested changes. On GitHub, a user's latest approving, changes-requested or dismissed review sets the status (dismissed → `none`); requested teams aren't listed.
- **Threads:** deleted comments are left out. On GitHub, review summaries (the text submitted with a review) and conversation comments are `general` threads with `resolved: null`.
- **State:** Bitbucket `DECLINED` and `SUPERSEDED` are `closed`.

## 5. Commands

### 5.1 `alpr auth set <profile>`

Stores a token in the macOS Keychain item that the profile's `token` reference names.

- Reads the token from stdin. On a terminal, alpr prompts without echoing it (Q1).
- The token is never passed as a process argument and never printed.
- A profile whose reference is `env:…` is an error (exit 2); set the variable instead.

JSON: `{"profile": "work", "token_ref": "keychain:alpr/work"}`

### 5.2 `alpr auth test`

Asks each platform who the token belongs to, for all profiles or only `--profile`. Exit 0 if every check passes; otherwise the first failure's exit code.

```json
{
  "profiles": [
    {
      "profile": "work",
      "provider": { "type": "bitbucket-cloud", "ok": true, "user": { "id": "{…}", "username": "<nickname>", "display_name": "<name>" }, "error": null }
    },
    {
      "profile": "personal",
      "provider": { "type": "github", "ok": false, "user": null, "error": { "scope": "personal", "code": "auth_invalid", "message": "…" } }
    }
  ]
}
```

### 5.3 `alpr pr list [--reviewer <user>] [--all-repos] [--limit <n>]`

Open pull requests, most recently updated first.

- **Scope:** the current repository, or `--repo`. `--all-repos` covers every repository the profile's `match` patterns cover (all profiles unless `--profile` is given).
- `--reviewer <user>`: only pull requests the user is reviewing. `<user>` is `@me`, a GitHub login or a Bitbucket account UUID.
  - GitHub: the user is requested, directly or through a team, or has submitted a review (including a comment-only review).
  - Bitbucket: the user is a requested reviewer. Bitbucket can't filter on participants, so pull requests the user approved or commented on without being requested aren't listed.
- `--limit <n>`: at most n results; default 50.
- `--all-repos` can't be combined with `--repo`. A `match` pattern with a wildcard in the owner segment can't be listed and is reported in `errors`.
- A failure in one repository, pattern or profile doesn't stop the others. It's listed in `errors` (text output prints each as `alpr: <scope>: <message>` on stderr), and the exit code is that of the first error.

JSON: `{"pull_requests": [PullRequestSummary], "errors": [ErrorItem]}`
Text: `REPO  ID  TITLE  AUTHOR  UPDATED`, drafts marked.

Bitbucket has no workspace-wide reviewer query, so `--all-repos` lists each workspace's repositories and queries the matching ones, 8 at a time (§9, Q3). On GitHub, each pattern is one search: `repo:<owner>/<repo>` for a single repository, otherwise `user:<owner>` (which covers organizations too), with partial patterns such as `team-*` filtered locally.

### 5.4 `alpr pr show <id>`

JSON: `PullRequest`. Text: title, URL, state, author, branches and reviewers; then the description; then each thread with its location, resolved state and comments.

### 5.5 `alpr pr comment <id>`

```
alpr pr comment <id> (--body <text> | --body-file <path>)
                     [--file <path> --line <n> [--side new|old] [--start-line <n>]]
                     [--reply-to <comment-id>]
                     [--dry-run]
```

- No `--file`: a general comment.
- `--file` and `--line`: an inline comment on that line of `--side` (default `new`). `--start-line` makes it a multi-line comment that ends at `--line`.
- `--reply-to`: a reply in the thread that contains the comment. Can't be combined with `--file`. On GitHub only inline comments can be replied to.

**Diff-line check.** Before posting an inline comment, alpr loads the pull request diff. The file must be in it, and the whole line range must lie inside one hunk on the chosen side (added, removed or context lines). Otherwise nothing is posted and alpr exits 5, listing the lines that can be commented on:

```json
{
  "error": {
    "code": "line_not_in_diff",
    "message": "src/app.go:120 (new side) is not part of the diff",
    "details": { "path": "src/app.go", "side": "new", "line": 120, "commentable_ranges": [[10, 24], [88, 97]] }
  }
}
```

A file that isn't in the diff gives `file_not_in_diff`.

JSON: the created `Comment`, or `DryRun`.

### 5.6 `alpr pr unblock <id> [--dry-run]`

Clears my "changes requested" status if rule B holds:
1. Every thread I started that the platform can resolve is resolved.
2. The author has pushed at least one commit since my latest change request.

```
UnblockResult {
  repo:       string
  id:         string
  result:     "unblocked" | "would_unblock" | "conditions_not_met" | "not_blocked"
  conditions: {
    threads_resolved:      { met: boolean, unresolved_thread_ids: [string] }
    commits_after_request: { met: boolean, requested_at: string | null }
  } | null                                 // null when not_blocked
  requests:   [{ method, url, body }]      // filled for would_unblock
}
```

- `not_blocked`: my status isn't `changes_requested`, so there's nothing to clear.
- `would_unblock`: a dry run where the conditions hold.
- Every result exits 0; only errors use other codes.
- Bitbucket removes my change request. GitHub dismisses my changes-requested review with the message "Unblocked by alpr: my review threads are resolved and new commits were pushed" (Q2). Dismissing needs write access, and protected branches can restrict it; a refusal exits 3.

## 6. Exit codes and errors

| Exit | Meaning | Error codes |
|---|---|---|
| 0 | success, including dry runs and every `pr unblock` result | — |
| 1 | unexpected internal error | `internal` |
| 2 | bad input: flags, arguments, config, profile or repository | `usage`, `config_not_found`, `config_invalid`, `profile_not_found`, `profile_ambiguous`, `repo_not_detected` |
| 3 | authentication or permission | `auth_missing`, `auth_invalid`, `permission_denied` |
| 4 | not found | `not_found` |
| 5 | inline comment target not in the diff | `line_not_in_diff`, `file_not_in_diff` |
| 6 | not supported by the platform | `unsupported` |
| 7 | API or network failure | `api_error`, `network_error`, `rate_limited` |

Error output on stderr with `--json`:

```json
{ "error": { "code": "not_found", "message": "pull request 42 not found in github.com/<owner>/<repo>", "details": {} } }
```

- `details` is always an object, possibly empty. `unsupported` adds `capability` and `platform`; `rate_limited` adds `retry_after_seconds` when the platform sends one.
- Without `--json`, stderr gets one line: `alpr: <message>`.

## 7. Configuration

`$XDG_CONFIG_HOME/alpr/config.toml`, by default `~/.config/alpr/config.toml`. Unknown keys are errors, so typos don't pass silently.

```toml
[profiles.work]
provider = "bitbucket-cloud"
match    = ["bitbucket.org/<workspace>/*"]
token    = "keychain:alpr/work"

[profiles.personal]
provider = "github"
match    = ["github.com/<owner>/*"]
token    = "env:GITHUB_TOKEN"
```

Profile keys:

| Key | Required | Value |
|---|---|---|
| `provider` | yes | `bitbucket-cloud` or `github` |
| `match` | yes | patterns over `host/owner/repo`; `*` matches one path segment; case-insensitive |
| `token` | yes | token reference (§8) |
| `base_url` | no | API base URL. Defaults: `https://api.bitbucket.org/2.0`, `https://api.github.com`. GitHub Enterprise Server: `https://<host>/api/v3` |

Profile names may contain letters, digits, `-` and `_`.

## 8. Credentials

| Reference | Resolves to |
|---|---|
| `keychain:<service>/<account>` | the macOS Keychain generic password with that service and account |
| `env:<NAME>` | the environment variable `NAME` |

- `keychain:` references work only on macOS; elsewhere they're an error (exit 2) and `env:` must be used.
- Tokens are never printed, logged, shown in dry runs or passed as process arguments.

| Platform | Token | Scopes |
|---|---|---|
| Bitbucket Cloud | API token with scopes, sent as a Bearer token | `read:pullrequest:bitbucket`, `write:pullrequest:bitbucket`, `read:user:bitbucket`; `read:repository:bitbucket` to list workspace repositories |
| GitHub | personal access token, classic or fine-grained | read and write access to pull requests in the matched repositories |

Bitbucket app passwords stopped working on 2026-06-09, so only API tokens are supported. The minimum scopes are confirmed during implementation and documented in the README.

## 9. Platform notes

Checked on 2026-10-31 against the Bitbucket Cloud OpenAPI spec, the GitHub REST OpenAPI description and GraphQL schema, and the docs listed under Sources.

### Capabilities

| Capability | Bitbucket Cloud | GitHub |
|---|---|---|
| `inline_comments` | yes | yes |
| `resolvable_threads` | yes: comment `resolution`, resolve endpoint | yes: GraphQL `isResolved` |
| `request_changes` (clear my request) | yes: remove change request | yes: dismiss my review; needs write access, and protected branches can restrict it |
| `draft_comments` | not in v1: comments have a `pending` flag, but no documented publish endpoint | not in v1: pending reviews |

### Pull request operations

Bitbucket paths are relative to `https://api.bitbucket.org/2.0`, with `{pr}` = `/repositories/{workspace}/{repo_slug}/pullrequests/{id}`. GitHub REST paths are relative to `https://api.github.com`, with `{pr}` = `/repos/{owner}/{repo}/pulls/{number}`.

| Operation | Bitbucket Cloud | GitHub |
|---|---|---|
| who am I | `GET /user` | `GET /user` |
| open PRs in a repo | `GET /repositories/{workspace}/{repo_slug}/pullrequests?q=state = "OPEN"&sort=-updated_on`, optionally `AND reviewers.uuid = "{uuid}"` in `q` | GraphQL `search(type: ISSUE)` with `repo:{owner}/{repo} is:pr is:open sort:updated-desc`, optionally `review-involves:<login>` |
| my review queue across repos | `GET /repositories/{workspace}`, then the per-repo query; `/workspaces/{workspace}/pullrequests/{user}` only lists PRs the user authored | the same search without `repo:`, or with several `repo:` qualifiers |
| PR and reviewers | `GET {pr}`: `participants` with `role`, `approved` and `state` (`approved`, `changes_requested` or null) | `GET {pr}`, `GET {pr}/reviews`, `GET {pr}/requested_reviewers` |
| threads | `GET {pr}/comments`: `parent`, `inline` (`path`, `from`, `to`, `start_from`, `start_to`), `resolution`, `deleted`, `pending` | GraphQL `pullRequest.reviewThreads` (`isResolved`, `isOutdated`, `path`, `line`, `startLine`, `diffSide`); `GET /repos/{owner}/{repo}/issues/{number}/comments` |
| diff for the line check | `GET {pr}/diff` | `GET {pr}/files` (per-file `patch`, at most 3000 files) |
| inline comment | `POST {pr}/comments` with `inline.path` and `inline.to` (new side) or `inline.from` (old side); `start_to` / `start_from` for ranges | `POST {pr}/comments` with `commit_id`, `path`, `line`, `side` (`RIGHT` new, `LEFT` old), `start_line`, `start_side` |
| reply | `POST {pr}/comments` with `parent.id` | `POST {pr}/comments/{comment_id}/replies` |
| general comment | `POST {pr}/comments` | `POST /repos/{owner}/{repo}/issues/{number}/comments` |
| clear my change request | `DELETE {pr}/request-changes` | `PUT {pr}/reviews/{review_id}/dismissals` (`message` required) |
| commits since my request | `GET {pr}/activity`: `update` entries carry the source commit | my review's `commit_id` compared with the head SHA |

- Bitbucket returns `reviewers` and `participants` only when a single pull request is fetched, so reviewer details cost one request per pull request.
- Bitbucket's activity log includes change requests, but the docs show no example of that entry; its shape is confirmed in phase 4.
- Bitbucket ignores the `state` parameter when `q` is given, so the state goes into `q`. `reviewers.uuid` can be filtered on; `participants.uuid` can't (HTTP 400 "does not support filtering"). Both verified 2026-11-11.
- The pull request listing returns at most 50 per page on Bitbucket and 100 per GraphQL page on GitHub.
- GitHub's `review-involves:` also matches pull requests where only a team I belong to was requested, and keeps them after the review. GitHub search results can lag a few seconds behind new pull requests.

### Sources

- Bitbucket Cloud OpenAPI spec: <https://dac-static.atlassian.com/cloud/bitbucket/swagger.v3.json>
- Bitbucket API tokens: <https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/>, scopes: <https://support.atlassian.com/bitbucket-cloud/docs/api-token-permissions/>
- App password deprecation: <https://www.atlassian.com/blog/bitbucket/bitbucket-cloud-transitions-to-api-tokens-enhancing-security-with-app-password-deprecation>
- Bitbucket `q=reviewers.uuid=…` filter (an Atlassian Community answer, confirmed against the API on 2026-11-11): <https://community.atlassian.com/forums/Bitbucket-questions/How-do-I-get-all-the-PRs-I-ve-approved-or-reviewed/qaq-p/1958970>
- GitHub REST OpenAPI description: <https://github.com/github/rest-api-description>
- GitHub GraphQL schema: <https://docs.github.com/public/fpt/schema.docs.graphql>
- GitHub review dismissal: <https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/dismissing-a-pull-request-review>
- GitHub search qualifiers: <https://docs.github.com/en/search-github/searching-on-github/searching-issues-and-pull-requests>

## 10. Decisions

| # | Question | Decision |
|---|---|---|
| Q1 | Hidden token prompt in `auth set` | `golang.org/x/term`, maintained by the Go team. |
| Q2 | GitHub `pr unblock` posts a public dismissal message on the PR | Accepted. |
| Q3 | Bitbucket's queue needs one request per repository in the workspace | Accepted; the README recommends narrow `match` patterns. |
