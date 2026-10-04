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
Design and implementation tasks remain pending.

Hardware assumption:

- Motor limit switches stop physical movement while direction relays can remain energized

Design work:

- Full-travel duration value, output-operation timeout, OFF retry interval, and bounded shutdown period
- Output command ordering and result correlation within the existing event flow
- Persistence format and clean-shutdown detection
- Position estimation from elapsed time rather than rounded periodic increments
- Home Assistant discovery and representation of unknown position, pending operations, and faults

Remaining behavior questions:

- Startup sampling of held physical buttons
- Any hardware requirement for a delay before starting the other direction after confirmed switch-off
- Reporting after recovery from a failed switch-off following full-travel completion

Idempotency currently covers repeated same-direction requests while moving.
Duplicate delivery after intervening requests needs separate discussion.
Persistence remains best-effort.
A failure that prevents invalidating an old record can cause temporarily incorrect position reporting after restart.
Estimated position never restricts a full-travel run.
