# Unipi Sysfs Interface

## Overview

Nest discovers device value files beneath `<root>/sys/devices/platform/unipi_plc/`.
The configured root defaults to `/`; fixtures use `test/fixtures`.

## Directory Structure

```
/sys/devices/platform/unipi_plc/
├── io_group1/
│   ├── di_1_01/
│   │   ├── di_value          # Digital input value (0 or 1)
│   │   ├── counter           # Event counter
│   │   └── ...
│   ├── do_1_01/
│   │   ├── do_value          # Digital output value (0 or 1)
│   │   └── ...
│   └── ro_1_01/
│       ├── ro_value          # Relay output value (0 or 1)
│       └── ...
├── io_group2/
│   └── ...
└── io_group3/
    └── ...
```

## Device Types

### Digital Input (DI)

Used for push buttons, sensors, contact switches.

| File       | Type | Description                 |
| ---------- | ---- | --------------------------- |
| `di_value` | read | Current state (0=off, 1=on) |

### Digital Output (DO)

General purpose digital outputs (not used in current setup).

| File       | Type       | Description                 |
| ---------- | ---------- | --------------------------- |
| `do_value` | read/write | Current state (0=off, 1=on) |

### Relay Output (RO)

Used for controlling lights, motors, motorized shades.

| File       | Type       | Description                 |
| ---------- | ---------- | --------------------------- |
| `ro_value` | read/write | Current state (0=off, 1=on) |

## Device Naming

Format: `{type}_{io_group}_{number:02d}`

Examples:

- `di_2_15` = Digital Input, IO Group 2, Number 15
- `ro_3_14` = Relay Output, IO Group 3, Number 14

## Polling Interface

The current implementation uses polling (not inotify/fanotify):

1. Discover devices and read their initial values
2. Group devices by type, with one worker per type
3. Read each value file on the worker's polling tick
4. Interpret the first byte as `0` or `1`
5. Compare with the cached value and emit changed observations

Relay workers also publish initial relay observations at startup.
Read failures during polling skip that observation.

Default polling intervals:

| Device type    | Interval |
| -------------- | -------- |
| Digital input  | 100 ms   |
| Digital output | 100 ms   |
| Relay output   | 1 s      |

Override these values under `units.<unit>.actors.sysfs.poll_intervals` with `digital_input`, `digital_output`, and `relay_output` durations.

## Nest Implementation

- [Device discovery and value access](../internal/sysfs/device.go)
- [Worker configuration and polling defaults](../internal/sysfs/config.go)
- [Polling and output execution](../internal/sysfs/worker.go)
- [Command and observation types](../internal/sysfs/actor.go)

The sysfs actor accepts toggle, on, and off commands addressed by device ID.
Workers serialize commands and polling for their device type.
Output commands read the current value before writing and update cached state after successful writes.
Changed writes emit state observations; unchanged successful writes do not emit completion events.
Command failures are logged, not returned through a command-result channel.
This interface is not yet a cover-control execution contract.

## Path Mapping

| Sysfs Path                            | Device ID | Type          |
| ------------------------------------- | --------- | ------------- |
| `/sys/.../io_group2/di_2_15/di_value` | `di_2_15` | Digital Input |
| `/sys/.../io_group3/ro_3_14/ro_value` | `ro_3_14` | Relay Output  |

## Testing Fixtures

Test fixtures at `test/fixtures/sys/` mirror the real structure for unit testing.
