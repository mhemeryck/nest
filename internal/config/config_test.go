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

func TestLoadRejectsTrailingYAMLDocuments(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	writeTestFile(t, path, "sysfs:\n  root: /tmp\ndigital_inputs:\n  - id: button_input\n    device: di_3_16\n---\nextra: true\n")

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple YAML documents are not supported")
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
			name: "rejects configs with no devices",
			file: Root{
				Sysfs: SysfsConfig{Root: "/tmp"},
			},
			message: "at least one digital_input or relay is required",
		},
		{
			name: "rejects whitespace padded sysfs root",
			file: Root{
				Sysfs:  SysfsConfig{Root: " /tmp "},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "sysfs.root: must not have leading or trailing whitespace",
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
			name: "rejects duplicate digital input devices",
			file: Root{
				Sysfs: SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{
					{ID: "button_input_1", Device: "di_3_16"},
					{ID: "button_input_2", Device: "di_3_16"},
				},
			},
			message: `digital_inputs[1].device: duplicate digital input device "di_3_16"`,
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
			name: "rejects whitespace padded button ids",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: " button ", Name: "Button", Input: "button_input"}},
			},
			message: "push_buttons[0].id: must not have leading or trailing whitespace",
		},
		{
			name: "rejects whitespace padded button input references",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: "button", Name: "Button", Input: " button_input "}},
			},
			message: "push_buttons[0].input: must not have leading or trailing whitespace",
		},
		{
			name: "rejects invalid relay devices",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "di_3_16"}},
			},
			message: `relays[0].device: invalid relay device "di_3_16"`,
		},
		{
			name: "rejects duplicate relay devices",
			file: Root{
				Sysfs: SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{
					{ID: "relay_1", Name: "Relay 1", Device: "ro_3_14"},
					{ID: "relay_2", Name: "Relay 2", Device: "ro_3_14"},
				},
			},
			message: `relays[1].device: duplicate relay device "ro_3_14"`,
		},
		{
			name: "rejects whitespace padded relay devices",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: " ro_3_14 "}},
			},
			message: "relays[0].device: must not have leading or trailing whitespace",
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
