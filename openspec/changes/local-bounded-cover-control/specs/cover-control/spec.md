# Cover Control Spec Delta

## Purpose

Define local bounded cover movement with independent deadlines, exclusive direction outputs, automatic fault recovery, and safe lifecycle handling.

## ADDED Requirements

### Requirement: Source-independent requests

The controller SHALL route open, close, and stop requests to the shade runtime without tracking the initiating source.
The shade runtime SHALL apply the same request behavior for every source.
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
- **THEN** the shade runtime stops the cover without checking the initiating source

### Requirement: Single lifecycle authority

The shade runtime SHALL own movement intent, direction interlocking, movement deadlines, and OFF recovery for each cover.
The controller SHALL send requests and retain reported state without independently transitioning the movement lifecycle.

#### Scenario: Controller sends a movement request

- **WHEN** the controller routes an open request to a ready cover
- **THEN** the shade runtime prepares, activates, and bounds that movement
- **AND** it reports lifecycle state without requiring controller relay commands between those steps

#### Scenario: Report delivery is delayed

- **GIVEN** a cover is stopping or recovering
- **WHEN** the controller receives an older movement report
- **THEN** that report does not restore movement intent or change authoritative lifecycle state

### Requirement: Direction requests while stopped

The shade runtime SHALL start the requested direction only after confirming both direction outputs OFF.
An unknown or estimated position SHALL NOT restrict movement.

#### Scenario: Start opening

- **GIVEN** a stopped cover ready for movement
- **WHEN** an open request arrives
- **THEN** the shade runtime confirms both direction outputs OFF before activating the opening output

#### Scenario: Start closing with unknown position

- **GIVEN** a stopped cover with unknown position
- **WHEN** a close request arrives
- **THEN** the shade runtime prepares and starts closing without a position restriction

### Requirement: Repeated direction requests

The shade runtime SHALL preserve the existing start intent or movement on a same-direction request.
The request SHALL NOT extend the movement deadline.

#### Scenario: Repeat during start preparation

- **GIVEN** opening is pending
- **WHEN** another open request arrives
- **THEN** the shade runtime keeps the existing opening intent without adding another start

#### Scenario: Repeat during movement

- **GIVEN** a cover opening with an active movement deadline
- **WHEN** another open request arrives
- **THEN** opening continues with the original deadline

### Requirement: Opposite-direction requests

An opposite-direction request SHALL cancel unstarted movement or request switch-off after an ON write already in progress.
The shade runtime SHALL NOT reverse automatically.
A fresh direction request after confirmed switch-off SHALL be required to start again.

#### Scenario: Opposite request during start preparation

- **GIVEN** opening is pending but its ON write has not started
- **WHEN** a close request arrives
- **THEN** the shade runtime cancels opening and completes switch-off
- **AND** it does not start closing

#### Scenario: Opposite request after ON starts

- **GIVEN** an opening ON write has started but has not returned
- **WHEN** a close request arrives
- **THEN** OFF follows the ON write without concurrent or reordered writes
- **AND** brief opening is permitted before deactivation
- **AND** closing does not start automatically

#### Scenario: Opposite request during movement

- **GIVEN** a cover opening
- **WHEN** a close request arrives
- **THEN** the shade runtime switches both direction outputs OFF without starting closing
- **WHEN** switch-off succeeds and a fresh close request arrives
- **THEN** the shade runtime prepares and starts closing

### Requirement: Stop cancels movement intent

Stop handling SHALL initiate or continue switch-off in every cover state.
A stop request SHALL cancel unstarted movement intent.
An ON write already in progress SHALL finish before subsequent OFF writes.
Brief activation after release SHALL be permitted when ON has already started.
If stop admission fails because the request queue is full, per-shade recovery SHALL establish switch-off instead.
A late ON completion SHALL NOT restore canceled movement intent.

#### Scenario: Release before ON starts

- **GIVEN** opening is pending while the shade runtime confirms both outputs OFF
- **WHEN** a stop request is handled before ON starts
- **THEN** the shade runtime completes switch-off without activating the opening output

#### Scenario: Stop after ON starts

- **GIVEN** an opening ON write has started but has not completed
- **WHEN** a stop request arrives
- **THEN** the shade runtime ensures OFF executes after that ON write
- **AND** brief activation after release is permitted
- **AND** an ON completion does not restore movement intent
- **AND** the cover remains unavailable for movement until both outputs are confirmed OFF

#### Scenario: Stop while already stopped

- **GIVEN** both outputs are confirmed OFF with no pending start
- **WHEN** a stop request arrives
- **THEN** the cover remains stopped

### Requirement: No retained starts during switch-off

The shade runtime SHALL NOT retain direction requests received while switch-off is pending.
A fresh direction request after confirmed switch-off SHALL be required for movement.

#### Scenario: Direction request during switch-off

- **GIVEN** a cover switching its outputs OFF
- **WHEN** an open or close request arrives
- **THEN** the shade runtime continues switch-off without retaining the request
- **WHEN** both outputs are confirmed OFF
- **THEN** the cover remains stopped until a fresh direction request arrives

### Requirement: Direction output exclusivity

The shade runtime SHALL NOT activate both direction outputs together.
It SHALL confirm both outputs OFF before activating a direction.
It SHALL NOT allow a new start until switch-off succeeds.

#### Scenario: Switch-off remains incomplete

- **GIVEN** one direction output has not been confirmed OFF
- **WHEN** the shade runtime prepares a start or completes a stop
- **THEN** it does not activate either direction until both outputs are confirmed OFF

### Requirement: Independent covers

A slow or failed output operation for one cover SHALL NOT delay another cover's requests, stops, or movement deadline.
MQTT and Modbus backpressure SHALL NOT delay local cover stops or movement deadlines.
This independence SHALL apply while the process runs; a stuck-write timeout SHALL cause fatal process failure.

#### Scenario: One cover has a blocked output

- **GIVEN** an output operation for one cover is blocked
- **AND** the write timeout has not caused fatal process failure
- **WHEN** another cover receives a request or reaches its movement deadline
- **THEN** that cover handles its request or deadline without waiting for the blocked operation

#### Scenario: Integration backpressure during movement

- **GIVEN** MQTT or Modbus dispatch is congested
- **WHEN** a local cover receives stop or reaches its movement deadline
- **THEN** switch-off proceeds without waiting for that integration

### Requirement: Bounded movement independent of position

Each cover SHALL have an independent full-travel deadline starting when its direction ON command succeeds.
The shade runtime SHALL initiate switch-off at that deadline, regardless of estimated position or physical end-limit switches.
Button readability SHALL NOT control the movement deadline or movement admission.

#### Scenario: Full-travel deadline expires

- **GIVEN** a direction ON command succeeds
- **WHEN** the configured full-travel duration elapses without an earlier stop
- **THEN** the shade runtime initiates switch-off without requiring another controller command
- **AND** it requires confirmed OFF before allowing another start

#### Scenario: Motor reaches its physical endpoint first

- **GIVEN** the motor stops at its built-in end-limit switch before the movement deadline
- **WHEN** the deadline expires
- **THEN** the shade runtime still switches the direction relay OFF

#### Scenario: Button release cannot be read

- **GIVEN** a cover moving with an active deadline
- **WHEN** a button becomes unreadable and no stop request arrives
- **THEN** the existing movement deadline remains in effect
- **AND** the shade runtime initiates switch-off at that deadline
- **AND** button readability does not create a cover movement lockout

### Requirement: Output fault containment

A returned output write error SHALL block movement for the affected cover and initiate OFF recovery.
The shade runtime SHALL NOT treat a failed write as confirmed OFF.
Stuck-write timeouts SHALL follow fatal failure handling rather than automatic recovery.

#### Scenario: ON write returns an error

- **GIVEN** a cover starting movement
- **WHEN** its ON write returns an error
- **THEN** the shade runtime cancels movement intent and attempts to switch both outputs OFF
- **AND** it blocks new movement until both outputs are confirmed OFF

#### Scenario: OFF write returns an error

- **GIVEN** a cover switching outputs OFF
- **WHEN** an OFF write returns an error
- **THEN** the shade runtime keeps movement blocked and continues OFF recovery

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

The shade runtime SHALL retry OFF writes that return errors at a fixed interval until OFF succeeds or execution stops.
It SHALL NOT accumulate retries while an earlier OFF write remains pending.
Recovery SHALL NOT retry movement commands or resume interrupted movement.
After confirmed OFF, the shade runtime SHALL allow fresh movement requests without an explicit reset unless shutdown has started.

#### Scenario: Repeated switch-off failure

- **GIVEN** an OFF attempt fails
- **WHEN** the retry interval elapses
- **THEN** the shade runtime retries switch-off with the same interval for subsequent retries
- **AND** movement remains blocked

#### Scenario: Earlier OFF attempt remains pending

- **GIVEN** an OFF attempt is still pending
- **WHEN** another retry would otherwise become due
- **THEN** the shade runtime does not accumulate additional pending attempts

#### Scenario: Recovery succeeds

- **GIVEN** a cover in OFF recovery after interrupted movement outside shutdown
- **WHEN** both outputs are confirmed OFF
- **THEN** the shade runtime permits fresh direction requests without an explicit reset
- **AND** the interrupted movement does not resume

### Requirement: Startup switch-off

At startup, each shade runtime SHALL switch both direction outputs OFF for its cover.
It SHALL NOT accept movement for a cover before confirming both outputs OFF.
Startup SHALL NOT resume previous movement.

#### Scenario: Successful startup

- **WHEN** the controller starts
- **THEN** each shade runtime attempts to switch both outputs OFF for its cover
- **AND** each cover becomes available for fresh movement only after its outputs are confirmed OFF
- **AND** no previous movement resumes

#### Scenario: Startup switch-off fails

- **WHEN** startup switch-off returns an error for a cover
- **THEN** that cover remains blocked and enters automatic OFF recovery
- **AND** other covers can become available independently

### Requirement: Bounded shutdown

On shutdown, the runtime SHALL block new requests and attempt to confirm both outputs OFF for all covers.
Shutdown SHALL cancel unstarted movement and discard queued movement requests.
An in-progress write SHALL finish before cleanup writes start.
Shutdown signalling SHALL remain available independently of request-queue capacity.
The shutdown wait SHALL NOT exceed the configured shutdown timeout.

#### Scenario: Outputs switch OFF during shutdown

- **GIVEN** covers moving or preparing to move
- **WHEN** shutdown starts
- **THEN** each shade runtime cancels unstarted movement intent and establishes switch-off
- **AND** shade workers continue cleanup and OFF retries until confirmed OFF or the shutdown timeout
- **AND** no new movement starts

#### Scenario: Shutdown with queued movement

- **GIVEN** a movement request is queued but its ON write has not started
- **WHEN** shutdown starts
- **THEN** the shade runtime discards that movement and establishes both outputs OFF
- **AND** it does not activate the queued direction

#### Scenario: Shutdown during recovery

- **GIVEN** a cover is recovering with movement admission blocked
- **WHEN** shutdown starts and OFF recovery then succeeds
- **THEN** admission remains blocked and no movement resumes

#### Scenario: Shutdown with a full request queue

- **GIVEN** a shade request queue is full
- **WHEN** shutdown starts
- **THEN** the shade runtime receives shutdown independently of that queue and performs cleanup

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
The output-command timeout SHALL measure an in-progress write, not queue waiting or the movement duration.

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
- **THEN** movement can continue until its full-travel deadline
- **AND** stuck-write supervision does not treat normal activation time as a pending write

### Requirement: Reported lifecycle state

The shade runtime SHALL report lifecycle phase and direction to the controller.
Stopped SHALL mean both direction outputs are confirmed OFF with no pending movement.
Stopped SHALL NOT imply a known fully open or fully closed position.
Relay observations SHALL NOT substitute for successful write results in the shade lifecycle.

#### Scenario: Movement finishes

- **WHEN** both outputs are confirmed OFF after movement
- **THEN** the shade runtime reports Stopped without claiming a known physical endpoint

#### Scenario: Failed switch-off followed by recovery

- **WHEN** an OFF write fails
- **THEN** the shade runtime enters Recovering without claiming confirmed OFF
- **WHEN** subsequent OFF writes confirm both outputs OFF
- **THEN** it transitions to Stopped unless shutdown requires admission to remain blocked

#### Scenario: Output already OFF

- **GIVEN** a direction output is already OFF
- **WHEN** an explicit OFF write succeeds
- **THEN** the shade runtime uses that successful result despite the unchanged value

### Requirement: Bounded non-blocking state reporting

State reporting SHALL use bounded storage without delaying requests, movement deadlines, recovery, OFF retries, or shutdown cleanup.
After delayed delivery becomes available, reporting SHALL provide the latest lifecycle state without requiring every intermediate transition.
Reports for one cover SHALL NOT overwrite newer delivered state with older state.
Repeated OFF retry errors SHALL be logged without requiring repeated lifecycle transitions.

#### Scenario: State delivery is delayed at expiry

- **GIVEN** ON succeeds but the controller cannot receive the movement report
- **WHEN** the movement duration expires
- **THEN** the shade runtime attempts OFF without waiting for report delivery
- **WHEN** state delivery becomes available
- **THEN** the controller receives the latest lifecycle state

#### Scenario: Repeated OFF failures during delayed reporting

- **GIVEN** a cover is Recovering and state delivery is delayed
- **WHEN** subsequent OFF retries return errors
- **THEN** the shade runtime continues fixed-interval retries and logs the errors
- **AND** state-reporting storage remains bounded
- **WHEN** both outputs are confirmed OFF and delivery becomes available
- **THEN** the controller receives the resulting state without controlling completion of recovery

### Requirement: Per-shade request saturation recovery

Each cover SHALL have bounded request admission that does not block other covers or the central controller loop.
A full request queue SHALL return an explicit failure and trigger recovery only for the affected cover.
Admission SHALL block before recovery is signalled.
Recovery signalling SHALL remain available independently of request-queue capacity.
The existing shade worker SHALL discard queued movement and establish both outputs OFF without replacement workers or concurrent writes.
Requests during recovery SHALL be rejected rather than retained.
Admission SHALL reopen only after both outputs are confirmed OFF and stale movement can no longer execute.
A fresh request after recovery SHALL be required to move again.

#### Scenario: Shade request queue is full

- **WHEN** an open, close, or stop request cannot enter a cover's bounded queue
- **THEN** submission returns an explicit failure and blocks that cover's admission
- **AND** the existing shade worker receives recovery independently of the full queue
- **AND** it discards queued movement and establishes both outputs OFF
- **AND** other covers continue without an application restart

#### Scenario: Request arrives during recovery handover

- **GIVEN** queue saturation has blocked admission but cleanup has not finished
- **WHEN** a new direction request or stop request arrives
- **THEN** submission rejects it without retaining movement or removing the cleanup obligation

#### Scenario: Recovery waits for an in-progress ON write

- **GIVEN** ON is in progress when saturation triggers recovery
- **WHEN** that write returns within its timeout
- **THEN** the same worker follows it with OFF cleanup
- **AND** the ON result does not restore movement intent

#### Scenario: OFF recovery fails after saturation

- **GIVEN** saturation recovery is switching both outputs OFF
- **WHEN** an OFF write returns an error
- **THEN** the shade runtime keeps admission blocked and retries OFF at the configured interval

#### Scenario: Fresh request after recovery

- **GIVEN** queued movement has been discarded and both outputs are confirmed OFF
- **WHEN** recovery completes outside shutdown
- **THEN** admission reopens without replaying queued or rejected requests
- **WHEN** a fresh direction request is admitted
- **THEN** the shade runtime prepares that new movement

### Requirement: Explicit integration queue rejection

Rejected MQTT and Modbus commands SHALL be logged without a deferred-delivery guarantee.
Integration queue saturation SHALL NOT block local cover control.

#### Scenario: Integration queue is full

- **WHEN** an MQTT or Modbus command cannot enter its bounded queue
- **THEN** dispatch logs an explicit rejection without retaining the command for later delivery
- **AND** local cover control continues
