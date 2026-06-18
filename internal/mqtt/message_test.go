package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvailabilityMessage(t *testing.T) {
	message := AvailabilityMessage(NewTopics("nest", "controller_1"), AvailabilityOnline)

	assert.Equal(t, "nest/units/controller_1/availability", message.Topic)
	assert.Equal(t, []byte("online"), message.Payload)
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}

func TestLightStateMessage(t *testing.T) {
	message, err := LightStateMessage(NewTopics("nest", "controller_1"), LightObservation{
		LightID: entity.LightID("office_light"),
		State:   "ON",
	})
	require.NoError(t, err)

	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", message.Topic)
	assert.JSONEq(t, `{"state":"ON"}`, string(message.Payload))
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}

func TestSemanticSourceEventMessage(t *testing.T) {
	message, err := SemanticSourceEventMessage(NewTopics("nest", "controller_1"), SemanticSourceEventObservation{
		SourceID: entity.ID("controller_1.button.office_button"),
		Event:    "pressed",
	})
	require.NoError(t, err)

	assert.Equal(t, "nest/units/controller_1/sources/controller_1.button.office_button/event", message.Topic)
	assert.JSONEq(t, `{"source":"controller_1.button.office_button","event":"pressed"}`, string(message.Payload))
	assert.False(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}
