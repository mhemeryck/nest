# nest

New home automation setup in go.

## Development

Install [Nix](https://nixos.org/download/) and [devenv](https://devenv.sh/getting-started/).
Enter the pinned development environment:

```nu
devenv shell
nest-check
```

Individual checks: `nest-lint`, `nest-vet`, `nest-test`, and `nest-build`
`nest-test` includes race detection.
The shell includes Nushell, Go 1.26, gopls, Delve, golangci-lint, GoReleaser, and OpenSpec.
`nest-check` uses Nushell.
Tool revisions are recorded in `devenv.lock`.
The local linter is 2.14; CI currently uses 2.12 with the same repository configuration.

Run a check without entering an interactive shell:

```nu
devenv shell -- nest-check
```

Use `devenv update` to update locked inputs, then run `nest-check`.
Run `openspec update` inside the shell after an OpenSpec upgrade to refresh generated integrations.

## Specification Workflow

Develop requirements collaboratively before implementation.
Start with `/opsx-explore` in OpenCode to discuss one focused behavior.

- [Documentation index](docs/README.md)
- [Implementation roadmap](docs/implementation-plan.md)
