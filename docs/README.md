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

## Development

| Document                                      | Purpose                                                  |
| --------------------------------------------- | -------------------------------------------------------- |
| [Implementation Plan](implementation-plan.md) | Development roadmap, completed phases, and proposed work |

- Discuss substantial changes before implementation; keep agreed models and decisions in living design documents
- Historical plans provide context, not automatic requirements or mandatory architecture

## Architecture

| Document                                                  | Purpose                                                |
| --------------------------------------------------------- | ------------------------------------------------------ |
| [Overview](architecture.md)                               | System architecture and guiding principles             |
| [Design](design.md)                                       | Nest package responsibilities and design direction     |
| [Long-Term Ideas](ideas.md)                               | Parked ideas and future directions                     |
| [Repositories](repos.md)                                  | Repository roles and migration context                 |
| [Covers](covers.md)                                       | Legacy cover behavior and earlier Nest design ideas    |
| [Sysfs](sysfs.md)                                         | Unipi sysfs interface and current Nest implementation  |
| [Home Automation Landscape](home-automation-landscape.md) | Protocol context behind the transport model            |
| [Distributed Light Model](distributed-light-model.md)     | Semantic entities, bindings, and distributed execution |

## Reading Order

1. Start with [architecture.md](architecture.md) for system context
2. Review [repos.md](repos.md) to understand the current multi-repo landscape
3. Read [design.md](design.md) for the intended consolidation direction
4. Read [home-automation-landscape.md](home-automation-landscape.md) for protocol and transport context
5. Read [distributed-light-model.md](distributed-light-model.md) for the current distributed light control direction
6. Use [sysfs.md](sysfs.md) and [covers.md](covers.md) for subsystem details
7. Review [implementation-plan.md](implementation-plan.md) for progress and proposed next steps
