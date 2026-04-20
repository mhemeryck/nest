package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	path := filepath.Join("..", "..", "test", "fixtures", "config.local.yaml")

	file, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "test/fixtures", file.Sysfs.Root)
	assert.Len(t, file.DigitalInputs, 1)
	assert.Len(t, file.PushButtons, 1)
	assert.Len(t, file.Relays, 2)
	assert.Equal(t, []string{"di_3_16", "ro_3_14", "ro_3_13"}, DeviceIDs(file))
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	writeTestFile(t, path, "sysfs:\n  root: /tmp\nunknown: true\n")

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field unknown not found")
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		file    Root
		message string
	}{
		{
			name:    "requires sysfs root",
			file:    Root{},
			message: "sysfs.root: required",
		},
		{
			name: "rejects duplicate digital input ids",
			file: Root{
				Sysfs: SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{
					{ID: "button_input", Device: "di_3_16"},
					{ID: "button_input", Device: "di_3_15"},
				},
			},
			message: `digital_inputs[1].id: duplicate id "button_input"`,
		},
		{
			name: "rejects invalid digital input devices",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "ro_3_14"}},
			},
			message: `digital_inputs[0].device: invalid digital input device "ro_3_14"`,
		},
		{
			name: "requires buttons to reference known inputs",
			file: Root{
				Sysfs:       SysfsConfig{Root: "/tmp"},
				PushButtons: []PushButtonConfig{{ID: "button", Name: "Button", Input: "missing"}},
			},
			message: `push_buttons[0].input: unknown digital input "missing"`,
		},
		{
			name: "rejects invalid relay devices",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "di_3_16"}},
			},
			message: `relays[0].device: invalid relay device "di_3_16"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.file)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestValidateAcceptsValidFile(t *testing.T) {
	file := Root{
		Sysfs: SysfsConfig{Root: "/tmp"},
		DigitalInputs: []DigitalInputConfig{
			{ID: "office_button_input", Device: "di_3_16"},
		},
		PushButtons: []PushButtonConfig{
			{ID: "office_button", Name: "Office button", Input: "office_button_input"},
		},
		Relays: []RelayConfig{
			{ID: "office_shade_up", Name: "Office shade up", Device: "ro_3_14"},
			{ID: "office_shade_down", Name: "Office shade down", Device: "ro_3_13"},
		},
	}

	require.NoError(t, Validate(&file))
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
