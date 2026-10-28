# Tasks

## 1. Shade lifecycle model and transition tests

- [x] 1.1 Define the shade lifecycle states, inputs, and effects as plain functions in a dedicated runtime module beside sysfs; verify the model builds and exposes no receiver-based transition API
- [x] 1.2 Implement startup preparation, movement intent, direction interlocking, stop, opposite-direction cancellation, and deadline transitions; add transition tests for normal movement, same-direction requests, stop timing, and no automatic reversal
- [x] 1.3 Implement returned-write-error and shutdown transition behavior in the lifecycle model; add tests proving movement is canceled, OFF remains required, and shutdown never reopens admission
- [x] 1.4 Document the lifecycle phases and transition invariants in the existing cover documentation; verify the documented behavior matches the model and its tests

## 2. Sequential sysfs execution and watchdog

- [ ] 2.1 Add explicit-value sysfs writes that return success for unchanged writes and do not read before OFF; add tests for write success, unchanged values, and read/write errors
- [ ] 2.2 Implement one sequential worker per shade for both relays, movement deadlines, and fixed-interval OFF retries; add worker tests proving output interlocking, deadline behavior, and retry ordering
- [ ] 2.3 Add shared watchdog supervision for in-progress writes; test that stuck writes trigger fatal failure while queue waits and movement deadlines do not
- [ ] 2.4 Document write sequencing, deadline handling, and watchdog limits in the existing cover documentation; verify examples and terminology match the implementation

## 3. Request admission, saturation recovery, and reporting

- [ ] 3.1 Add bounded non-blocking per-shade request admission and an independent recovery signal; test full-queue rejection, admission closure, and isolation between shades
- [ ] 3.2 Implement same-worker saturation recovery that discards queued movement, confirms both relays OFF, retries returned OFF errors, and reopens admission only after cleanup; test recovery races and fresh-request requirements
- [ ] 3.3 Add bounded latest-state reporting to the controller boundary; test delayed delivery, per-shade report ordering, and continued deadlines and cleanup
- [ ] 3.4 Document request rejection, recovery, and reported lifecycle state in the existing cover documentation; verify documented guarantees against tests

## 4. Runtime integration, startup, and shutdown

- [ ] 4.1 Wire typed cover IDs, relay ownership, shade workers, controller request routing, and reported-state retention; add integration tests for source-independent open, close, and stop requests
- [ ] 4.2 Validate shared positive timing settings and shade relay configuration before runtime starts; add tests for invalid timing and conflicting relay ownership
- [ ] 4.3 Integrate startup switch-off and staged shutdown with bounded waiting and continued OFF retries; add tests for startup errors, shutdown during movement or recovery, and full request queues
- [ ] 4.4 Keep configured cover movement unavailable until the complete safety runtime is wired; verify the application cannot start partially configured cover control
- [ ] 4.5 Update the existing cover roadmap and operational documentation for the completed first slice; verify docs describe position reporting, persistence, and MQTT / Home Assistant as later slices
- [ ] 4.6 Run `devenv tasks run dev:check`, strict OpenSpec validation, and `git diff --check`; verify all pass before completing the change
