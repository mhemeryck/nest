# Design

## Context

See `proposal.md` for motivation and `specs/cover-control/spec.md` for behavior.
This document captures agreed design decisions and both reviewed FSMs.
Implementation increments and tasks follow separately.

The current controller normalizes inputs and dispatches semantic events through a central loop.
Sysfs accepts commands and returns state observations through `states`.
Current sysfs workers group devices by type, so relay outputs share execution.
Current explicit ON/OFF commands require a preliminary read and do not return results for unchanged writes.
MQTT and Modbus command sends can block the controller's dispatch path.

## Goals / Non-Goals

**Goals:**

- Keep cover and executor decisions visible in separate cohesive FSM modules
- Keep output activation, its deadline, and its cleanup under one executor's ownership
- Preserve sequential hardware writes without blocking unrelated covers
- Reuse existing package boundaries, channels, and central dispatch
- Make multi-step functions read as their named behavioral responsibilities

**Non-Goals:**

- A generic transaction framework or a history of all timed-out commands
- Recovery from hung writes, replacement workers, or concurrent writes after timeout
- A custom cancelable mailbox, queued-command removal, or command TTL
- Dimming-specific behavior, despite possible future bounded-activation reuse
- Receiver methods for cover FSM logic
- Final task breakdown or implementation in this planning step

## Decisions

### Generic bounded activation in sysfs

Use a bounded-activation operation instead of unrelated ON and later OFF commands for normal cover movement.
The operation identifies an output and a maximum activation duration.
After ON succeeds, the executor owns the deadline and the required OFF attempt.
An explicit deactivation ends activation earlier.
The executor retries failed OFF writes at the configured fixed interval until OFF succeeds or execution stops during shutdown.
It does not accumulate retries while an earlier write remains pending.
Plain deactivation also supports startup, interlocking, and recovery without first activating an output.

This operation has no cover direction, button, position, or initiating-source semantics.
Push-to-dim inputs could reuse the operation later without adding dimming behavior now.

Alternative: keep movement deadlines and OFF retries in the central controller loop.
Reject this split because it separates activation from its required cleanup.
Do not retain a second normal movement deadline in the controller alongside the executor deadline.

### One sequential output worker per cover

Give each cover's two direction relays one sysfs worker.
Keep unrelated devices in the existing worker groups.
The worker serializes hardware writes, including ON followed by deactivation.
Waiting for an activation deadline must not prevent the worker from handling early deactivation.
The operation is not an ON write followed by an uninterruptible sleep.

Alternative: one shared relay worker.
Reject it because one blocked output operation could delay another cover's stop.
An idle worker goroutine waits rather than continuously consuming CPU.

### FIFO execution, not strict cancellation

Preserve admitted operation order within each worker.
A stop or opposite-direction request cancels start intent that has not been submitted.
Once activation is submitted, deactivation follows it rather than removing it from the queue.
Brief activation after button release is permitted in this case.
Do not activate the opposite direction automatically.

Alternative: cancel queued activation through shared pending-operation storage.
Reject the additional queue manipulation and synchronization complexity.
TTL would address stale queued commands but would not cancel an executing write.
Do not introduce it for this change.

### Execution feedback through states

Keep `commands` as sysfs input and `states` as its output channel.
Distinguish observations from execution results with explicit message kinds.
Correlate results to commands or operations through identifiers.

Bounded activation returns:

- Activated: ON succeeded, with its actual completion time
- Finished: OFF succeeded after expiry or early deactivation
- Failed: a write failed, identifying the failed operation and write

Failure does not imply OFF and need not be the last result: cleanup can later succeed.
Emit at most one failure event per operation.
Log subsequent OFF retry errors without adding failure events.
Still emit finished after OFF eventually succeeds.
Successful unchanged writes still return results.
An observation does not substitute for command completion.

Alternative: separate channels for observations and results.
Keep the existing channel structure instead of adding another controller input.
Establish the local deadline and cleanup obligation before potentially delayed result delivery.
Result publication must not prevent deadline handling or OFF retries.

The executor event loop selects between commands, timers, and sending pending results through `states`.
If result delivery cannot proceed, command and timer handling remains available.
Keep pending lifecycle feedback bounded and reserve capacity for accepted operations' lifecycle results.
Reject operations when that capacity is unavailable rather than drop results or accumulate an unbounded backlog.
Retain result order for each operation, including activated feedback followed by completion.
Cleanup of accepted operations uses their retained feedback capacity, even when delivery is delayed.

Alternative: a blocking result send after each write, or another publisher worker.
Reject blocking publication because a full states channel could prevent required OFF.
Use the executor event loop rather than adding a publisher worker.

### Shared stuck-write watchdog

Use one shared watchdog for in-progress sysfs writes.
Each worker records the write start time before its synchronous write and clears that record after the write returns.
The watchdog independently checks active writes against the configured output-command timeout.
Do not hold bookkeeping synchronization during hardware I/O.
Queue waiting and normal activation duration are not in-progress writes.

An exceeded write timeout causes fatal process failure, not cover or executor recovery.
Do not create replacement workers, bypass the write, or process late results as a timeout-recovery workflow.
A returned write error remains a normal executor recovery input.
Fatal exit does not confirm OFF or replace startup switch-off in the next process.

The watchdog does not own activation deadlines, OFF retries, or cover state.
Keep one synchronous, sequential I/O path within each worker.

Alternative: a responsive executor plus another I/O worker for every cover, with late-result recovery.
Reject that machinery for exceptional hung-write failures.
The shared watchdog only supervises fatal liveness failure; it is not another FSM.

### Explicit writes without preliminary reads

Explicit ON/OFF writes use the requested value directly.
Return the write result even when the cached value matches the requested value.
Toggle still reads first because the current value determines its target.

Alternative: retain the current read-before-write requirement for all commands.
Reject it because a failed read must not prevent an OFF attempt.

### Cover FSM separate from orchestration

Keep a runtime-state map keyed by typed cover ID in the central controller loop.
Keep configured cover definitions and relay lookups in the registry, separate from changing runtime state.
Only the controller loop changes cover runtime state.
Workers return events; they do not access the cover runtime map.

Put transition logic in a dedicated controller FSM module using plain functions with explicit state and input arguments.
The loop routes an event, applies the transition, stores state, and executes the resulting effects.
The FSM decides direction, interlocking, request admission, and whether outputs are safe for fresh movement.
The executor owns bounded-activation deadlines and hardware OFF retries.
A late activation result must not restore movement intent after stop or failure.

Alternative: distribute cover transition rules through the main loop, registry, and worker callbacks.
Reject it because the FSM would become difficult to inspect as one behavior.
One FSM per cover does not require one controller goroutine per cover.

### Cover FSM

Keep the cover transition function and its state handlers together in the controller FSM module.
Direction is separate state rather than duplicated opening and closing phases.
The main loop routes inputs but does not contain cover transition rules.

| State                | Input                                            | Next state and action                                 |
| -------------------- | ------------------------------------------------ | ----------------------------------------------------- |
| Stopped              | Direction request                                | Preparing; request both outputs OFF                   |
| Preparing            | Both outputs confirmed OFF                       | Activating; submit bounded activation                 |
| Preparing            | Stop or opposite direction                       | Stopping; abandon unsubmitted start and establish OFF |
| Activating           | Activated result                                 | Moving                                                |
| Activating or Moving | Stop or opposite direction                       | Stopping; submit deactivation behind activation       |
| Moving               | Finished with confirmed OFF                      | Stopped, with both direction outputs confirmed OFF    |
| Stopping             | Delayed activated result                         | Stay Stopping; do not restore intent                  |
| Stopping             | Both OFF confirmed after submitted activation    | Stopped                                               |
| Operational state    | Returned write error or output admission failure | Recovering; establish both outputs OFF                |
| Recovering           | Both OFF confirmed after outstanding writes      | Stopped                                               |

Startup enters switch-off handling with unknown output state, not Stopped or resumed movement.
Same-direction requests during Preparing, Activating, or Moving do not add activation or extend its duration.
Direction requests during Stopping or Recovering are not retained.
Stop in Stopped leaves the cover stopped; stop during cleanup preserves the cleanup obligation.
Delayed activation feedback in Recovering does not restore intent.
Only results for the relevant operations establish confirmed OFF; earlier OFF results cannot override later submitted activation.
Fatal stuck-write timeout terminates the process instead of transitioning this FSM into recovery.

### Executor FSM

Keep executor transition logic in one sysfs FSM module, separate from cover logic and routing.
Use plain functions with explicit state and input arguments, without receiver methods.
The worker executes the FSM's writes sequentially and returns lifecycle feedback through `states`.

The phases below describe an output operation.
Initial output state is unknown; deactivation establishes confirmed OFF before cover movement is admitted.

| State                 | Input or result                               | Next state and action                                       |
| --------------------- | --------------------------------------------- | ----------------------------------------------------------- |
| Unknown               | Deactivation                                  | Deactivating; attempt OFF                                   |
| Off                   | Bounded activation                            | Activating; execute ON                                      |
| Activating            | ON succeeds                                   | Active; arm deadline and retain activated feedback          |
| Active                | Duration expires or deactivation is processed | Deactivating; attempt OFF                                   |
| Deactivating          | OFF succeeds                                  | Off; retain finished feedback                               |
| Write-executing phase | Write returns an error                        | Recovering; retain first failure feedback and establish OFF |
| Recovering            | Retry interval expires                        | Attempt OFF, preserving sequential writes                   |
| Recovering            | OFF returns another error                     | Stay Recovering; log error and schedule next retry          |
| Recovering            | OFF succeeds                                  | Off; retain finished feedback                               |
| Off                   | Deactivation                                  | Confirm OFF through an explicit write and return its result |

Deactivation queued behind activation is processed without waiting for the activation duration.
The deadline is armed after ON succeeds, before waiting to publish its feedback.
Never retry ON as part of recovery.
A stuck write is fatal through the shared watchdog, not an executor recovery transition.

The executor's Off phase describes an output.
The cover's Stopped phase means both direction outputs are safe for a fresh request.
Do not substitute one for the other.

### Normal interaction

```text
Cover FSM                         Executor
---------                         --------
Preparing  -- deactivate both --> OFF writes
           <-- confirmed OFF ----
Activating -- bounded activate -> ON write; arm deadline
           <-- activated --------
Moving                            deadline expires; OFF succeeds
           <-- finished ---------
Stopped
```

Normal expiry needs no second controller command.
Release after activation submission moves the cover to Stopping and queues deactivation in FIFO order.
Activated feedback received while Stopping does not return the cover to Moving.

### Bounded, non-blocking admission

Use bounded command queues at controller submission and worker routing boundaries.
Reject an unadmitted command explicitly instead of blocking the central loop or shared router.
Cover output rejection blocks movement and requires establishing confirmed OFF.
A rejected deactivation is not an accepted cleanup operation or evidence of safe outputs.

Keep pending deactivation admission explicit in cover state, within Stopping or Recovering.
The controller retries rejected admission at the configured retry interval while cleanup remains required and execution runs.
After admission succeeds, the executor owns OFF execution and retries.
Do not resubmit an admitted operation as an admission retry.
Do not add a top-level cover phase merely for the pending admission detail.
An accepted bounded activation retains its own deadline while early deactivation admission is congested.

MQTT and Modbus queue rejection is logged without retaining rejected commands for deferred delivery.
Preserve the order of admitted commands.

Alternative: wait for queue space or retain an unbounded backlog.
Reject both because congestion must not delay local control or create unbounded memory use.

### Staged startup and shutdown

Startup uses deactivation to establish both outputs OFF for each cover independently.
Do not admit movement for a cover until that succeeds.
Executor OFF recovery remains available when startup writes fail.

Shutdown sequence:

1. Block new activation submissions and cancel unsubmitted start intent
2. Request deactivation for all cover outputs, preserving admitted FIFO order
3. Keep executors and result handling running, including deadlines and OFF retries
4. Wait for all outputs confirmed OFF or the configured shutdown timeout
5. Signal execution stop without claiming unconfirmed outputs are OFF

Use separate begin-shutdown and execution-stop signals.
Alternative: cancel controller and workers together.
Reject it because workers could exit before attempting required OFF writes.

### Responsibility-level functions

Describe each multi-step function as named behavioral responsibilities.
Represent those responsibilities with helper calls in the same order.
Keep ordering, branching, continuation, and failure rules visible in the caller.
Keep integration details and related bookkeeping inside the responsible helper.
Resolve mismatches between explanation and calls during each increment's review.
Do not postpone this criterion to a later structural cleanup.
Apply the same criterion to both FSMs, including their states, inputs, and transition handlers.
Use transition tests for normal paths, early stops, returned errors, and delayed feedback.
Keep event-loop orchestration separate from behavioral rules in both modules.

## Risks / Trade-offs

- Delayed queued activation after release: accept brief activation and preserve subsequent FIFO deactivation
- Hung sysfs write: shared watchdog causes fatal process failure instead of introducing timeout recovery machinery
- Fatal exit can leave relays energized: do not claim safe outputs; require startup switch-off in the next process
- Slow writes below the fatal timeout: isolate workers; do not claim activation duration can interrupt an operating-system write
- Full command queues: reject explicitly, keep the affected cover blocked, and do not assume rejected OFF was admitted
- Shared states delivery congestion: select between result delivery, commands, and timers; reserve bounded lifecycle feedback capacity
- Repeated OFF errors: log each error but emit one failure event per operation, followed by eventual confirmed completion
- Integration rejection: accept lost integration commands during congestion; log failures without a deferred-delivery promise
- Shutdown timeout: end the wait without reporting unconfirmed outputs as safe

## Verification Before Implementation Completion

- Normal activation: executor arms its deadline before result delivery and switches OFF without another controller command
- Queued activation then release: FIFO allows brief ON, followed by OFF; delayed activated feedback does not restore intent
- Rejected deactivation: controller retries admission until accepted; executor then owns execution retries
- Hung ON or OFF write: watchdog causes fatal failure without replacement writes or per-cover timeout recovery
- Full states channel: deadlines, deactivation, and returned-error retries continue with bounded retained feedback
- Repeated OFF errors: one FSM failure event, repeated error logs, and a finished result after successful OFF
- Shutdown during activation or recovery: block new submissions and keep cleanup alive until confirmed OFF or the bounded wait ends

## Migration Plan

Select small implementation increments from these agreed boundaries and scenario checks.
Introduce prerequisites before exposing configured cover movement.
Verify the spec scenarios, including delayed activation, queue rejection, OFF failures, and staged shutdown.
Before deployment or rollback, request confirmed OFF through the bounded shutdown sequence.
Do not infer safe physical outputs from process exit or timeout alone.
