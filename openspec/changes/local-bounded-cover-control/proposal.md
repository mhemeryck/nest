# Proposal

## Why

Local cover control needs bounded movement and exclusive direction outputs, including during failures and restarts.
Define the agreed safety behavior before selecting small implementation increments.

## What Changes

- Add source-independent open, close, and stop requests with physical press-and-hold mapping
- Cancel unsubmitted starts; follow submitted activations with FIFO deactivation on stop or opposite-direction requests
- Accept brief activation after release when activation was already submitted; never reverse automatically
- Confirm both direction outputs OFF before activation and after stopping
- Use generic bounded activation with executor-owned deadlines and fixed-interval OFF retries
- Return activation, confirmed completion, and failure feedback through the existing states channel
- Isolate cover output execution and reject commands explicitly when bounded queues are full
- Reject congested MQTT / Modbus commands explicitly rather than block local control
- Confirm safe outputs at startup and attempt safe outputs within a bounded shutdown wait
- Reject invalid relay references and conflicting relay ownership
- Configure shared positive durations for full travel, output timeout, OFF retries, and shutdown

This change covers the first slice in `docs/covers.md`.
Position reporting, position persistence, requested-position stopping, and MQTT / Home Assistant cover integration remain out of scope.
Position never restricts movement.
Cover control does not track the initiating source or depend on button readability.
Motors have built-in end-limit switches, but the controller must still switch the direction relays OFF.

## Capabilities

### New Capabilities

- `cover-control`: Local bounded movement, request ordering, output safety, independent covers, recovery, and lifecycle behavior

### Modified Capabilities

None

## Impact

- Cover configuration, domain entities, relay ownership validation, and controller request handling
- Sysfs bounded activation, FIFO output ordering, worker isolation, and execution feedback
- Physical button normalization and controller lifecycle
- Shared integration dispatch where blocking could delay local stops or movement deadlines
- Scenario tests for normal movement, pending commands, failures, and lifecycle behavior

See `design.md` for agreed ownership and execution decisions.
Detailed FSM review and implementation increments follow separately.
The historical `spec-driven-covers` branch provides reference material, not mandatory requirements or architecture.
During design, multi-step functions must expose the ordered named responsibilities used to explain their behavior.
Callers retain ordering, branching, and failure rules.
