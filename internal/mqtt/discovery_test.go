package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryStates(t *testing.T) {
	cfg := Config{UnitID: "tesla"}
	root := &entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Relays:        []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", Device: entity.DeviceID("ro_3_14")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
	}
	index := registry.Build(root)

	states := DiscoveryStates(cfg, root, index)

	require.Len(t, states, 3)
	assert.Equal(t, "tesla/discovery", states[0].Topic)
	assert.True(t, states[0].Retain)
	assert.JSONEq(t, `{
		"schema_version": 1,
		"unit_id": "tesla",
		"availability_topic": "tesla/availability",
		"commands_enabled": false,
		"inputs": [{"id":"office_button_input","device_id":"di_3_16","state_topic":"tesla/input/3_16/state","event_topic":"tesla/input/3_16/event","command_topic":"tesla/input/3_16/set"}],
		"push_buttons": [{"id":"office_button","name":"Office button","input_id":"office_button_input","event_topic":"tesla/push_button/office_button/event","command_topic":"tesla/input/3_16/set"}],
		"relays": [{"id":"office_light_relay","name":"Office light relay","device_id":"ro_3_14","state_topic":"tesla/relay/3_14/state","command_topic":"tesla/relay/3_14/set"}],
		"lights": [{"id":"office_light","name":"Office light","relay_id":"office_light_relay","state_topic":"tesla/relay/3_14/state","command_topic":"tesla/relay/3_14/set"}]
	}`, string(states[0].Payload))

	assert.Equal(t, "homeassistant/switch/tesla_office_button/config", states[1].Topic)
	assert.JSONEq(t, `{
		"name":"Office button",
		"object_id":"office_button",
		"unique_id":"tesla_office_button",
		"state_topic":"tesla/input/3_16/state",
		"command_topic":"tesla/input/3_16/set",
		"payload_on":"ON",
		"payload_off":"OFF",
		"availability_topic":"tesla/availability",
		"payload_available":"ON",
		"payload_not_available":"OFF",
		"device":{"identifiers":["nest_tesla"],"name":"tesla","manufacturer":"Unipi","model":"nest"}
	}`, string(states[1].Payload))

	assert.Equal(t, "homeassistant/light/tesla_office_light/config", states[2].Topic)
	assert.JSONEq(t, `{
		"name":"Office light",
		"object_id":"office_light",
		"unique_id":"tesla_office_light",
		"state_topic":"tesla/relay/3_14/state",
		"command_topic":"tesla/relay/3_14/set",
		"payload_on":"ON",
		"payload_off":"OFF",
		"availability_topic":"tesla/availability",
		"payload_available":"ON",
		"payload_not_available":"OFF",
		"device":{"identifiers":["nest_tesla"],"name":"tesla","manufacturer":"Unipi","model":"nest"}
	}`, string(states[2].Payload))
}
