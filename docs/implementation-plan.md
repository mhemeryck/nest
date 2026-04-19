# Implementation Plan

## Overview

Phased approach to building `nest`, a universal controller for Unipi hardware.

This plan is ordered to reduce risk early.
It starts with direct hardware access, then local control semantics, and only
after that adds higher-level entity behavior and transport integrations.

The first priority is a reliable local controller.
Once that foundation is stable, `nest` can take over responsibilities currently
handled across multiple repositories.

Because the current light setup spans multiple physical units, the first full
vertical slice should target single-unit shade control.
That keeps the initial implementation local to one device, even if the shade
logic itself is more complex than simple light toggling.

MQTT should be exposed relatively early as a thin interface.
That allows command and state flows to be exercised before the full system is
complete, and gives better observability while refining controller behavior.

## Phase 1: Sysfs Layer

- [x] Read digital inputs from sysfs (`/sys/devices/platform/unipi_plc/...`)
- [x] Write relay outputs to sysfs
- [x] Basic error handling for hardware access
- [x] Worker pool for configurable poll intervals per device type

**Deliverable**: Reliable low-level sysfs primitives for reading, writing, and polling devices.

## Phase 2: Minimal Config Model

- [ ] Define a minimal config schema for local devices and mappings
- [ ] Load config from file and validate required fields
- [ ] Represent local lights and relay targets without transport concerns

**Deliverable**: `nest` can describe a single-unit setup from configuration.

## Phase 3: Local Cover Controller

- [ ] Config-driven mapping of inputs to cover actions
- [ ] Up/down relay control with safety interlocks
- [ ] Open/close/stop/toggle commands
- [ ] End-to-end single-unit control loop

**Deliverable**: Single-unit shade control working reliably on one hardware unit.

## Phase 4: MQTT Interface

- [ ] Publish cover state changes to MQTT
- [ ] Accept a minimal command set over MQTT
- [ ] Keep topic structure and payloads simple while the domain model stabilizes

**Deliverable**: `nest` can be observed and controlled externally during development.

## Phase 5: Input Semantics

- [ ] Debounce logic for inputs
- [ ] Edge handling and button event semantics
- [ ] Multi-input triggers for the same light or relay

**Deliverable**: Stable local input handling that can support real wall-switch behavior.

## Phase 6: Cover Refinement

- [ ] Position tracking
- [ ] Timing calibration and `max_time` handling
- [ ] Recovery behavior after restart or interrupted movement

**Notes:**

- Covers should build on the same config and event model defined earlier
- Safety rules must ensure up/down relays are never active simultaneously

**Deliverable**: Cover entities behave predictably under real-world timing and interruption scenarios.

## Phase 7: Distributed Light Control

- [ ] Define input-unit to output-unit light mappings
- [ ] Support deterministic light toggle behavior across units
- [ ] Reuse the config and event model proven by local cover control

**Deliverable**: Light control model is defined in a way that can be executed over inter-unit transport.

## Phase 8: Inter-Unit Transport

Inter-unit transport comes after local control and the external MQTT interface
are proven.
At that point, the remaining transport work is focused on cross-unit execution
rather than local observability or testability.

### Modbus RTU

- [ ] Serial port configuration
- [ ] Coil read/write over Modbus
- [ ] Input unit: sends commands
- [ ] Output unit: receives and executes

**Deliverable**: Multi-unit communication via RS-485 for distributed lighting and other cross-unit control.

### MQTT Expansion

- [ ] Refine topic structure as needed for Home Assistant integration
- [ ] Add Home Assistant auto-discovery if it still fits the design

**Deliverable**: MQTT interface evolves from a development/testing interface into a stable HA-facing integration.

## Early Decisions

These should be decided before transport and entity complexity increase.

- [ ] Testing strategy (mock sysfs vs real hardware)
- [ ] Config file shape and CLI interface design
- [ ] Local event model for buttons, toggles, and repeated presses

## Open Questions

- [ ] Cover support timing requirements
- [ ] Output unit relay state publishing to MQTT
- [ ] Final topic structure and HA-facing MQTT contract
