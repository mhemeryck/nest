# Design

## Context

See `proposal.md` for motivation and `specs/cover-control/spec.md` for behavior.
This document records the agreed single shade lifecycle model and per-shade recovery.
Implementation increments and tasks follow separately.

The current controller normalizes inputs and dispatches semantic events through a central loop.
Sysfs accepts commands and returns state observations through `states`.
Current sysfs workers group devices by type, so relay outputs share execution.
Current explicit ON/OFF commands require a preliminary read and do not return results for unchanged writes.
MQTT and Modbus command sends can block the controller's dispatch path.

## Goals / Non-Goals

**Goals:**

- Keep the complete shade lifecycle visible in one cohesive FSM module
- Keep interlocking, movement deadlines, and OFF recovery under one runtime's ownership
- Preserve sequential hardware writes without blocking unrelated shades
- Separate source routing, shade behavior, and low-level sysfs access
- Make multi-step functions read as their named behavioral responsibilities

**Non-Goals:**

- A second authoritative movement FSM in the central controller
- A generic bounded-activation framework for unrelated devices
- Recovery from hung writes, replacement workers, or concurrent writes after timeout
- Custom request coalescing, reserved stop slots, or controller admission-retry timers
- Receiver methods for shade FSM logic
- Final task breakdown or implementation in this planning step

## Decisions

### One authoritative shade lifecycle

Put the complete lifecycle in a dedicated shade runtime module beside sysfs.
Keep source-independent shade semantics in that module, not in low-level sysfs device access.
Use plain functions with explicit state and input arguments, without receiver methods.
Keep states, transitions, and behavioral helpers together so the full lifecycle remains inspectable.

The shade runtime owns both direction relays, interlocking, movement intent, deadlines, and OFF retries.
It also owns startup switch-off, per-shade recovery, and shutdown cleanup.
Hardware write outcomes and timer expiry are internal FSM inputs.
They do not require relay-operation round trips through the central controller.

Alternative: a controller cover FSM cooperating with a generic bounded-activation executor FSM.
Reject that split because the full lifecycle requires reconstructing transitions across both models and their result handover.
Do not merely relocate both FSMs into the same package.
Sequential I/O and timers execute one lifecycle model rather than form another independently authoritative FSM.

### Controller requests and reported state

The controller maps buttons and other sources to Open, Close, and Stop requests.
Route each request by typed cover ID to its shade runtime.
Keep configured definitions and relay lookups in the registry, separate from runtime state.
Only the owning shade worker changes authoritative lifecycle state.

The controller retains the latest reported shade state in a typed-ID map.
That reported state does not restrict the controller's ability to send requests.
The shade runtime decides how to handle each request safely.
The controller does not infer movement from relay observations or decide when recovery completes.

```text
Buttons / future integrations
             |
             v
Controller -- Open / Close / Stop --> Shade runtime --> Sysfs writes
Controller <-- lifecycle state ----- Shade runtime
```

Reported lifecycle phase and direction are distinct from position or an integration's cover state vocabulary.
Stopped means both relays are confirmed OFF, not that the shade is fully open or closed.
MQTT / Home Assistant cover mapping remains a later slice.

Alternative: retain a simplified controller transition model in addition to the shade FSM.
Reject duplicate authority; the controller needs a state snapshot, not another movement FSM.

### One sequential worker per shade

Give each shade's two relays one worker and one bounded request channel.
Keep unrelated devices in the existing sysfs worker groups.
Each worker owns its FSM state and executes hardware writes synchronously and sequentially.
Waiting for a movement deadline or retry interval must not block request or recovery handling.
Do not implement movement as ON followed by an uninterruptible sleep.

The worker arms the full-travel deadline immediately after ON succeeds, before attempting state publication.
The same model switches both outputs OFF after expiry, stop, opposite-direction request, or recovery.
Retry returned OFF errors at the configured interval without accumulating attempts or retrying ON.
An in-progress write finishes before another write starts; watchdog supervision handles a write that remains stuck.

Alternative: one shared relay worker.
Reject it because a slow write could delay another shade's stop.
An idle worker waits rather than continuously consuming CPU.

### Lifecycle transitions

Direction is separate state rather than duplicated opening and closing phases.
Track confirmed outputs, movement intent, deadline, recovery admission, and shutdown intent within the same model.
Startup begins in cleanup with unknown output state, not Stopped or resumed movement.

| State             | Input or outcome                              | Next state and action                                                      |
| ----------------- | --------------------------------------------- | -------------------------------------------------------------------------- |
| Startup cleanup   | Both outputs confirmed OFF                    | Stopped; allow fresh requests                                              |
| Stopped           | Direction request                             | Preparing; confirm both outputs OFF                                        |
| Preparing         | Both outputs confirmed OFF and intent remains | Activating; execute selected ON                                            |
| Preparing         | Stop or opposite direction before ON starts   | Stopping; cancel movement intent and finish switch-off                     |
| Activating        | ON succeeds with movement intent              | Moving; arm full-travel deadline                                           |
| Activating        | Stop or opposite direction after ON starts    | Stopping after the write returns; follow ON with OFF                       |
| Moving            | Same-direction request                        | Stay Moving; preserve original deadline                                    |
| Moving            | Stop, opposite direction, or deadline expiry  | Stopping; switch both outputs OFF                                          |
| Stopping          | Both outputs confirmed OFF                    | Stopped; require a fresh direction request                                 |
| Any running phase | Returned write error                          | Recovering; cancel intent and establish both outputs OFF                   |
| Any running phase | Request queue saturation                      | Recovering; block admission, discard queued movement, and establish OFF    |
| Recovering        | OFF retry interval expires                    | Retry required OFF writes sequentially                                     |
| Recovering        | Both outputs confirmed OFF                    | Stopped; reopen admission unless shutting down                             |
| Any running phase | Begin shutdown                                | Block admission, cancel unstarted movement, and establish both outputs OFF |

Same-direction requests during Preparing or Activating preserve the existing intent without adding activation.
Opposite-direction requests stop without automatic reversal.
Stop in Stopped leaves the shade stopped; repeated stops during cleanup preserve the cleanup obligation.
Direction requests received during Stopping or Recovering are not retained for later movement.
Do not allow requests admitted during cleanup to execute as fresh movement after cleanup completes.
An ON result after canceled intent cannot return the shade to Moving.
A stuck-write timeout terminates the process rather than entering another recovery transition.
Shutdown keeps admission blocked after confirmed OFF, regardless of the resulting lifecycle phase.

### Normal request order and recovery exceptions

Handle admitted external requests in FIFO order during normal operation.
Request admission is not a commitment to execute an ON write.
Cancel movement if stop is handled before ON starts.
If ON has started, preserve write order and follow it with OFF; brief activation after release remains possible.

Recovery and shutdown override queued movement.
Discard unstarted movement rather than replay it after cleanup.
This supersedes the earlier rule that every admitted activation must execute before cleanup.
It does not permit canceling or bypassing a hardware write already in progress.

Alternative: preserve all queued movement through recovery.
Reject it because stale requests could restart a shade without a fresh request after confirmed OFF.

### Bounded admission and per-shade recovery

Use non-blocking submission to each shade's bounded request channel.
Do not block the central loop or a shared router while waiting for one shade.
Full admission returns an explicit failure and triggers recovery for that shade, not application shutdown.

Block admission before signalling recovery.
Use a recovery signal independent of the request channel so a full channel cannot prevent cleanup.
Keep the admission gate and recovery handover coordinated so requests cannot enter between blocking and signalling.
The existing worker checks recovery before starting further queued movement and clears stale requests before reopening admission.
An in-progress write returns before that worker performs cleanup.

While recovering, reject new requests explicitly rather than queue them for deferred movement.
Repeated recovery requests preserve the same cleanup obligation.
Reopen admission only after both relays are confirmed OFF and stale movement requests cannot remain executable.
A request rejected during recovery is not a fresh request after recovery.
Other shades continue normally.

The admission gate is delivery coordination, not a second controller-owned shade FSM.
The controller does not retry rejected stop delivery or submit relay cleanup commands.
The independent recovery signal transfers the cleanup obligation to the owning shade worker.

Alternatives: custom coalescing mailbox, reserved stop capacity, controller admission retries, or fatal queue saturation.
Use simple request channels with per-shade recovery instead of those mechanisms.
Fatal failure remains reserved for stuck writes, not normal queue saturation.

MQTT and Modbus output queue rejection is logged without retaining rejected commands for deferred delivery.
Those integration queues must not block shade requests, deadlines, or cleanup.

### State reporting without blocking cleanup

Report shade lifecycle state through the existing `states` path with a distinct message kind and typed cover ID.
Relay observations remain distinct from shade state; observations do not replace successful write results inside the model.
The controller stores reported state but does not feed it back as an authoritative transition input.

Keep state-reporting storage bounded.
Delayed delivery must not block requests, deadlines, recovery signals, OFF retries, or shutdown cleanup.
Report the latest lifecycle state when delivery becomes available; do not require lossless relay-operation history at this boundary.
Preserve report order per shade so an older snapshot cannot overwrite a newer delivered state.
Delayed publication cannot reopen admission or restore canceled movement.

Log the first returned write error and repeated OFF retry errors.
Repeated errors do not require repeated lifecycle transitions while the shade remains Recovering.
Only successful writes establish confirmed OFF, including writes whose requested value was already present.

Alternative: reserve lifecycle feedback capacity for every relay operation admitted by the controller.
Remove that handover because relay results now stay inside the shade runtime.
Do not block cleanup on state publication or add an unbounded result backlog.

### Explicit writes without preliminary reads

Explicit ON/OFF writes use the requested value directly.
Return the result to the shade runtime even when a cached value matches the requested value.
Toggle still reads first because the current value determines its target.

Alternative: retain read-before-write for all commands.
Reject it because a failed read must not prevent an OFF attempt.

### Shared stuck-write watchdog

Use one shared watchdog for in-progress sysfs writes.
Each worker records the start time before its synchronous write and clears that record after the write returns.
Do not hold bookkeeping synchronization during hardware I/O.
The watchdog checks active writes against the configured output-command timeout.
Queue waiting, movement duration, and waiting between retries are not in-progress writes.

An exceeded write timeout causes fatal process failure, not per-shade recovery.
Do not create replacement workers or bypass a blocked write with concurrent I/O.
A returned write error remains a normal shade recovery input.
Fatal exit does not confirm OFF or replace startup switch-off in the next process.
The watchdog does not own movement deadlines, OFF retries, or lifecycle transitions.

Alternative: separate I/O workers and late-result recovery after timeout.
Reject that machinery for exceptional hung-write failures.
Service-manager restart can provide operational recovery after fatal exit; configuring that policy is outside this change.

### Staged startup and shutdown

Startup establishes both outputs OFF independently for each shade before opening movement admission.
Returned write errors enter the same shade recovery path; no previous movement resumes.

Shutdown sequence:

1. Block new requests and signal shutdown independently of request-channel capacity
2. Cancel unstarted movement and discard queued movement requests
3. Finish any in-progress write, then establish both outputs OFF
4. Keep workers and state handling running, including OFF retries
5. Wait for confirmed OFF or the configured shutdown timeout
6. Signal execution stop without claiming unconfirmed outputs are OFF

Shutdown uses the same cleanup behavior as per-shade recovery but never reopens admission.
Keep shutdown intent authoritative if recovery completes concurrently.
Use separate begin-shutdown and execution-stop signals.

Alternative: cancel controller and workers together.
Reject it because workers could exit before attempting required OFF writes.

### Responsibility-level functions

Describe each multi-step function as named behavioral responsibilities.
Represent those responsibilities with helper calls in the same order.
Keep ordering, branching, continuation, and failure rules visible in the caller.
Keep integration details and related bookkeeping inside the responsible helper.
Apply this criterion to the lifecycle transitions, admission handover, and worker orchestration.
Keep event-loop orchestration separate from behavioral rules without splitting lifecycle authority.
Resolve structural mismatches during each increment's review, not in a later cleanup.

## Risks / Trade-offs

- Full request channel: block admission and signal independent per-shade recovery; discard stale movement
- Concurrent request and recovery: coordinate admission closure and reopening; test the handover explicitly
- Stop during synchronous ON: follow the completed write with OFF; do not claim interruptible hardware writes
- Hung sysfs write: shared watchdog causes fatal failure rather than introducing replacement workers
- Fatal exit can leave relays energized: require startup switch-off; restart alone does not guarantee safe outputs
- Delayed state delivery: retain bounded current state without making cleanup depend on the controller
- No position tracking: Stopped confirms relay state, not a physical endpoint
- Shutdown timeout: end the wait without reporting unconfirmed outputs as safe

## Verification Before Implementation Completion

- Normal movement: one model prepares both relays, activates one direction, arms its deadline, and switches OFF
- Stop and opposite direction: cancel unstarted movement or follow in-progress ON with OFF; never reverse automatically
- Same direction: preserve intent and original deadline without duplicate activation
- Queue saturation: block only that shade, discard queued movement, and confirm OFF before reopening
- Recovery races: reject concurrent requests, clear stale movement, and require a fresh request after confirmed OFF
- Returned errors: retry OFF at the fixed interval without resuming movement or accumulating writes
- Hung ON or OFF: watchdog causes fatal failure without replacement writes
- Delayed state reports: deadlines and cleanup continue; stale reports cannot change authoritative state
- Shutdown during activation or recovery: discard unstarted movement and never reopen admission
- Controller boundary: route source-independent requests and retain state without implementing movement transitions

## Migration Plan

Select small implementation increments from these agreed boundaries and scenario checks.
Introduce prerequisites before exposing configured cover movement.
Verify the specification scenarios before completing implementation.
Before deployment or rollback, request confirmed OFF through the bounded shutdown sequence.
Do not infer safe physical outputs from process exit or timeout alone.
