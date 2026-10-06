# Cover Control Runtime

## Requests and relay ownership

- OPEN and CLOSE start a full-travel run when the cover is ready
- Same-direction requests preserve the active deadline
- Opposite-direction requests stop the cover; a subsequent request starts the other direction
- Physical releases produce STOP, regardless of which source starts movement
- Each cover exclusively owns two distinct relays

The central controller confirms both OFF writes before submitting either ON command.
One sequential sysfs output worker executes each cover's commands in FIFO order.
A blocked operation on one cover does not occupy another cover's worker.
Write completion confirms the requested output state without physical relay-contact feedback.
Motor limit switches stop physical movement at the endpoints.

## Configuration

Example: [config.covers.yaml](../test/fixtures/config.covers.yaml)

All units use the root `cover_control` settings.
All durations must be positive when covers are configured.
Example values illustrate the format; deployment values require measurement on the selected unit.
Choose one full-travel duration sufficient for every cover in both directions.

| Setting                | Example | Purpose                                       |
| ---------------------- | ------- | --------------------------------------------- |
| `full_travel_duration` | `30s`   | Shared maximum run time in either direction   |
| `operation_timeout`    | `2s`    | Maximum wait for an output completion result  |
| `off_retry_interval`   | `1s`    | Delay before the next switch-off retry        |
| `shutdown_period`      | `5s`    | Bound for output confirmation and persistence |
| `reporting_interval`   | `500ms` | Position observation interval                 |

Set `units.<unit>.actors.persistence.path` to a distinct snapshot file for each unit.
The example uses `.devenv/cover-positions/controller_1.json`.
Select a persistent writable location for deployment.

Validate the example from the repository root:

```nu
devenv shell -- go run ./cmd/nest --config test/fixtures/config.covers.yaml --validate controller_1
```

## Position, faults, and persistence

Position uses elapsed time from successful activation, not rounded periodic increments.
An estimated endpoint does not stop a run or establish an endpoint state.
Only completed full travel and successful switch-off establish `open` or `closed`.
Early stops preserve known estimates and report `stopped`.
Unknown estimates remain unknown until a successful full-travel run.

Output failures make the affected cover unavailable.
The controller rejects movement and retries both OFF writes.
Recovery clears the fault without resuming movement.
Incomplete failed runs recover with unknown position.

Persistence records unfinished movement from start intent.
Confirmed switch-off clears that record.
Clean shutdown follows output confirmation and ordered snapshot writes.
Snapshots include version, unit identity, cover identity, and relay assignment.
Unclean sessions, unfinished records, and changed assignments restore unknown position.
Storage errors do not block local control.
Undetected stale records can restore an incorrect estimate, but never restrict a new full-travel run.

## MQTT and Home Assistant

- Commands: `<prefix>/units/<unit>/covers/<cover>/command`
- Payloads: `OPEN`, `CLOSE`, and `STOP`
- Ignore retained OPEN and CLOSE; accept retained STOP
- Observation: `<prefix>/units/<unit>/covers/<cover>/state`

One retained JSON observation contains semantic state, estimated position, and output availability.
`estimated_position` contains a rounded number or `null`.
`state` preserves Nest semantics, including `stopped` and `unknown`.
`ha_state` supplies the native Home Assistant state payload.
Unknown native state uses `None`.

Device discovery uses the observation topic for state, JSON attributes, and per-cover availability.
Both unit availability and per-cover availability must be online.
Native position topics and position-target commands are omitted.
Replacement attributes clear a previous estimate when position becomes unknown.
MQTT backpressure and actor failure do not block local cover control.
Reconnect requests fresh observations without changing movement deadlines.
Input-read failures also send STOP requests to MQTT-bound remote covers.

Modbus handoff also remains non-blocking.
Pending event commands preserve FIFO order; pending light-state updates coalesce per coil.
Event overflow produces a logged integration failure.
Cover command transport over Modbus is unsupported.

## Migration

1. Measure full-travel duration and select timing values
2. Assign distinct relays and validate configuration
3. Confirm outputs OFF and stop the legacy relay owner
4. Deploy to one non-critical unit
5. Verify startup OFF, local press-and-hold control, independent covers, and travel expiry
6. Verify failed-write recovery and bounded shutdown
7. Verify Home Assistant discovery, unknown-position replacement, stopped mapping, and reconnect on the deployed version
8. Verify clean and unclean restart behavior

For rollback, confirm outputs OFF and stop Nest before restoring the legacy relay owner.
