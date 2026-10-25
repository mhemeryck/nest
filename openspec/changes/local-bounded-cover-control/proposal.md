# Proposal

## Why

Local cover control needs bounded movement and exclusive direction outputs, including during failures and restarts.
Define the agreed safety behavior before selecting small implementation increments.

## What Changes

- Add source-independent open, close, and stop requests with physical press-and-hold mapping
- Cancel pending starts on stop or opposite-direction requests; never reverse automatically
- Confirm both direction outputs OFF before activation and after stopping
- Give each cover an independent movement deadline and output fault recovery
- Retry only OFF commands at a fixed interval; never resume interrupted movement
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
- Sysfs output ordering and command completion feedback
- Physical button normalization and controller lifecycle
- Shared integration dispatch where blocking could delay local stops or movement deadlines
- Scenario tests for normal movement, pending commands, failures, and lifecycle behavior

Design and implementation increments follow separately.
The historical `spec-driven-covers` branch provides reference material, not mandatory requirements or architecture.
During design, multi-step functions must expose the ordered named responsibilities used to explain their behavior.
Callers retain ordering, branching, and failure rules.
