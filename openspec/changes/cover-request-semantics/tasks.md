# Tasks

## 1. Cover configuration and domain model

- [ ] 1.1 Add typed cover IDs, relay references, and actions; verify entity conversion and ID tests
- [ ] 1.2 Add cover configuration and shared timing settings; verify parsing, projection, and positive-duration validation tests
- [ ] 1.3 Add cover registry indexes and exclusive relay validation; verify lookup and conflicting-ownership tests
- [ ] 1.4 Extend bindings for cover press and release requests; verify existing light bindings and cover normalization scenarios

## 2. Sysfs feedback through the states channel

- [ ] 2.1 Add tagged observation, completion, and input-failure reports; verify normalization never converts completion into a button edge
- [ ] 2.2 Add command identifiers and completion timestamps; verify unchanged writes, failed writes, and invalid targets return matching results
- [ ] 2.3 Make explicit ON/OFF writes independent of preliminary reads; verify write success after read failure and unchanged toggle behavior
- [ ] 2.4 Preserve worker ordering and report input failures; verify ON followed by OFF execution and affected-input identification
- [ ] 2.5 Add bounded non-blocking command admission and router rejection; verify correlated failures, rejected commands never execute, and per-output FIFO ordering
- [ ] 2.6 Wire one sequential sysfs output worker per cover from registry ownership; verify both relays share its FIFO queue and unrelated covers use separate workers
- [ ] 2.7 Exclude cover relays from type-grouped workers; verify existing device routing, single-owner polling, and unchanged per-relay polling intervals

## 3. Cover transitions and semantic events

- [ ] 3.1 Add the unified runtime model and consumer-independent semantic events; verify phase, state, position, and fault transition tests
- [ ] 3.2 Implement controller-owned startup and prepare-before-activate interlocking across both cover relays; verify both OFF results precede every ON command
- [ ] 3.3 Implement request handling and cancellation; verify same-direction, opposite-direction, and stop requests during preparation and activation, release handling, and direction rejection during stopping or recovery
- [ ] 3.4 Correlate results by command and generation; verify cancelled, duplicate, and stale results cannot advance newer operations
- [ ] 3.5 Implement stopping and output recovery; verify complete-travel endpoints, incomplete-travel uncertainty, and independent-cover scenarios
- [ ] 3.6 Map detected bound input failures to stopping; verify integration-started movement stops only on affected covers

## 4. Deadlines and position estimates

- [ ] 4.1 Integrate earliest-deadline scheduling into dispatch; verify travel expiry, operation timeout, retry timing, and stale expiry tests
- [ ] 4.2 Prevent deadline starvation during event traffic; verify due OFF commands under sustained queued events
- [ ] 4.3 Estimate position from completion timestamps and elapsed time; verify clamping, rounding, unknown position, and delayed-result scenarios
- [ ] 4.4 Emit periodic semantic observations independently of travel timing; verify reporting never extends or shortens movement

## 5. Persistence integration

- [ ] 5.1 Add persistence dispatch mapping and actor wiring; verify domain event payloads contain no storage-specific instructions
- [ ] 5.2 Implement versioned snapshots and ordered replacement writes; verify clean, unfinished, malformed, and reassigned-relay records
- [ ] 5.3 Restore trusted position during startup without movement; verify clean and unclean restart scenarios
- [ ] 5.4 Keep storage failures and overload non-blocking; verify failure logging, trust invalidation, and continued local stopping

## 6. Ordered lifecycle handling

- [ ] 6.1 Stage controller shutdown before actor cancellation in internal/nest; verify sysfs and normalization remain alive for OFF completion
- [ ] 6.2 Bound shutdown and finish persistence in order; verify confirmed shutdown, missing OFF results, and interrupted-write scenarios
- [ ] 6.3 Handle actor termination during shutdown; verify runtime exits without deadlock or falsely marking movement clean

## 7. MQTT and Home Assistant

- [ ] 7.1 Normalize cover commands with retained metadata; verify retained movement rejection and retained stop acceptance
- [ ] 7.2 Map semantic observations into retained state, nullable position attributes, and availability; verify no successful final state during pending OFF
- [ ] 7.3 Extend device discovery with cover components; verify combined availability and absence of unsupported position-target commands
- [ ] 7.4 Add bounded latest-observation handoff and reconnect snapshots; verify queue saturation cannot block local requests or deadlines
- [ ] 7.5 Validate Home Assistant state and attribute mapping; verify known-to-unknown position, stopped-state mapping, faults, and reconnect behavior

## 8. Integration verification

- [ ] 8.1 Exercise simultaneous multi-cover runs through fake sysfs and MQTT actors; verify at most one direction relay per cover is ON after every hardware write under rapid competing requests, cancellation, delayed results, failures, timeouts, and recovery
- [ ] 8.2 Keep one cover worker blocked through OFF retry queue saturation; verify another cover completes preparation, activation, and stop or deadline switch-off before the blocked operation returns, rejected activation never executes, and admission never confirms switch-off
- [ ] 8.3 Document timing values, persistence location, and migration steps; verify example configuration passes validation
- [ ] 8.4 Run nest-format and nest-check inside devenv shell; verify formatting, lint, vet, race tests, and builds pass
- [ ] 8.5 Validate the change with openspec and compare delivered behavior with scenarios; record verification before archive
