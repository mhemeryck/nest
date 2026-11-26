# Nest

Nest is custom software for distributed home automation control.
It runs on home automation hardware, so controllers can operate connected devices locally and work together across the house.

See the [documentation index](docs/README.md) for design, planning, reference, and operations.

## Quick start

Install [Nix](https://nixos.org/download/) and [devenv](https://devenv.sh/getting-started/).
From the repository root, enter the development environment and run the project checks:

```nu
devenv shell
devenv tasks run dev:check
```
