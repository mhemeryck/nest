package mqtt

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildHomeAssistantLightDiscovery(t *testing.T) {
	light := testEntityRoot().Lights[0]
	topics := NewTopics("nest", "controller_1")

	doc := BuildHomeAssistantLightDiscovery(light, topics)

	assert.Equal(t, "Office light", doc.Name)
	assert.Equal(t, "nest_controller_1_office_light", doc.UniqueID)
	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", doc.StateTopic)
	assert.Equal(t, "{{ 'ON' if value_json.value == 1 else 'OFF' }}", doc.ValueTemplate)
	assert.Equal(t, "ON", doc.PayloadOn)
	assert.Equal(t, "OFF", doc.PayloadOff)
	assert.Equal(t, "nest/units/controller_1/availability", doc.AvailabilityTopic)
	assert.Equal(t, HomeAssistantDevice{
		Identifiers:  []string{"nest_controller_1_unit"},
		Name:         "nest controller_1",
		Manufacturer: "nest",
	}, doc.Device)
}

func TestHomeAssistantLightDiscoveryPayload(t *testing.T) {
	payload, err := HomeAssistantLightDiscoveryPayload(testEntityRoot().Lights[0], NewTopics("nest", "controller_1"))
	require.NoError(t, err)

	var doc HomeAssistantLightDiscovery
	require.NoError(t, json.Unmarshal(payload, &doc))
	assert.Equal(t, "nest_controller_1_office_light", doc.UniqueID)
	assert.Empty(t, discoveryMapValue(t, payload, "command_topic"))
}

func TestHomeAssistantLightDiscoveryMessage(t *testing.T) {
	message, err := HomeAssistantLightDiscoveryMessage(testEntityRoot().Lights[0], NewTopics("nest", "controller_1"))
	require.NoError(t, err)

	assert.Equal(t, "homeassistant/light/nest_controller_1_office_light/config", message.Topic)
	assert.Contains(t, string(message.Payload), `"value_template"`)
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}

func discoveryMapValue(t *testing.T, payload []byte, key string) any {
	t.Helper()

	var values map[string]any
	require.NoError(t, json.Unmarshal(payload, &values))

	return values[key]
}
