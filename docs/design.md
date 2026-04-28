# Nest Design

## Overview

Universal controller for Unipi hardware. Replaces evok2mqtt + modbusbackup with a single config-driven binary.

## Goals

- **Standalone**: Runs on each Unipi unit independently
- **Reliable**: Direct sysfs access (no evok dependency)
- **Flexible**: Support lights, covers, and future entity types
- **Simple**: Config-driven, minimal abstractions

## Architecture

```
nest instance (per Unipi)
├── config.yaml          # Per-unit configuration
├── sysfs layer          # Direct hardware access
├── transport layer      # Modbus RTU / MQTT
└── (optional) HA integration
```

## Key Design Decisions

### Per-Unit Configs

Each unit has its own config. No shared "source of truth" config.

### Input-Centric Mappings

Actions originate at the input side. The sending unit knows what it controls.

```
Input unit: "I control the office lights"
Output unit: "I expose these relays"
```

### Controller-Centric Integration Topology

Runtime coordination should use a star topology around the central controller.
Each integration should have a command input channel and an event output channel.
The controller sends commands to integrations and consumes the resulting events.
Integrations should not communicate with each other directly.
The same channel should not be reused in both directions.

This keeps ownership explicit.
It also fits sysfs, MQTT, and Modbus well.
Each integration can own its protocol or device state locally while the controller remains the only router between domains.

Actor packages should stay focused on their own protocol or device behavior.
They should expose actor-specific observations and commands, such as `sysfs.StateChange`, `sysfs.Command`, or future actor equivalents.
They should not translate directly to other actors.

The controller owns runtime translation between actor-specific observations and controller-level events.
Event types should be data-only and describe the controller's semantic event language.
Controller-owned normalization code should translate actor observations plus registry lookups into those semantic events.
As new actors are added, each actor should add one controller-side normalization path into controller events rather than direct actor-to-actor translations.

The registry is the runtime lookup layer built from domain entities.
It maps configured entity relationships and actor-facing identifiers back to typed entities, such as sysfs device IDs to inputs or relays.
Future transport address lookup indexes can live in the registry when command handling needs them.
The registry should not carry actor runtime configuration, credentials, live clients, channels, or reconnect state.

The top-level `internal/nest` package owns application wiring.
It loads config, builds domain entities, builds registries, discovers configured devices, maps domain config into actor-specific runtime config, starts actors, and coordinates shutdown.
The `cmd/nest` package should remain a thin process entrypoint for CLI parsing, signal handling, and exit status.

### Runtime Flow

The controller is the central coordinator.
It reads state changes from integrations, translates them into domain events, applies bindings, and sends commands back to integrations.

For the current local sysfs flow, that looks like this.

```text
                states
  +------------------------------+
  |                              |
  v                              |
+--------+    domain logic    +------------+    commands    +--------+
| sysfs  | -----------------> | controller | -------------> | sysfs  |
| actor  |                    |            |                | actor  |
+--------+ <----------------- +------------+ <------------- +--------+
              owned state                           owned devices
```

The left and right `sysfs` boxes are the same actor viewed from two directions.
The important part is the direction of flow.
The controller reads `states` from the actor and writes `commands` back to it.

The current local light path is:

```text
sysfs state
  -> digital input event
  -> push button event
  -> binding lookup
  -> light lookup
  -> relay lookup
  -> sysfs command
```

That same pattern should extend to future integrations.

```text
             states                         commands
  +------+ ----------> +------------+ <---------- +------+
  | sysfs|             | controller |             | mqtt |
  +------+ <---------- +------------+ ----------> +------+
             commands                      states

  +--------+ ---------> +------------+ <--------- +-------+
  | modbus |            | controller |            | dali  |
  +--------+ <--------- +------------+ ---------> +-------+
             commands                      states
```

This keeps the runtime wiring explicit.
It also avoids a mesh where integrations talk directly to each other.

### Transport Options

| Transport      | Use Case                 | Notes                             |
| -------------- | ------------------------ | --------------------------------- |
| **Modbus RTU** | Inter-unit communication | RS-485 serial, reliable           |
| **MQTT**       | HA integration           | Publish state, subscribe commands |
| **Sysfs**      | Local relay control      | For covers, same-unit setups      |

## Config Structure

### Input Unit (Lights)

```yaml
serial:
  port: /dev/ttyNS0
  baudrate: 19200

mqtt:
  host: localhost
  prefix: nest/living-room

mappings:
  - input: di-2-03
    name: kelder inkom inbouw
    modbus:
      coil: 1
  - input: di-2-04
    name: kelder inkom opbouw
    modbus:
      coil: 2
```

### Output Unit (Relays)

```yaml
serial:
  port: /dev/ttyNS0
  baudrate: 19200
  unit_id: 1

mqtt:
  host: localhost
  prefix: nest/living-room

relays:
  - coil: 1
    sysfs: ro-2-01
    name: kelder inkom inbouw
  - coil: 2
    sysfs: ro-2-02
    name: kelder inkom opbouw
```

### Cover Unit

```yaml
mqtt:
  host: localhost
  prefix: nest/office

covers:
  office:
    up_relay: ro-3-14
    down_relay: ro-3-13
    max_time: 30

input:
  - sysfs: di-2-15
    name: office shade toggle
    action: toggle
```

## Mapping Reference

### Light Mapping (Input Unit)

```yaml
mappings:
  - input: di-{io_group}-{number}
    name: human-readable name
    modbus:
      coil: N # Modbus coil address
```

### Relay Definition (Output Unit)

```yaml
relays:
  - coil: N
    sysfs: ro-{io_group}-{number}
    name: human-readable name
```

## Future Considerations

- **MQTT auto-discovery** for HA
- **REST API** for terraform provisioning
- **Discovery** via mDNS (optional)

## Open Questions

- Should output unit publish relay state to MQTT?
- How to handle multi-input triggers (same light, multiple buttons)?
