# Unipi Sysfs Interface

## Overview

Unipi devices expose I/O through Linux sysfs at `/run/unipi` or `/sys/devices/platform/unipi_plc/`.

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

Format: `{type}-{io_group}-{number:02d}`

Examples:

- `di-2-15` = Digital Input, IO Group 2, Number 15
- `ro-3-14` = Relay Output, IO Group 3, Number 14

## Polling Interface

The current implementation uses polling (not inotify/fanotify):

1. Open the value file
2. Seek to start
3. Read 1 byte (0 or 1)
4. Compare with previous value
5. If changed, emit event
6. Sleep for poll interval (250ms default)
7. Repeat

## Nest Implementation

From `pkg/device/device.go`:

```go
const (
    pollIntervalMillis = 250
)

var filenameRegex = regexp.MustCompile(
    `/io_group(1|2|3)/(?P<device_fmt>di|do|ro)_(?P<io_group>1|2|3)_(?P<number>[0-9]{2})/(di|do|ro)_value$`
)

type Device struct {
    Path       string
    ReadEvents chan<- DevicePayload
    // ...
}

func (d *Device) Read() (DevicePayload, error) {
    // Open file, seek to start, read 1 byte
    // Return MessageType_TurnOn or MessageType_TurnOff
}

func (d *Device) Write(payload bool) error {
    // Open file, write "1" or "0"
}
```

## Path Mapping

| Sysfs Path                            | Device ID | Type          |
| ------------------------------------- | --------- | ------------- |
| `/sys/.../io_group2/di_2_15/di_value` | `di-2-15` | Digital Input |
| `/sys/.../io_group3/ro_3_14/ro_value` | `ro-3-14` | Relay Output  |

## Testing Fixtures

Test fixtures at `test/fixtures/sys/` mirror the real structure for unit testing.
