# Design

## Context

See `proposal.md` for motivation and `specs/cover-control/spec.md` for behavior.
This document records agreed runtime boundaries and remaining interface decisions.
The committed lifecycle model is a checkpoint, not an accepted worker interface.
Complete the caller and worker design before further runtime implementation.

The current controller normalizes inputs and dispatches semantic events through a central loop.
Sysfs accepts commands and returns state observations through `states`.
Current sysfs workers group devices by type, so relay outputs share execution.
Current explicit ON/OFF commands require a preliminary read and do not return results for unchanged writes.
MQTT and Modbus command sends can block the controller's dispatch path.

## Goals / Non-Goals

**Goals:**

- Keep the complete shade lifecycle visible under one worker's ownership
- Keep interlocking, movement deadlines, and OFF recovery under one runtime's ownership
- Preserve sequential hardware writes without blocking unrelated shades
- Separate source routing, shade behavior, and low-level sysfs access
- Make runtime interfaces understandable from concrete caller examples
- Make multi-step functions expose meaningful behavioral responsibilities

**Non-Goals:**

- A second authoritative movement FSM in the central controller
- A generic bounded-activation framework for unrelated devices
- Recovery from hung writes, replacement workers, or concurrent writes after timeout
- Custom request coalescing, reserved stop slots, or controller admission-retry timers
- Receiver methods for shade FSM logic
- Callback-based hardware access or function fields for relay operations
- Position commands or payloads in this slice

## Decisions

### One authoritative shade lifecycle

Put the complete lifecycle in `internal/sysfs/shade`.
Keep shade behavior separate from low-level writes in the parent `sysfs` package.
The shade package imports sysfs; sysfs does not import the shade package.
Application wiring connects both packages without an import cycle.
Use concrete relay references and plain package-level functions, without receiver methods or hardware callbacks.
The package location makes the concrete sysfs dependency explicit.
Do not add a hardware-independent abstraction without a demonstrated need.

The shade runtime owns both direction relays, interlocking, movement intent, deadlines, and OFF retries.
It also owns startup switch-off, per-shade recovery, and shutdown cleanup.
Hardware write outcomes and timer expiry remain internal to the worker.
Handle a synchronous write result beside its call.
Do not require callers to coordinate a claim-and-completion protocol through hidden lifecycle mutations.
Private state and transition helpers remain possible where they simplify the demonstrated worker flow.

Alternative: a controller cover FSM cooperating with a generic bounded-activation executor FSM.
Reject that split because the full lifecycle requires reconstructing transitions across both models and their result handover.
Do not merely relocate both FSMs into the same package.
Sequential I/O and timers share one lifecycle authority.

### Per-shade public interface

Use the singular package name `shade`.
Construct one handle with the opening relay, closing relay, and shared timing settings.
Store `openingRelay` and `closingRelay` directly in private handle fields.
Reuse `*sysfs.Device` for both relay references rather than introducing a separate relay type.
Exclude shade-owned devices from shared sysfs workers to prevent competing writes and cached-state access.
Do not introduce a separate `Outputs` struct.

The agreed caller-facing shape is:

```go
handle, err := shade.New(openingRelay, closingRelay, timing)
err = shade.Submit(handle, shade.Open)
```

The fragment shows the interface shape, not complete application wiring or implemented declarations.
Check construction errors before using the handle.
`New` validates local arguments and creates bounded storage.
It starts no goroutine and performs no hardware writes.
Application wiring starts one blocking `Run` call in a goroutine for each handle and collects its result.
Application wiring supplies the typed cover ID, shared state output, and fatal write-failure output to the worker.
The exact `Run` signature and shutdown function names remain tentative.
Replace the earlier per-handle `States` proposal with the shared state output described below.

`Submit` accepts the commands Open, Close, and Stop.
It does not accept a position payload.
It returns acceptance or explicit rejection without waiting for hardware or queue capacity.
Acceptance does not promise that movement starts.
Reject submission before startup confirms both relays OFF and after shutdown begins.

Keep lifecycle state private.
The public boundary exposes commands and state snapshots, not relay-write bookkeeping.
Alternative: expose the isolated lifecycle model as the runtime interface.
Reject that approach because its use depends on coordination that the caller has not demonstrated.

### Controller requests and reported state

The controller maps buttons and other sources to Open, Close, and Stop requests.
The controller selects a shade handle by typed cover ID before submitting a command.
`Submit` receives one handle, not all shades or a cover ID.
The shade worker accesses only its own two relays and state.
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
A write blocks only its owning shade, not other shades or controller submission.
Commands can enter the bounded queue while a write runs.
The worker handles them after the write returns; it does not process commands concurrently with that write.
The existing worker loop handles commands, timers, state publication, and hardware writes sequentially.
Only communication with other components and write-deadline handling run independently of that loop.
Waiting for a movement deadline or retry interval must not block request or recovery handling.
Do not implement movement as ON followed by an uninterruptible sleep.

The worker arms the full-travel deadline immediately after ON succeeds, before attempting state publication.
The same model switches both outputs OFF after expiry, stop, opposite-direction request, or recovery.
Retry returned OFF errors at the configured interval without accumulating attempts or retrying ON.
An in-progress write finishes before another write starts; context deadline supervision detects a write that remains stuck.
Before activation, process already queued commands in FIFO order so a pending Stop can cancel unstarted movement.
Check shutdown and saturation recovery before starting another write.
Do not infer that admission can interrupt an active sysfs call.

Alternative: one shared relay worker.
Reject it because a slow write could delay another shade's stop.
An idle worker waits rather than continuously consuming CPU.

### Lifecycle transitions

Direction is separate state rather than duplicated opening and closing phases.
The worker owns confirmed outputs, movement intent, deadlines, and cleanup state.
Admission synchronization is separate from worker-owned lifecycle state.
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
Protect shared admission state with a mutex used by submission and the worker.
The command channel provides queue synchronization; the mutex coordinates admission closure and reopening.
Never hold the admission mutex during hardware writes.
Worker-owned lifecycle state does not need a mutex.
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

Use one shared shade-state channel as a controller input, consistent with the existing actor boundaries.
Use the term states consistently with the existing application.
Each state message contains the typed cover ID, lifecycle phase, and direction.
Application wiring supplies the cover ID to each worker; handle lookup remains outside the worker.
Each worker retains at most one latest unsent snapshot in its private state.
New snapshots replace the pending snapshot without blocking the worker.
Only the owning worker publishes snapshots.
Use an unbuffered shared state channel so unsent snapshots remain replaceable inside their owning workers.
Enable a send case in the existing worker loop only while a snapshot is pending.
If the controller cannot receive, continue command handling, timers, and required hardware work.
Clear pending publication after a successful send; a later state change enables it again.
The controller normalizes shade states into its existing central event dispatch path.
Do not add forwarding goroutines, reflection-based selection, or a shared snapshot map read by the controller.
These mechanisms are unnecessary when workers publish directly through their existing loops.
Do not make the parent sysfs package import shade snapshot types.
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
The controller can stop receiving during shutdown without preventing worker cleanup.
Use worker completion results, not state snapshots, to establish shutdown completion.

### Explicit writes without preliminary reads

Explicit ON/OFF writes use the requested value directly.
Return the result to the shade runtime even when a cached value matches the requested value.
The parent sysfs package provides a plain synchronous write function for a device and an explicit value.
It does not publish shade states or mutate the device's cached value for shade lifecycle bookkeeping.
Toggle still reads first because the current value determines its target.
Keep existing light behavior outside this shade-specific supervision change.

Alternative: retain read-before-write for all commands.
Reject it because a failed read must not prevent an OFF attempt.

### Context-based stuck-write supervision

Keep shade write supervision inside `internal/sysfs/shade`, not in generic sysfs device setup.
Use a deadline context for each in-progress write with the configured output-command timeout.
Do not introduce a custom watchdog object, active-write registry, or polling monitor.
Do not attach watchdog references or supervision settings to `sysfs.Device`.
A private shade helper supervises the synchronous parent-package write and returns its actual result.
Register deadline handling with `context.AfterFunc` before starting the write.
On deadline expiry, notify the application through a shared bounded fatal-error channel without blocking the handler.
After write completion, stop deadline handling and cancel the deadline context.
Normal completion cancellation is not a write timeout.
Coordinate completion and expiry so a handler cannot report a timeout after supervision has successfully ended.
Verify the completion-versus-expiry race explicitly; the exact coordination remains a review item.
The deadline handler runs independently because the worker cannot observe context expiry while blocked in synchronous I/O.
The context does not interrupt the sysfs call or automatically propagate failure to the application.
Queue waiting, movement duration, and waiting between retries are not in-progress writes.

An exceeded write timeout causes fatal process failure, not per-shade recovery.
Do not create replacement workers or bypass a blocked write with concurrent I/O.
A returned write error remains a normal shade recovery input.
Fatal exit does not confirm OFF or replace startup switch-off in the next process.
Write supervision does not own movement deadlines, OFF retries, or lifecycle transitions.
The application receives fatal write errors during normal operation and shutdown cleanup.
It returns an error through the existing process-exit path without waiting for a blocked worker.
Keep the failure channel available until process exit or all supervised writes have ended.

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
4. Stop command producers and the controller; keep shade workers performing cleanup and OFF retries
5. Wait for worker results while also observing fatal write errors and the configured shutdown timeout
6. After confirmed OFF, stop remaining infrastructure; on fatal error or timeout, return an error without unconditional actor waits

Shutdown uses the same cleanup behavior as per-shade recovery but never reopens admission.
Keep shutdown intent authoritative if recovery completes concurrently.
Use separate begin-shutdown and execution-stop signals.
Beginning shutdown synchronously closes admission and signals cleanup without waiting for hardware.
Keep per-write deadline supervision active while workers perform shutdown cleanup.
Do not derive write deadlines from the normal shutdown cancellation context.
Worker completion results establish confirmed cleanup; state publication is not a prerequisite for completion.
An execution-stop signal cannot interrupt an active sysfs call.
If shutdown timeout expires before a write deadline, return a shutdown error without claiming confirmed OFF.
The existing command entry point exits with failure when the application returns an error.
Exact function signatures and lifecycle wiring still require a final caller review.

Alternative: cancel controller and workers together.
Reject it because workers could exit before attempting required OFF writes.

### Responsibility-level functions

Describe each multi-step function as named behavioral responsibilities.
Make those responsibilities recognizable in the function structure.
Extract helpers for meaningful concepts, not mechanical steps or request renaming.
Keep ordering, branching, continuation, and failure rules visible in the caller.
Keep integration details and related bookkeeping inside the responsible helper.
Keep small, cohesive transition updates together.
Use explicit phase handling rather than exclusion lists with different hidden reasons.
Separate shutdown policy from temporary cleanup policy.
Prefer direct selection returns over temporary result variables.
Use early returns where each exit has a consistent responsibility.
Keep selection separate from mutation when both responsibilities exist.
Apply this criterion to the lifecycle transitions, admission handover, and worker orchestration.
Keep event-loop orchestration separate from behavioral rules without splitting lifecycle authority.
Resolve structural mismatches during each increment's review, not in a later cleanup.

## Risks / Trade-offs

- Full request channel: block admission and signal independent per-shade recovery; discard stale movement
- Concurrent request and recovery: coordinate admission closure and reopening; test the handover explicitly
- Stop during synchronous ON: follow the completed write with OFF; do not claim interruptible hardware writes
- Hung sysfs write: context deadline handling signals fatal failure without introducing replacement workers
- Write completion races with expiry: coordinate handler cancellation and verify the terminal decision
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
- Hung ON or OFF: context deadline handling causes fatal failure without replacement writes
- Completed write: cancel deadline handling without false fatal notification; verify races with expiry
- Delayed state reports: deadlines and cleanup continue; stale reports cannot change authoritative state
- Shutdown during activation or recovery: discard unstarted movement and never reopen admission
- Controller boundary: route source-independent requests and retain state without implementing movement transitions

## Migration Plan

Preserve the existing model and tests as a checkpoint, not a required implementation shape.
Complete caller examples and worker interaction before choosing the remaining internal model API.
Review open, stop during ON, queue saturation, and shutdown across the same runtime boundary.
Replace or adapt existing model code only after that review.
Introduce prerequisites before exposing configured cover movement.
Verify the specification scenarios before completing implementation.
Before deployment or rollback, request confirmed OFF through the bounded shutdown sequence.
Do not infer safe physical outputs from process exit or timeout alone.

## Pending Design Review

These decisions block further runtime implementation; they are not optional implementation details.

- Final caller signatures using concrete devices, shared state output, fatal-error output, and separate shutdown signalling
- Deadline completion-versus-expiry coordination and non-blocking fatal-error delivery
- Application startup and shutdown wiring that preserves supervision and avoids unconditional waits on failure
- Shared shade-state input through controller normalization and dispatch, with bounded downstream delivery
- Private state and helpers justified by the complete worker flow, including whether pending-write bookkeeping remains necessary

Resolve these boundaries with caller examples before implementing the revised tasks.
The behavioral spec remains unchanged.
