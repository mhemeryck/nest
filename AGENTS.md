# Agent Guidelines

## General

This document contains conventions and guidelines for AI agents working on this codebase.

## Markdown Formatting

Write each sentence on its own line.
This makes diffs cleaner and reviews easier.
It also makes it simple to reorder or modify individual sentences.

## Git Commit Messages

Follow the standard commit message format.
The first line should be 50 characters or fewer.
It should be a concise summary of the change.
Leave a blank line after the first line.
Body text should wrap at 72 characters.
Use the body to explain the what and why rather than the how.

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
