# Runtime Boundary Refactor Notes

## Context

The passive MQTT branch exposed several package boundary issues.
The MQTT work is useful as a prototype, but the package structure should be cleaned up from `master` before carrying the MQTT implementation further.

The refactor is not MQTT-specific.
It affects the baseline runtime architecture around controller coordination, event normalization, registry lookups, actor boundaries, and top-level wiring.

## Recommended Sequence

Keep the current MQTT pull request as a review artifact.
Use it to identify the required boundaries and flaws in the current shape.

Create a new architecture refactor branch from `master`.
Move the non-MQTT-specific runtime refactors there first.

After that refactor is merged, rebuild or rebase the MQTT work on top of the cleaned architecture.
This should keep the MQTT PR focused on passive observability rather than mixing it with foundational package movement.

## Target Responsibilities

### `cmd/nest`

`cmd/nest` should stay a thin process entrypoint.
It should own CLI parsing, signal setup, calling the application runtime, logging fatal errors, and process exit status.

It should not own application wiring, actor startup, config-to-runtime mapping, or shutdown ordering.

### `internal/nest`

`internal/nest` should own top-level application wiring.
It should load parsed config, build domain entities, build runtime lookup indexes, discover configured devices, create channels, start actors, start the controller, and coordinate shutdown.

It is also the right place for config-to-package-specific runtime mapping.
For example, parsed or domain MQTT config should be translated into the MQTT actor runtime config before starting the MQTT actor.

### Actor Packages

Actor packages should stay focused on their own protocol or device behavior.
Examples are sysfs, MQTT, and future Modbus.

Actors should expose actor-specific observations and commands.
Examples are `sysfs.StateChange`, `sysfs.Command`, `mqtt.State`, and future Modbus equivalents.

Actors should not translate directly to other actors.
They should not know about controller policy or unrelated actor protocols.

The MQTT package currently imports too much domain and controller context.
Longer term, MQTT should become actor-only: connection lifecycle, publish/subscribe, and MQTT command/state envelopes.
Domain-to-MQTT topic and payload generation should move out of the actor package unless there is a specific reason to keep it there.

### `internal/controller`

The controller is the central runtime bus and policy layer.
It consumes actor observations, normalizes them into controller-level events, applies bindings and runtime rules, and emits actor commands or actor state publications.

The controller is allowed to import actor packages, registry lookups, and controller-owned event translation code.
This is the intended integration point.

Avoid global controller state.
If the function signatures become noisy, prefer explicit state or dependency structs passed to package-level functions.
Do not introduce receiver methods for project types.

### Controller Events

Events are controller-owned normalized messages.
They are not actors and are not a global app-wide abstraction unless later proven necessary.

A reasonable package shape is `internal/controller/event`.
That package can define controller event types and hold actor-to-event normalization helpers.

This keeps translation code separate from the main controller loop while making the ownership clear.
For example, sysfs observations can normalize to digital input or push button events before controller policy is applied.

### Registry

The registry is a table of mappings used by the controller and event normalization code.
It should answer what a runtime address, topic, coil, or entity relationship refers to.

The registry is complementary to events.
The registry resolves identities and relationships.
Events describe what happened after normalization.
The controller uses both to decide what should happen next.

The registry may contain actor-facing mappings such as sysfs device IDs to entities, MQTT topics to entities, or Modbus coils to entities.
It should not contain live actor runtime state such as MQTT clients, goroutines, channels, reconnect state, or mutable protocol state.

MQTT broker connection config and credentials should not be part of the registry.
MQTT topic mappings may become part of the registry once command handling or stable topic lookup requires them.

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
    mqtt/
    modbus/
  config/
  entity/
```

This should not be done just to mirror the architecture diagram.
Move actors under an `actors` folder when another actor such as Modbus makes the grouping useful.

## Immediate Refactor Candidates

Extract `cmd/nest` runtime wiring into `internal/nest`.
Move current `internal/event` under `internal/controller/event` or otherwise make it clearly controller-owned.
Remove broad MQTT config from the registry if it includes broker, client ID, credentials, or live actor concerns.
Group registry mappings by purpose, such as domain mappings, sysfs mappings, and later MQTT or Modbus mappings.
Make MQTT actor-only where possible, with domain-to-MQTT translation owned by controller-side code.

## Open Questions

Should registry remain app-wide, or should it become `internal/controller/registry`?
Should MQTT discovery be generated by controller-side code, top-level wiring, or a separate presentation package?
When MQTT commands are added, should topics be parsed by the MQTT actor or resolved through registry topic mappings?
When Modbus is added, should actor grouping move all actors under `internal/actors` in the same refactor?
