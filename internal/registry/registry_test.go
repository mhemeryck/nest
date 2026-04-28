package registry

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	root := &entity.Root{
		DigitalInputs: []entity.DigitalInput{
			{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")},
		},
		PushButtons: []entity.PushButton{
			{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")},
		},
		Lights: []entity.Light{
			{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")},
		},
		Relays: []entity.Relay{
			{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")},
		},
		Bindings: []entity.Binding{
			{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle},
		},
	}

	index := Build(root)
	require.NotNil(t, index)

	assert.Equal(t, root.DigitalInputs[0], index.DigitalInputsByDevice[entity.SysfsDeviceID("di_3_16")])
	assert.Equal(t, root.PushButtons[0], index.PushButtonsByID[entity.PushButtonID("office_button")])
	assert.Equal(t, root.PushButtons, index.PushButtonsByInputID[entity.DigitalInputID("office_button_input")])
	assert.Equal(t, root.Lights[0], index.LightsByID[entity.LightID("office_light")])
	assert.Equal(t, root.Bindings, index.BindingsByButtonID[entity.PushButtonID("office_button")])
	assert.Equal(t, root.Relays[0], index.RelaysByID[entity.RelayID("office_light_relay")])
	assert.Equal(t, root.Relays[0], index.RelaysByDevice[entity.SysfsDeviceID("ro_3_14")])
	assert.Equal(t, []entity.SysfsDeviceID{"di_3_16", "ro_3_14"}, SysfsDeviceIDs(index))
}
