package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCoverController() (*coverController, time.Time) {
	reg := registry.Build(&entity.Root{
		CoverControl:  entity.CoverControl{FullTravelDuration: 10 * time.Second, OperationTimeout: 2 * time.Second, OffRetryInterval: time.Second, ShutdownPeriod: 5 * time.Second, ReportingInterval: time.Second},
		Covers:        []entity.Cover{{ID: "a", OpenRelay: "a_open", CloseRelay: "a_close"}, {ID: "b", OpenRelay: "b_open", CloseRelay: "b_close"}},
		Relays:        []entity.Relay{{ID: "a_open", SysfsDevice: "ro_1_1"}, {ID: "a_close", SysfsDevice: "ro_1_2"}, {ID: "b_open", SysfsDevice: "ro_1_3"}, {ID: "b_close", SysfsDevice: "ro_1_4"}},
		DigitalInputs: []entity.DigitalInput{{ID: "input", SysfsDevice: "di_1_1"}},
		PushButtons:   []entity.PushButton{{ID: "button", Input: "input"}},
		Bindings:      []entity.Binding{{Source: "button", Target: "a", Action: "open"}},
	})
	return newCoverController(reg, nil), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func coverOutputCommands(events []event.Event) []event.OutputCommand {
	var commands []event.OutputCommand
	for _, incoming := range events {
		if incoming.Kind == event.OutputCommandKind {
			commands = append(commands, *incoming.OutputCommand)
		}
	}
	return commands
}

func coverResult(controller *coverController, command event.OutputCommand, now time.Time, failure error) event.OutputResult {
	relay, _ := registry.RelayByID(controller.reg, command.RelayID)
	return event.OutputResult{CommandID: command.CommandID, SysfsDevice: relay.SysfsDevice, Action: command.Action, CompletedAt: now, Error: failure}
}

func completeCoverCommands(t *testing.T, controller *coverController, commands []event.OutputCommand, now time.Time, hardware map[entity.RelayID]bool) []event.Event {
	t.Helper()
	var observations []event.Event
	for len(commands) > 0 {
		command := commands[0]
		commands = commands[1:]
		hardware[command.RelayID] = command.Action == entity.OutputActionOn
		for _, runtime := range controller.covers {
			require.False(t, hardware[runtime.cover.OpenRelay] && hardware[runtime.cover.CloseRelay], "both cover relays energized after a hardware write")
		}
		events := handleCoverOutputResult(controller, coverResult(controller, command, now, nil), now)
		commands = append(commands, coverOutputCommands(events)...)
		observations = append(observations, events...)
	}
	return observations
}

func readyTestCovers(t *testing.T, controller *coverController, now time.Time, hardware map[entity.RelayID]bool) {
	t.Helper()
	commands := coverOutputCommands(initializeCovers(controller, now))
	require.Len(t, commands, 4)
	completeCoverCommands(t, controller, commands, now, hardware)
	for _, runtime := range controller.covers {
		require.Equal(t, coverIdle, runtime.phase)
	}
}

func TestCoverPreparationAndSameDirectionIdempotency(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	assert.Empty(t, handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	readyTestCovers(t, controller, now, hardware)
	prepare := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	require.Len(t, prepare, 2)
	for _, command := range prepare {
		assert.Equal(t, entity.OutputActionOff, command.Action)
	}
	assert.Empty(t, handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now.Add(time.Millisecond)))
	assert.Empty(t, coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, prepare[0], now, nil), now)))
	activation := coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, prepare[1], now, nil), now))
	require.Len(t, activation, 1)
	assert.Equal(t, entity.RelayID("a_open"), activation[0].RelayID)
	assert.Equal(t, entity.OutputActionOn, activation[0].Action)
	assert.Empty(t, handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	completeCoverCommands(t, controller, activation, now, hardware)
	deadline := controller.covers["a"].travelDeadline
	assert.Empty(t, handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now.Add(time.Second)))
	assert.Equal(t, deadline, controller.covers["a"].travelDeadline)
	assert.Equal(t, entity.CoverStateOpening, controller.covers["a"].state)
}

func TestCoverCancellationAndStaleResults(t *testing.T) {
	for _, phase := range []coverPhase{coverPreparing, coverActivating, coverMoving} {
		for _, action := range []entity.CoverAction{entity.CoverActionStop, entity.CoverActionClose} {
			t.Run(string(phase)+"/"+string(action), func(t *testing.T) {
				controller, now := testCoverController()
				hardware := make(map[entity.RelayID]bool)
				readyTestCovers(t, controller, now, hardware)
				oldCommands := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
				if phase != coverPreparing {
					var activation []event.OutputCommand
					for _, command := range oldCommands {
						activation = append(activation, coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, command, now, nil), now))...)
					}
					oldCommands = activation
					if phase == coverMoving {
						completeCoverCommands(t, controller, activation, now, hardware)
					}
				}
				stopCommands := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: action}, now.Add(time.Second)))
				require.Len(t, stopCommands, 2)
				assert.Empty(t, handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionClose}, now.Add(time.Second)))
				completeCoverCommands(t, controller, oldCommands, now, hardware)
				assert.Equal(t, coverStopping, controller.covers["a"].phase)
				assert.True(t, controller.covers["a"].travelDeadline.IsZero())
				completeCoverCommands(t, controller, stopCommands, now.Add(time.Second), hardware)
				assert.Equal(t, entity.CoverStateStopped, controller.covers["a"].state)
				assert.Equal(t, coverIdle, controller.covers["a"].phase)
				fresh := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionClose}, now.Add(2*time.Second)))
				for _, old := range oldCommands {
					assert.Empty(t, handleCoverOutputResult(controller, coverResult(controller, old, now, nil), now.Add(2*time.Second)))
				}
				assert.Equal(t, coverPreparing, controller.covers["a"].phase)
				completeCoverCommands(t, controller, fresh, now.Add(2*time.Second), hardware)
				assert.Equal(t, entity.CoverStateClosing, controller.covers["a"].state)
			})
		}
	}
}

func TestCoverCompletedEndpointRecovery(t *testing.T) {
	for _, direction := range []entity.CoverAction{entity.CoverActionOpen, entity.CoverActionClose} {
		controller, now := testCoverController()
		hardware := make(map[entity.RelayID]bool)
		readyTestCovers(t, controller, now, hardware)
		completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: direction}, now)), now, hardware)
		stopAt := now.Add(controller.settings.FullTravelDuration)
		commands := coverOutputCommands(processCoverDeadlines(controller, stopAt))
		require.Len(t, commands, 2)
		failed := handleCoverOutputResult(controller, coverResult(controller, commands[0], stopAt, fmt.Errorf("OFF failed")), stopAt)
		require.NotEmpty(t, failed)
		assert.False(t, failed[0].CoverObservation.Available)
		handleCoverOutputResult(controller, coverResult(controller, commands[1], stopAt, nil), stopAt)
		assert.Empty(t, handleCoverRequest(controller, event.Cover{CoverID: "a", Action: direction}, stopAt))
		retries := coverOutputCommands(processCoverDeadlines(controller, stopAt.Add(time.Second)))
		require.Len(t, retries, 2)
		observations := completeCoverCommands(t, controller, retries, stopAt.Add(time.Second), hardware)
		require.Len(t, observations, 1)
		assert.Equal(t, event.CoverStoppedKind, observations[0].Kind)
		assert.True(t, observations[0].CoverObservation.Available)
		wantPosition, wantState := 100.0, entity.CoverStateOpen
		if direction == entity.CoverActionClose {
			wantPosition, wantState = 0, entity.CoverStateClosed
		}
		assert.Equal(t, wantState, controller.covers["a"].state)
		require.NotNil(t, controller.covers["a"].position)
		assert.Equal(t, wantPosition, *controller.covers["a"].position)
		assert.False(t, hardware["a_open"])
		assert.False(t, hardware["a_close"])
	}
}

func TestCoverActivationFailureRecoversUnknownWithoutResuming(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	position := 60.0
	controller.covers["a"].position = &position
	prepare := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	var activation []event.OutputCommand
	for _, command := range prepare {
		activation = append(activation, coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, command, now, nil), now))...)
	}
	require.Len(t, activation, 1)
	recovery := coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, activation[0], now, fmt.Errorf("activation failed")), now))
	require.Len(t, recovery, 2)
	completeCoverCommands(t, controller, recovery, now, hardware)
	assert.Equal(t, entity.CoverStateStopped, controller.covers["a"].state)
	assert.Nil(t, controller.covers["a"].position)
	assert.NoError(t, controller.covers["a"].fault)
	assert.Empty(t, processCoverDeadlines(controller, now.Add(time.Hour)))
}

func TestCancelledActivationFailureInvalidatesKnownPosition(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	position := 60.0
	controller.covers["a"].position = &position
	prepare := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	var activation []event.OutputCommand
	for _, command := range prepare {
		activation = append(activation, coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, command, now, nil), now))...)
	}
	require.Len(t, activation, 1)
	stops := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionStop}, now))
	failure := handleCoverOutputResult(controller, coverResult(controller, activation[0], now, fmt.Errorf("cancelled ON write failed")), now)
	require.Len(t, failure, 1)
	assert.False(t, failure[0].CoverObservation.Available)
	completeCoverCommands(t, controller, stops, now, hardware)
	assert.Equal(t, entity.CoverStateStopped, controller.covers["a"].state)
	assert.Nil(t, controller.covers["a"].position)
	assert.NoError(t, controller.covers["a"].fault)
}

func TestCoverInputFailureStopsOnlyAffectedCover(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	for _, id := range []entity.CoverID{"a", "b"} {
		completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: id, Action: entity.CoverActionOpen}, now)), now, hardware)
	}
	commands := coverOutputCommands(handleCoverInputFailure(controller, "input", now.Add(time.Second)))
	require.Len(t, commands, 2)
	assert.Equal(t, coverMoving, controller.covers["b"].phase)
	completeCoverCommands(t, controller, commands, now.Add(time.Second), hardware)
	assert.Equal(t, coverIdle, controller.covers["a"].phase)
	assert.True(t, hardware["b_open"])
}

func TestCoverPositionTimingAndClamping(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	position := 90.0
	controller.covers["a"].position = &position
	completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now)), now, hardware)
	observation := coverObservation(controller.covers["a"], now.Add(2*time.Second), controller.settings.FullTravelDuration, event.CoverObservationKind)
	require.NotNil(t, observation.CoverObservation.EstimatedPosition)
	assert.Equal(t, 100.0, *observation.CoverObservation.EstimatedPosition)
	assert.Equal(t, entity.CoverStateOpening, observation.CoverObservation.State)
	assert.Equal(t, now.Add(10*time.Second), controller.covers["a"].travelDeadline)
	controller.covers["a"].initialPosition = nil
	assert.Nil(t, estimateCoverPosition(controller.covers["a"], now.Add(time.Second), controller.settings.FullTravelDuration))
}

func TestCoverEarlyStopUsesOffCompletionTime(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	position := 20.0
	controller.covers["a"].position = &position
	completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now)), now, hardware)
	stops := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionStop}, now.Add(time.Second)))
	completeCoverCommands(t, controller, stops, now.Add(2*time.Second), hardware)
	require.NotNil(t, controller.covers["a"].position)
	assert.InDelta(t, 40.0, *controller.covers["a"].position, 0.001)
	assert.Equal(t, entity.CoverStateStopped, controller.covers["a"].state)
}

func TestCoverTimeoutRetryAndStaleCompletion(t *testing.T) {
	controller, now := testCoverController()
	startup := coverOutputCommands(initializeCovers(controller, now))
	expiredAt := now.Add(controller.settings.OperationTimeout)
	processCoverDeadlines(controller, expiredAt)
	assert.Error(t, controller.covers["a"].fault)
	assert.Empty(t, coverOutputCommands(processCoverDeadlines(controller, expiredAt.Add(time.Second-time.Nanosecond))))
	retries := coverOutputCommands(processCoverDeadlines(controller, expiredAt.Add(time.Second)))
	require.Len(t, retries, 4)
	for _, stale := range startup {
		assert.Empty(t, handleCoverOutputResult(controller, coverResult(controller, stale, now, nil), expiredAt.Add(time.Second)))
	}
	assert.Equal(t, coverRecovering, controller.covers["a"].phase)
	completeCoverCommands(t, controller, retries, expiredAt.Add(time.Second), make(map[entity.RelayID]bool))
	assert.NoError(t, controller.covers["a"].fault)
	assert.True(t, nextCoverDeadline(controller).IsZero())
}
