# Tasks

## 1. Cover configuration and domain model

- [x] 1.1 Add typed cover IDs, relay references, and actions; verify entity conversion and ID tests
- [x] 1.2 Add cover configuration and shared timing settings; verify parsing, projection, and positive-duration validation tests
- [x] 1.3 Add cover registry indexes and exclusive relay validation; verify lookup and conflicting-ownership tests
- [x] 1.4 Extend bindings for cover press and release requests; verify existing light bindings and cover normalization scenarios

## 2. Sysfs feedback through the states channel

- [x] 2.1 Add tagged observation, completion, and input-failure reports; verify normalization never converts completion into a button edge
- [x] 2.2 Add command identifiers and completion timestamps; verify unchanged writes, failed writes, and invalid targets return matching results
- [x] 2.3 Make explicit ON/OFF writes independent of preliminary reads; verify write success after read failure and unchanged toggle behavior
- [x] 2.4 Preserve worker ordering and report input failures; verify ON followed by OFF execution and affected-input identification
- [x] 2.5 Add bounded non-blocking command admission and router rejection; verify correlated failures, rejected commands never execute, and per-output FIFO ordering
- [x] 2.6 Wire one sequential sysfs output worker per cover from registry ownership; verify both relays share its FIFO queue and unrelated covers use separate workers
- [x] 2.7 Exclude cover relays from type-grouped workers; verify existing device routing, single-owner polling, and unchanged per-relay polling intervals

## 3. Cover transitions and semantic events

- [x] 3.1 Add the unified runtime model and consumer-independent semantic events; verify phase, state, position, and fault transition tests
- [x] 3.2 Implement controller-owned startup and prepare-before-activate interlocking across both cover relays; verify both OFF results precede every ON command
- [x] 3.3 Implement request handling and cancellation; verify same-direction, opposite-direction, and stop requests during preparation and activation, release handling, and direction rejection during stopping or recovery
- [x] 3.4 Correlate results by command and generation; verify cancelled, duplicate, and stale results cannot advance newer operations
- [x] 3.5 Implement stopping and output recovery; verify complete-travel endpoints, incomplete-travel uncertainty, and independent-cover scenarios
- [x] 3.6 Map detected bound input failures to stopping; verify integration-started movement stops only on affected covers
- [x] 3.7 Add bounded non-blocking Modbus handoff; verify event-command FIFO, per-coil light-state coalescing, logged overflow failures, and continued cover request and deadline processing

## 4. Deadlines and position estimates

- [x] 4.1 Integrate earliest-deadline scheduling into dispatch; verify travel expiry, operation timeout, retry timing, and stale expiry tests
- [x] 4.2 Prevent deadline starvation during event traffic; verify due OFF commands under sustained queued events
- [x] 4.3 Estimate position from completion timestamps and elapsed time; verify clamping, rounding, unknown position, and delayed-result scenarios
- [x] 4.4 Emit periodic semantic observations independently of travel timing; verify reporting never extends or shortens movement

## 5. Persistence integration

- [x] 5.1 Add persistence dispatch mapping and actor wiring; verify domain event payloads contain no storage-specific instructions
- [x] 5.2 Implement versioned snapshots and ordered replacement writes; verify clean, unfinished, malformed, and reassigned-relay records
- [x] 5.3 Restore trusted position during startup without movement; verify clean and unclean restart scenarios
- [x] 5.4 Keep storage failures and overload non-blocking; verify failure logging, trust invalidation, and continued local stopping

## 6. Ordered lifecycle handling

- [x] 6.1 Stage controller shutdown before actor cancellation in internal/nest; verify sysfs and normalization remain alive for OFF completion
- [x] 6.2 Bound shutdown and finish persistence in order; verify confirmed shutdown, missing OFF results, and interrupted-write scenarios
- [x] 6.3 Handle actor termination during shutdown; verify runtime exits without deadlock or falsely marking movement clean

## 7. MQTT and Home Assistant

- [x] 7.1 Normalize cover commands with retained metadata; verify retained movement rejection and retained stop acceptance
- [x] 7.2 Map semantic observations into retained state, nullable position attributes, and availability; verify no successful final state during pending OFF
- [x] 7.3 Extend device discovery with cover components; verify combined availability and absence of unsupported position-target commands
- [x] 7.4 Add bounded latest-observation handoff and reconnect snapshots; verify queue saturation cannot block local requests or deadlines
- [ ] 7.5 Validate Home Assistant state and attribute mapping; verify known-to-unknown position, stopped-state mapping, faults, and reconnect behavior

## 8. Integration verification

- [x] 8.1 Exercise simultaneous multi-cover runs through fake sysfs and MQTT actors; verify at most one direction relay per cover is ON after every hardware write under rapid competing requests, cancellation, delayed results, failures, timeouts, and recovery
- [x] 8.2 Keep one cover worker blocked through OFF retry queue saturation; verify another cover completes preparation, activation, and stop or deadline switch-off before the blocked operation returns, rejected activation never executes, and admission never confirms switch-off
- [x] 8.3 Document timing values, persistence location, and migration steps; verify example configuration passes validation
- [x] 8.4 Run nest-format and nest-check inside devenv shell; verify formatting, lint, vet, race tests, and builds pass
- [x] 8.5 Validate the change with openspec and compare delivered behavior with scenarios; record verification before archive
