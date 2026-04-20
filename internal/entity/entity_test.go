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
		Relays: []config.RelayConfig{{
			ID:     "office_shade_up",
			Name:   "Office shade up",
			Device: "ro_3_14",
		}},
	})

	require.NotNil(t, root)
	assert.Equal(t, "test/fixtures", root.SysfsRoot)
	assert.Equal(t, []DigitalInput{{ID: "office_button_input", Device: "di_3_16"}}, root.DigitalInputs)
	assert.Equal(t, []PushButton{{ID: "office_button", Name: "Office button", Input: "office_button_input"}}, root.PushButtons)
	assert.Equal(t, []Relay{{ID: "office_shade_up", Name: "Office shade up", Device: "ro_3_14"}}, root.Relays)
}

func TestFromConfigNil(t *testing.T) {
	assert.Nil(t, FromConfig(nil))
}
