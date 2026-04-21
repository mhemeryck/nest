package entity

import (
	"testing"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromConfig(t *testing.T) {
	root := FromConfig(&config.Root{
		Sysfs: config.SysfsConfig{Root: "test/fixtures"},
		DigitalInputs: []config.DigitalInputConfig{{
			ID:     "office_button_input",
			Device: "di_3_16",
		}},
		PushButtons: []config.PushButtonConfig{{
			ID:    "office_button",
			Name:  "Office button",
			Input: "office_button_input",
		}},
		Lights: []config.LightConfig{{
			ID:    "office_light",
			Name:  "Office light",
			Relay: "office_light_relay",
		}},
		Relays: []config.RelayConfig{{
			ID:     "office_light_relay",
			Name:   "Office light relay",
			Device: "ro_3_14",
		}},
		Bindings: []config.BindingConfig{{
			Button: "office_button",
			Light:  "office_light",
			Action: "toggle",
		}},
	})

	require.NotNil(t, root)
	assert.Equal(t, "test/fixtures", root.SysfsRoot)
	assert.Equal(t, []DigitalInput{{ID: DigitalInputID("office_button_input"), Device: DeviceID("di_3_16")}}, root.DigitalInputs)
	assert.Equal(t, []PushButton{{ID: PushButtonID("office_button"), Name: "Office button", Input: DigitalInputID("office_button_input")}}, root.PushButtons)
	assert.Equal(t, []Light{{ID: LightID("office_light"), Name: "Office light", Relay: RelayID("office_light_relay")}}, root.Lights)
	assert.Equal(t, []Relay{{ID: RelayID("office_light_relay"), Name: "Office light relay", Device: DeviceID("ro_3_14")}}, root.Relays)
	assert.Equal(t, []Binding{{Button: PushButtonID("office_button"), Light: LightID("office_light"), Action: LightActionToggle}}, root.Bindings)
}

func TestFromConfigNil(t *testing.T) {
	assert.Nil(t, FromConfig(nil))
}
