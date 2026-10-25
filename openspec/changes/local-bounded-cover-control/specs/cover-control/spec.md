# Cover Control Spec Delta

## Purpose

Define local bounded cover movement with independent deadlines, exclusive direction outputs, automatic fault recovery, and safe lifecycle handling.

## ADDED Requirements

### Requirement: Source-independent requests

The controller SHALL apply open, close, and stop requests without tracking the initiating source.
A physical direction-button press SHALL produce its bound direction request.
A release of either bound direction button SHALL produce a stop request.

#### Scenario: Hold and release a direction button

- **GIVEN** a stopped cover ready for movement
- **WHEN** the user presses its open button
- **THEN** the controller handles an open request
- **WHEN** the user releases either bound direction button
- **THEN** the controller handles a stop request

#### Scenario: Stop from a different source

- **GIVEN** a cover opening after a request from another source
- **WHEN** either bound direction button produces a release
- **THEN** the controller stops the cover without checking the initiating source

### Requirement: Direction requests while stopped

The controller SHALL start the requested direction only after confirming both direction outputs OFF.
An unknown or estimated position SHALL NOT restrict movement.

#### Scenario: Start opening

- **GIVEN** a stopped cover ready for movement
- **WHEN** an open request arrives
- **THEN** the controller confirms both direction outputs OFF before activating the opening output

#### Scenario: Start closing with unknown position

- **GIVEN** a stopped cover with unknown position
- **WHEN** a close request arrives
- **THEN** the controller prepares and starts closing without a position restriction

### Requirement: Repeated direction requests

The controller SHALL preserve the existing start intent or movement on a same-direction request.
The request SHALL NOT extend the movement deadline.

#### Scenario: Repeat during start preparation

- **GIVEN** opening is pending
- **WHEN** another open request arrives
- **THEN** the controller keeps the existing opening intent without adding another start

#### Scenario: Repeat during movement

- **GIVEN** a cover opening with an active movement deadline
- **WHEN** another open request arrives
- **THEN** opening continues with the original deadline

### Requirement: Opposite-direction requests

An opposite-direction request SHALL cancel an unsubmitted start or request switch-off after a submitted activation.
The controller SHALL NOT reverse automatically.
A fresh direction request after confirmed switch-off SHALL be required to start again.

#### Scenario: Opposite request during start preparation

- **GIVEN** opening is pending but activation has not been submitted
- **WHEN** a close request arrives
- **THEN** the controller cancels opening and completes switch-off
- **AND** the controller does not start closing

#### Scenario: Opposite request after activation submission

- **GIVEN** opening activation is submitted but has not executed
- **WHEN** a close request arrives
- **THEN** deactivation follows the submitted activation in FIFO order
- **AND** brief opening is permitted before deactivation
- **AND** closing does not start automatically

#### Scenario: Opposite request during movement

- **GIVEN** a cover opening
- **WHEN** a close request arrives
- **THEN** the controller switches both direction outputs OFF without starting closing
- **WHEN** switch-off succeeds and a fresh close request arrives
- **THEN** the controller prepares and starts closing

### Requirement: Stop cancels movement intent

The controller SHALL accept stop requests in every cover state.
A stop request SHALL cancel unsubmitted movement intent and initiate or continue switch-off.
Submitted activations SHALL execute in FIFO order before subsequent deactivation.
Brief activation after release SHALL be permitted for an already-submitted activation.
A late ON completion SHALL NOT restore canceled movement intent.

#### Scenario: Release before activation submission

- **GIVEN** opening is pending while the controller confirms both outputs OFF
- **WHEN** a stop request arrives before activation submission
- **THEN** the controller completes switch-off without activating the opening output

#### Scenario: Stop after activation submission

- **GIVEN** an opening activation is submitted but has not completed
- **WHEN** a stop request arrives
- **THEN** the controller ensures OFF executes after the submitted ON
- **AND** brief activation after release is permitted if ON has not executed yet
- **AND** an ON completion does not restore movement intent
- **AND** the cover remains unavailable for movement until both outputs are confirmed OFF

#### Scenario: Stop while already stopped

- **GIVEN** both outputs are confirmed OFF with no pending start
- **WHEN** a stop request arrives
- **THEN** the cover remains stopped

### Requirement: No retained starts during switch-off

The controller SHALL NOT retain direction requests received while switch-off is pending.
A fresh direction request after confirmed switch-off SHALL be required for movement.

#### Scenario: Direction request during switch-off

- **GIVEN** a cover switching its outputs OFF
- **WHEN** an open or close request arrives
- **THEN** the controller continues switch-off without retaining the request
- **WHEN** both outputs are confirmed OFF
- **THEN** the cover remains stopped until a fresh direction request arrives

### Requirement: Direction output exclusivity

The controller SHALL NOT activate both direction outputs together.
It SHALL confirm both outputs OFF before activating a direction.
It SHALL NOT allow a new start until switch-off succeeds.

#### Scenario: Switch-off remains incomplete

- **GIVEN** one direction output has not been confirmed OFF
- **WHEN** the controller prepares a start or completes a stop
- **THEN** it does not activate either direction until both outputs are confirmed OFF

### Requirement: Independent covers

A slow or failed output operation for one cover SHALL NOT delay another cover's requests, stops, or movement deadline.
MQTT and Modbus backpressure SHALL NOT delay local cover stops or movement deadlines.
This independence SHALL apply while the process runs; a stuck-write timeout SHALL cause fatal process failure.

#### Scenario: One cover has a blocked output

- **GIVEN** an output operation for one cover is blocked
- **AND** the write timeout has not caused fatal process failure
- **WHEN** another cover receives a request or reaches its movement deadline
- **THEN** the controller handles that cover without waiting for the blocked operation

#### Scenario: Integration backpressure during movement

- **GIVEN** MQTT or Modbus dispatch is congested
- **WHEN** a local cover receives stop or reaches its movement deadline
- **THEN** switch-off proceeds without waiting for that integration

### Requirement: Bounded movement independent of position

Each cover SHALL have an independent full-travel deadline starting when its direction ON command succeeds.
The bounded-activation executor SHALL initiate switch-off at that deadline, regardless of estimated position or physical end-limit switches.
Button readability SHALL NOT control the movement deadline or movement admission.

#### Scenario: Full-travel deadline expires

- **GIVEN** a direction ON command succeeds
- **WHEN** the configured full-travel duration elapses without an earlier stop
- **THEN** the executor initiates switch-off without requiring another controller command
- **AND** it requires confirmed OFF before allowing another start

#### Scenario: Motor reaches its physical endpoint first

- **GIVEN** the motor stops at its built-in end-limit switch before the movement deadline
- **WHEN** the deadline expires
- **THEN** the controller still switches the direction relay OFF

#### Scenario: Button release cannot be read

- **GIVEN** a cover moving with an active deadline
- **WHEN** a button becomes unreadable and no stop request arrives
- **THEN** the existing movement deadline remains in effect
- **AND** the executor initiates switch-off at that deadline
- **AND** button readability does not create a cover movement lockout

### Requirement: Output fault containment

A returned output write error SHALL block movement for the affected cover and initiate OFF recovery.
The controller SHALL NOT treat a failed write as confirmed OFF.
Stuck-write timeouts SHALL follow fatal failure handling rather than automatic recovery.

#### Scenario: ON write returns an error

- **GIVEN** a cover starting movement
- **WHEN** its ON write returns an error
- **THEN** the controller cancels movement intent and attempts to switch both outputs OFF
- **AND** it blocks new movement until both outputs are confirmed OFF

#### Scenario: OFF write returns an error

- **GIVEN** a cover switching outputs OFF
- **WHEN** an OFF write returns an error
- **THEN** the controller keeps movement blocked and continues OFF recovery

### Requirement: Fatal stuck-write handling

An in-progress sysfs write that exceeds the configured output-command timeout SHALL cause fatal process failure.
The process SHALL NOT recover from that timeout or start replacement writes.
Fatal failure SHALL NOT imply confirmed OFF.

#### Scenario: ON write remains blocked

- **GIVEN** an ON write has started but has not returned
- **WHEN** stuck-write supervision detects that the write exceeds its timeout
- **THEN** the process fails fatally instead of entering per-cover timeout recovery
- **AND** no replacement worker or write attempts to bypass the blocked write
- **AND** the process does not claim the output is OFF

#### Scenario: OFF retry remains blocked

- **GIVEN** an OFF retry has started but has not returned
- **WHEN** stuck-write supervision detects that the write exceeds its timeout
- **THEN** the process fails fatally rather than continuing OFF retries

#### Scenario: Restart after fatal failure

- **GIVEN** the process previously failed during a write
- **WHEN** a new process starts
- **THEN** startup confirms both outputs OFF before allowing movement
- **AND** it does not infer safe outputs from the previous process exit

### Requirement: Automatic OFF recovery

The executor SHALL retry OFF writes that return errors at a fixed interval until OFF succeeds or execution stops.
It SHALL NOT accumulate retries while an earlier OFF write remains pending.
Recovery SHALL NOT retry movement commands or resume interrupted movement.
After confirmed OFF, the controller SHALL allow fresh movement requests without an explicit reset.

#### Scenario: Repeated switch-off failure

- **GIVEN** an OFF attempt fails
- **WHEN** the retry interval elapses
- **THEN** the executor retries switch-off with the same interval for subsequent retries
- **AND** movement remains blocked

#### Scenario: Earlier OFF attempt remains pending

- **GIVEN** an OFF attempt is still pending
- **WHEN** another retry would otherwise become due
- **THEN** the executor does not accumulate additional pending attempts

#### Scenario: Recovery succeeds

- **GIVEN** a cover in OFF recovery after interrupted movement
- **WHEN** both outputs are confirmed OFF
- **THEN** the controller permits fresh direction requests without an explicit reset
- **AND** the interrupted movement does not resume

### Requirement: Startup switch-off

At startup, the controller SHALL switch both direction outputs OFF for each cover.
It SHALL NOT accept movement for a cover before confirming both outputs OFF.
Startup SHALL NOT resume previous movement.

#### Scenario: Successful startup

- **WHEN** the controller starts
- **THEN** it attempts to switch both outputs OFF for every cover
- **AND** each cover becomes available for fresh movement only after its outputs are confirmed OFF
- **AND** no previous movement resumes

#### Scenario: Startup switch-off fails

- **WHEN** startup switch-off returns an error for a cover
- **THEN** that cover remains blocked and enters automatic OFF recovery
- **AND** other covers can become available independently

### Requirement: Bounded shutdown

On shutdown, the controller SHALL prevent new activation submissions and attempt to confirm both outputs OFF for all covers.
Already-submitted activations SHALL retain FIFO order before shutdown deactivation.
The shutdown wait SHALL NOT exceed the configured shutdown timeout.

#### Scenario: Outputs switch OFF during shutdown

- **GIVEN** covers moving or preparing to move
- **WHEN** shutdown starts
- **THEN** the controller cancels unsubmitted movement intent and requests switch-off for every cover
- **AND** executors continue deadlines and OFF retries until confirmed OFF or the shutdown timeout
- **AND** the controller submits no new activations

#### Scenario: Shutdown with queued activation

- **GIVEN** an activation is submitted but has not executed
- **WHEN** shutdown starts
- **THEN** deactivation follows that activation in FIFO order
- **AND** brief activation is permitted before switch-off

#### Scenario: Shutdown switch-off cannot complete

- **GIVEN** shutdown cannot confirm all outputs OFF
- **WHEN** the shutdown timeout expires
- **THEN** the controller ends its shutdown wait without claiming the unconfirmed outputs are OFF

### Requirement: Exclusive relay ownership validation

Configuration validation SHALL reject missing relay references and conflicting cover relay ownership before runtime starts.
Each cover SHALL reference two different relays.
No cover relay SHALL also belong to another cover or another controlled entity.

#### Scenario: Invalid cover relay assignment

- **WHEN** a cover references a missing relay or uses the same relay for both directions
- **THEN** configuration validation rejects startup

#### Scenario: Relay shared with another entity

- **WHEN** a cover relay also belongs to another cover or a light
- **THEN** configuration validation rejects startup

### Requirement: Shared timing configuration

The configuration SHALL provide separate positive durations for full travel, output-command timeout, OFF-retry interval, and shutdown timeout.
These values SHALL apply to all covers.
Each cover's output execution SHALL track commands and deadlines independently.
The output-command timeout SHALL measure an in-progress write, not queue waiting or the bounded activation duration.

#### Scenario: Shared durations with independent deadlines

- **GIVEN** two covers start at different times
- **WHEN** their ON commands succeed
- **THEN** each receives its own deadline using the shared full-travel duration

#### Scenario: Non-positive duration

- **WHEN** any configured cover timing duration is zero or negative
- **THEN** configuration validation rejects startup

#### Scenario: Activation lasts longer than write timeout

- **GIVEN** the full-travel duration exceeds the output-command timeout
- **WHEN** ON returns successfully within its write timeout
- **THEN** the bounded activation can continue until its full-travel deadline
- **AND** stuck-write supervision does not treat normal activation time as a pending write

### Requirement: Bounded activation feedback

Output execution SHALL distinguish successful activation, confirmed OFF completion, and write failure.
A failed write SHALL NOT imply confirmed OFF.
Successful unchanged writes SHALL still produce a result.

#### Scenario: Bounded activation completes

- **WHEN** ON succeeds for a bounded activation
- **THEN** execution returns activation feedback with its actual activation time
- **WHEN** OFF succeeds after expiry or early stop
- **THEN** execution returns confirmed completion

#### Scenario: Failed switch-off followed by recovery

- **WHEN** an OFF write fails
- **THEN** execution returns failure feedback without claiming confirmed OFF
- **WHEN** a subsequent OFF retry succeeds
- **THEN** execution returns confirmed completion

#### Scenario: Output already OFF

- **GIVEN** a direction output is already OFF
- **WHEN** an explicit OFF write succeeds
- **THEN** execution returns a successful result despite the unchanged value

### Requirement: Bounded lifecycle feedback

Execution SHALL retain lifecycle results within bounded feedback capacity without delaying deadlines, deactivation, or OFF retries.
Admission SHALL limit accepted operations to the capacity needed for their lifecycle results.
Each operation SHALL emit at most one failure event; repeated retry errors SHALL be logged.
Successful OFF after failure SHALL still produce confirmed completion.

#### Scenario: Feedback delivery is delayed at expiry

- **GIVEN** ON succeeds but the states channel cannot accept its activation result
- **WHEN** the activation duration expires
- **THEN** the executor attempts OFF without waiting for result delivery
- **AND** it retains lifecycle results for delivery in order

#### Scenario: Repeated OFF failures during delayed feedback

- **GIVEN** OFF returns an error and failure feedback is pending or delivered
- **WHEN** subsequent OFF retries return errors
- **THEN** the executor continues fixed-interval retries and logs the errors without adding failure events
- **WHEN** OFF succeeds
- **THEN** it retains confirmed completion for delivery

#### Scenario: Feedback capacity is reserved

- **WHEN** accepting another operation would exceed available lifecycle feedback capacity
- **THEN** admission rejects that operation explicitly
- **AND** cleanup of accepted operations continues using their reserved feedback capacity

### Requirement: Explicit queue rejection

Full command queues SHALL reject commands explicitly rather than block local cover control.
A rejected cover output command SHALL cause the affected cover to remain blocked and seek confirmed OFF.
Rejected MQTT and Modbus commands SHALL be logged without a deferred-delivery guarantee.

#### Scenario: Cover output queue is full

- **WHEN** a cover output command cannot enter its bounded queue
- **THEN** submission returns an explicit failure
- **AND** the cover blocks movement and seeks confirmed OFF
- **AND** other covers continue independently

#### Scenario: Integration queue is full

- **WHEN** an MQTT or Modbus command cannot enter its bounded queue
- **THEN** dispatch logs an explicit rejection without retaining the command for later delivery
- **AND** local cover control continues

### Requirement: Deactivation admission retry

The controller SHALL retry rejected deactivation admission at the configured retry interval while cleanup remains required and execution runs.
After admission succeeds, the executor SHALL own OFF execution and retries.
The controller SHALL NOT resubmit admitted deactivation as an admission retry.
Movement SHALL remain blocked until both outputs are confirmed OFF.

#### Scenario: Rejected stop is later admitted

- **GIVEN** the cover is stopping and its deactivation submission is rejected
- **WHEN** the admission retry interval elapses
- **THEN** the controller retries submission without retaining a new movement request
- **WHEN** deactivation is admitted
- **THEN** the controller stops admission retries for that operation
- **AND** the executor handles OFF writes and any returned errors

#### Scenario: Activation deadline remains effective during admission congestion

- **GIVEN** a bounded activation is admitted and early deactivation is rejected
- **WHEN** its activation duration expires before deactivation admission succeeds
- **THEN** the executor initiates OFF without waiting for the rejected deactivation to be resubmitted
