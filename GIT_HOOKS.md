# Git Hooks Setup

This project uses Git hooks to ensure code quality before commits and pushes.

## Installed Hooks

### pre-commit
- Runs `golangci-lint` before each commit
- Prevents commits if linting errors are found
- Location: `.git/hooks/pre-commit`

### pre-push
- Runs `golangci-lint` before each push
- Prevents pushes if linting errors are found
- Can be extended to run tests
- Location: `.git/hooks/pre-push`

## Setup for New Contributors

After cloning the repository, run:

```bash
mise run setup-hooks
```

This will ensure all hooks are executable. Alternatively:

```bash
chmod +x .git/hooks/pre-commit .git/hooks/pre-push
```

## Requirements

- [golangci-lint](https://golangci-lint.run/welcome/install/) must be installed

## Bypassing Hooks (Not Recommended)

If you need to bypass hooks temporarily:

```bash
# Skip pre-commit hook
git commit --no-verify

# Skip pre-push hook
git push --no-verify
```

⚠️ **Warning**: Only bypass hooks if you have a good reason and understand the consequences.

## Manual Linting

You can run linting manually at any time:

```bash
mise run lint
```
