package controller

import (
	"path/filepath"
	"testing"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistenceMapsLifecycleAndConfirmedStops(t *testing.T) {
	position := 60.0
	observation := &event.CoverObservation{CoverID: "a", State: entity.CoverStateStopped, EstimatedPosition: &position, Available: true}
	for _, tc := range []struct {
		kind    event.Kind
		command persistence.CommandKind
	}{
		{event.SessionStartedKind, persistence.StartSession}, {event.CoverStartIntentKind, persistence.StartMovement},
		{event.CoverStoppedKind, persistence.StopMovement}, {event.SessionStoppedKind, persistence.StopSession},
	} {
		command, handled := persistenceCommand(event.Event{Kind: tc.kind, CoverObservation: observation})
		require.True(t, handled)
		assert.Equal(t, tc.command, command.Kind)
	}
	_, handled := persistenceCommand(event.Event{Kind: event.CoverObservationKind, CoverObservation: observation})
	assert.False(t, handled, "pending and periodic reports must not clear unfinished movement")
}

func TestPersistenceOverloadDoesNotBlockStopping(t *testing.T) {
	store := persistence.NewStore(filepath.Join(t.TempDir(), "snapshot"), "unit", nil, 1)
	require.Empty(t, dispatchPersistenceEvent(store, event.Event{Kind: event.SessionStartedKind}))
	var failures []event.Event
	logs := captureLogs(t, func() { failures = dispatchPersistenceEvent(store, event.Event{Kind: event.SessionStartedKind}) })
	require.Len(t, failures, 1)
	assert.Contains(t, logs, "persistence integration failure")
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now)), now, hardware)
	assert.Len(t, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionStop}, now)), 2)
}

func TestRestoredPositionDoesNotResumeOrRestrictMovement(t *testing.T) {
	controller, now := testCoverController()
	position := 100.0
	controller = newCoverController(controller.reg, map[entity.CoverID]*float64{"a": &position})
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	assert.False(t, hardware["a_open"])
	require.NotNil(t, controller.covers["a"].position)
	assert.Equal(t, 100.0, *controller.covers["a"].position)
	completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now)), now, hardware)
	assert.Equal(t, now.Add(controller.settings.FullTravelDuration), controller.covers["a"].travelDeadline)
}
