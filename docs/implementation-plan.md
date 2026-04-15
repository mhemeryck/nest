# Implementation Plan

## Overview

Phased approach to building Nest, a universal controller for Unipi hardware.

## Phase 1: Sysfs Layer

- [ ] Read digital inputs from sysfs (`/sys/class/gpio/...`)
- [ ] Write relay outputs to sysfs
- [ ] Basic error handling for hardware access

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
- [ ] Testing strategy (mock sysfs vs real hardware)
- [ ] Multi-input triggers (same light, multiple buttons)
- [ ] Output unit relay state publishing to MQTT
