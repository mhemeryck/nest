# Implementation Plan

## Overview

Phased approach to building `nest`, a universal controller for Unipi hardware.

This plan is ordered around gradual migration from the existing controller setup.
The first priority is to make `nest` deployable to the Raspberry Pi controller and safe to run beside the current system.
After that, `nest` should observe real sysfs behavior and publish what it sees before it takes ownership of relay outputs.

MQTT should be added early as a passive observability and contract-discovery interface.
Each unit should publish state topics and a retained autodiscovery document that describes the command and state topics it exposes.
Command topics may be advertised before command handling is enabled, but the discovery payload must make that disabled state explicit.

The first control migration should be lights rather than covers.
The existing light setup spans multiple physical units, so distributed light control and Modbus RTU behavior need to be proven before cover control.
Covers come later because motor control adds safety requirements around up/down relay interlocks, timing, and restart recovery.

## Phase 1: Sysfs Layer

- [x] Read digital inputs from sysfs (`/sys/devices/platform/unipi_plc/...`)
- [x] Write relay outputs to sysfs
- [x] Basic error handling for hardware access
- [x] Worker pool for configurable poll intervals per device type

**Deliverable**: Reliable low-level sysfs primitives for reading, writing, and polling devices.

## Phase 2: Minimal Local Model

- [x] Define a minimal config schema for local devices and mappings
- [x] Load config from file and validate required fields
- [x] Represent local lights and relay targets without transport concerns
- [x] Build startup indexes for local entities and bindings
- [x] Support local `push_button -> light -> relay` execution without transport concerns

**Deliverable**: `nest` can describe and execute a single-unit local light setup from configuration.

## Phase 3: Build and Deploy Pipeline

- [x] Add GoReleaser configuration
- [x] Build Linux ARM artifacts for the Raspberry Pi controller
- [x] Keep CI checks for vet, tests, and normal builds
- [x] Run validation on pull requests through a reusable workflow
- [x] Support manual snapshot artifact builds for a selected ref
- [x] Upload manual snapshot artifacts for controller deployment
- [x] Create automatic GitHub releases on merges to `master`
- [x] Use UTC CalVer release tags with date, hour, minute, and second
- [x] Attach controller artifacts and checksums to GitHub releases
- [x] Include generated changelogs in GitHub releases

**Deliverable**: `nest` can produce controller-ready artifacts on demand and publish release artifacts automatically from `master`.

## Phase 4: Passive MQTT Observability and Discovery

- [ ] Add MQTT broker configuration
- [ ] Publish unit availability
- [ ] Publish observed raw sysfs state changes
- [ ] Publish mapped digital input events and state
- [ ] Publish mapped relay state changes
- [ ] Publish a retained unit autodiscovery document on a dedicated discovery topic
- [ ] Include state topics, command topics, entity IDs, capabilities, and command enablement in discovery
- [ ] Do not subscribe to command topics yet
- [ ] Do not let MQTT behavior write relay outputs yet

**Deliverable**: `nest` can run beside the current setup and expose what it observes without taking control.

## Phase 5: MQTT Contract Hardening

- [ ] Stabilize the MQTT topic structure
- [ ] Stabilize the autodiscovery payload schema
- [ ] Add schema versioning for discovery documents
- [ ] Decide retained vs non-retained behavior per topic class
- [ ] Handle reconnects and republish availability and discovery
- [ ] Add logging for publish failures and dropped messages
- [ ] Add tests for generated topics and discovery payloads

**Deliverable**: MQTT telemetry and discovery are reliable enough to guide migration decisions.

## Phase 6: Input Semantics

- [ ] Debounce logic for inputs
- [ ] Edge handling beyond rising-edge only
- [ ] Press and release button event semantics
- [ ] Multi-input triggers for the same light or relay target
- [ ] Deterministic behavior for repeated physical button events

Current status:
Rising-edge to `pressed` push button events exists already.
Debounce and richer button semantics still need to be added.

**Deliverable**: Stable local input handling that can support real wall-switch behavior.

## Phase 7: Distributed Light Control Model

- [ ] Define input-unit to output-unit light mappings
- [ ] Represent light bindings that can target local or remote relays
- [ ] Keep command source, transport, and actuator execution separate
- [ ] Support deterministic light toggle behavior across units
- [ ] Read or observe relay state before toggling when needed
- [ ] Keep MQTT observability active during light migration

**Deliverable**: The existing multi-unit light topology can be represented in `nest` configuration and runtime indexes.

## Phase 8: Modbus RTU Transport

- [ ] Serial port configuration
- [ ] Modbus unit ID configuration
- [ ] Map remote relay targets to Modbus coils
- [ ] Read coils for relay state feedback
- [ ] Write coils for relay control
- [ ] Define retry and error behavior for transient serial failures
- [ ] Decide whether output units execute commands directly or expose relay coils only

**Deliverable**: `nest` can execute distributed light control across units via RS-485.

## Phase 9: Light Control Migration

- [ ] Start with one migrated light circuit
- [ ] Enable relay writes only for selected migrated lights
- [ ] Compare command execution and relay feedback through MQTT observability
- [ ] Expand migration circuit by circuit
- [ ] Keep existing controller behavior available until each circuit is verified

**Deliverable**: Existing distributed light control is migrated safely to `nest` before cover control begins.

## Phase 10: Local Cover Controller

- [ ] Config-driven cover model
- [ ] Covers composed from `up_relay` and `down_relay`
- [ ] Config-driven mapping of inputs to cover actions
- [ ] Open, close, stop, and toggle commands
- [ ] Up/down relay control with safety interlocks
- [ ] End-to-end single-unit control loop

**Deliverable**: Single-unit shade control works reliably on one hardware unit.

## Phase 11: MQTT Commands

- [ ] Subscribe to command topics already advertised in autodiscovery
- [ ] Require explicit config before commands are enabled
- [ ] Reflect command enablement in the autodiscovery payload
- [ ] Accept a minimal command set for migrated lights
- [ ] Accept a minimal command set for covers after local cover behavior is proven
- [ ] Publish resulting state changes after command execution

**Deliverable**: External systems can control migrated entities through MQTT.

## Phase 12: Cover Refinement

- [ ] Position tracking
- [ ] Timing calibration and `max_time` handling
- [ ] Recovery behavior after restart or interrupted movement

**Notes:**

- Covers should build on the same config and event model defined earlier
- Safety rules must ensure up/down relays are never active simultaneously

**Deliverable**: Cover entities behave predictably under real-world timing and interruption scenarios.

## Phase 13: MQTT and Home Assistant Expansion

- [ ] Refine topic structure as needed for Home Assistant integration
- [ ] Add Home Assistant auto-discovery if it still fits the design
- [ ] Finalize availability and state payloads
- [ ] Document deployment and migration behavior

**Deliverable**: MQTT interface evolves from a development/testing interface into a stable HA-facing integration.

## Early Decisions

These should be decided before transport and entity complexity increase.

- [ ] Testing strategy (mock sysfs vs real hardware)
- [x] Config file shape and CLI interface design
- [ ] Local event model for buttons, toggles, and repeated presses
- [ ] MQTT topic and autodiscovery schema
- [ ] Modbus topology and relay addressing model

## Open Questions

- [ ] Cover support timing requirements
- [ ] Output unit relay state publishing to MQTT
- [ ] Final topic structure and HA-facing MQTT contract
- [ ] Whether command topics should be advertised for disabled entities
- [ ] Whether Modbus output units execute semantic commands or expose coil-level relay control
