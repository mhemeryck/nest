# Cover Control Spec Delta

## Purpose

Define bounded cover movement, position reporting, and consistent request behavior across physical inputs and integrations.
Keep each cover's direction outputs mutually exclusive.

## ADDED Requirements

### Requirement: Source-independent semantic requests

The controller SHALL handle open, close, and stop requests according to their meaning, regardless of integration origin.

#### Scenario: Equivalent requests from different integrations

- **GIVEN** the same cover state
- **WHEN** equivalent direction requests arrive from physical input normalization or an integration
- **THEN** the controller applies the same movement rule

### Requirement: Physical press-and-hold mapping

A physical direction-button press SHALL produce the bound direction request for its cover.
A release of either bound direction button SHALL produce a stop request for that cover.
Release handling SHALL NOT depend on which source started movement.

#### Scenario: Hold and release open

- **GIVEN** a stopped cover
- **WHEN** its open button is pressed
- **THEN** the cover starts opening
- **WHEN** that button is released
- **THEN** the cover stops

#### Scenario: Hold and release close

- **GIVEN** a stopped cover
- **WHEN** its close button is pressed
- **THEN** the cover starts closing
- **WHEN** that button is released
- **THEN** the cover stops

#### Scenario: Release without movement ownership

- **GIVEN** a cover is moving
- **WHEN** a bound direction button is released
- **THEN** the cover stops regardless of which source started movement

### Requirement: Direction request while stopped

A ready, stopped cover SHALL start the requested direction when it receives a new open or close request.
The cover SHALL be ready only after both outputs have successful OFF results and no output fault remains.
The controller SHALL NOT require all physical buttons to be released before accepting that request.

#### Scenario: Fresh press while the other button remains held

- **GIVEN** pressing open started opening
- **AND** pressing close then stopped the cover
- **AND** both buttons remain held
- **WHEN** the open button is released and pressed again while close remains held
- **THEN** the cover starts opening
- **AND** the closing output remains off

### Requirement: Same-direction requests are idempotent during movement

A moving cover SHALL continue its current movement when it receives another request for the same direction.
The repeated request SHALL NOT extend the active movement deadline.

#### Scenario: Repeated open request

- **GIVEN** a cover is opening
- **WHEN** another open request arrives
- **THEN** opening continues without a stop or restart
- **AND** any active movement deadline remains unchanged

#### Scenario: Repeated close request

- **GIVEN** a cover is closing
- **WHEN** another close request arrives
- **THEN** closing continues without a stop or restart
- **AND** any active movement deadline remains unchanged

### Requirement: Opposite-direction request stops movement

A moving cover SHALL stop when it receives a request for the opposite direction.
That request SHALL NOT automatically start movement in the opposite direction.
Starting again SHALL require a subsequent direction request.

#### Scenario: Close request while opening

- **GIVEN** a cover is opening
- **WHEN** a close request arrives
- **THEN** the cover stops
- **AND** closing does not start automatically

#### Scenario: Open request while closing

- **GIVEN** a cover is closing
- **WHEN** an open request arrives
- **THEN** the cover stops
- **AND** opening does not start automatically

#### Scenario: Subsequent request starts movement

- **GIVEN** an opposite-direction request stopped the cover
- **WHEN** the cover is stopped and a subsequent close request arrives
- **THEN** the cover starts closing

### Requirement: Stop requests are always accepted

A stop request SHALL stop a moving cover.
A stop request for a stopped cover SHALL leave it stopped.

#### Scenario: Stop during movement

- **GIVEN** a cover is opening or closing
- **WHEN** a stop request arrives
- **THEN** the cover stops

#### Scenario: Repeated stop

- **GIVEN** a cover is stopped
- **WHEN** a stop request arrives
- **THEN** the cover remains stopped

### Requirement: Per-cover direction exclusivity

The open and close outputs of one cover MUST NOT be energized simultaneously.
This invariant SHALL hold independently of request origin and request sequence.
The cover controller SHALL enforce this invariant across both direction outputs.
Before each activation, the controller SHALL command both outputs OFF and wait for both successful results.
The controller SHALL NOT add an explicit direction-reversal delay after confirmed switch-off.
Each cover SHALL exclusively own two distinct direction outputs.

#### Scenario: Preparation before activation

- **GIVEN** a ready cover with both outputs already off
- **WHEN** a direction request starts a new run
- **THEN** the controller commands both outputs OFF again
- **AND** it activates the requested output only after both commands succeed

#### Scenario: Conflicting output ownership

- **WHEN** configuration assigns a cover output to another cover or a light
- **THEN** configuration validation rejects the assignment

#### Scenario: Competing direction requests

- **GIVEN** a cover's opening output is energized
- **WHEN** a close request arrives
- **THEN** the controller stops the cover without energizing its closing output

#### Scenario: Starting the other direction after a stop

- **GIVEN** a cover was opening and has stopped
- **WHEN** a subsequent close request starts closing
- **THEN** its opening output remains off while its closing output is energized

### Requirement: Independent covers

Request handling and direction exclusivity SHALL apply independently to each cover.
Movement or conflicting requests for one cover SHALL NOT prevent an unrelated cover from moving.
A blocked output operation for one cover SHALL NOT prevent another cover from activating or switching off its outputs.

#### Scenario: Two covers move simultaneously

- **GIVEN** cover A is opening
- **WHEN** cover B receives an open request
- **THEN** cover B starts opening
- **AND** cover A continues opening

#### Scenario: Conflict on one cover

- **GIVEN** covers A and B are opening
- **WHEN** cover A receives a close request
- **THEN** cover A stops
- **AND** cover B continues opening

#### Scenario: Blocked output operation on one cover

- **GIVEN** an output operation for cover A remains blocked
- **WHEN** cover B receives an open request
- **THEN** cover B completes preparation and activates opening without waiting for cover A's operation
- **WHEN** cover B receives a stop request or reaches its movement deadline
- **THEN** cover B switches both outputs off without waiting for cover A's operation

### Requirement: Full-travel duration independent of position

The controller SHALL use one global full-travel duration for all covers.
Each cover SHALL have an independent movement timer.
The timer SHALL start after successful direction activation.
The controller SHALL request both outputs off when the timer expires.
Estimated position SHALL NOT prevent movement or shorten the full-travel duration.
The same duration SHALL apply to opening, closing, physical holds, and integration requests.

#### Scenario: Incorrect estimated endpoint

- **GIVEN** a ready cover has estimated position 100% but is physically partly closed
- **WHEN** an open request arrives and activation succeeds
- **THEN** the cover receives a full-travel opening run
- **AND** estimated position 100% does not end the run early

#### Scenario: Independent deadlines with a shared duration

- **GIVEN** cover A starts opening
- **WHEN** cover B starts closing later
- **THEN** each cover has a deadline based on its own activation time
- **AND** both runs use the same full-travel duration

#### Scenario: Delayed activation

- **WHEN** a direction command is pending
- **THEN** the full-travel timer has not started
- **WHEN** direction activation succeeds
- **THEN** the full-travel timer starts

### Requirement: Estimated position and completed endpoints

The controller SHALL estimate known position during movement and keep the estimate between 0% and 100%.
Reaching an estimated endpoint SHALL NOT complete movement.
After full-travel completion and successful switch-off, the controller SHALL report the endpoint for the completed direction.
Opening SHALL establish state `open` and position 100%.
Closing SHALL establish state `closed` and position 0%.
After an early stop and successful switch-off, the controller SHALL report `stopped` and preserve the estimated position.
Unknown position SHALL remain unknown until a full-travel run completes successfully.

#### Scenario: Estimate reaches the endpoint early

- **GIVEN** a cover is opening with a known position
- **WHEN** its estimate reaches 100% before timer expiry
- **THEN** the reported movement state remains `opening`
- **AND** movement continues until a stop condition occurs

#### Scenario: Complete opening

- **WHEN** the opening timer expires
- **THEN** the controller requests both outputs off
- **WHEN** both OFF writes succeed
- **THEN** the controller reports `open` and position 100%

#### Scenario: Complete closing from unknown position

- **GIVEN** position is unknown
- **WHEN** a closing run completes and both OFF writes succeed
- **THEN** the controller reports `closed` and position 0%

#### Scenario: Early stop with known position

- **GIVEN** a cover is moving with a known position estimate
- **WHEN** a stop request ends movement before full-travel completion and both OFF writes succeed
- **THEN** the controller reports `stopped` with the estimated position
- **AND** the controller does not establish an endpoint from the early stop

#### Scenario: Early stop with unknown position

- **GIVEN** a cover is moving with unknown position
- **WHEN** an early stop completes successfully
- **THEN** the controller reports `stopped` with unknown position

### Requirement: Output write completion

Successful output write completion SHALL count as confirmation of the requested output state.
The output integration SHALL return success or failure for every command, including commands that leave the value unchanged.
Confirmation SHALL NOT require an immediate read-back or physical relay-contact feedback.
The controller SHALL report final cover state only after successful OFF results for both outputs.
The controller SHALL treat a missing output result as failure after a bounded operation timeout.

#### Scenario: Output already off

- **GIVEN** an output is already off
- **WHEN** the integration executes an OFF command successfully
- **THEN** it returns a successful completion result

#### Scenario: Pending switch-off

- **WHEN** a full-travel run ends but one OFF result is pending
- **THEN** the controller does not report successful endpoint completion

#### Scenario: Missing activation result

- **WHEN** activation has no completion result before the operation timeout
- **THEN** the controller treats activation as failed and requests both outputs off
- **AND** the controller does not establish an endpoint

### Requirement: Pending requests preserve event ordering

Cover control SHALL preserve existing event queue behavior and sequential request processing.
Pending output operations SHALL NOT block event processing.
Preparation SHALL be distinct from stopping and recovery, even while its OFF results are pending.
Same-direction requests during preparation or activation SHALL preserve the pending start without restarting pending operations.
A stop or opposite-direction request during preparation SHALL cancel movement intent and enter stopping.
A stop processed during activation SHALL cancel movement intent and request both outputs off.
An opposite-direction request processed during activation SHALL cancel movement without automatically reversing it.
Late results for cancelled activation SHALL NOT start a movement timer or resume movement.
The controller SHALL discard direction requests processed during stopping or recovery.
Output execution SHALL preserve ordering so a late activation cannot undo switch-off.

#### Scenario: Same-direction request during preparation

- **GIVEN** opening preparation is waiting for both OFF results
- **WHEN** another open request arrives
- **THEN** the controller preserves opening intent and the pending operations
- **AND** it activates opening only after both preparation OFF results succeed

#### Scenario: Opposite request during preparation

- **GIVEN** opening preparation is waiting for both OFF results
- **WHEN** a close request arrives
- **THEN** the controller cancels opening intent and enters stopping
- **AND** neither direction starts after switch-off completes
- **AND** movement requires a subsequent direction request

#### Scenario: Stop during preparation

- **GIVEN** opening preparation is waiting for both OFF results
- **WHEN** a stop request arrives
- **THEN** the controller cancels opening intent and enters stopping
- **AND** late preparation results do not activate opening

#### Scenario: Stop during activation

- **GIVEN** opening activation is pending
- **WHEN** the controller processes a stop request
- **THEN** it cancels opening intent and requests both outputs off
- **WHEN** the cancelled activation result arrives
- **THEN** it does not start the travel timer or restart opening

#### Scenario: Opposite request during activation

- **GIVEN** opening activation is pending
- **WHEN** the controller processes a close request
- **THEN** it cancels opening and requests both outputs off
- **AND** closing does not start automatically

#### Scenario: Direction request during switch-off

- **GIVEN** the cover is stopping or recovering with OFF completion pending
- **WHEN** an open or close request is processed
- **THEN** the controller discards the request
- **AND** it does not execute that request after switch-off completes

### Requirement: Per-cover output fault recovery

An output operation failure SHALL put the affected cover into a fault state.
While faulted, the controller SHALL reject direction requests, accept stop requests, and report the cover as unavailable.
The controller SHALL retry both OFF commands at a bounded interval.
Failed switch-off SHALL NOT produce a successful final state report.
Successful OFF results for both outputs SHALL automatically clear the output fault.
Recovery SHALL NOT resume interrupted movement.
A fault on one cover SHALL NOT prevent unrelated covers from operating.
After recovery from failed switch-off following full-travel completion, the controller SHALL report the completed endpoint.
After recovery from an output failure before full-travel completion, the controller SHALL report stopped with unknown position.

#### Scenario: Recovery after completed opening

- **GIVEN** a full opening run completed but switch-off failed
- **WHEN** retries successfully switch both outputs off
- **THEN** the controller reports open with position 100% and restores availability
- **AND** it does not resume movement

#### Scenario: Recovery after incomplete movement

- **GIVEN** an output failure occurred before full-travel completion
- **WHEN** retries successfully switch both outputs off
- **THEN** the controller reports stopped with unknown position and restores availability
- **AND** it requires a new direction request to move

#### Scenario: OFF write fails

- **WHEN** an OFF write fails during stopping
- **THEN** the affected cover becomes unavailable
- **AND** the controller retries both OFF commands without reporting a successful stop
- **AND** unrelated covers continue operating

#### Scenario: Requests during a fault

- **GIVEN** a cover has an output fault
- **WHEN** direction and stop requests arrive
- **THEN** the controller rejects direction requests and accepts stop requests

#### Scenario: Automatic recovery

- **GIVEN** a cover is faulted after an output failure
- **WHEN** both OFF writes succeed
- **THEN** the controller clears the fault and permits new direction requests
- **AND** interrupted movement does not resume automatically

### Requirement: Non-blocking output command admission

Output command admission SHALL be bounded and SHALL NOT block controller request or deadline processing.
A saturated worker queue SHALL NOT block command routing to unrelated workers.
Rejected commands SHALL produce correlated output failures and SHALL NOT execute later.
Admission SHALL NOT count as output state confirmation.
Admitted commands SHALL preserve per-output execution order.
Rejected OFF commands SHALL leave the cover faulted until successful switch-off through bounded retries.

#### Scenario: Saturated worker during OFF retries

- **GIVEN** cover A's worker remains blocked until OFF retries saturate its command queue
- **AND** cover B uses an unrelated worker
- **WHEN** cover B receives a stop request or reaches its movement deadline
- **THEN** the controller requests both outputs of cover B off without waiting for cover A's worker
- **AND** cover A remains unavailable and retries rejected OFF commands at the bounded interval
- **AND** admission of an OFF command does not report a successful stop

#### Scenario: Rejected activation

- **GIVEN** preparation completed successfully
- **WHEN** admission rejects the requested ON command
- **THEN** the controller treats activation as failed and requests both outputs off
- **AND** the rejected ON command never executes later

### Requirement: Best-effort position persistence

The persistence integration SHALL persist position after successful stopping and record unfinished movement for restart detection.
After a clean shutdown, the controller SHALL restore the last saved position when available.
Detected unclean shutdown, unfinished movement, missing records, or untrusted records SHALL produce unknown position.
Persistence failures SHALL be logged and SHALL NOT block local cover control.
A stale saved position SHALL NOT restrict movement or prevent a completed run from establishing the endpoint.

#### Scenario: Clean restart

- **GIVEN** a clean shutdown saved position 60%
- **WHEN** the controller restarts and restores the record
- **THEN** estimated position is 60%
- **AND** the controller does not resume movement

#### Scenario: Detected unclean restart

- **WHEN** restart detects an unclean shutdown or an unfinished movement record
- **THEN** position is unknown even if an older position value exists

#### Scenario: Persistence failure during operation

- **WHEN** a position or movement-record write fails
- **THEN** the persistence integration logs the failure
- **AND** local control continues

#### Scenario: Undetected stale position

- **GIVEN** a persistence failure leaves a stale endpoint record that restart cannot identify as untrusted
- **WHEN** a direction request arrives after startup readiness
- **THEN** the controller permits a full-travel run despite the restored endpoint estimate
- **WHEN** that run completes and both OFF writes succeed
- **THEN** the controller reports the endpoint for the completed direction

### Requirement: Startup and shutdown output handling

At startup, the controller SHALL request both outputs off for each cover before accepting movement for that cover.
Startup SHALL NOT resume saved movement.
Shutdown SHALL reject new movement requests and request both outputs off.
Shutdown SHALL wait for output results within a bounded shutdown period.
The persistence integration SHALL save final shutdown position only after successful switch-off.
Failed or interrupted switch-off SHALL leave movement recorded as unfinished when persistence succeeds.

#### Scenario: Startup readiness

- **WHEN** the controller starts
- **THEN** it requests both outputs off for each cover
- **AND** each cover rejects movement until both OFF results succeed

#### Scenario: Clean shutdown during movement

- **WHEN** shutdown starts during movement
- **THEN** the controller rejects new movement and requests both outputs off
- **WHEN** both OFF results succeed within the shutdown period
- **THEN** the controller saves the final position for clean restart

#### Scenario: Shutdown cannot confirm switch-off

- **WHEN** switch-off fails or remains incomplete at the shutdown deadline
- **THEN** the controller does not save a successfully stopped position
- **AND** successful persistence leaves the movement marked unfinished

### Requirement: Bound input failures to affected covers

A detected read failure of a bound physical input during movement SHALL request both outputs off for the affected cover.
This rule SHALL apply regardless of which source started movement.
An input failure SHALL NOT stop unrelated covers.
A missed release without a detected failure SHALL remain bounded by the full-travel timer.

#### Scenario: Bound input fails during integration-started movement

- **GIVEN** an integration request started cover A and cover B is also moving
- **WHEN** a physical input bound only to cover A has a detected read failure
- **THEN** the controller requests both outputs of cover A off
- **AND** cover B continues moving

#### Scenario: Release is missed without a detected failure

- **GIVEN** a physical press started movement
- **WHEN** its release is not observed and no input failure is detected
- **THEN** the full-travel timer still requests both outputs off at its deadline

### Requirement: MQTT-independent control and reconnect reporting

Local movement, timers, and output recovery SHALL operate independently of MQTT connectivity.
MQTT publication backpressure SHALL NOT block local request or deadline processing.
On reconnect, the controller SHALL publish current cover state and position when known.
Reconnect SHALL NOT restart movement or reset movement timers.

#### Scenario: MQTT cannot accept publications

- **GIVEN** MQTT reporting is backlogged
- **WHEN** a local stop request or movement deadline is processed
- **THEN** the controller requests both outputs off without waiting for MQTT publication
- **AND** pending cover reporting retains the latest observation

#### Scenario: Disconnect during movement

- **GIVEN** a cover is moving
- **WHEN** MQTT disconnects
- **THEN** local requests remain effective and the movement deadline remains active

#### Scenario: Reconnect during movement

- **GIVEN** a cover is moving with a known position
- **WHEN** MQTT reconnects
- **THEN** the controller publishes current state and estimated position
- **AND** it preserves the active movement deadline without restarting movement

#### Scenario: Reconnect with unknown position

- **WHEN** MQTT reconnects while position is unknown
- **THEN** the controller publishes current cover state without inventing a numeric position

### Requirement: Non-blocking Modbus handoff

Modbus command backpressure SHALL NOT block local cover requests or deadline processing.
Pending Modbus event commands SHALL use bounded FIFO storage.
Pending Modbus light-state updates SHALL retain the latest value per coil.
Event-command overflow SHALL produce a logged integration failure without later replay of the rejected command.
Pending commands SHALL progress when actor capacity returns without requiring another semantic input.

#### Scenario: Modbus cannot accept commands

- **GIVEN** the Modbus command channel and event handoff queue are full
- **WHEN** another event command arrives
- **THEN** the controller logs and reports an integration failure
- **AND** local cover stop requests and deadlines remain effective
- **AND** the rejected event command does not execute later

#### Scenario: Pending light-state replacement

- **GIVEN** a Modbus light-state update is pending for a coil
- **WHEN** a newer update arrives for that coil
- **THEN** pending reporting retains the newer value
- **WHEN** actor capacity returns
- **THEN** the integration sends the latest pending value without another semantic input

### Requirement: Retained MQTT request handling

The controller SHALL ignore retained MQTT open and close commands.
The controller SHALL accept retained MQTT stop commands under the normal stop rules.

#### Scenario: Retained movement command replay

- **WHEN** MQTT delivers a retained open or close command
- **THEN** the command does not start movement

#### Scenario: Retained stop command

- **GIVEN** a cover is moving
- **WHEN** MQTT delivers a retained stop command
- **THEN** the controller requests both outputs off
