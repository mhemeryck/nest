# Cover Control Spec Delta

## Purpose

Define consistent cover request behavior across physical inputs and integrations while keeping each cover's direction outputs mutually exclusive.

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

A stopped cover SHALL start the requested direction when it receives a new open or close request.
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
