# Design

## Context

See `proposal.md` for motivation and `specs/cover-control/spec.md` for behavior.
This document records the reviewed design direction, not implemented behavior.

`internal/controller/controller.go` normalizes actor reports and dispatches semantic events through one central loop.
The loop processes derived events through a local queue.
MQTT and Modbus receive commands from dispatch targets.
Sysfs routes commands to sequential device workers.
Current worker configuration groups devices by type, so all relay outputs share one worker.
This change gives each cover a dedicated output worker for its two direction relays.

Sysfs currently reports initial relay observations and changed values through `StateChange`.
It does not report unchanged command success or command failures.
Input read failures currently do not reach the controller.
The runtime currently cancels the controller and actors together.

## Goals / Non-Goals

**Goals:**

- Extend the existing event model with cover behavior
- Keep state transitions in one owner and hardware operations in actors
- Emit consumer-independent semantic events
- Make pending operations and fault recovery explicit
- Keep integration I/O outside semantic-event handling

**Non-Goals:**

- Replacement event bus or per-cover semantic-control goroutines
- Shared hardware worker pool or general-purpose I/O scheduler
- Cover-specific startup button sampling
- Physical relay-contact feedback
- Position-target commands or new Modbus cover transport

## Decisions

### 1. Controller-owned cover runtime state

Keep per-cover runtime state in `internal/controller`.
Use the existing dispatcher and derived-event queue for requests, output results, and deadline handling.
Do not add another consumer to the shared semantic-event channel.
Multiple direct consumers would divide events rather than broadcast them.

Each cover receives handling when a relevant event or deadline arrives.
Handlers record pending operations instead of waiting for hardware results.
Unrelated covers can progress while an operation remains pending.

Alternative: per-cover goroutines.
Per-cover semantic-control goroutines introduce extra ownership and ordering rules without a concrete need.
Dedicated sysfs output workers execute hardware operations but do not own cover transitions.

### 2. One model with distinct state responsibilities

| Field                | Responsibility                                                             |
| -------------------- | -------------------------------------------------------------------------- |
| Semantic cover state | Open, opening, closed, closing, stopped, or unknown                        |
| Estimated position   | Known 0–100% or unknown                                                    |
| Operation phase      | Initializing, idle, preparing, activating, moving, stopping, or recovering |
| Pending direction    | Requested direction before activation succeeds                             |
| Output fault         | Failed operation or timeout details                                        |
| Pending operations   | Command identifiers and operation generation                               |
| Timing               | Activation time, travel deadline, operation deadlines, and retry time      |
| Stop reason          | Early stop, completed full travel, cancellation, or failure                |

The same transition functions update these fields together.
An estimated endpoint does not establish semantic open or closed state.
During faults, retain the last established semantic state but report unavailable.
The retained state does not confirm current physical movement.

Normal start: idle → preparing → activating → moving.
Preparation commands both relays OFF and requires both successful results.
Activation commands only the requested relay ON.
Successful activation establishes opening or closing and starts the movement timer.
Add no explicit reversal delay.

Same-direction requests during preparation or activation preserve the pending start.
Stop or opposite-direction requests cancel intent and enter stopping.
Late activation results cannot restore cancelled intent.
Direction requests during stopping or recovery are discarded.
Preparation is distinct from stopping even though both phases command both relays OFF.

Successful switch-off establishes the final semantic state from the stop reason.
Recovery after completed full travel establishes the endpoint.
Recovery after an earlier output failure establishes stopped with unknown position.
Recovery clears the fault but never resumes movement.

Alternative: derive all cover state from position and direction.
That requires extra flags to distinguish estimated endpoints, early stops, and completed travel.
Explicit semantic state expresses these distinctions directly.

### 3. Extend reports on the existing sysfs states channel

Keep commands in and states out as the actor interface.
Extend the states report type with tagged observation, command-completion, and input-failure variants.
Normalize these reports through the existing semantic-event path.
Completion reports must not produce button edges.

Commands carry identifiers.
Completion reports carry the identifier, device, operation, completion time, and success or error.
Return completion for unchanged writes, invalid targets, and execution failures.
An explicit ON or OFF write does not require a successful preliminary read.
Toggle still requires a current value.

Alternative: infer command success from observed changes.
That cannot distinguish unchanged success, pending execution, and failure.
A separate result channel is unnecessary.

### 4. Per-cover output workers, interlocking, and ownership

Build one sequential sysfs output worker per cover, with both direction relays routed to its bounded command queue.
Translate registry ownership into sysfs worker configuration during runtime wiring.
Keep cover transitions and direction interlocking in the central controller.
The worker executes hardware operations and returns results without waiting for semantic state transitions.
FIFO execution alone cannot prevent both direction relays from becoming energized.
The controller must confirm both OFF writes before submitting the selected ON command.
Exclusive ownership prevents lights or another cover from bypassing this interlock.

Preserve FIFO execution for admitted commands across both relays of a cover.
Give unrelated covers separate workers so blocked I/O on one cover cannot occupy another cover's worker.
Retain existing type-grouped workers for devices outside cover ownership.
Exclude cover-owned relays from those workers to avoid duplicate execution or polling.
Each cover worker polls its own relays at the configured relay interval.
Do not increase per-relay polling frequency when partitioning workers.
Waiting workers use Go goroutines, not a dedicated operating-system thread per cover.

Make controller command admission bounded and non-blocking.
Reject commands when admission capacity is exhausted; treat rejection as an output operation failure.
The router must reject a command to a full worker queue without waiting for that worker.
Return correlated failures for rejected commands through the states channel.
Only successful execution results confirm output state; admission never confirms switch-off.
Keep rejected OFF operations faulted and retry them at the bounded retry interval.
Never bypass an earlier admitted ON with a later OFF or allow a rejected ON to execute later.
Queue saturation on one worker must not block routing to other workers or controller deadline processing.
Cancellation submits OFF after any previously submitted ON for that output.
An expired operation remains potentially executable; timeout does not cancel hardware work.
Ignore stale results for transition purposes without assuming the command never executed.

Each switch-off attempt uses new identifiers and a generation.
Require both successful OFF results from the current attempt before recovery or activation.
Do not accumulate retries while the current attempt remains pending before its timeout.

Verify direction exclusivity after every simulated hardware write, including cancelled and timed-out operations that execute late.
Test rapid competing requests, failures, delayed results, and recovery without assuming stale commands never execute.
Block one cover worker and verify another cover can start and stop before the blocked operation returns.

Add typed cover IDs, two relay references, and cover actions through config, entities, and registries.
Validate distinct relays and exclusive ownership across covers and lights.
Physical bindings map presses to direction requests and releases to stop requests.
Input initialization owns startup sampling policy.

Alternative: a shared fixed-size worker pool with per-device scheduling.
Blocked operations can occupy every pool slot and prevent unrelated covers from executing commands.
Dedicated cover workers provide the required isolation without a shared fairness scheduler.

### 5. Stored deadlines and elapsed-time estimates

Store deadlines in runtime state rather than creating operation-specific cancellation contexts.
Use one controller-owned timer for the earliest due deadline.
Handle due work through the existing sequential dispatch flow.
Check due deadlines between dispatch batches so sustained event traffic cannot starve timers.
Associate expiry with the operation generation to reject stale timeout work.

Calculate position from activation time, initial estimate, direction, and the shared full-travel duration.
Use successful write completion time rather than result-delivery time.
Clamp estimates and round only at reporting boundaries.
Unknown position remains unknown until successful full-travel completion.
Periodic semantic observations can use a reporting deadline independent of the movement deadline.

Alternative: update position with rounded periodic increments.
Elapsed-time calculation avoids cumulative rounding error and handles delayed reporting.

### 6. Staged runtime shutdown

Adjust lifecycle wiring in `internal/nest/nest.go` and `runtime.go`.
Application shutdown first signals the controller to reject movement and command both outputs OFF.
Keep sysfs and normalization alive while results arrive.
Finish controller shutdown after confirmation or the bounded deadline, then stop actors.
Allow persistence to complete final writes within the shutdown bound.

Use context lifetimes and existing done signals without adding a cover shutdown channel.
Review the exact context wiring during implementation.
Preserve the ordinary event dispatch model.

Alternative: simultaneous cancellation.
Workers can exit before executing shutdown OFF commands.

### 7. Persistence as a semantic-event integration

Add persistence as another dispatch target and actor, alongside MQTT and Modbus.
Cover logic emits semantic events without knowing which integrations consume them.
Events describe lifecycle, pending movement, cover observations, and confirmed stop outcomes.
They are not storage instructions or MQTT payloads.

Persistence maps these events into ordered storage commands.
It owns file format, clean-session tracking, unfinished movement records, and error logging.
Use a versioned unit snapshot keyed by cover identity and relay assignment.
Write through temporary-file replacement and serialize writes in the actor.
Load trusted records during runtime initialization and seed position without resuming movement.

Mark the session unclean before recording clean shutdown.
Mark movement unfinished from start intent, before successful activation is observed.
Only confirmed switch-off can clear unfinished movement.
A clean-shutdown marker follows preceding writes and confirmed output handling.
Bound the integration handoff; overload invalidates trust and logs failure instead of blocking local control.
Best-effort persistence does not delay activation while waiting for disk acknowledgement.

Alternative: write files directly in cover handlers.
That couples domain behavior to storage and can block all covers on disk I/O.

### 8. MQTT maps semantic events and owns reporting state

MQTT maintains the latest reporting snapshot from consumer-independent cover events.
Reconnect republishes current observations without changing movement intent or deadlines.
Keep handoff bounded and non-blocking; coalesce pending cover observations by cover ID.
Preserve state, position, and availability as one coherent pending observation.
Audit shared MQTT dispatch paths so button-event publication cannot block local cover requests.

Normalize OPEN, CLOSE, and STOP through existing command handling.
Preserve retained metadata for cover commands.
Reject retained movement commands and accept retained stop commands.
Extend existing device discovery rather than introducing another discovery model.
Require unit availability and per-cover output availability together.

Home Assistant does not expose stopped as a separate cover entity state.
Its MQTT adapter maps stopped using position or prior direction.
Keep Nest stopped semantics explicit in the published observation regardless of this UI mapping.

The checked MQTT cover implementation accepts numeric position but does not reset it on a non-numeric payload.
Deleting a retained position does not clear a value already held by a connected subscriber.
Adapter proposal: use semantic state topics and JSON attributes containing nullable `estimated_position` for the initial mapping.
Do not advertise a native `position_topic` that can leave stale numeric position after uncertainty.
Unknown semantic state maps to the documented `None` state payload.
Expose Nest semantic state in attributes when the native Home Assistant mapping loses detail.
Publish replacement attribute snapshots so unknown estimates clear previous values.
Validate this mapping against the deployed Home Assistant version during implementation.
This adapter detail comes from protocol inspection, not the collaborative behavior decisions.

References checked during design:

- [MQTT cover documentation](https://www.home-assistant.io/integrations/cover.mqtt/)
- [MQTT cover implementation](https://github.com/home-assistant/core/blob/dev/homeassistant/components/mqtt/cover.py)

## Risks / Trade-offs

- Successful writes do not prove physical relay state → retain the explicit software-confirmation assumption
- Motor limit behavior permits energized endpoint relays → verify the existing limit-switch assumption on the migration unit
- Slow or blocked sysfs execution delays OFF → use timeouts and recovery without claiming physical shutdown
- Saturated sysfs queues reject commands → fault affected covers and retry OFF without blocking unrelated workers or controller deadlines
- Persistence invalidation can fail → report detected uncertainty and keep position independent of movement permission
- Event traffic can delay deadlines → check deadlines between dispatch batches and test sustained traffic
- Native Home Assistant position remains unavailable → expose nullable estimated position as an attribute without stale numeric state
- Shutdown wiring crosses actor lifetimes → test ordered shutdown and actor failure before deployment

## Migration Plan

1. Implement and verify scenario tests before hardware migration
2. Validate configuration and exclusive relay ownership
3. Stop legacy owners of the selected cover relays
4. Deploy to one non-critical unit and verify startup OFF, local hold control, deadlines, and shutdown
5. Verify Home Assistant discovery, uncertainty, and reconnect reporting
6. For rollback, confirm outputs OFF and stop Nest before restoring the legacy owner

## Open Questions

- Deployment values for full-travel duration, operation timeout, OFF retry interval, and shutdown period
- Persistence location and position reporting interval

These values do not change the state model or integration boundaries.
