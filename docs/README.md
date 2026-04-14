# Documentation Index

## Architecture

- [Overview](architecture.md) - High-level system architecture and guiding principles
- [Design](design.md) - Nest controller design and configuration approach
- [Repositories](repos.md) - Detailed overview of all home automation repositories
- [Covers](covers.md) - Motorized shade controller design and implementation
- [Sysfs](sysfs.md) - Unipi sysfs interface reference

## Reading Order

1. Start with [architecture.md](architecture.md) for context
2. Review [repos.md](repos.md) to understand existing components
3. Read [sysfs.md](sysfs.md) for the low-level interface
4. Study [covers.md](covers.md) for the cover controller specification

## Key Decisions

### Why Direct Sysfs Access?

- **Independence**: Doesn't require evok service running
- **Simplicity**: Direct file I/O vs websocket + parsing
- **Reliability**: Fewer dependencies to fail
- **Latency**: Polling is predictable, no network overhead

### Why Go?

- **Simplicity**: Easy to read and maintain
- **Deployment**: Single static binary
- **Dependencies**: Minimal (MQTT library only)
- **Development Speed**: Faster iteration than Rust
