# Distributed Light Model

## Purpose

This note captures the naming and routing direction for distributed light control.
It is the design reference for implementation plan phase 7.

The immediate goal is to let `nest` represent cross-unit light control cleanly before Modbus execution exists.
The longer-term goal is to let sysfs, MQTT, and Modbus act as transport actors behind the same controller-level semantic model.

## Core Direction

The domain model should use globally unique semantic entity IDs.
Those IDs should be separate from actor-local transport addresses.
The canonical configuration should describe the whole installation as one global tree.

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
In the global config tree, these IDs are derived from the owning unit, the entity section, and the local bare ID.

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
In the global config tree, these addresses live under unit-local actor sections such as `units.<unit>.actors.sysfs`.

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
It also avoids mixing transport ownership and addressing concerns directly into semantic entity definitions.

That matters because:

- Modbus should become another actor rather than a special case in the light model.
- MQTT should also be able to carry cross-unit commands when that is the right routing path.
- the controller should operate on semantic entities and resolved command targets rather than on transport-specific identifiers.

## Ownership Convention

The leading hardware-unit segment in the semantic ID indicates the owning unit namespace.
For example, `garage_io.light.driveway` belongs to the `garage_io` unit namespace.

This is a naming convention first.
The global config tree should make that ownership explicit structurally as well.
We should avoid pushing too much routing logic into string parsing beyond the owning-unit prefix unless there is a concrete need.

## Config Direction

Configuration should eventually express shared actor config, unit-local actor resources, and semantic entities in one global tree.

Preferred top-level structure:

```text
actors
units
bindings
```

Where:

- `actors` contains shared actor configuration when it is truly shared across units
- `units` contains unit-local actor resources and semantic entities
- `bindings` contains house-wide behavior and cross-unit relationships

Per-unit actor participation should be explicit.
We should avoid inheritance or implicit enablement rules.

Within typed sections, local references should use bare IDs.
The surrounding unit and section provide the namespace and type context.

Illustrative direction:

```yaml
actors:
  mqtt:
    broker:
      host: localhost
      port: 1883
      topic_prefix: nest

units:
  panel_1:
    actors:
      mqtt:
        enabled: true
      sysfs:
        digital_inputs:
          - id: entry_left
            device: di_3_16
          - id: entry_right
            device: di_3_17
        relays:
          - id: entry_ceiling
            device: ro_3_14
      modbus:
        role: master
        port: /dev/ttyNS0
        baudrate: 19200
        slaves:
          garage_io:
            unit_id: 1

    entities:
      buttons:
        - id: entry_left
          input: entry_left
        - id: entry_right
          input: entry_right
      lights:
        - id: entry
          actuator: entry_ceiling

  garage_io:
    actors:
      mqtt:
        enabled: true
      sysfs:
        relays:
          - id: driveway
            device: ro_2_01
      modbus:
        role: slave
        port: /dev/ttyNS0
        baudrate: 19200
        unit_id: 1
        coils:
          - id: driveway
            address: 1
            target: driveway

    entities:
      lights:
        - id: driveway
          actuator: driveway

bindings:
  - source: panel_1.button.entry_left
    target: panel_1.light.entry
    action: toggle
  - source: panel_1.button.entry_right
    target: garage_io.light.driveway
    action: toggle
```

This example is directional rather than final schema.
It shows the intended layering:

- shared actor configuration stays out of unit blocks when it is global
- unit blocks own local actor resources and semantic entities
- semantic IDs are globally unique after unit and section qualification
- local hardware mapping stays explicit
- bindings target semantic entities rather than raw transport addresses

Derived fully qualified semantic IDs from the example include:

- `panel_1.button.entry_left`
- `panel_1.button.entry_right`
- `panel_1.light.entry`
- `garage_io.light.driveway`

Local actor resource IDs remain local to their unit and actor section.
For example, `panel_1` can refer to sysfs relay `entry_ceiling` without repeating `relay.` in the local ID.
The surrounding section already supplies that meaning.

## Runtime Direction

The controller should continue to normalize actor observations into semantic events.
It should also resolve semantic targets into transport-specific commands through registries or indexes.
Those registries or indexes should be built from the global config tree or from a unit-local projection derived from it.

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

Within one unit, local references should remain short and section-scoped.
At runtime, those local references are resolved to fully qualified semantic IDs and actor-local addresses through the owning unit context.

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
That semantic ownership is described in the unit subtree and does not depend on the Modbus role used at runtime.

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

- adopting the global `actors` / `units` / `bindings` config tree
- adopting globally unique semantic IDs derived from unit, entity type, and bare local ID
- separating semantic IDs from actor-local sysfs and Modbus identifiers
- reshaping bindings to target semantic entities cleanly
- preparing registry or index resolution for future transport routing and unit-local projection

Phase 8 can then add Modbus-specific addressing and execution as an actor concern.

## Open Questions

- Should actuator references remain direct entity IDs or become a more explicit target type?
- How much routing should be inferred from the unit prefix versus declared explicitly in config?
- When cross-unit command routing exists, how should MQTT and Modbus be prioritized or selected?
- What exact Modbus runtime configuration is needed to declare master versus slave mode?
- Should the slave side emit controller-visible relay observations after incoming writes, or execute locally without publishing a transport-derived event?
- How should resolved remote actuator targets map to slave address and coil address in phase 8?
- When a unit supports both local and remote light control, what is the smallest clean runtime wiring for that mixed role?
