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
			{ID: "office_button_input", Device: "di_3_16"},
		},
		PushButtons: []entity.PushButton{
			{ID: "office_button", Name: "Office button", Input: "office_button_input"},
		},
		Relays: []entity.Relay{
			{ID: "office_shade_up", Name: "Office shade up", Device: "ro_3_14"},
		},
	}

	index := Build(root)
	require.NotNil(t, index)

	assert.Equal(t, root.DigitalInputs[0], index.DigitalInputsByDevice["di_3_16"])
	assert.Equal(t, root.PushButtons, index.PushButtonsByInputID["office_button_input"])
	assert.Equal(t, root.Relays[0], index.RelaysByDevice["ro_3_14"])
	assert.Equal(t, []string{"di_3_16", "ro_3_14"}, DeviceIDs(index))
}
