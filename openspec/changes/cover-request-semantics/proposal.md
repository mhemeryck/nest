# Proposal

## Why

The Python cover controller stops movement at estimated endpoints.
An incorrect position estimate can prevent useful movement toward a physical endpoint.
This change records agreed cover behavior before implementation.
Bounded full-travel runs let the motor reach its physical limit and correct the estimated position.

## What Changes

- Normalize physical presses and integration commands into open, close, and stop requests
- Preserve physical press-and-hold behavior by mapping releases to stop requests
- Apply the same request rules regardless of integration origin
- Continue movement on same-direction requests without extending its deadline
- Stop on opposite-direction requests; require a fresh request before starting again
- Omit initiating-source ownership and the all-buttons-released lockout
- Keep request handling and output interlocking independent for each cover
- Use one global full-travel duration with independent timers for each cover
- Keep movement independent of estimated endpoints; establish position after a completed full-travel run
- Use successful output writes as confirmation without immediate read-back
- Confirm both outputs OFF before every activation, without an explicit reversal delay
- Handle pending operations, output failures, OFF retries, and automatic recovery
- Restore position after clean shutdown; keep persistence failures independent of local control
- Define startup, shutdown, and detected input-failure behavior
- Keep local control independent of MQTT; publish current state on reconnect
- Ignore retained MQTT movement commands; accept retained stop commands

## Capabilities

### New Capabilities

- `cover-control`: Requests, bounded movement, position estimates, output recovery, persistence, lifecycle, and MQTT behavior

### Modified Capabilities

None

## Impact

- Future cover behavior in `internal/controller` and physical binding normalization
- Future command normalization in MQTT and other integrations
- Output execution must enforce per-cover direction exclusivity
- Output execution must return completion results, including failures and writes that leave values unchanged
- Position persistence and cover state reporting require new integration work
- No implementation or new dependencies in this checkpoint

## Checkpoint Scope

Source context: collaborative exploration, `../covers/covers.py`, and the historical cover plan at `14bf97a855d52cac134fdc5af227395fe50d6b86`.
The scenarios record agreed target behavior, not behavior already implemented on this branch.
The design records the reviewed implementation approach.
Implementation tasks remain pending.

Hardware assumption:

- Motor limit switches stop physical movement while direction relays can remain energized

Reviewed design decisions:

- Preserve the central semantic-event dispatch model
- Keep semantic cover state, operation phase, estimated position, and output fault in one runtime model
- Return output completion and input failures through the existing sysfs states channel
- Correlate commands and results; preserve per-output execution order
- Keep sysfs command admission bounded and non-blocking; isolate saturated worker queues
- Use one sequential sysfs output worker per cover, owning both direction relays
- Keep direction interlocking in the central cover controller; worker ordering alone does not enforce it
- Give each cover exclusive ownership of its two relays
- Handle stored deadlines with a controller-owned timer, not per-operation context cancellation
- Estimate position from elapsed time
- Stage shutdown so output execution and feedback remain available
- Dispatch consumer-independent semantic events to persistence and MQTT integrations
- Keep MQTT reporting bounded and non-blocking; coalesce pending cover observations

Resolved behavior questions:

- Startup button sampling belongs to input initialization, outside cover-specific behavior
- No explicit reversal delay; confirm both outputs OFF before energizing either direction
- During preparation, preserve same-direction intent and cancel it on stop or opposite-direction requests
- Discard direction requests during stopping or recovery, not preparation
- A blocked output operation on one cover must not prevent another cover from starting or stopping
- After output recovery, report the endpoint if full travel completed
- After an output failure before full-travel completion, report stopped with unknown position

Deferred deployment values:

- Full-travel duration, output-operation timeout, OFF retry interval, shutdown period, and persistence location

Idempotency currently covers repeated same-direction requests while moving.
Duplicate delivery after intervening requests needs separate discussion.
Persistence remains best-effort.
A failure that prevents invalidating an old record can cause temporarily incorrect position reporting after restart.
Estimated position never restricts a full-travel run.
