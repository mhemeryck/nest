package mqtt

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverCommandsPreserveRetainedMetadata(t *testing.T) {
	client := &fakeClient{token: fakeToken{}}
	events := make(chan Event, 1)
	topics := NewTopics("nest", "unit")
	require.NoError(t, subscribeCoverCommands(t.Context(), client, topics, events))
	assert.Equal(t, CoverCommandSubscriptionTopic(topics), client.subscribedTopic)
	client.handler(nil, &fakeMessage{topic: "nest/units/unit/covers/office/command", payload: []byte("STOP"), retained: true})
	received := <-events
	assert.True(t, received.Message.Retained)
	assert.Equal(t, "STOP", string(received.Message.Payload))
}

func TestCoverObservationRoundingAndUnknownReplacement(t *testing.T) {
	topics := NewTopics("nest", "unit")
	position := 60.49
	message, err := CoverStateMessage(topics, CoverObservation{CoverID: "unit.cover.office", State: entity.CoverStateStopped, EstimatedPosition: &position, Available: true})
	require.NoError(t, err)
	assert.Equal(t, "nest/units/unit/covers/office/state", message.Topic)
	assert.True(t, message.Retain)
	var known map[string]any
	require.NoError(t, json.Unmarshal(message.Payload, &known))
	assert.Equal(t, 60.0, known["estimated_position"])
	assert.Equal(t, "stopped", known["state"])
	assert.Equal(t, "stopped", known["ha_state"])
	assert.Equal(t, 60.49, position, "reporting must not round runtime estimates")
	unknown, err := CoverStateMessage(topics, CoverObservation{CoverID: "unit.cover.office", State: entity.CoverStateUnknown})
	require.NoError(t, err)
	var replacement map[string]any
	require.NoError(t, json.Unmarshal(unknown.Payload, &replacement))
	assert.Contains(t, replacement, "estimated_position")
	assert.Nil(t, replacement["estimated_position"])
	assert.Equal(t, "None", replacement["ha_state"])
	assert.Equal(t, false, replacement["available"])
}

func TestCoverDiscoveryCombinedAvailabilityWithoutNativePosition(t *testing.T) {
	topics := NewTopics("nest", "unit")
	cover := entity.Cover{ID: "unit.cover.office", Name: "Office"}
	payload, err := HomeAssistantDeviceDiscoveryPayload([]entity.Light{{ID: "unit.light.office", Name: "Office light"}}, topics, []entity.Cover{cover})
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(payload, &document))
	components := document["cmps"].(map[string]any)
	component := components["cover.office"].(map[string]any)
	assert.Equal(t, "cover", component["p"])
	assert.Equal(t, "all", component["availability_mode"])
	assert.Len(t, component["availability"], 2)
	assert.Equal(t, CoverStateTopic(topics, cover.ID), component["json_attributes_topic"])
	assert.Equal(t, "{{ value_json.ha_state }}", component["value_template"], "MQTT cover schema uses value_template")
	assert.NotContains(t, component, "state_value_template")
	assert.NotContains(t, component, "position_topic")
	assert.NotContains(t, component, "set_position_topic")
	assert.NotContains(t, component, "payload_on")
	assert.Equal(t, AvailabilityTopic(topics), components["office"].(map[string]any)["availability_topic"])
	assert.NotContains(t, document, "availability_topic", "cover availability must not conflict with inherited availability_topic")
}

func TestCoverHandoffCoalescesCoherentSnapshots(t *testing.T) {
	handoff := NewHandoff(2)
	topics := NewTopics("nest", "unit")
	position := 60.0
	old, err := CoverStateMessage(topics, CoverObservation{CoverID: "unit.cover.office", State: entity.CoverStateStopped, EstimatedPosition: &position, Available: true})
	require.NoError(t, err)
	latest, err := CoverStateMessage(topics, CoverObservation{CoverID: "unit.cover.office", State: entity.CoverStateOpening, EstimatedPosition: nil, Available: false})
	require.NoError(t, err)
	require.NoError(t, QueueMessage(handoff, old))
	require.NoError(t, QueueMessage(handoff, latest))
	commands := make(chan Command)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go RunHandoff(ctx, handoff, commands, done)
	defer func() { cancel(); <-done }()
	select {
	case command := <-commands:
		assert.Equal(t, latest, command.Publish)
	case <-time.After(time.Second):
		require.FailNow(t, "latest observation did not progress without new input")
	}
	select {
	case command := <-commands:
		require.Failf(t, "obsolete observation was published", "%+v", command)
	case <-time.After(20 * time.Millisecond):
	}
}
