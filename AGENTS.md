# AGENTS.md

## First Step
- Read `README.md` first to understand the project goals, structure, and workflows before making changes.

## Go Guidelines
- Run gofmt on any Go code changes.
- Prefer the standard library; avoid new third-party dependencies unless necessary.
- Wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Do not change public APIs without explicit request.
- Keep package layout consistent (`cmd/`, `internal/`, `pkg/`); place new code accordingly.
- Be explicit about concurrency ownership: context cancellation, goroutine lifetimes, channel closure.
- Handle immediately (`if err != nil`). Wrap errors with context. No `panic`.
- Minimal nesting. Use guard clauses (return early).
- Use `any` instead of `interface{}`. No naked returns in long functions.
- Avoid global state and `init()` functions where possible.
- Consider `sync.Pool` only for proven hot allocations.

## Tests
- If you add behavior, add or update a test when it is simple and nearby.
- Prefer table-driven tests; avoid flaky tests and `time.Sleep` unless necessary.

## Dependencies
- Do not add new dependencies without a clear need and a brief justification.
- Do not change `go.mod` or `go.sum` unless required by the change.

## Git (Local)
- Commit directly to `main`.
- Make small, frequent commits; use `git add <files>` by default and `git add -p` only when you need to split changes.
- Before confirming changes, check `git status -sb`, `git diff`, and `git diff --staged`.
- For quick rollback, prefer `git restore --staged .` and `git restore .`.
- Use Conventional Commits: `type(scope): summary` (e.g., `feat(cli): add --json output`).
- After implementing code changes, run `mise run lint`.

## Docs
- After completing a task and running tests/lint, update `README.md` to reflect any new functionality or structure changes.
- If task changes behavior, update documentation to reflect the new behavior.

## Definition of Done
- At least one relevant test or verification step was run.
- Documentation reflects behavior/structure changes.
- `git status -sb` is clean, or known unrelated local changes are called out.

## Tooling
- Use `mise` instead of `make` for tasks.
- Use `mise` for tool management.
- Provide short `mise` tasks for build, test, and lint.
- Consider pre-commit hooks for `gofmt` and a fast `go test` subset.
