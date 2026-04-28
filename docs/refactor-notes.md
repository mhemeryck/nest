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
They also should not translate actor observations into semantic controller events.

Actors may normalize protocol or device noise into actor-owned observations.
For example, the sysfs actor may emit a `sysfs.StateChange` with old and new values, rising-edge metadata, and device type information.
It should not decide that a sysfs device is a configured push button or that a button press controls a light.

That keeps this boundary intact:

```text
actors know protocols and devices
controller knows policy and semantic translation
registry knows lookup tables
entity knows configured domain concepts
```

### `internal/entity`

`internal/entity` is the config-derived semantic model.
It translates parsed config into typed IDs and configured domain concepts such as digital inputs, push buttons, relays, lights, and bindings.

It should not become a runtime registry or actor implementation detail container.
It may contain actor-facing addresses while those addresses are part of the configured model, but those addresses should be explicit.

The current `entity.DeviceID` name is generic, but it effectively means a sysfs device identifier.
As more actors are added, actor-facing address types should become explicit rather than sharing one generic device ID.
For example, sysfs devices, Modbus coils, and command topics should not all collapse into the same identifier type.

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

The event package should be data-only.
It should define the controller's semantic event language, such as push button or light events.
It should not perform registry lookups or actor-to-event mapping itself.
Event type names do not all need an `Event` suffix.
Use names that read well at call sites and avoid stutter, especially when the package name already provides context.

Controller-owned normalization code should translate actor observations plus registry lookups into semantic events.
For example, sysfs observations can normalize to push button events before controller policy is applied.
Do not introduce intermediate semantic events unless they represent useful domain facts on their own.
For the current local light path, a configured digital input change does not need to be a controller event if it only exists to produce a push button event.

The current local light path shows the intended split:

```text
sysfs.StateChange
  -> controller normalization + registry lookup
  -> event.PushButton
  -> controller policy + registry lookup
  -> event.Light
  -> controller policy + registry lookup
  -> sysfs.RelayCommand
```

This avoids an event package that is half data model and half mapping layer.
It also keeps the controller as the explicit owner of the two runtime phases: normalize observations, then apply policy.
The semantic light step is important because light behavior should remain a domain decision before it becomes a concrete relay command.

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

Registry callers should not need to understand every raw map if small lookup helpers make intent clearer.
Helpers such as `PushButtonsByInput`, `BindingsByButton`, or `RelayByLight` can be added gradually when they simplify normalization or policy code.
The registry should still remain a lookup/index layer, not a behavioral policy layer.

### Controller Normalization and Policy

The controller should own both sides of the semantic boundary.

Observation normalization turns actor observations into semantic events:

```text
actor observation + registry -> semantic event
```

Policy turns semantic events into actor commands or actor state publications:

```text
semantic event + registry -> actor command
```

These phases can start as package-level functions in `internal/controller`.
Only split them into subpackages when the code grows enough to justify the extra names.

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
Make `internal/event` data-only by moving registry-backed mapping functions into controller-owned normalization code.
After `internal/event` is data-only, decide whether it should remain `internal/event` or move under `internal/controller/event`.
Add minimal registry lookup helpers where they make normalization or policy code clearer.
Consider renaming `entity.DeviceID` to a sysfs-specific address type before adding other actor address types.
Group registry mappings by purpose, such as domain mappings, sysfs mappings, and later transport mappings.
Keep actor packages focused on actor behavior and keep cross-actor translation in controller-owned code.

## Open Questions

Should registry remain app-wide, or should it become `internal/controller/registry`?
Should semantic event types remain in `internal/event`, or move under `internal/controller/event` after mapping functions are removed?
Should `entity.DeviceID` be renamed now, or only when the next actor introduces a second address type?
When command topics or transport addresses are added, should they be parsed by the actor or resolved through registry mappings?
When additional actors are added, should actor grouping move all actors under `internal/actors` in the same refactor?
