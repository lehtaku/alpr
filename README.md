# alpr

A command-line tool for routine code-review work across Git hosting platforms and ticket trackers: list and inspect pull requests, post review comments, clear a "changes requested" status once comments are handled, and read or comment on tickets. It's meant for the terminal, cron jobs and AI assistants alike: every command has `--json` output and stable exit codes, and every write command has `--dry-run`.

Supported platforms: Bitbucket Cloud and GitHub for pull requests; Jira Cloud and GitHub Issues for tickets.

**Status:** early development; no commands work yet. The planned interface is in [docs/spec.md](docs/spec.md).

## Install

Requires Go 1.27 or later.

```sh
go install github.com/lehtaku/alpr/cmd/alpr@latest
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
