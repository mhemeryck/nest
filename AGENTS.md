# Agent Guidelines

## General

This document contains conventions and guidelines for AI agents working on this codebase.

## Markdown Formatting

Write each sentence on its own line.
This makes diffs cleaner and reviews easier.
It also makes it simple to reorder or modify individual sentences.

## Git Commit Messages

NEVER commit changes without explicit permission from the user.
Always ask before running git add, git commit, or any other git commands that modify the repository state.
This includes git push, git merge, git rebase, etc.

NEVER use --no-gpg-sign or -n flags to bypass GPG signing.
Always sign commits normally.

Follow the standard commit message format.
The first line should be 50 characters or fewer.
It should be a concise summary of the change.
Leave a blank line after the first line.
Body text should wrap at 72 characters.
Use the body to explain the what and why rather than the how.
When creating commit messages from the shell, do not embed literal `\n` escape sequences in `git commit -m` arguments.
Prefer `git commit -F -` with a heredoc for multi-line messages.
Using multiple `-m` flags is acceptable for short messages when the body does not need careful wrapping.

## Pull Requests

When creating a GitHub pull request, write the PR title and description as the intended squash commit message.
The PR title should be the commit subject line and must follow the same 50-character guidance as commit messages.
The PR description should be plain commit-message body text, not a template with `Summary` and `Verification` sections.
Use the body to explain what changed and why, wrapping prose at 72 characters where practical.
Do not include test checklists unless they are relevant to the final squash commit message.
If verification details are useful during review, add them as a PR comment instead of putting them in the description.

Example:

```
Add user authentication to the login endpoint

Implement JWT-based authentication for the /login endpoint.
This replaces the previous session-based approach for better
scalability across multiple server instances.
```

## Code Style

Follow existing patterns in the codebase.
Match the style of surrounding code.
Use existing libraries and utilities before adding new dependencies.
Prefer procedural programming over object-oriented programming.
Use functions and modules over classes when possible.
Do not introduce receiver functions on project types by default.
Prefer plain package-level functions that take explicit arguments.
Only use receiver functions when there is a clear external constraint, such as implementing a required library interface.
Prefer `fmt.Errorf` for constructed errors in normal control flow.
Reserve `errors.New` for sentinel error variables that are compared or reused.
When accumulating multiple independent errors, prefer an `error` accumulator with `errors.Join` over building ad hoc string lists.
When several independent validations happen in the same block, prefer a single grouped `errors.Join(...)` call over repetitive one-line joins.

## Architecture Preferences

Keep parsing models, domain entities, and hardware or integration types separate when their responsibilities differ.
Do not reuse config structs as runtime or domain models.
Translate parsed config into explicit entity types before building runtime indexes or controller logic.

Prefer explicit typed IDs in the domain layer when different identifiers have different meanings.
Preserve typed IDs through registries and controller logic.
Only convert to plain strings at external boundaries such as sysfs, serialization, or logging.

When runtime code needs repeated lookups by several keys, introduce a registry or index layer built from domain entities rather than passing raw config through the system.
Keep registry types aligned with domain types instead of loose strings.

Prefer a single central event dispatch loop for multi-stage event flow.
Avoid designs where components consume from and publish back into the same shared bus.

Keep `main` focused on wiring.
Move long-running orchestration and event coordination into internal packages.

## Testing

Run lint and typecheck before marking a task complete.
Verify solutions with tests when possible.
Never assume specific test frameworks are available.

### Go Testing

Use [stretchr/testify](https://pkg.go.dev/github.com/stretchr/testify) for assertions and requires.
Import packages as:

```go
import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Use `assert` for checks that should not halt the test.
Use `require` for checks that must pass or the test should stop.

### Go Project Layout

Follow the [Go project layout](https://github.com/golang-standards/project-layout) conventions.

Key directories:

- `/cmd/` - application entry points
- `/internal/` - private application code
- `/pkg/` - library code importable by external applications
- `/test/` - integration and external test fixtures
- `/docs/` - design and user documentation
- `/go.mod` and `/go.sum` - module definitions

Do not use a `/vendor/` directory.
Use `/internal/` over `/pkg/` when code should not be imported externally.
