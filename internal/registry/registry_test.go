package registry

import (
	"testing"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	file := &config.Root{
		DigitalInputs: []config.DigitalInputConfig{
			{ID: "office_button_input", Device: "di_3_16"},
		},
		PushButtons: []config.PushButtonConfig{
			{ID: "office_button", Name: "Office button", Input: "office_button_input"},
		},
		Relays: []config.RelayConfig{
			{ID: "office_shade_up", Name: "Office shade up", Device: "ro_3_14"},
		},
	}

	index := Build(file)
	require.NotNil(t, index)

	assert.Equal(t, file.DigitalInputs[0], index.DigitalInputsByDevice["di_3_16"])
	assert.Equal(t, file.PushButtons, index.PushButtonsByInputID["office_button_input"])
	assert.Equal(t, file.Relays[0], index.RelaysByDevice["ro_3_14"])
	assert.Equal(t, []string{"di_3_16", "ro_3_14"}, DeviceIDs(index))
}
