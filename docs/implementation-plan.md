# Implementation Plan

## Overview

Phased approach to building Nest, a universal controller for Unipi hardware.

## Phase 1: Sysfs Layer

- [x] Read digital inputs from sysfs (`/sys/devices/platform/unipi_plc/...`)
- [x] Write relay outputs to sysfs
- [x] Basic error handling for hardware access
- [x] Worker pool for configurable poll intervals per device type

**Deliverable**: Can read input state and toggle relays locally.

## Phase 2: Local Input→Relay Mapping

- [ ] Config-driven mapping of inputs to relays
- [ ] Debounce logic for inputs
- [ ] Support for lights (toggle behavior)

**Deliverable**: Single-unit control loop working.

## Phase 3: Cover Support

- [ ] Up/down relay control with max_time
- [ ] Position tracking
- [ ] Open/close/stop/toggle commands

**Deliverable**: Cover entities work locally.

## Phase 4: Modbus RTU

- [ ] Serial port configuration
- [ ] Coil read/write over Modbus
- [ ] Input unit: sends commands
- [ ] Output unit: receives and executes

**Deliverable**: Multi-unit communication via RS-485.

## Phase 5: MQTT Integration

- [ ] State publishing
- [ ] Command subscription
- [ ] Home Assistant auto-discovery (future)

**Deliverable**: HA integration ready.

## Open Questions

- [ ] Cover support timing requirements
- [x] Testing strategy (fixtures in test/fixtures/)
- [ ] Multi-input triggers (same light, multiple buttons)
- [ ] Output unit relay state publishing to MQTT
