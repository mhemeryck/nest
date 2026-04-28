# Runtime Boundary Refactor Notes

## Context

The next runtime phases exposed several package boundary issues.
The package structure should be cleaned up before adding more integrations or observability behavior.

The refactor is not tied to a single actor or transport.
It affects the baseline runtime architecture around controller coordination, event normalization, registry lookups, actor boundaries, and top-level wiring.

## Recommended Sequence

Move the non-integration-specific runtime refactors first.
After that refactor is merged, add new actor behavior on top of the cleaned architecture.
This should keep future feature branches focused on behavior rather than mixing them with foundational package movement.

## Target Responsibilities

### `cmd/nest`

`cmd/nest` should stay a thin process entrypoint.
It should own CLI parsing, signal setup, calling the application runtime, logging fatal errors, and process exit status.

It should not own application wiring, actor startup, config-to-runtime mapping, or shutdown ordering.

### `internal/nest`

`internal/nest` should own top-level application wiring.
It should load parsed config, build domain entities, build runtime lookup indexes, discover configured devices, create channels, start actors, start the controller, and coordinate shutdown.

It is also the right place for config-to-package-specific runtime mapping.
Parsed or domain config should be translated into actor runtime config before starting actors.

### Actor Packages

Actor packages should stay focused on their own protocol or device behavior.
Examples are sysfs and future transport or integration actors.

Actors should expose actor-specific observations and commands.
Examples are `sysfs.StateChange`, `sysfs.Command`, and future actor equivalents.

Actors should not translate directly to other actors.
They should not know about controller policy or unrelated actor protocols.

### `internal/controller`

The controller is the central runtime bus and policy layer.
It consumes actor observations, normalizes them into controller-level events, applies bindings and runtime rules, and emits actor commands or actor state publications.

The controller is allowed to import actor packages, registry lookups, and controller-owned event translation code.
This is the intended integration point.

Avoid global controller state.
If function signatures become noisy, prefer explicit state or dependency structs passed to package-level functions.
Do not introduce receiver methods for project types by default.

### Controller Events

Events are controller-owned normalized messages.
They are not actors and are not a global app-wide abstraction unless later proven necessary.

A reasonable package shape is `internal/controller/event`.
That package can define controller event types and hold actor-to-event normalization helpers.

This keeps translation code separate from the main controller loop while making ownership clear.
For example, sysfs observations can normalize to digital input or push button events before controller policy is applied.

### Registry

The registry is a table of mappings used by the controller and event normalization code.
It should answer what a runtime address, topic, coil, or entity relationship refers to.

The registry is complementary to events.
The registry resolves identities and relationships.
Events describe what happened after normalization.
The controller uses both to decide what should happen next.

The registry may contain actor-facing mappings such as sysfs device IDs to entities or future transport addresses to entities.
It should not contain live actor runtime state such as clients, goroutines, channels, reconnect state, credentials, or mutable protocol state.

It may be worth moving registry under the controller namespace later, for example `internal/controller/registry`.
That should only happen after separating controller lookup needs from general setup or discovery needs.

## Possible Future Shape

One possible long-term package layout is:

```text
internal/
  nest/
  controller/
    event/
    registry/
  actors/
    sysfs/
    modbus/
  config/
  entity/
```

This should not be done just to mirror the architecture diagram.
Move actors under an `actors` folder when another actor makes the grouping useful.

## Immediate Refactor Candidates

Extract `cmd/nest` runtime wiring into `internal/nest`.
Move current `internal/event` under `internal/controller/event` or otherwise make it clearly controller-owned.
Group registry mappings by purpose, such as domain mappings, sysfs mappings, and later transport mappings.
Keep actor packages focused on actor behavior and keep cross-actor translation in controller-owned code.

## Open Questions

Should registry remain app-wide, or should it become `internal/controller/registry`?
When command topics or transport addresses are added, should they be parsed by the actor or resolved through registry mappings?
When additional actors are added, should actor grouping move all actors under `internal/actors` in the same refactor?
