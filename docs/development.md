# Development

## Tasks

Use `mise` for tools and tasks.

| Command | Description |
| --- | --- |
| `mise run` | Show available tasks |
| `mise tasks ls` | Show available tasks |
| `mise run all` | Tidy, lint, test, build |
| `mise run ci` | Clean then run the full cycle |
| `mise run build` | Compile `bin/tg-arc` |
| `mise run install` | Install to `$GOPATH/bin` |
| `mise dev -- <args>` | Run `go run ./cmd/tg-arc <args>` |
| `mise start` | Build and run `bin/tg-arc` |
| `mise run clean` | Remove build and coverage artifacts |
| `mise run tidy` | Run `go mod tidy` and `go mod verify` |
| `mise run lint` | Run `golangci-lint` |
| `mise run test` | Run unit tests |
| `mise run test-nocache` | Run unit tests without cache |
| `mise run test-cover` | Generate `coverage.html` |
| `mise run setup-hooks` | Ensure local git hooks are executable |

## Code Rules

- Run `gofmt` on changed Go files.
- Prefer the standard library; add dependencies only when clearly needed.
- Wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Keep public APIs stable unless the task explicitly asks for a change.
- Handle errors immediately; avoid `panic`.
- Keep nesting shallow with guard clauses.
- Be explicit about goroutine ownership, context cancellation, and channel closure.

## Tests

Add or update a nearby test when behavior changes and the test is simple. Prefer table-driven tests. Avoid flaky waits; do not use `time.Sleep` unless it is necessary and justified.

Before committing:

```bash
gofmt -w <changed-go-files>
mise run lint
mise run test
```

## Git Hooks

The repository has local hook scripts under `.git/hooks` when configured. Enable them with:

```bash
mise run setup-hooks
```

Manual lint remains the source of truth:

```bash
mise run lint
```

## Releases

Release tags named `v*` trigger `.github/workflows/release.yml`. The workflow uses `actions/checkout@v6`, `actions/cache@v5`, and `jdx/mise-action@v4`, verifies lint/tests, builds Linux `x64` and `arm64` archives, publishes checksums, and creates or updates the GitHub release.

```bash
git tag v0.1.0
git push origin v0.1.0
```

Published archives are compatible with mise's GitHub backend:

```bash
mise use -g github:xsfunc/tg-arc
tg-arc
```
