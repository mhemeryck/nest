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
			{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")},
		},
		PushButtons: []entity.PushButton{
			{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")},
		},
		Relays: []entity.Relay{
			{ID: entity.RelayID("office_shade_up"), Name: "Office shade up", Device: entity.DeviceID("ro_3_14")},
		},
	}

	index := Build(root)
	require.NotNil(t, index)

	assert.Equal(t, root.DigitalInputs[0], index.DigitalInputsByDevice[entity.DeviceID("di_3_16")])
	assert.Equal(t, root.PushButtons, index.PushButtonsByInputID[entity.DigitalInputID("office_button_input")])
	assert.Equal(t, root.Relays[0], index.RelaysByDevice[entity.DeviceID("ro_3_14")])
	assert.Equal(t, []string{"di_3_16", "ro_3_14"}, DeviceIDs(index))
}
