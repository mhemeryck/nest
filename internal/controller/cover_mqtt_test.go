package controller

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMQTTCoverRetainedMovementRejectedButStopAccepted(t *testing.T) {
	topics := mqtt.NewTopics("nest", "unit")
	reg := registry.Build(&entity.Root{Covers: []entity.Cover{{ID: "unit.cover.office"}}})
	for _, payload := range []string{"OPEN", "CLOSE", "STOP"} {
		for _, retained := range []bool{false, true} {
			message := mqtt.ReceivedMessage{Topic: mqtt.CoverCommandTopic(topics, "unit.cover.office"), Payload: []byte(payload), Retained: retained}
			incoming, handled := semanticEventFromMQTTMessage(reg, topics, message)
			if retained && payload != "STOP" {
				assert.False(t, handled)
				continue
			}
			require.True(t, handled)
			assert.Equal(t, event.CoverKind, incoming.Kind)
			assert.Equal(t, entity.CoverID("unit.cover.office"), incoming.Cover.CoverID)
		}
	}
}

func TestReconnectPublishesCurrentEstimateWithoutResettingDeadline(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	position := 20.0
	controller.covers["a"].position = &position
	completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now)), now, hardware)
	deadline := controller.covers["a"].travelDeadline
	observations := handleCoverEvent(controller, event.Event{Kind: event.MQTTConnectedKind}, now.Add(time.Second))
	require.Len(t, observations, 2)
	assert.Equal(t, deadline, controller.covers["a"].travelDeadline)
	require.NotNil(t, observations[0].CoverObservation.EstimatedPosition)
	assert.InDelta(t, 30, *observations[0].CoverObservation.EstimatedPosition, 0.001)
	assert.Nil(t, observations[1].CoverObservation.EstimatedPosition)
}

func TestPendingOffDoesNotPublishCompletedEndpoint(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now)), now, hardware)
	processCoverDeadlines(controller, now.Add(controller.settings.FullTravelDuration))
	commands := make(chan mqtt.Command, 1)
	observation := coverObservation(controller.covers["a"], now.Add(controller.settings.FullTravelDuration), controller.settings.FullTravelDuration, event.CoverObservationKind)
	dispatchMQTTCommand(t.Context(), controller.reg, commands, mqtt.NewTopics("nest", "unit"), observation, nil)
	var payload map[string]any
	require.NoError(t, json.Unmarshal((<-commands).Publish.Payload, &payload))
	assert.Equal(t, "opening", payload["state"])
	assert.Nil(t, payload["estimated_position"])
}

func TestBoundInputFailureStopsMQTTBoundRemoteCover(t *testing.T) {
	reg := registry.Build(&entity.Root{
		DigitalInputs:        []entity.DigitalInput{{ID: "input", SysfsDevice: "di_1_1"}},
		PushButtons:          []entity.PushButton{{ID: "source.button.open", Input: "input"}},
		RemoteSourceBindings: []entity.Binding{{Source: "source.button.open", Target: "target.cover.office", Action: "open", ExecutionTransport: entity.ExecutionTransportMQTT}},
	})
	controller := newCoverController(reg, nil)
	events := handleCoverEvent(controller, event.Event{Kind: event.InputFailureKind, InputFailure: &event.InputFailure{InputID: "input"}}, time.Now())
	require.Len(t, events, 1)
	assert.Equal(t, event.CoverKind, events[0].Kind)
	assert.Equal(t, entity.CoverActionStop, events[0].Cover.Action)
	commands := make(chan mqtt.Command, 1)
	dispatchMQTTCommand(t.Context(), reg, commands, mqtt.NewTopics("nest", "source"), events[0], nil)
	message := (<-commands).Publish
	assert.Equal(t, "nest/units/target/covers/office/command", message.Topic)
	assert.Equal(t, "STOP", string(message.Payload))
	assert.False(t, message.Retain)
}
