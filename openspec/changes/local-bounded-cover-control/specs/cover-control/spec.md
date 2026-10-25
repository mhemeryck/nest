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

An opposite-direction request SHALL cancel a pending start or stop current movement.
The controller SHALL NOT reverse automatically.
A fresh direction request after confirmed switch-off SHALL be required to start again.

#### Scenario: Opposite request during start preparation

- **GIVEN** opening is pending
- **WHEN** a close request arrives
- **THEN** the controller cancels opening and completes switch-off
- **AND** the controller does not start closing

#### Scenario: Opposite request during movement

- **GIVEN** a cover opening
- **WHEN** a close request arrives
- **THEN** the controller switches both direction outputs OFF without starting closing
- **WHEN** switch-off succeeds and a fresh close request arrives
- **THEN** the controller prepares and starts closing

### Requirement: Stop cancels movement intent

The controller SHALL accept stop requests in every cover state.
A stop request SHALL cancel pending movement intent and initiate or continue switch-off.
A late ON completion SHALL NOT restore canceled movement intent.

#### Scenario: Release before ON submission

- **GIVEN** opening is pending while the controller confirms both outputs OFF
- **WHEN** a stop request arrives before ON submission
- **THEN** the controller completes switch-off without activating the opening output

#### Scenario: Stop after ON submission

- **GIVEN** an opening ON command is submitted but has not completed
- **WHEN** a stop request arrives
- **THEN** the controller ensures OFF executes after the submitted ON
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

#### Scenario: One cover has a blocked output

- **GIVEN** an output operation for one cover is blocked
- **WHEN** another cover receives a request or reaches its movement deadline
- **THEN** the controller handles that cover without waiting for the blocked operation

#### Scenario: Integration backpressure during movement

- **GIVEN** MQTT or Modbus dispatch is congested
- **WHEN** a local cover receives stop or reaches its movement deadline
- **THEN** the controller initiates switch-off without waiting for that integration

### Requirement: Bounded movement independent of position

Each cover SHALL have an independent full-travel deadline starting when its direction ON command succeeds.
At the deadline, the controller SHALL initiate relay switch-off regardless of estimated position or physical end-limit switches.
Button readability SHALL NOT control the movement deadline or movement admission.

#### Scenario: Full-travel deadline expires

- **GIVEN** a direction ON command succeeds
- **WHEN** the configured full-travel duration elapses without an earlier stop
- **THEN** the controller initiates switch-off
- **AND** it requires confirmed OFF before allowing another start

#### Scenario: Motor reaches its physical endpoint first

- **GIVEN** the motor stops at its built-in end-limit switch before the movement deadline
- **WHEN** the deadline expires
- **THEN** the controller still switches the direction relay OFF

#### Scenario: Button release cannot be read

- **GIVEN** a cover moving with an active deadline
- **WHEN** a button becomes unreadable and no stop request arrives
- **THEN** the existing movement deadline remains in effect
- **AND** the controller initiates switch-off at that deadline
- **AND** button readability does not create a cover movement lockout

### Requirement: Output fault containment

An output-command failure or timeout SHALL block movement for the affected cover and initiate OFF recovery.
The controller SHALL NOT treat a failed or timed-out command as confirmed OFF.

#### Scenario: ON command fails or times out

- **GIVEN** a cover starting movement
- **WHEN** its ON command fails or exceeds the output-command timeout
- **THEN** the controller cancels movement intent and attempts to switch both outputs OFF
- **AND** it blocks new movement until both outputs are confirmed OFF

#### Scenario: OFF command fails or times out

- **GIVEN** a cover switching outputs OFF
- **WHEN** an OFF command fails or exceeds the output-command timeout
- **THEN** the controller keeps movement blocked and continues OFF recovery

### Requirement: Automatic OFF recovery

The controller SHALL retry OFF commands at a fixed interval while running until both outputs are confirmed OFF.
It SHALL NOT accumulate retries while an earlier OFF attempt remains pending.
Recovery SHALL NOT retry movement commands or resume interrupted movement.
After confirmed OFF, the controller SHALL allow fresh movement requests without an explicit reset.

#### Scenario: Repeated switch-off failure

- **GIVEN** an OFF attempt fails
- **WHEN** the retry interval elapses
- **THEN** the controller retries switch-off with the same interval for subsequent retries
- **AND** movement remains blocked

#### Scenario: Earlier OFF attempt remains pending

- **GIVEN** an OFF attempt is still pending
- **WHEN** another retry would otherwise become due
- **THEN** the controller does not accumulate additional pending attempts

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

- **WHEN** startup switch-off fails or times out for a cover
- **THEN** that cover remains blocked and enters automatic OFF recovery
- **AND** other covers can become available independently

### Requirement: Bounded shutdown

On shutdown, the controller SHALL prevent new starts and attempt to confirm both outputs OFF for all covers.
The shutdown wait SHALL NOT exceed the configured shutdown timeout.

#### Scenario: Outputs switch OFF during shutdown

- **GIVEN** covers moving or preparing to move
- **WHEN** shutdown starts
- **THEN** the controller cancels movement intent and attempts switch-off for every cover
- **AND** it does not start new movement

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
Each cover SHALL track its own commands and deadlines independently.

#### Scenario: Shared durations with independent deadlines

- **GIVEN** two covers start at different times
- **WHEN** their ON commands succeed
- **THEN** each receives its own deadline using the shared full-travel duration

#### Scenario: Non-positive duration

- **WHEN** any configured cover timing duration is zero or negative
- **THEN** configuration validation rejects startup
