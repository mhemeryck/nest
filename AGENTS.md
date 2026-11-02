# Agent Guidelines

## Writing

- Fragments over full sentences
- Full sentences only for narrative or necessary precision
- One sentence per line in Markdown prose
- Compact scan-oriented comments, lists, and headings
- Design documents: ASD-STE100 Simplified Technical English; active voice, consistent terms, one idea per sentence
- Format all Markdown files with dprint; run `devenv tasks run dev:format` after edits

## Git

- Explicit user approval before `git add`, commits, pushes, merges, rebases, or other repository mutations
- Normal GPG signing only
- Commit subject: concise, 50 characters maximum
- Commit body: what and why, 72-column wrapping
- Multi-line commit messages: `git commit -F -` with a quoted heredoc
- PR title and description: intended squash commit message
- PR descriptions: plain commit-message body, not review templates

## Code Style

- Existing patterns and utilities before new dependencies
- Procedural functions and modules over classes
- Package-level functions with explicit arguments over receivers by default
- Receivers only for required external interfaces or clear constraints
- `fmt.Errorf` for constructed control-flow errors
- `errors.New` for reused or compared sentinels
- `errors.Join` for independent validation failures

## Code Structure

- Multi-step code: logical responsibilities over low-level implementation steps
- Keep the main algorithm and failure decisions visible in the caller
- Extract helpers when they reduce reasoning, not merely duplication or function length
- Example: `receiveCommand`, `validateCommand`, `persistCommand`
- Behavior- and direction-oriented names: `handleMasterToSlaveCommand`, `pollSlaveStatePoints`
- Top-level functions: visible lifecycle or responsibility sequence
- Compact responsibility list above functions when structure alone lacks a clear high-level map
- Fragment-based comments and lists
- No helpers for trivial control flow alone

## Data and State

- Choose representations that prevent invalid states
- Explicit alternatives over independent flags for mutually exclusive states
- Derive state from existing information when possible
- Make ownership, mutation, resource lifetimes, and side effects explicit
- Distinguish expected failures from violated invariants
- Clear failure for violated invariants over silent compensation
- Safety at hardware boundaries; no speculative retries or recovery mechanisms

## Architecture

- Parsing models, domain entities, and integration types: separate responsibilities
- Parsed config translated into domain entities before runtime indexes or controller logic
- Typed domain IDs preserved through registries and controllers
- Plain strings only at external boundaries
- Registries for repeated domain lookups
- Registry types aligned with domain types
- Single central event dispatch loop for multi-stage event flow
- No consume-and-publish cycle on one shared bus
- `main`: wiring only
- Long-running orchestration: internal packages

## Testing

- Lint and typecheck before task completion
- Tests where practical
- No assumed test framework
- Go assertions: `stretchr/testify`
- `assert`: non-fatal checks
- `require`: prerequisite checks

## Development Workflow

- Discuss substantial changes before implementation: problem, data model, invariants, and main algorithm
- Challenge assumptions and compare alternatives; observations do not automatically become requirements
- Keep lightweight, living design documents in `/docs/`
- Document responsibilities, data flow, significant decisions, and tradeoffs
- Representative scenarios only when they clarify the model; no exhaustive scenario catalogs
- Historical plans, OpenSpec artifacts, and existing code: evidence for discussion, not authoritative implementation instructions
- Do not generate OpenSpec artifacts or use the OpenSpec workflow
- Agree on a sufficiently clear model before implementation; refine it through small, reviewable changes
- Discuss new architectural concepts before introducing them
- Reconsider an inadequate model instead of accumulating workarounds
- Optimize for understanding, minimal necessary state, and real failure conditions
- Run `devenv tasks run dev:check` before task completion
- Nushell for development scripts and shell examples

## Layout

- `/cmd/`: entry points
- `/internal/`: private application code
- `/pkg/`: external libraries
- `/test/`: integration tests and fixtures
- `/docs/`: documentation
- No `/vendor/`
- `/internal/` over `/pkg/` for non-public code
