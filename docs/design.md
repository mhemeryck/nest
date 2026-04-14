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

### Transport Options

| Transport | Use Case | Notes |
|-----------|----------|-------|
| **Modbus RTU** | Inter-unit communication | RS-485 serial, reliable |
| **MQTT** | HA integration | Publish state, subscribe commands |
| **Sysfs** | Local relay control | For covers, same-unit setups |

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
      coil: N          # Modbus coil address
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
