# Covers Controller

## Overview

Motorized window shades controlled via relays on Unipi Neuron, with both physical push buttons and Home Assistant integration.

## Status

Nest implements a standalone movement FSM in `internal/controller/cover`.
Cover configuration, entity translation, and registry lookups are implemented.
Nest implements sysfs cover execution, local button bindings, and time-based position estimates.
MQTT cover integration remains pending.
The Nest diagram below describes current movement decisions, not confirmed hardware state.
The later legacy sections describe the earlier cover service and design ideas, not implemented Nest behavior.
See [Implementation Plan](implementation-plan.md#phase-9-local-cover-controller) for the proposed migration scope.

## Nest Movement FSM

```mermaid
stateDiagram-v2
    [*] --> Stopped
    Stopped --> Opening: Open / StartOpening (set deadline)
    Stopped --> Closing: Close / StartClosing (set deadline)
    Stopped --> Stopped: Stop / StopMovement
    Stopped --> Stopped: CheckDeadline / no effect
    Opening --> Opening: Open / no effect (preserve deadline)
    Closing --> Closing: Close / no effect (preserve deadline)
    Opening --> Stopped: Close or Stop / StopMovement (clear deadline)
    Closing --> Stopped: Open or Stop / StopMovement (clear deadline)
    Opening --> Stopped: Deadline reached / StopMovement (clear deadline)
    Closing --> Stopped: Deadline reached / StopMovement (clear deadline)
```

- Labels show input or condition, followed by effect and deadline change
- State describes movement intent, not output confirmation or estimated position
- The initial Stopped state does not confirm that hardware outputs are off
- At or past the deadline, stopping takes precedence over every input
- Before the deadline, `CheckDeadline` has no effect in either moving state
- An opposite-direction request stops movement without queuing a reversal
- A subsequent direction request can start another movement
- The sysfs runtime owner owns timers and submits `CheckDeadline` with the current time
- Deadline checks use the current state, so stale notifications cannot stop a newer movement early
- Effects perform no I/O; runtime integration must enforce relay interlocks and handle output failures

## Runtime Ownership

The sysfs actor owns each local cover's FSM state, deadline timer, relay execution, and cleanup.
The controller routes semantic cover requests and normalizes cover observations.
The controller maps local button events to cover requests.
The commands channel carries cover requests; the states channel carries cover state and failures.
Neither channel requires an acknowledgement for each relay write.
Cover observations identify the cover rather than repeat hardware addresses or output-pair commands.

This deliberately makes sysfs cover-aware to avoid cross-layer movement coordination.
The pure FSM remains unchanged and independent of I/O.
Its current location under `internal/controller/cover` does not assign runtime ownership to the controller.
See [Local Cover Execution Ownership](design.md#local-cover-execution-ownership) for responsibilities and safety constraints.
Cover-owned outputs are excluded from ordinary polling workers and reject direct single-device commands.
One actor-local timer schedules the earliest movement deadline.
Cover snapshots coalesce when consumers are slow; they do not guarantee delivery of every intermediate transition.
Output failures stop the actor and runtime after cleanup attempts.
Failure snapshots include the previous movement state and an error, not a claim that the outputs stopped.
Runtime errors preserve failures even if the states channel cannot accept a failure snapshot.
Shutdown attempts to turn both outputs off for every cover without waiting for observation delivery.
Cleanup failures are logged and returned; they cannot guarantee that the hardware stopped.
Existing light control remains in the controller and continues to use single-device sysfs commands.

## Nest Position Estimates

- Start at unknown after each restart
- Keep position unknown after a stop before the first movement deadline
- Estimate 100% after an opening deadline and 0% after a closing deadline
- Apply the endpoint only after both relays turn off successfully
- Estimate later positions from elapsed movement time and the last known estimate
- Freeze the estimate when either direction stops
- Report known estimates in movement snapshots and once per second during movement
- Keep the movement deadline active regardless of the position estimate

The estimate assumes a full travel fits within the configured maximum run duration.
Verify that assumption on hardware before relying on the estimate.

## Nest Configuration

Set one global maximum run duration for all covers:

```yaml
cover_control:
  max_run: 30s
```

Omitting `max_run` or setting it to zero selects the 30-second default.
Negative values are rejected.
This duration bounds relay activation and supplies the full-travel time for position estimates.
Per-cover overrides are not supported.

Declare covers under each unit's `entities`:

```yaml
covers:
  - id: office
    name: Office
    open_actuator: { actor: sysfs, kind: relay, id: office_up }
    close_actuator: { actor: sysfs, kind: relay, id: office_down }
```

Actuator references resolve to relays under the same unit's `actors.sysfs.relays`.
Each cover requires two distinct, exclusively owned physical relays.
Neither relay can serve another cover or a light, including through a differently named alias.
The semantic cover ID is `<unit>.cover.office`.
The cover entity contains identity and typed relay references, not live movement state or a per-cover duration.
The registry exposes shared cover settings, cover lookup by ID, and owner lookup by relay ID.
MQTT cover commands remain deferred.
See [the cover fixture](../test/fixtures/config.covers.yaml) for a complete example with two units and a separate light.
Starting the runtime initializes configured covers with both outputs off and enables their local execution owner.
Loading or validating configuration alone does not start cover control.

## Hardware

### Wiring

- **Up relay**: Opens shade (connected to motor's "up" wire)
- **Down relay**: Closes shade (connected to motor's "down" wire)
- **Push buttons**: Wall-mounted toggle buttons connected to Unipi DI

### Relay Logic

Motorized shades typically use two relays:

- **Relay ON (1)**: Activates the relay, motor runs
- **Relay OFF (0)**: Deactivates the relay, motor stops

**Important:** Both relays should never be ON simultaneously (short circuit protection).

## Legacy State Machine

```
        ┌──────────────┐
        │   STOPPED    │
        │  (idle)      │
        └──────┬───────┘
               │
     ┌─────────┴─────────┐
     │                   │
     ▼                   ▼
┌─────────────┐   ┌─────────────┐
│  OPENING    │   │  CLOSING    │
│ (motor up)  │   │ (motor down)│
└─────────────┘   └─────────────┘
     │                   │
     │           ┌───────┘
     ▼           ▼
┌─────────────┐
│    OPEN     │
│ (position=100)│
└─────────────┘

┌─────────────┐
│   CLOSED   │
│ (position=0)│
└─────────────┘
```

### State Definitions

| State     | Motor           | Position | Description              |
| --------- | --------------- | -------- | ------------------------ |
| `OPEN`    | Both OFF        | 100      | Shade fully open         |
| `CLOSED`  | Both OFF        | 0        | Shade fully closed       |
| `OPENING` | Up ON, Down OFF | 0→100    | Motor running up         |
| `CLOSING` | Up OFF, Down ON | 100→0    | Motor running down       |
| `STOPPED` | Both OFF        | 0-100    | Motor stopped mid-travel |

### Transitions

| From                    | Command       | To      |
| ----------------------- | ------------- | ------- |
| STOPPED/OPENING/CLOSING | `OPEN`        | OPENING |
| STOPPED/OPENING/CLOSING | `CLOSE`       | CLOSING |
| OPENING/CLOSING         | `STOP`        | STOPPED |
| OPENING                 | (reaches 100) | OPEN    |
| CLOSING                 | (reaches 0)   | CLOSED  |

## Position Tracking

Position is tracked as percentage (0-100):

- `0` = fully closed
- `100` = fully open

**Tracking Method:** Time-based estimation

- Configure `max_time` (seconds to go from 0→100)
- Increment position every `sleep_time` based on direction

**Edge Cases:**

- Position stops at boundaries (0 and 100)
- Motor automatically stops at limits (if properly wired)
- If stopped mid-travel, position is preserved

## MQTT Interface

### Topics

```
covers/{name}/command     # Incoming commands (OPEN/CLOSE/STOP)
covers/{name}/state       # Current state (OPEN/OPENING/STOPPED/CLOSING/CLOSED)
covers/{name}/position    # Position as integer 0-100
shady/{relay}/command      # Relay commands (ON/OFF)
shady/{relay}/state       # Relay state (ON/OFF)
```

### Messages

**Commands (incoming):**

- `OPEN` - Start opening
- `CLOSE` - Start closing
- `STOP` - Stop motor

**States (outgoing):**

- `open`
- `closed`
- `opening`
- `closing`
- `stopped`

**Position:** Integer `0` to `100`

## Home Assistant Integration

### MQTT Cover Configuration

```yaml
cover:
  - platform: mqtt
    name: "office"
    command_topic: "covers/office/command"
    state_topic: "covers/office/state"
    position_topic: "covers/office/position"
    set_position_topic: "covers/office/position/set"
    device_class: shade
    optimistic: true
```

### Push Button Automation

```yaml
automation:
  - alias: Office toggle open start
    trigger:
      platform: state
      entity_id: switch.office_open
      from: "off"
      to: "on"
    action:
      service: cover.open_cover
      entity_id: cover.office

  - alias: Office toggle open stop
    trigger:
      platform: state
      entity_id: switch.office_open
      from: "on"
      to: "off"
    action:
      service: cover.stop_cover
      entity_id: cover.office
```

## Current Implementation (covers/)

**Language:** Python (asyncio)\
**Architecture:** MQTT → covers.py → MQTT (relay commands)

**Limitations:**

- Depends on evok2mqtt for relay control
- Requires MQTT broker
- Position tracking via MQTT relay states

## Planned Implementation (nest)

**Language:** Go\
**Architecture:** Direct sysfs → nest → MQTT

**Advantages:**

- Direct relay control (no evok2mqtt dependency)
- Standalone from Home Assistant
- Single binary deployment
- HA still receives state updates via MQTT

**Todo:**

- [x] MQTT client and Home Assistant light integration; cover integration remains pending
- [ ] Cover entity with state machine
- [ ] Position tracking
- [ ] Push button input handling
- [ ] Relay output control
- [ ] Configuration via YAML

## Configuration Schema

```yaml
covers:
  office:
    up_relay: "ro-3-14"
    down_relay: "ro-3-13"
    max_time: 30 # seconds
  bedroom:
    up_relay: "ro-3-11"
    down_relay: "ro-3-12"
    max_time: 25

mqtt:
  host: "localhost"
  port: 1883
  base_topic: "covers"

sysfs:
  path: "/run/unipi"
```
