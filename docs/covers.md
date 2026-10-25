# Local Cover Controller

Implementation reference for [Phase 9](implementation-plan.md#phase-9-local-covers-with-home-assistant-integration).
Planned behavior unless explicitly described as the current setup.

## Deployment Baseline

- No Nest deployment yet
- Twelve covers on the cover unit, with local inputs and motor relays
- Current path: `evok2mqtt` plus the separate Python `covers` service and Home Assistant button automations
- `hausmaus` recently replaced by `evok2mqtt` after network recovery failures; root cause unconfirmed
- Remaining problem: state drift between the bridge, cover service, and Home Assistant
- Existing HA percentage display available, but percentage-position commands not currently used

The first deployment replaces local button behavior and cover control with Nest, while retaining HA open/close/stop integration.
Network loss must affect remote access only: local input handling, relay control, timers, and motion tracking continue.

## Model and Ownership

### Cover Entity

- Semantic cover ID and display name
- Separate `open_actuator` and `close_actuator` references
- Movement timeout and reversal delay

Actuator references use the existing typed endpoint shape: `actor`, `kind`, and `id`.
Two relay references directly on the cover are sufficient for this milestone.
No separate motor entity required.

### Buttons and Bindings

Buttons remain independent entities backed by input endpoints.
Bindings connect their press/release events to cover actions; buttons are not fields of the cover entity.
Multiple buttons may bind to one cover.

Bindings preserve the initiating source and held-button intent so the controller can distinguish local operation from a remote movement request.
Use `hold_open` or `hold_close` with the existing semantic `source` and `target` fields.
Each binding describes both edges: press requests held movement; release clears that source's intent.
One button may select only one direction per cover.
Cover bindings must remain on the same unit and omit `execution_transport`.

Configuration and registry support are implemented; controller execution remains pending.
Both timing fields require explicit positive durations; the values below are illustrative commissioning placeholders.

```yaml
# Under units.shady.entities
covers:
  - id: office
    name: Office
    open_actuator:
      actor: sysfs
      kind: relay
      id: office_up
    close_actuator:
      actor: sysfs
      kind: relay
      id: office_down
    movement_timeout: 30s
    reversal_delay: 500ms

# At the global root
bindings:
  - source: shady.button.office_up
    target: shady.cover.office
    action: hold_open
  - source: shady.button.office_down
    target: shady.cover.office
    action: hold_close
```

Actuators must reference distinct local sysfs relays, with no ownership shared with lights or other covers.

```text
button press/release → binding ─┐
                              ├→ cover controller → actuator commands
MQTT OPEN / CLOSE / STOP ──────┘          │
                                         └→ cover state → MQTT → HA
```

### Runtime State

Controller-owned state per cover:

- Stopped, opening, or closing motion
- Initiating source and active held-input intents
- Movement deadline and timer generation
- Pending stop/start operations and reversal delay
- Output confirmation and fault state

The registry remains configuration-derived lookup data.
Live timers, motion state, and output operations belong to the controller runtime.
The state machine remains independent of sysfs and MQTT.

## Initial Behavior

| Input or command                               | Behavior                                                            |
| ---------------------------------------------- | ------------------------------------------------------------------- |
| Fresh open/close button press                  | Move in the bound direction while held                              |
| Release of the initiating held input           | Stop                                                                |
| HA `OPEN` or `CLOSE`                           | Move until stop or maximum movement duration                        |
| HA `STOP`                                      | Always accept                                                       |
| Physical press during remote movement          | Stop; require release and a fresh press before local movement       |
| Both directions held                           | Stop; require all participating held inputs released before restart |
| Remote movement while a physical input is held | Ignore movement request                                             |
| Repeated movement request                      | Do not extend the active movement deadline                          |
| Direction reversal                             | Confirm stop, wait configured delay, then start the new direction   |
| Button already held at startup                 | Record held state; require release and a fresh press                |

Source-aware release handling must prevent an unrelated input release from cancelling another input's movement accidentally.
Arbitration for multiple same-direction bindings must be made explicit and tested during implementation.
Stale timers and superseded output completions must not restart or stop a newer movement incorrectly.

## Output Execution and Lifecycle

Never energize both directions simultaneously.
Confirm the opposite relay is off before starting movement.
If a stop fails, do not energize the opposite direction.

Current sysfs commands report changed state but only log write failures.
Unchanged successful output writes produce no state event.
Cover execution therefore needs explicit command completion/failure reporting rather than assuming queued commands succeeded.

Required handling:

- Validate distinct actuators and reject conflicting configured output ownership
- Correlate output results with pending cover operations
- Confirm stop completion before applying reversal delay and starting again
- Establish both outputs off before accepting movement at startup
- Sample initial input levels without generating fresh-press behavior
- Handle input-read failures explicitly so a missing release does not silently leave normal operation running
- Complete bounded stop handling before cancelling sysfs workers on orderly shutdown
- Report output faults without claiming requested motion succeeded

An orderly shutdown stop does not cover abrupt power loss or process termination.
Startup must establish stopped outputs again; persisted state must never authorize automatic movement.

Movement durations and reversal delay remain commissioning values to establish per cover or hardware requirements.
No defaults inferred from the old configuration, which contains relay mappings but no explicit travel times.

## Home Assistant Contract

Included in the first deployment:

- Cover components in the existing per-unit device discovery payload
- `OPEN`, `CLOSE`, and `STOP` commands normalized into cover requests
- Retained canonical JSON cover state and unit availability
- Existing retained-command rejection
- Current state snapshots republished after reconnect without waiting for another relay change
- Local control independent of broker availability or MQTT publication success

Nest owns current cover state; HA consumes it.
Reconnect publication reflects the latest local state rather than replaying a backlog of movements.

Percentage reporting and percentage-position commands are omitted initially.
Stopped motion does not prove that the cover is fully open or closed.
Position remains unknown until position estimation is introduced.
The MQTT mapping must use HA-supported cover states and templates without inventing an endpoint or advertising unsupported position capabilities.
Verify that contract with the disposable HA/MQTT stack before deployment.

The legacy HA definitions use explicit YAML topics and names.
Migration to Nest discovery must account for existing entity references in dashboards and automations and remove duplicate legacy definitions.

## Existing Hardware Mappings

Source snapshot: sibling `homelab` checkout reviewed on 2026-09-29.

- Relay mappings: `../homelab/picl/covers.yaml`, embedded `config.yaml`
- Button mappings: `../homelab/picl/configuration.yaml`, `# cover buttons` section
- Legacy HA cover definitions: same file, embedded `covers.yaml`
- Legacy press/release behavior: same file, cover start/stop automations

All configured cover button topics use the `shady` unit namespace.
The relay config supplies circuit IDs without unit IDs; confirm the expected single-unit mapping during commissioning.
Input and relay circuit namespaces are separate; identical numbers do not imply a shared endpoint.

| Cover                  | Open input | Close input | Open relay | Close relay |
| ---------------------- | ---------- | ----------- | ---------- | ----------- |
| `bureau_voorgevel`     | `3_09`     | `3_10`      | `3_09`     | `3_10`      |
| `bureau_zijgevel`      | `3_11`     | `3_12`      | `3_12`     | `3_11`      |
| `inkom_kelder`         | `3_01`     | `3_02`      | `3_02`     | `3_03`      |
| `keuken_achter_links`  | `2_09`     | `2_10`      | `2_02`     | `2_03`      |
| `keuken_achter_rechts` | `2_11`     | `2_12`      | `2_08`     | `2_01`      |
| `keuken_zijkant`       | `2_13`     | `2_14`      | `2_05`     | `2_04`      |
| `living_voorkant`      | `2_03`     | `2_04`      | `2_13`     | `2_14`      |
| `living_zijkant`       | `2_01`     | `2_02`      | `3_13`     | `3_14`      |
| `praktijk_achter`      | `3_07`     | `3_08`      | `3_07`     | `3_06`      |
| `praktijk_midden`      | `3_05`     | `3_06`      | `3_05`     | `3_04`      |
| `eetkamer_links`       | `2_05`     | `2_06`      | `2_07`     | `2_06`      |
| `eetkamer_rechts`      | `2_07`     | `2_08`      | `2_10`     | `2_09`      |

Translate input IDs to configured sysfs digital-input endpoints and relay IDs to relay-output endpoints.
Verify physical direction rather than inferring it from numbering, especially for `bureau_zijgevel`.

## Acceptance Scenarios

- [ ] Hold either direction and release: movement starts and stops locally
- [ ] Disconnect MQTT during a local hold, then release: cover stops
- [ ] Start remotely and lose connectivity: local movement timeout still stops the cover
- [ ] Operate while disconnected: HA receives current state after reconnect without another input event
- [ ] Press a physical button during remote movement: stop locally, then require a fresh press
- [ ] Hold conflicting directions: stop and remain stopped until inputs are released
- [ ] Reverse direction: confirm off, apply delay, then energize the other relay
- [ ] Fail an off write: prevent opposite-direction activation and expose failure
- [ ] Repeat commands or deliver stale timers: do not extend motion indefinitely or affect a newer operation
- [ ] Start with a held button: no movement until release and fresh press
- [ ] Restart during movement: establish stopped outputs without claiming a known position
- [ ] Shut down orderly: complete stop handling before output workers exit
- [ ] Lose an input read during movement: follow explicit failure handling and retain the movement bound
- [ ] Send retained MQTT commands: no replayed movement
- [ ] Verify HA discovery and open/close/stop without percentage capabilities

Verification layers: pure state-machine tests, actor/controller output-result tests, simulated sysfs, disposable HA/MQTT stack, and physical single-cover trial.
Run lint, vet, race tests, and builds before deployment.

## Implementation and Cutover

1. Model/configuration: cover IDs, actuators, validation, independent button bindings, installation fixture
2. Controller/output execution: state machine, timers, output results, interlocks, startup/shutdown
3. HA integration: discovery, commands, canonical state, reconnect convergence
4. Verification/deployment: local stack, one-cover trial, then remaining covers

Deployment work:

- [ ] Define service installation, config paths, permissions, logs, and rollback
- [ ] Validate device mappings and establish movement timings
- [ ] Remove legacy control ownership for the trial cover before enabling Nest output writes
- [ ] Verify one cover, then expand across the unit
- [ ] Replace migrated HA cover definitions with discovery and update entity references as needed
- [ ] Remove migrated HA button-to-cover automations; behavior now belongs to Nest bindings
- [ ] Retire the Python `covers` service after all covers migrate
- [ ] Check other devices on the unit before retiring `evok2mqtt` responsibilities
- [ ] Verify rollback without simultaneous legacy and Nest output control

## Follow-Up: Position Estimation and Persistence

Tracked in Phase 10; not required for the first deployment.

- Separate calibrated open and close travel times
- Estimated percentage derived from locally confirmed relay activity
- Validity tracked independently of motion
- HA percentage reporting before optional percentage-position commands
- Versioned local state file keyed by semantic cover ID
- Atomic file replacement and explicit persistence-error handling
- Saved estimate, validity, movement-in-progress flag, recording time, and calibration context

A local file keeps recovery independent of MQTT and is sufficient as the initial persistence direction.
SQLite remains an option if broader transactional state requirements emerge.

| Saved state                                | Recovery                                                    |
| ------------------------------------------ | ----------------------------------------------------------- |
| Valid stopped estimate                     | Restore estimate, assuming no movement outside Nest control |
| Movement in progress or interrupted update | Mark position uncertain                                     |
| Missing, invalid, or incompatible state    | Position unknown                                            |

Define how a full-travel operation re-establishes an endpoint estimate.
Persistence preserves an estimate, not proof of physical position after unobserved movement.
Never resume movement automatically from saved state.
