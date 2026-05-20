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

## Modbus Transport Direction

The current distributed light model has to fit the existing hardware topology.
Changing the hardware layout is out of scope for the initial migration.

In particular:

- we should not require a new dedicated Modbus master device
- we should not require a central network-dependent command router
- cross-unit light migration should still work on the existing RS-485 capable units

### Core Observation

Modbus RTU is not a symmetric peer-to-peer transport in the same way MQTT is.

At the protocol level:

- a master initiates requests
- a slave responds to requests
- a slave does not spontaneously publish events on the bus

That means Modbus fits the actor model, but not as a symmetric actor role.

### Actor Model Fit

The Modbus integration should still use the same runtime boundary pattern as other actors:

- one command channel into the actor
- one event channel back out of the actor

Internally, the Modbus actor can run in one of these roles:

- master only
- slave only
- both later if a concrete use case requires it

The controller-facing contract should stay the same regardless of the internal role.

### Planned Role Split

For the current migration path, units may take different Modbus roles depending on what they need to do.

#### Master Role

The master side is responsible for:

- consuming resolved controller commands that target remote actuators
- issuing Modbus RTU requests to remote units
- reporting write failures, connection issues, and similar transport outcomes back to the controller

#### Slave Role

The slave side is responsible for:

- exposing local relay or actuator control points over Modbus RTU
- receiving writes from a master
- turning those writes into local actuator effects or controller-visible observations

### Semantic Model Versus Transport Role

Units remain equal at the semantic level.
Any unit may own local buttons, lights, relays, or covers.

Units are not necessarily equal at the Modbus transport-role level.
One unit may need to act as a Modbus master for a given deployment, while another acts as a slave.

This distinction is important:

- semantic ownership belongs in the domain model
- master or slave behavior belongs in actor runtime configuration

### Recommended Scope For Modbus

For the initial migration, Modbus should stay relatively low-level.

Recommended direction:

- use Modbus primarily as a transport for remote actuator control
- expose coil-level or actuator-level control points on the slave side
- let the controller keep semantic command and routing decisions above Modbus

This matches the current Python implementation more closely than trying to turn Modbus into a full semantic controller-to-controller bus.

### Relationship To MQTT

MQTT and Modbus should not be treated as interchangeable.

MQTT is well suited for:

- discovery
- Home Assistant integration
- semantic state publication
- optional command transport when network dependence is acceptable

Modbus is better suited for:

- direct bus-based remote actuator control
- deployments where the lighting path should not depend on the IP network

For the current project direction, remote light migration should not depend on MQTT or the network.

### Topology Direction

We should design for the current distributed hardware rather than a hypothetical future central master box.

That means:

- some units may run a Modbus master role when they need to initiate remote control
- some units may run a Modbus slave role when they expose local relay control to other units
- a unit could support both later, but that is not required by the current migration plan

We should avoid assuming that only one unit in the whole system can ever be the master unless a real deployment need forces that constraint.

### Practical Interpretation Of The Existing Python

The current Python script already reflects this asymmetric transport model.

In client mode it:

- listens for local events
- maps those events to Modbus coil writes
- sends requests as the Modbus initiator

In server mode it:

- exposes local Modbus control points
- receives coil writes
- maps them to local relay actions

`nest` should preserve that transport shape while moving the higher-level logic into the controller and registry layers.

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
- What exact Modbus runtime configuration is needed to declare master versus slave mode?
- Should the slave side emit controller-visible relay observations after incoming writes, or execute locally without publishing a transport-derived event?
- How should resolved remote actuator targets map to slave address and coil address in phase 8?
- When a unit supports both local and remote light control, what is the smallest clean runtime wiring for that mixed role?
