# nest

New home automation setup in go.

## Development

Install [Nix](https://nixos.org/download/) and [devenv](https://devenv.sh/getting-started/).
Enter the pinned development environment:

```nu
devenv shell
devenv tasks run dev:check
```

Individual checks: `devenv tasks run dev:lint`, `devenv tasks run dev:vet`, `devenv tasks run dev:test`, and `devenv tasks run dev:build`
Format all Markdown files with `devenv tasks run dev:format`; verify formatting with `devenv tasks run dev:format-check`.
The `dev:check` task runs formatting checks, lint, vet, race tests, and builds as dependencies.
The `dev:test` task includes race detection.
The shell includes Nushell, Go 1.26, gopls, Delve, golangci-lint, and GoReleaser.
Tool revisions are recorded in `devenv.lock`.
The local linter is 2.14; CI currently uses 2.12 with the same repository configuration.

Run tasks without entering an interactive shell:

```nu
devenv tasks run dev:check
```

Use `devenv update` to update locked inputs, then run `devenv tasks run dev:check`.

## Design and Development

Discuss the problem, data model, invariants, and main algorithm before substantial changes.
Keep living design notes in `docs/`, including responsibilities, data flow, decisions, and tradeoffs.
Use representative examples to clarify behavior, not to prescribe architecture.
Implement small, reviewable changes and reconsider the model when implementation exposes a problem.
Optimize for understandable code, explicit ownership, and minimal necessary state.

OpenSpec is no longer the development workflow.
Historical plans and specifications provide context, not automatic requirements or mandatory architecture.

- [Documentation index](docs/README.md)
- [Implementation roadmap](docs/implementation-plan.md)
