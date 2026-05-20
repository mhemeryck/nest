# Distributed Light Model

## Purpose

This note captures the naming and routing direction for distributed light control.
It is the design reference for implementation plan phase 7.

The immediate goal is to let `nest` represent cross-unit light control cleanly before Modbus execution exists.
The longer-term goal is to let sysfs, MQTT, and Modbus act as transport actors behind the same controller-level semantic model.

## Core Direction

The domain model should use globally unique semantic entity IDs.
Those IDs should be separate from actor-local transport addresses.

We will use a hardware-unit-rooted naming scheme rather than a house-location-rooted one.

Preferred shape:

```text
hardware_unit.entity_type.entity_name
```

Examples:

- `panel_1.button.entry_left`
- `panel_1.light.entry`
- `panel_1.relay.entry`
- `garage_io.cover.main_door`

This keeps IDs globally unique while matching the physical ownership boundary of the system.

## Naming Layers

There are three distinct layers.

### 1. Semantic Global IDs

These identify domain entities.
They are stable across transports and should be used by bindings, controller logic, MQTT discovery payload generation, and cross-unit references.

Examples:

- `panel_1.button.entry_left`
- `garage_io.light.driveway`

These IDs answer:

```text
What thing is this?
```

### 2. Actor-Local Addresses

These identify how one actor talks to a local integration endpoint.
They are not the primary domain identity.

Examples:

- sysfs device IDs such as `di_3_16` and `ro_3_14`
- future Modbus coil addresses
- MQTT topics

These addresses answer:

```text
How does this actor reach it?
```

### 3. Routing Resolution

Routing decides how a semantic entity is reached from the current unit.
This should be handled by registry or index lookups built from configuration.
It should not be hard-coded into controller logic and should not require transport-specific details to leak into semantic IDs.

Routing answers:

```text
Where does the command go next?
```

## Why This Split

This avoids coupling the domain model to a single transport.
It also avoids making light definitions depend on local relay ownership fields such as `output_unit`.

That matters because:

- Modbus should become another actor rather than a special case in the light model.
- MQTT should also be able to carry cross-unit commands when that is the right routing path.
- the controller should operate on semantic entities and resolved command targets rather than on transport-specific identifiers.

## Ownership Convention

The leading hardware-unit segment in the semantic ID indicates the owning unit namespace.
For example, `garage_io.light.driveway` belongs to the `garage_io` unit namespace.

This is a naming convention first.
We should avoid pushing too much routing logic into string parsing beyond the owning-unit prefix unless there is a concrete need.

## Config Direction

Configuration should eventually express both semantic IDs and actor-local addresses.

Illustrative direction:

```yaml
unit_id: panel_1

digital_inputs:
  - id: panel_1.button.entry_left
    sysfs_device: di_3_16

relays:
  - id: panel_1.relay.entry
    sysfs_device: ro_3_14

lights:
  - id: panel_1.light.entry
    actuator: panel_1.relay.entry

bindings:
  - source: panel_1.button.entry_left
    target: garage_io.light.driveway
    action: toggle
```

This example is directional rather than final schema.
It shows the intended layering:

- semantic IDs are globally unique
- local hardware mapping stays explicit
- bindings target semantic entities rather than raw transport addresses

## Runtime Direction

The controller should continue to normalize actor observations into semantic events.
It should also resolve semantic targets into transport-specific commands through registries or indexes.

That means future execution should conceptually look like this:

```text
sysfs observation
  -> semantic button event
  -> semantic light command
  -> resolved command target
  -> sysfs or MQTT or Modbus actor command
```

The controller should not need separate event models for local and remote lights.
The difference should emerge from resolution and routing.

## Phase 7 Scope

The immediate phase-7 objective is not to execute distributed commands yet.
It is to introduce the model needed to represent them cleanly.

Phase 7 should focus on:

- adopting globally unique semantic IDs for local entities
- separating semantic IDs from actor-local sysfs identifiers
- reshaping bindings to target semantic entities cleanly
- preparing registry or index resolution for future transport routing

Phase 8 can then add Modbus-specific addressing and execution as an actor concern.

## Open Questions

- What should the exact config schema look like after the semantic ID migration?
- Should actuator references remain direct entity IDs or become a more explicit target type?
- How much routing should be inferred from the unit prefix versus declared explicitly in config?
- When cross-unit command routing exists, how should MQTT and Modbus be prioritized or selected?
