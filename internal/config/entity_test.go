package config

import (
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToEntityRoot(t *testing.T) {
	root := ToEntityRoot(&Root{
		Sysfs: SysfsConfig{
			Root: "test/fixtures",
			PollIntervals: PollIntervalsConfig{
				DigitalInput: 100 * time.Millisecond,
				RelayOutput:  2 * time.Second,
			},
		},
		MQTT: MQTTConfig{
			Enabled:     true,
			Host:        "localhost",
			Port:        1883,
			ClientID:    "nest-controller-1",
			Username:    "nest",
			Password:    "secret",
			TopicPrefix: "nest",
			UnitID:      "controller_1",
		},
		DigitalInputs: []DigitalInputConfig{{
			ID:     "office_button_input",
			Device: "di_3_16",
		}},
		PushButtons: []PushButtonConfig{{
			ID:    "office_button",
			Name:  "Office button",
			Input: "office_button_input",
		}},
		Lights: []LightConfig{{
			ID:    "office_light",
			Name:  "Office light",
			Relay: "office_light_relay",
		}},
		Relays: []RelayConfig{{
			ID:     "office_light_relay",
			Name:   "Office light relay",
			Device: "ro_3_14",
		}},
		Bindings: []BindingConfig{{
			Source: "office_button",
			Target: "office_light",
			Action: "toggle",
		}},
		RemoteSourceBindings: []BindingConfig{{
			Source: "controller_1.button.office_button",
			Target: "controller_2.light.hall_light",
			Action: "toggle",
		}},
		RemoteTargetBindings: []BindingConfig{{
			Source: "controller_2.button.hall_button",
			Target: "controller_1.light.office_light",
			Action: "toggle",
		}},
	})

	require.NotNil(t, root)
	assert.Equal(t, "test/fixtures", root.SysfsRoot)
	assert.Equal(t, entity.PollIntervals{DigitalInput: 100 * time.Millisecond, RelayOutput: 2 * time.Second}, root.SysfsPollIntervals)
	assert.Equal(t, entity.MQTT{
		Enabled:     true,
		Host:        "localhost",
		Port:        1883,
		ClientID:    "nest-controller-1",
		Username:    "nest",
		Password:    "secret",
		TopicPrefix: "nest",
		UnitID:      "controller_1",
	}, root.MQTT)
	assert.Equal(t, []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")}}, root.DigitalInputs)
	assert.Equal(t, []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}}, root.PushButtons)
	assert.Equal(t, []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}}, root.Lights)
	assert.Equal(t, []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}}, root.Relays)
	assert.Equal(t, []entity.Binding{{Source: entity.ID("office_button"), Target: entity.ID("office_light"), Action: entity.ActionToggle}}, root.Bindings)
	assert.Equal(t, []entity.Binding{{Source: entity.ID("controller_1.button.office_button"), Target: entity.ID("controller_2.light.hall_light"), Action: entity.ActionToggle}}, root.RemoteSourceBindings)
	assert.Equal(t, []entity.Binding{{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle}}, root.RemoteTargetBindings)
}

func TestToEntityRootNil(t *testing.T) {
	assert.Nil(t, ToEntityRoot(nil))
}
