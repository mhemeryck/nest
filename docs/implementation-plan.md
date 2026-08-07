# Implementation Plan

## Overview

Phased approach to building `nest`, a universal controller for Unipi hardware.

This plan is ordered around gradual migration from the existing controller setup.
The first priority is to make `nest` deployable to the Raspberry Pi controller and safe to run beside the current system.
After that, `nest` should observe real sysfs behavior and publish what it sees before it takes ownership of relay outputs.

MQTT should be added early as a passive observability and contract-discovery interface.
Each unit should publish state topics and a retained autodiscovery document that describes the command and state topics it exposes.
Command topics may be advertised before command handling is enabled, but the discovery payload must make that disabled state explicit.

The first control migration should be covers on a non-critical unit.
The existing light setup is operational and spans multiple physical units, so it should remain on the current controller while Modbus RTU and the longer-term hardware direction mature.
Covers are a suitable first migration target because their buttons and motor relays are on the same physical unit and can replace the existing `hausmaus` plus `covers` path as one standalone controller.

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

## Phase 4: Runtime Boundary Refactor

- [x] Document runtime package boundaries and refactor direction
- [x] Move top-level runtime wiring from `cmd/nest` into `internal/nest`
- [x] Keep `cmd/nest` focused on CLI parsing, signal setup, fatal logging, and process exit status
- [x] Add test coverage for `internal/nest` runtime validation and configured sysfs device filtering
- [x] Make controller events data-only semantic messages
- [x] Move registry-backed event normalization into controller-owned code
- [x] Represent local light control as `sysfs.StateChange -> push button event -> light event -> sysfs.Command`
- [x] Remove the intermediate digital input controller event until it represents a useful domain fact on its own
- [x] Make sysfs actor addresses explicit with `entity.SysfsDeviceID`
- [x] Rename configured input and relay actor-address fields to `SysfsDevice`
- [x] Add small registry lookup helpers where they make normalization or policy code clearer
- [x] Add a synchronous controller event dispatch boundary without introducing an internal event queue yet
- [x] Group controller helpers by phase so actor observation handling, dispatch, normalization, and light execution are separated by file
- [x] Decide that semantic event types are controller-owned and should live under `internal/controller/event`
- [x] Move semantic event types from `internal/event` to `internal/controller/event`
- [x] Keep registry app-wide for now because both `internal/nest` setup and controller policy use it
- [x] Defer revisiting registry ownership until a second actor adds non-sysfs lookup paths
- [x] Keep actor packages domain-agnostic and avoid passing registry or semantic controller events into actors
- [x] Defer `internal/actors/...` package grouping until a second actor makes the grouping useful

**Deliverable**: Runtime package boundaries are clear before MQTT, Modbus, and richer input semantics add more actor and translation paths.

## Phase 5: Passive MQTT Observability and Discovery

- [x] Add MQTT broker configuration
- [x] Treat MQTT as an actor with a command channel into MQTT and an event channel back to `nest`
- [x] Publish only semantic observations emitted by `nest` runtime or controller code
- [x] Publish unit availability
- [x] Publish mapped input observations and state
- [x] Publish mapped push button observations and state
- [x] Publish mapped relay state changes
- [x] Publish mapped light state changes
- [x] Publish a retained unit autodiscovery document on a dedicated discovery topic
- [x] Include state topics, command topics, entity IDs, capabilities, and command enablement in discovery
- [x] Do not subscribe to command topics yet
- [x] Do not let MQTT behavior write relay outputs yet
- [x] Add local MQTT fixture for manual broker verification
- [x] Document local Mosquitto and `mosquitto_sub` verification flow
- [x] ~~Consider replacing repeated controller dispatch parameters with a data-only dispatch context~~

**Deliverable**: `nest` can run beside the current setup and expose what it observes without taking control.

Boundary note:

- MQTT is a passive publisher for `nest` observations in this phase
- MQTT communication with the rest of the runtime uses two unidirectional channels
- `nest` sends publish commands to the MQTT actor
- the MQTT actor sends connection and publish status events back to `nest`
- MQTT must not consume from sysfs actor channels directly
- MQTT should publish semantic observations only, not raw sysfs diagnostics
- MQTT must not subscribe to command topics or produce actor commands

Current status:

- MQTT config, actor wiring, startup availability, retained unit discovery, and Home Assistant light discovery are implemented
- Earlier passive digital input, push button, and relay telemetry was superseded by the decision to expose Home Assistant-facing house entities rather than raw hardware diagnostics by default
- Light state publishing is derived from relay state for relay-backed local lights and uses a canonical JSON entity snapshot
- ~~Controller dispatch currently passes registry, sysfs command channel, MQTT command channel, and MQTT topics through several helpers~~
- Superseded by splitting semantic event dispatch into actor-specific command dispatchers instead of adding a context bag
- Reconnect republishing and offline availability are deferred to MQTT contract hardening unless needed earlier

Manual verification sample:

```text
nest/units/local/availability online
homeassistant/device/nest_local_unit/config {"dev":{"ids":["nest_local_unit"],...},"cmps":{"office_light":{...}}}
nest/units/local/lights/office_light/state {"state":"ON"}
nest/units/local/lights/office_light/state {"state":"OFF"}
```

## Phase 6: Home Assistant MQTT Discovery Contract

- [x] Treat Home Assistant MQTT discovery as the primary MQTT integration contract
- [x] Drop the unit-level `nest` discovery document in favor of the Home Assistant-facing contract
- [x] Define one Home Assistant device identity per physical `nest` controller unit
- [x] Publish one retained Home Assistant device discovery payload per controller unit
- [x] Define stable Home Assistant unique IDs derived from `unit_id` and entity IDs
- [x] Do not preserve current Home Assistant entity IDs during migration when it adds unnecessary complexity
- [x] Publish retained Home Assistant discovery components for configured lights
- [x] Advertise required Home Assistant light command topics and handle `ON` and `OFF` light commands for migrated local lights
- [x] Publish only meaningful Home Assistant entities by default, not raw hardware diagnostics
- [x] Lock canonical JSON state payloads for each entity class
- [x] Decide retained vs non-retained behavior per Home Assistant state topic class
- [x] Handle reconnects and republish Home Assistant discovery and availability
- [x] Add MQTT Last Will and graceful offline availability publishing
- [x] Add logging for publish failures and dropped messages
- ~~[ ] Decide whether dropped publish warnings need rate limiting or counters~~
- [x] Add tests for Home Assistant discovery topics and payloads

Notes from the current Home Assistant migration context:

- MQTT exists primarily to integrate with Home Assistant without maintaining YAML MQTT entity definitions
- Home Assistant is the primary MQTT consumer, so MQTT should expose the house model rather than the hardware implementation
- Newer Home Assistant setups should discover `nest` entities from retained MQTT discovery config topics
- Home Assistant discovery uses one device discovery payload per physical `nest` controller unit, grouping the entities that unit exposes
- `nest` should own hardware-control behavior such as wall-button-to-light mappings so that local control survives Home Assistant or MQTT outages
- Home Assistant should remain responsible for UI, dashboards, notifications, alarm orchestration, and higher-level time, sun, and external-service automations
- State and command topics may still live under a `nest/...` namespace as long as the Home Assistant discovery payloads reference them correctly
- State topics should publish canonical JSON entity snapshots, even for simple on/off entities
- The topic identifies the entity, while the payload carries the current entity state and dynamic attributes
- Static metadata belongs in Home Assistant discovery payloads or local `nest` configuration, not in every state update
- Hardware details such as sysfs device IDs, relay IDs, and raw input addresses should stay out of MQTT by default
- Diagnostic MQTT topics should only be added later for concrete debugging needs
- Home Assistant discovery should be verified locally with a broker and disposable Home Assistant instance before depending on it for migration
- MQTT Last Will and offline availability are useful, but should be part of contract hardening rather than the first passive publishing slice
- Reconnect handling should republish retained discovery and availability so subscribers recover after broker interruptions
- Reconnect republishing was verified locally against the disposable Home Assistant and Mosquitto stack
- Home Assistant discovery payloads, availability, and entity state payloads are retained so Home Assistant can recover current state after reconnects or restarts
- Retained MQTT command messages must be ignored so stale broker state cannot replay hardware actions after reconnect
- Dropped MQTT publish logging is useful for visibility, but may need rate limiting if frequent input changes happen while the broker is unavailable

**Deliverable**: `nest` publishes Home Assistant-compatible MQTT discovery and state messages for migrated entities, and accepts minimal Home Assistant light commands without making local hardware control depend on Home Assistant.

## Phase 7: Distributed Light Control Model

See [Distributed Light Model](distributed-light-model.md) for the current naming and routing direction.

- [x] Replace the local flat config with the global `actors` / `units` / `bindings` tree
- [x] Add explicit runtime unit selection via positional CLI argument
- [x] Project the selected unit subtree into a local runtime view
- [x] Represent semantic entity IDs as derived global IDs from unit, entity type, and bare local ID
- [x] Separate semantic entities from actor-local sysfs addresses through typed endpoint references
- [x] Reshape bindings around semantic `source` and `target` references for local light execution
- [x] Build distributed binding indexes from the unit-local projection
- [x] Keep local sysfs light execution working under the new model
- [x] Keep MQTT observability compatible with the new semantic model
- [x] Represent remote bindings from both source-local and target-local perspectives
- [x] Publish semantic source events over MQTT for projected local sources
- [x] Subscribe to semantic source events needed by target-local bindings
- [x] Execute target-local bindings from replicated MQTT source events
- [x] Represent abstract Modbus event-signal and state-poll routes without executing them yet

Current status:

- Global config loading and explicit CLI unit selection are implemented
- Unit projection derives globally qualified semantic button and light IDs such as `controller_1.button.office_button`
- Global entity config uses typed endpoint references such as `{actor: sysfs, kind: relay, id: office_light_relay}`
- The projection still adapts entities and actor resources into the existing local runtime config shape while runtime code catches up
- Semantic ID construction and validation helpers live in `internal/entity`
- Config-to-entity translation currently lives in `internal/config` to keep parsed YAML models and domain models separate
- Local bindings use generic semantic `source` and `target` references and are indexed by semantic source ID
- Cross-unit bindings are projected into source-local and target-local remote binding views
- Remote binding views are represented on `entity.Root` and indexed by semantic source ID in the registry
- Controller coverage now verifies that projected global config can drive local sysfs light execution with semantic button and light IDs
- MQTT topics and Home Assistant discovery keep local entity topic segments while command handling maps those segments back to semantic light IDs
- Binding execution strategy is now documented as transport-dependent: MQTT may use replicated semantic source events with target-side execution, while Modbus may deliver event signals through master writes or expose concrete command points depending on the actor model
- Source-local remote bindings now publish non-retained MQTT semantic source events for button press and release events
- Target-local remote bindings now subscribe to MQTT semantic source events and execute matching local light actions

Next useful checks:

- Exercise controller and registry behavior from the global config projection rather than hand-built bare-ID entity roots
- Keep Modbus event-signal representation separate from Modbus execution until the Modbus actor phase

**Deliverable**: The existing multi-unit light topology can be represented in the global config tree, projected into unit-local runtime indexes, and proven with MQTT-based semantic event sharing before Modbus execution is added.

## Phase 7.1: Runtime Readability Refactor

- [x] Simplify `internal/nest` startup flow so it reads as high-level runtime composition
- [x] Group actor channel, topic, subscription, and shutdown wiring behind small runtime helpers
- [x] Keep config, entity, registry, controller, sysfs, and MQTT package boundaries unchanged unless a concrete dependency issue appears
- [x] Make controller wiring easier to scan without introducing a generic context bag
- [x] Preserve existing behavior with tests before starting Modbus execution

**Deliverable**: Runtime wiring is easier to read and extend before Modbus adds another transport path.

## Phase 7.2: Runtime Registry Cleanup

- [x] Decide that `registry.Index` should become a runtime registry rather than a lookup-only index
- [x] Rename `registry.Index` to `registry.Registry`
- [x] Keep `entity.Root` as the bare projected domain model with entity lists, bindings, and config-derived settings
- [x] Use `registry.Registry` as the runtime translation catalog built from `entity.Root`
- [x] Include runtime data needed by actors and controllers in `registry.Registry` when it avoids passing both `entity.Root` and the registry
- [x] Keep registry fields private and expose package-level functions that take `reg *registry.Registry`
- [x] Return copied slices from registry functions so callers do not mutate registry-owned data accidentally
- [x] Update `internal/nest` runtime wiring to pass one explicit `reg` value into actor and controller setup
- [x] Update controller code to depend on `reg *registry.Registry` instead of both `root *entity.Root` and `index *registry.Index`
- [x] Keep MQTT topic generation and protocol-specific behavior in the MQTT package rather than moving actor-specific addressing into the registry

Current direction:

- `entity.Root` is the bare projected domain model produced by config-to-entity translation
- `registry.Registry` is the runtime lookup and translation catalog built from `entity.Root`
- `internal/nest` remains responsible for runtime composition, actor lifecycle, channel wiring, startup, and shutdown
- controller code uses the registry to translate observations and semantic events into follow-up events or actor commands
- actor packages keep ownership of external protocol details such as MQTT topics, sysfs crawling, and future Modbus transport addresses

**Deliverable**: Runtime code no longer needs to pass both the canonical entity root and lookup index when a single registry better represents the unit-local translation catalog.

## Phase 8: Modbus RTU Transport

- [x] Serial port configuration
- [x] Modbus unit ID configuration
- [ ] Map remote relay targets to Modbus coils
- [x] Read coils for relay state feedback
- [x] Write coils for relay control
- [ ] Define retry and error behavior for transient serial failures
- [ ] Decide whether output units execute commands directly or expose relay coils only

**Deliverable**: `nest` has a tested Modbus RTU transport foundation suitable for merging, but it is not yet connected to production runtime control.

Current status:

- The global configuration models RTU serial ports, baud rates, timeouts, slave unit IDs, and coil addresses.
- Configuration validation requires exactly one Modbus master when Modbus is configured.
- `internal/modbus` uses ModbusOne for RTU client behavior instead of implementing RTU framing locally.
- Master coil reads and writes are proven against an in-memory ModbusOne RTU server.
- The current Modbus work is intentionally mergeable infrastructure and does not enable or migrate existing light hardware.
- The Modbus entrypoint is not wired into `internal/nest` or the controller yet.
- The standalone slave RTU actor now serves configured event-signal and state-point coils and emits events for incoming event-signal writes.
- Runtime lifecycle wiring, semantic event translation, state projection, retries, and production relay routing remain incomplete.

Remaining work is grouped into five chunks:

### 1. Slave Transport

- [x] Implement the slave RTU actor using ModbusOne `RTUServer` callbacks.
- [x] Serve configured event-signal and state-point coils.
- [x] Emit events for incoming event-signal writes.
- [x] Provide state-point values from actor-owned state.
- [x] Test the standalone slave against the existing in-memory master.

### 2. Runtime Lifecycle Wiring

- [x] Start the existing master runtime from `internal/nest`.
- [x] Start the slave runtime from `internal/nest`.
- [x] Connect Modbus command and event channels to runtime startup and shutdown.
- [x] Replace configuration-only Modbus logging with actor lifecycle wiring.

### 3. Controller Route Execution

- [ ] Convert source-local semantic events into configured master event-signal coil writes.
- [ ] Convert slave event-signal writes into target-local semantic actions.
- [ ] Project local relay state into slave state-point coils.
- [ ] Convert master state-poll results into remote entity observations.

### 4. State Polling Policy

- [ ] Decide when master state polls occur.
- [ ] Implement the minimum useful polling policy.
- [ ] Test startup, periodic, and command-related polling behavior as applicable.

### 5. Hardening and Hardware Verification

- [ ] Define retry and transient serial-error behavior.
- [ ] Add a PTY-backed integration test for the production `modbus.Run` serial-opening path.
- [ ] Test master and slave actors together across the serial abstraction.
- [ ] Verify the complete path against physical RS-485 hardware.

## Phase 9: Local Cover Controller

- [ ] Define a config-driven cover model with separate open and close actuator references
- [ ] Configure local up and down relay endpoints
- [ ] Configure local open and close button inputs
- [ ] Implement press-and-hold open button behavior
- [ ] Implement press-and-hold close button behavior
- [ ] Stop on button release
- [ ] Stop remote motion when either physical button is pressed
- [ ] Define deterministic behavior when both buttons are active
- [ ] Implement open, close, stop, and remote movement commands
- [ ] Enforce up/down relay interlocks and stop before reversing
- [ ] Ensure both relays are off during startup and shutdown
- [ ] Add motion timeout handling
- [ ] Add unit tests for the state machine and safety rules
- [ ] Run the controller on an isolated, non-critical unit with `hausmaus` and `covers` disabled
- [ ] Replace the legacy `hausmaus` plus `covers` control path on that unit

**Deliverable**: `nest` can safely control and observe covers on one physical unit without depending on Modbus, MQTT, Home Assistant, `hausmaus`, or the legacy `covers` service.

**Migration boundary:**

- `nest` owns the configured cover inputs and relays on the migrated unit.
- Existing light controllers remain unchanged on other units.
- MQTT and Home Assistant are external command and state interfaces, not local cover-control dependencies.

## Phase 10: Cover Refinement

- [ ] Add time-based position tracking
- [ ] Add timing calibration and `max_time` handling
- [ ] Define behavior when position is unknown after restart
- [ ] Add recovery behavior after restart or interrupted movement

**Notes:**

- Covers should build on the same config and event model defined earlier.
- Safety rules must ensure up/down relays are never active simultaneously.
- The cover state machine should remain independent of sysfs so it can be moved to better-suited hardware later.

**Deliverable**: Cover entities behave predictably under real-world timing and interruption scenarios.

## Phase 11: Light Control Migration

- [ ] Start with one migrated light circuit
- [ ] Enable relay writes only for selected migrated lights
- [ ] Compare command execution and relay feedback through MQTT observability
- [ ] Expand migration circuit by circuit
- [ ] Keep existing controller behavior available until each circuit is verified

**Notes:**

- This phase is intentionally deferred while the existing light integration remains operational.
- Modbus runtime integration and the longer-term controller hardware direction should be settled before critical light migration.

**Deliverable**: Existing distributed light control is migrated safely to `nest` after cover behavior and transport foundations have been proven.

## Phase 12: Controller-Side Button Semantics

- [ ] Verify whether hardware and sysfs behavior already provide sufficient debounce for deployed buttons
- [x] Treat `pressed` and `released` as controller-owned semantic events derived from sysfs state changes
- [ ] Add press duration tracking for button holds
- [ ] Add semantic events such as `long_press` or `held_for` for actions like dimmer control
- [ ] Define deterministic behavior for repeated physical button events and timer cancellation
- [ ] Define configurable edge-triggered cover behavior where a press starts movement without requiring the button to be held
- [ ] Define behavior for repeated presses and opposite-direction presses
- [ ] Add multi-input trigger support when a concrete lighting or cover use case requires it

Current status:
Press and release button events already exist in controller normalization.
The first standalone cover migration preserves the current press-and-hold behavior.
Press-to-move behavior is deferred until the standalone migration is stable.

**Deliverable**: Controller-side button semantics support configurable hold-aware and edge-triggered actions without pushing timing policy into sysfs.

## Phase 13: Cover MQTT Commands

- [ ] Subscribe to cover command topics advertised in Home Assistant discovery
- [ ] Require explicit configuration before cover commands are enabled
- [ ] Reflect cover command enablement in the discovery payload
- [ ] Accept a minimal command set for migrated covers
- [ ] Publish resulting cover state changes after command execution

Light command handling and discovery are already covered by Phase 6.

**Deliverable**: Home Assistant can control migrated covers through MQTT without becoming a local cover-control dependency.

## Phase 14: MQTT and Home Assistant Expansion

- [ ] Refine topic structure as needed for Home Assistant integration
- [ ] Expand Home Assistant discovery coverage when additional entity classes are ready
- [ ] Finalize availability and state payloads
- [ ] Document deployment and migration behavior

**Deliverable**: MQTT interface evolves from a development/testing interface into a stable HA-facing integration.

## Early Decisions

These should be decided before transport and entity complexity increase.

- [ ] Testing strategy (mock sysfs vs real hardware)
- [x] Config file shape and CLI interface design
- [x] Local event model boundary between actor observations, semantic events, and actor commands
- [ ] Controller-side button semantics for long press, hold, and repeated presses
- [x] MQTT topic and autodiscovery schema centered on the Home Assistant discovery contract
- [ ] Modbus topology and relay addressing model

## Open Questions

- [ ] Cover support timing requirements
- [ ] Output unit relay state publishing to MQTT
- [ ] Final topic structure refinements around the HA-facing MQTT contract
- [ ] Whether command topics should be advertised for disabled entities
- [ ] Whether Modbus output units execute semantic commands or expose coil-level relay control
- [ ] Whether command topics or transport addresses should be parsed by the actor or resolved through registry mappings
- [ ] Whether actor grouping should move integrations under `internal/actors` once a second actor makes the grouping useful

## Future Config Distribution Direction

Out of scope for the current MQTT observability phase, but useful for guiding registry and topic-index decisions.

Eventually, configuration may come from a central/global source that describes all controller units and their relationships.

That global configuration could be distributed to each unit, possibly over MQTT or another control plane.
Each unit would project the global configuration into the subset it needs locally, then build its local runtime state from that projection.

The intended layering should stay roughly:

```text
global config
  -> unit-local config projection
  -> parsed config structs
  -> domain entities
  -> runtime registries/indexes
  -> actor-specific addressing
```

Implications:

- The domain registry should stay focused on semantic/domain lookup for the local unit
- MQTT topic names and other transport addresses should not be stored directly in the domain registry by default
- MQTT topics should remain actor-specific addressing, owned by the MQTT package or a future MQTT topic index
- If topic generation spreads or the contract hardens, introduce a dedicated MQTT topic index built from domain entities and MQTT configuration
- Global config projection should decide what each unit knows about, while actor-specific indexes decide how that local knowledge maps to transports
