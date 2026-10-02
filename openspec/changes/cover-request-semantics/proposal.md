# Proposal

## Why

The historical cover plan includes interaction policies that need explicit review before implementation.
This checkpoint records agreed request behavior so further exploration builds on shared decisions.

## What Changes

- Normalize physical presses and integration commands into open, close, and stop requests
- Preserve physical press-and-hold behavior by mapping releases to stop requests
- Apply the same request rules regardless of integration origin
- Continue movement on same-direction requests without extending its deadline
- Stop on opposite-direction requests; require a fresh request before starting again
- Omit initiating-source ownership and the all-buttons-released lockout
- Keep request handling and output interlocking independent for each cover

## Capabilities

### New Capabilities

- `cover-control`: Semantic request handling, physical input mapping, and per-cover output exclusivity

### Modified Capabilities

None

## Impact

- Future cover behavior in `internal/controller` and physical binding normalization
- Future command normalization in MQTT and other integrations
- Output execution must enforce per-cover direction exclusivity
- No implementation or new dependencies in this checkpoint

## Checkpoint Scope

Source context: cover plan at `14bf97a855d52cac134fdc5af227395fe50d6b86` and collaborative exploration.
The scenarios record agreed target behavior, not behavior already implemented on this branch.
Design and implementation tasks remain pending.

Further exploration:

- Movement timeout policy and whether it applies equally to physical holds and integration commands
- Output confirmation, pending operations, reversal delay, and failure handling
- Startup input sampling, input-read failures, and shutdown behavior
- Home Assistant state, discovery, and reconnect behavior
- Position estimation and alternative physical button interactions

Idempotency currently covers repeated same-direction requests while moving.
Duplicate delivery after intervening requests needs separate discussion.
