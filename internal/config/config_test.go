package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	path := filepath.Join("..", "..", "test", "fixtures", "config.local.yaml")

	file, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "test/fixtures", file.Sysfs.Root)
	assert.Equal(t, 100*time.Millisecond, file.Sysfs.PollIntervals.DigitalInput)
	assert.Equal(t, 250*time.Millisecond, file.Sysfs.PollIntervals.DigitalOutput)
	assert.Equal(t, time.Second, file.Sysfs.PollIntervals.RelayOutput)
	assert.Len(t, file.DigitalInputs, 1)
	assert.Len(t, file.PushButtons, 1)
	assert.Len(t, file.Lights, 1)
	assert.Len(t, file.Relays, 1)
	assert.Len(t, file.Bindings, 1)
	assert.Equal(t, []string{"di_3_16", "ro_3_14"}, DeviceIDs(file))
}

func TestLoadAcceptsSysfsPollIntervals(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	writeTestFile(t, path, "sysfs:\n  root: /tmp\n  poll_intervals:\n    digital_input: 100ms\n    digital_output: 250ms\n    relay_output: 2s\ndigital_inputs:\n  - id: button_input\n    device: di_3_16\n")

	file, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 100*time.Millisecond, file.Sysfs.PollIntervals.DigitalInput)
	assert.Equal(t, 250*time.Millisecond, file.Sysfs.PollIntervals.DigitalOutput)
	assert.Equal(t, 2*time.Second, file.Sysfs.PollIntervals.RelayOutput)
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
			name: "rejects negative sysfs poll intervals",
			file: Root{
				Sysfs: SysfsConfig{
					Root: "/tmp",
					PollIntervals: PollIntervalsConfig{
						DigitalInput: -time.Second,
					},
				},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
			},
			message: "sysfs.poll_intervals.digital_input: must not be negative",
		},
		{
			name: "requires enabled MQTT host",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithHost(""),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.host: required",
		},
		{
			name: "rejects enabled MQTT port below range",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithPort(0),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.port: must be between 1 and 65535",
		},
		{
			name: "rejects enabled MQTT port above range",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithPort(65536),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.port: must be between 1 and 65535",
		},
		{
			name: "rejects enabled MQTT topic prefix with empty segments",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithTopicPrefix("nest//controller"),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.topic_prefix: must be a relative MQTT topic prefix without empty segments",
		},
		{
			name: "rejects whitespace padded MQTT username",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithUsername(" user "),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.username: must not have leading or trailing whitespace",
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
			name: "requires lights to reference known relays",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Lights: []LightConfig{{ID: "light", Name: "Light", Relay: "missing"}},
			},
			message: `lights[0].relay: unknown relay "missing"`,
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
		{
			name: "requires bindings to reference known push buttons",
			file: Root{
				Sysfs:    SysfsConfig{Root: "/tmp"},
				Relays:   []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Lights:   []LightConfig{{ID: "light", Name: "Light", Relay: "relay"}},
				Bindings: []BindingConfig{{Button: "missing", Light: "light", Action: BindingActionToggle}},
			},
			message: `bindings[0].button: unknown push button "missing"`,
		},
		{
			name: "requires bindings to reference known lights",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: "button", Name: "Button", Input: "button_input"}},
				Bindings:      []BindingConfig{{Button: "button", Light: "missing", Action: BindingActionToggle}},
			},
			message: `bindings[0].light: unknown light "missing"`,
		},
		{
			name: "rejects unsupported binding actions",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: "button", Name: "Button", Input: "button_input"}},
				Relays:        []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Lights:        []LightConfig{{ID: "light", Name: "Light", Relay: "relay"}},
				Bindings:      []BindingConfig{{Button: "button", Light: "light", Action: "press"}},
			},
			message: `bindings[0].action: unsupported action "press"`,
		},
		{
			name: "rejects duplicate bindings",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: "button", Name: "Button", Input: "button_input"}},
				Relays:        []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Lights:        []LightConfig{{ID: "light", Name: "Light", Relay: "relay"}},
				Bindings: []BindingConfig{
					{Button: "button", Light: "light", Action: BindingActionToggle},
					{Button: "button", Light: "light", Action: BindingActionToggle},
				},
			},
			message: `bindings[1]: duplicate binding button "button" light "light" action "toggle"`,
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
		MQTT:  validMQTTConfig(),
		DigitalInputs: []DigitalInputConfig{
			{ID: "office_button_input", Device: "di_3_16"},
		},
		PushButtons: []PushButtonConfig{
			{ID: "office_button", Name: "Office button", Input: "office_button_input"},
		},
		Relays: []RelayConfig{
			{ID: "office_light_relay", Name: "Office light relay", Device: "ro_3_14"},
		},
		Lights: []LightConfig{
			{ID: "office_light", Name: "Office light", Relay: "office_light_relay"},
		},
		Bindings: []BindingConfig{
			{Button: "office_button", Light: "office_light", Action: BindingActionToggle},
		},
	}

	require.NoError(t, Validate(&file))
}

func validMQTTConfig() MQTTConfig {
	return validMQTTConfigWithPort(1883)
}

func validMQTTConfigWithPort(port int) MQTTConfig {
	return MQTTConfig{
		Enabled:     true,
		Host:        "localhost",
		Port:        port,
		ClientID:    "nest-controller-1",
		TopicPrefix: "nest",
		UnitID:      "controller_1",
	}
}

func validMQTTConfigWithTopicPrefix(topicPrefix string) MQTTConfig {
	mqtt := validMQTTConfig()
	mqtt.TopicPrefix = topicPrefix
	return mqtt
}

func validMQTTConfigWithHost(host string) MQTTConfig {
	mqtt := validMQTTConfig()
	mqtt.Host = host
	return mqtt
}

func validMQTTConfigWithUsername(username string) MQTTConfig {
	mqtt := validMQTTConfig()
	mqtt.Username = username
	return mqtt
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
