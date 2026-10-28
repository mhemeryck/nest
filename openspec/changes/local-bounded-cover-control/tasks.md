# Tasks

## 1. Existing lifecycle model checkpoint

These completed tasks record the original isolated model increment.
They do not establish the worker interface or require the implementation to preserve the current model structure.
The agreed runtime location is now `internal/sysfs/shade`; code migration has not started.

- [x] 1.1 Define the shade lifecycle states, inputs, and effects as plain functions in a dedicated runtime module beside sysfs; verify the model builds and exposes no receiver-based transition API
- [x] 1.2 Implement startup preparation, movement intent, direction interlocking, stop, opposite-direction cancellation, and deadline transitions; add transition tests for normal movement, same-direction requests, stop timing, and no automatic reversal
- [x] 1.3 Implement returned-write-error and shutdown transition behavior in the lifecycle model; add tests proving movement is canceled, OFF remains required, and shutdown never reopens admission
- [x] 1.4 Document the lifecycle phases and transition invariants in the existing cover documentation; verify the documented behavior matches the model and its tests

## 2. Caller-first interface review

Complete this review before further runtime implementation.
The tasks below do not authorize implementation of unresolved boundaries.

- [ ] 2.1 Agree on concrete sysfs relay references and direct writes; verify caller examples use plain functions without callbacks or receiver methods
- [ ] 2.2 Agree on watchdog construction, write registration, monitoring, and fatal-error delivery; verify examples distinguish returned errors from stuck writes
- [ ] 2.3 Agree on application startup and staged shutdown; verify examples keep watchdog supervision active during cleanup and bound the shutdown wait during an active write
- [ ] 2.4 Agree on per-shade state merging; verify a controller receive example preserves typed cover IDs, bounded storage, and latest-state ordering
- [ ] 2.5 Review caller and worker together for open, stop during ON, saturation recovery, and shutdown; verify ownership and sequencing without requiring the existing claim-and-completion protocol
- [ ] 2.6 Reconcile the design and remaining implementation tasks with the reviewed interfaces; verify no blocking design decisions remain before implementation

## 3. Complete per-shade runtime boundary

This increment demonstrates the public caller and worker together, not another isolated lifecycle API.
Keep application cover movement disabled until all safety behavior and integration are complete.

- [ ] 3.1 Add the reviewed concrete sysfs write API with shared watchdog supervision; test unchanged-value success, no preliminary OFF read, returned errors, and fatal stuck-write detection
- [ ] 3.2 Add `New`, `Run`, `Submit`, Open, Close, and Stop in `internal/sysfs/shade`, storing both relay references directly; verify construction and per-handle caller examples without receivers or callbacks
- [ ] 3.3 Implement bounded command admission, its synchronization, and independent recovery signalling; test saturation rejection, admission closure, and isolation between shades
- [ ] 3.4 Implement synchronous sequential writes, startup OFF, movement deadlines, and fixed-interval OFF recovery in the owning worker; test complete caller-to-hardware flows and adapt the checkpoint model only where useful
- [ ] 3.5 Integrate same-worker saturation recovery with queued-command handling; test stop during ON, stale-command disposal, recovery races, and fresh-request requirements
- [ ] 3.6 Add `States` with bounded latest-state delivery; test delayed consumption, state ordering, and continued deadlines and OFF retries
- [ ] 3.7 Implement worker shutdown under the reviewed lifecycle contract; test pending writes, shutdown during recovery, full command queues, bounded waiting, and admission that never reopens
- [ ] 3.8 Update cover documentation for the reviewed runtime and interface; verify caller examples and safety terminology against tests and run `devenv tasks run dev:check`

## 4. Application integration and validation

- [ ] 4.1 Wire typed cover-ID lookup, per-shade handles, state merging, and controller state retention; add integration tests for source-independent open, close, and stop
- [ ] 4.2 Validate shared positive timing settings and exclusive relay ownership before runtime starts; test invalid timing, missing relays, conflicting ownership, and exclusion from shared sysfs command workers
- [ ] 4.3 Wire application startup, watchdog lifetime, and staged shutdown; test independent startup recovery and bounded cleanup across multiple shades
- [ ] 4.4 Keep configured cover movement unavailable until the complete safety runtime is wired; verify the application cannot start partially configured cover control
- [ ] 4.5 Update the cover roadmap and operational documentation; verify position reporting, persistence, and MQTT / Home Assistant remain later slices
- [ ] 4.6 Run `devenv tasks run dev:check`, strict OpenSpec validation, and `git diff --check`; verify all pass before completing the change
