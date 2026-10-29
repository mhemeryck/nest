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
The shell includes Nushell, Go 1.26, gopls, Delve, golangci-lint, GoReleaser, and OpenSpec.
Tool revisions are recorded in `devenv.lock`.
The local linter is 2.14; CI currently uses 2.12 with the same repository configuration.

Run tasks without entering an interactive shell:

```nu
devenv tasks run dev:check
```

Use `devenv update` to update locked inputs, then run `devenv tasks run dev:check`.
Run `openspec update` inside the shell after an OpenSpec upgrade to refresh generated integrations.

## Specification Workflow

Develop requirements collaboratively before implementation.
Start with `/opsx-explore` in OpenCode to discuss one focused behavior.

- [Documentation index](docs/README.md)
- [Implementation roadmap](docs/implementation-plan.md)
