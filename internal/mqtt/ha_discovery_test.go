package mqtt

import (
	"encoding/json"
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEntityRoot() *entity.Root {
	return &entity.Root{
		MQTT: entity.MQTT{
			TopicPrefix: "nest",
			UnitID:      "controller_1",
		},
		DigitalInputs: []entity.DigitalInput{{
			ID:          entity.DigitalInputID("office_button_input"),
			SysfsDevice: entity.SysfsDeviceID("di_3_16"),
		}},
		PushButtons: []entity.PushButton{{
			ID:    entity.PushButtonID("office_button"),
			Name:  "Office button",
			Input: entity.DigitalInputID("office_button_input"),
		}},
		Relays: []entity.Relay{{
			ID:          entity.RelayID("office_light_relay"),
			Name:        "Office light relay",
			SysfsDevice: entity.SysfsDeviceID("ro_3_14"),
		}},
		Lights: []entity.Light{{
			ID:    entity.LightID("office_light"),
			Name:  "Office light",
			Relay: entity.RelayID("office_light_relay"),
		}},
	}
}

func TestBuildHomeAssistantDeviceDiscovery(t *testing.T) {
	root := testEntityRoot()
	topics := NewTopics("nest", "controller_1")

	doc := BuildHomeAssistantDeviceDiscovery(root, topics)

	assert.Equal(t, HomeAssistantDevice{
		Identifiers:  []string{"nest_controller_1_unit"},
		Name:         "nest controller_1",
		Manufacturer: "nest",
	}, doc.Device)
	assert.Equal(t, HomeAssistantOrigin{Name: "nest"}, doc.Origin)
	assert.Equal(t, "nest/units/controller_1/availability", doc.AvailabilityTopic)
	require.Contains(t, doc.Components, "office_light")
	assert.Equal(t, HomeAssistantComponentDiscovery{
		Platform:           "light",
		Name:               "Office light",
		UniqueID:           "nest_controller_1_office_light",
		DefaultEntityID:    "light.controller_1_office_light",
		StateTopic:         "nest/units/controller_1/lights/office_light/state",
		CommandTopic:       "nest/units/controller_1/lights/office_light/command",
		StateValueTemplate: "{{ value_json.state }}",
		PayloadOn:          "ON",
		PayloadOff:         "OFF",
	}, doc.Components["office_light"])
}

func TestHomeAssistantDeviceDiscoveryPayload(t *testing.T) {
	payload, err := HomeAssistantDeviceDiscoveryPayload(testEntityRoot(), NewTopics("nest", "controller_1"))
	require.NoError(t, err)

	var doc HomeAssistantDeviceDiscovery
	require.NoError(t, json.Unmarshal(payload, &doc))
	assert.Equal(t, "nest_controller_1_unit", doc.Device.Identifiers[0])
	assert.Equal(t, "nest/units/controller_1/lights/office_light/command", doc.Components["office_light"].CommandTopic)
}

func TestHomeAssistantDeviceDiscoveryMessage(t *testing.T) {
	message, err := HomeAssistantDeviceDiscoveryMessage(testEntityRoot(), NewTopics("nest", "controller_1"))
	require.NoError(t, err)

	assert.Equal(t, "homeassistant/device/nest_controller_1_unit/config", message.Topic)
	assert.Contains(t, string(message.Payload), `"cmps"`)
	assert.Contains(t, string(message.Payload), `"p":"light"`)
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}
