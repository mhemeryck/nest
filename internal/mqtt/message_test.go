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

func TestDiscoveryMessage(t *testing.T) {
	message, err := DiscoveryMessage(testEntityRoot(), NewTopics("nest", "controller_1"))
	require.NoError(t, err)

	assert.Equal(t, "nest/units/controller_1/discovery", message.Topic)
	assert.Contains(t, string(message.Payload), `"commands_enabled":false`)
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
