package mqtt

import (
	"encoding/json"
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDiscovery(t *testing.T) {
	root := testEntityRoot()
	topics := NewTopics("nest", "controller_1")

	doc := BuildDiscovery(root, topics)

	assert.Equal(t, DiscoverySchemaVersion, doc.SchemaVersion)
	assert.Equal(t, "controller_1", doc.UnitID)
	assert.Equal(t, "nest/units/controller_1/availability", doc.Availability)
	assert.Equal(t, []DigitalInputDiscovery{{
		ID:          "office_button_input",
		SysfsDevice: "di_3_16",
		StateTopic:  "nest/units/controller_1/digital_inputs/office_button_input/state",
	}}, doc.DigitalInputs)
	assert.Equal(t, []PushButtonDiscovery{{
		ID:         "office_button",
		Name:       "Office button",
		Input:      "office_button_input",
		StateTopic: "nest/units/controller_1/push_buttons/office_button/state",
	}}, doc.PushButtons)
	assert.Equal(t, []RelayDiscovery{{
		ID:          "office_light_relay",
		Name:        "Office light relay",
		SysfsDevice: "ro_3_14",
		StateTopic:  "nest/units/controller_1/relays/office_light_relay/state",
	}}, doc.Relays)
	assert.Equal(t, []LightDiscovery{{
		ID:              "office_light",
		Name:            "Office light",
		Relay:           "office_light_relay",
		Capabilities:    []string{"toggle"},
		StateTopic:      "nest/units/controller_1/lights/office_light/state",
		CommandTopic:    "nest/units/controller_1/lights/office_light/command",
		CommandsEnabled: false,
	}}, doc.Lights)
}

func TestDiscoveryPayload(t *testing.T) {
	payload, err := DiscoveryPayload(testEntityRoot(), NewTopics("nest", "controller_1"))
	require.NoError(t, err)

	var doc DiscoveryDocument
	require.NoError(t, json.Unmarshal(payload, &doc))
	assert.Equal(t, "controller_1", doc.UnitID)
	assert.False(t, doc.Lights[0].CommandsEnabled)
}

func testEntityRoot() *entity.Root {
	return &entity.Root{
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
