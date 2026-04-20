# Documentation Index

This directory documents the broader home automation system around `nest`,
not just the current implementation in this repository.

It serves two purposes:

- Describe the current architecture across multiple repositories and services
- Capture the intended direction for consolidating that functionality into `nest`

Some documents describe the current production setup.
Some describe legacy components.
Some describe planned or target-state design.
Unless explicitly stated otherwise, they should be read as architecture and
migration documentation rather than as an exact description of the current Go
codebase.

## Architecture

- [Overview](architecture.md) - High-level system architecture and guiding principles
- [Design](design.md) - Target design for `nest` as the consolidated controller
- [Long-Term Ideas](ideas.md) - Parked ideas and future directions
- [Repositories](repos.md) - Overview of existing repositories, roles, and migration context
- [Covers](covers.md) - Cover controller behavior, legacy approach, and planned `nest` implementation
- [Sysfs](sysfs.md) - Unipi sysfs interface and how `nest` uses or plans to use it

## Reading Order

1. Start with [architecture.md](architecture.md) for system context
2. Review [repos.md](repos.md) to understand the current multi-repo landscape
3. Read [design.md](design.md) for the intended consolidation direction
4. Use [sysfs.md](sysfs.md) and [covers.md](covers.md) for subsystem details

## Key Decisions

### Why Direct Sysfs Access?

- **Independence**: Doesn't require evok service running
- **Simplicity**: Direct file I/O vs websocket + parsing
- **Reliability**: Fewer dependencies to fail
- **Latency**: Polling is predictable, no network overhead

### Why Go?

- **Simplicity**: Easy to read and maintain
- **Deployment**: Single static binary
- **Dependencies**: Minimal
- **Development Speed**: Faster iteration than Rust
