# Proposal

## Why

Local cover control needs bounded movement and exclusive direction outputs, including during failures and restarts.
Define the agreed safety behavior before selecting small implementation increments.

## What Changes

- Add source-independent open, close, and stop requests with physical press-and-hold mapping
- Keep the complete cover lifecycle in one shade runtime near sysfs, separate from low-level hardware access
- Route open, close, and stop requests through the controller; retain reported state without a second movement FSM
- Cancel unstarted movement; follow an ON write already in progress with OFF on stop or opposite-direction requests
- Never reverse automatically or resume interrupted movement
- Confirm both direction outputs OFF before activation and after stopping
- Keep movement deadlines, interlocking, and fixed-interval OFF retries inside the shade runtime
- Report lifecycle state without delaying deadlines or cleanup; keep relay write results internal to the shade runtime
- Use bounded per-shade request channels and independent execution for each cover
- Recover queue saturation within the affected shade; block requests, discard queued movement, and establish both outputs OFF
- Reopen admission only after confirmed OFF and require a fresh movement request
- Treat stuck sysfs writes as fatal process failure, detected by one shared watchdog
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
- Dedicated shade lifecycle model and workers beside sysfs, with sequential relay writes and reported state
- Shared stuck-write supervision without replacement workers or timeout recovery
- Physical button normalization and controller lifecycle
- Shared integration dispatch where blocking could delay local stops or movement deadlines
- Scenario tests for normal movement, pending commands, failures, and lifecycle behavior

See `design.md` for agreed ownership and execution decisions.
The design records one authoritative shade FSM, controller request and status handling, and per-shade recovery.
Implementation increments follow separately.
The historical `spec-driven-covers` branch provides reference material, not mandatory requirements or architecture.
During design, multi-step functions must expose the ordered named responsibilities used to explain their behavior.
Callers retain ordering, branching, and failure rules.
