# Design

## Context

See `proposal.md` for motivation and `specs/cover-control/spec.md` for behavior.
This document captures agreed design decisions, not an implementation-ready task plan.
The FSM outline still needs review before task planning.

The current controller normalizes inputs and dispatches semantic events through a central loop.
Sysfs accepts commands and returns state observations through `states`.
Current sysfs workers group devices by type, so relay outputs share execution.
Current explicit ON/OFF commands require a preliminary read and do not return results for unchanged writes.
MQTT and Modbus command sends can block the controller's dispatch path.

## Goals / Non-Goals

**Goals:**

- Keep cover decisions visible in one FSM module
- Keep output activation, its deadline, and its cleanup under one executor's ownership
- Preserve sequential hardware writes without blocking unrelated covers
- Reuse existing package boundaries, channels, and central dispatch
- Make multi-step functions read as their named behavioral responsibilities

**Non-Goals:**

- A generic transaction framework or a history of all timed-out commands
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
Successful unchanged writes still return results.
An observation does not substitute for command completion.

Alternative: separate channels for observations and results.
Keep the existing channel structure instead of adding another controller input.
Establish the local deadline and cleanup obligation before potentially delayed result delivery.
Result publication must not prevent deadline handling or OFF retries.

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

### FSM outline for review

The earlier FSM diagram assumed controller-owned movement deadlines.
Revise that diagram before treating it as final.
The current responsibility outline is:

| Phase      | Responsibility                                             |
| ---------- | ---------------------------------------------------------- |
| Stopped    | Accept a fresh direction request                           |
| Preparing  | Obtain confirmed OFF for both outputs                      |
| Activating | Wait for submitted bounded activation to report ON success |
| Moving     | Handle requests and wait for executor-driven completion    |
| Stopping   | Wait for deactivation behind any submitted activation      |
| Recovering | Block movement until both outputs are confirmed OFF        |

Direction is separate state rather than duplicated opening and closing phases.
Startup begins by establishing confirmed OFF, not by resuming previous movement.
Stop or opposite direction leads toward stopping without automatic reversal.
Same-direction requests do not submit another activation or extend its duration.
Direction requests during stopping or recovery are not retained.

This table is a review outline, not an approved complete transition table.

### Bounded, non-blocking admission

Use bounded command queues at controller submission and worker routing boundaries.
Reject an unadmitted command explicitly instead of blocking the central loop or shared router.
Cover output rejection blocks movement and requires establishing confirmed OFF.
A rejected deactivation is not an accepted cleanup operation or evidence of safe outputs.

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

## Risks / Trade-offs

- Delayed queued activation after release: accept brief activation and preserve subsequent FIFO deactivation
- A timed-out operating-system write can still finish: retain write order and follow uncertain ON with OFF
- Permanently blocked hardware I/O: isolate other covers; do not claim bounded activation guarantees physical switch-off during an I/O hang
- Full command queues: reject explicitly, keep the affected cover blocked, and do not assume rejected OFF was admitted
- Shared states delivery congestion: keep deadline and cleanup execution independent of feedback delivery
- Integration rejection: accept lost integration commands during congestion; log failures without a deferred-delivery promise
- Shutdown timeout: end the wait without reporting unconfirmed outputs as safe

## Review Before Task Planning

Review the complete FSM transition table against FIFO execution and executor-owned completion.
Trace queue rejection through deactivation admission so saturation cannot leave cleanup responsibility unassigned.
Trace output timeouts and delayed states delivery without concurrent or reordered writes.
These are remaining design checks, not permission to choose additional mechanisms during implementation.

## Migration Plan

Select implementation increments only after the remaining design review.
Introduce prerequisites before exposing configured cover movement.
Verify the spec scenarios, including delayed activation, queue rejection, OFF failures, and staged shutdown.
Before deployment or rollback, request confirmed OFF through the bounded shutdown sequence.
Do not infer safe physical outputs from process exit or timeout alone.
