package controller

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunReturnsOnSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	index := registry.Build(&entity.Root{})
	stateChanges := make(chan sysfs.StateChange)
	commands := make(chan sysfs.Command)
	done := make(chan struct{})
	go Run(ctx, index, commands, nil, mqtt.Topics{}, stateChanges, done)

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "Run did not return after signal")
	}
}

func TestRunReturnsWhenPollEventsClose(t *testing.T) {
	ctx := t.Context()
	index := registry.Build(&entity.Root{})
	stateChanges := make(chan sysfs.StateChange)
	commands := make(chan sysfs.Command)
	done := make(chan struct{})
	go Run(ctx, index, commands, nil, mqtt.Topics{}, stateChanges, done)

	close(stateChanges)

	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "Run did not return after poll event channel closed")
	}
}

func TestDispatchPushButtonEventTogglesLightRelay(t *testing.T) {
	relayPath := filepath.Join(t.TempDir(), "ro_value")
	require.NoError(t, os.WriteFile(relayPath, []byte("0\n"), 0o644))

	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays:        []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
		Bindings:      []entity.Binding{{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle}},
	})
	commands := make(chan sysfs.Command, 1)

	logs := captureLogs(t, func() {
		dispatchEvent(t.Context(), index, commands, nil, mqtt.Topics{}, event.Event{
			Kind: event.PushButtonPressedKind,
			PushButton: &event.PushButton{
				ButtonID: entity.PushButtonID("office_button"),
				Name:     "Office button",
			},
		})
	})

	assert.Contains(t, logs, "push button event")
	assert.Contains(t, logs, "light event")
	assert.Contains(t, logs, "light toggled")
	cmd := <-commands
	require.Equal(t, sysfs.ToggleCommand, cmd.Kind)
	require.Equal(t, "ro_3_14", cmd.DeviceID)
	require.NoError(t, os.WriteFile(relayPath, []byte("1\n"), 0o644))

	data, err := os.ReadFile(relayPath)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(data))
}

func TestHandleStateChangePublishesMappedInputAndButtonState(t *testing.T) {
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays:        []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
		Bindings:      []entity.Binding{{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle}},
	})
	sysfsCommands := make(chan sysfs.Command, 1)
	mqttCommands := make(chan mqtt.Command, 2)
	topics := mqtt.NewTopics("nest", "controller_1")
	semanticEvents := make(chan event.Event, 2)

	normalizeStateChange(t.Context(), index, semanticEvents, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})
	dispatchEvent(t.Context(), index, sysfsCommands, mqttCommands, topics, <-semanticEvents)
	dispatchEvent(t.Context(), index, sysfsCommands, mqttCommands, topics, <-semanticEvents)

	inputCommand := <-mqttCommands
	assert.Equal(t, mqtt.PublishCommandKind, inputCommand.Kind)
	assert.Equal(t, "nest/units/controller_1/digital_inputs/office_button_input/state", inputCommand.Publish.Topic)
	assert.JSONEq(t, `{"input_id":"office_button_input","sysfs_device":"di_3_16","value":1}`, string(inputCommand.Publish.Payload))
	assert.True(t, inputCommand.Publish.Retain)

	buttonCommand := <-mqttCommands
	assert.Equal(t, mqtt.PublishCommandKind, buttonCommand.Kind)
	assert.Equal(t, "nest/units/controller_1/push_buttons/office_button/state", buttonCommand.Publish.Topic)
	assert.JSONEq(t, `{"button_id":"office_button","name":"Office button","state":"pressed"}`, string(buttonCommand.Publish.Payload))
	assert.False(t, buttonCommand.Publish.Retain)

	sysfsCommand := <-sysfsCommands
	assert.Equal(t, sysfs.ToggleCommand, sysfsCommand.Kind)
	assert.Equal(t, "ro_3_14", sysfsCommand.DeviceID)
}

func TestHandleStateChangePublishesMappedRelayState(t *testing.T) {
	index := registry.Build(&entity.Root{
		Relays: []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
	})
	mqttCommands := make(chan mqtt.Command, 1)
	topics := mqtt.NewTopics("nest", "controller_1")
	semanticEvents := make(chan event.Event, 1)

	normalizeStateChange(t.Context(), index, semanticEvents, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "ro_3_14", Path: "/sys/ro_3_14/ro_value"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})
	dispatchEvent(t.Context(), index, nil, mqttCommands, topics, <-semanticEvents)

	command := <-mqttCommands
	assert.Equal(t, mqtt.PublishCommandKind, command.Kind)
	assert.Equal(t, "nest/units/controller_1/relays/office_light_relay/state", command.Publish.Topic)
	assert.JSONEq(t, `{"relay_id":"office_light_relay","name":"Office light relay","sysfs_device":"ro_3_14","value":1}`, string(command.Publish.Payload))
	assert.True(t, command.Publish.Retain)
}

func TestHandleStateChangeLogsRawStateChangeForUnknownDevice(t *testing.T) {
	index := registry.Build(&entity.Root{})
	semanticEvents := make(chan event.Event, 1)

	logs := captureLogs(t, func() {
		normalizeStateChange(t.Context(), index, semanticEvents, sysfs.StateChange{
			Device:   sysfs.Device{Identifier: "ro_3_14", Path: "/sys/ro_3_14/ro_value"},
			OldValue: sysfs.Off,
			NewValue: sysfs.On,
			IsRising: true,
		})
	})

	assert.Contains(t, logs, "state change")
	assert.Contains(t, logs, "ro_3_14")
	assert.NotContains(t, logs, "push button event")
}

func TestHandleStateChangeLogsPushButtonRelease(t *testing.T) {
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Bindings:      []entity.Binding{{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle}},
	})
	semanticEvents := make(chan event.Event, 2)

	logs := captureLogs(t, func() {
		normalizeStateChange(t.Context(), index, semanticEvents, sysfs.StateChange{
			Device:   sysfs.Device{Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
			OldValue: sysfs.On,
			NewValue: sysfs.Off,
			IsRising: false,
		})
		dispatchEvent(t.Context(), index, nil, nil, mqtt.Topics{}, <-semanticEvents)
		dispatchEvent(t.Context(), index, nil, nil, mqtt.Topics{}, <-semanticEvents)
	})

	assert.Contains(t, logs, "push button event")
	assert.Contains(t, logs, "released")
	assert.NotContains(t, logs, "light event")
	assert.NotContains(t, logs, "light toggled")
}

func TestHandleStateChangeDoesNotBlockCommandSendAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays:        []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
		Bindings:      []entity.Binding{{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle}},
	})
	semanticEvents := make(chan event.Event)
	done := make(chan struct{})

	go func() {
		defer close(done)
		normalizeStateChange(ctx, index, semanticEvents, sysfs.StateChange{
			Device:   sysfs.Device{Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
			OldValue: sysfs.Off,
			NewValue: sysfs.On,
			IsRising: true,
		})
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "handleStateChange did not return after cancellation")
	}
}

func captureLogs(t *testing.T, fn func()) string {
	t.Helper()

	var buffer bytes.Buffer
	previous := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	fn()

	require.NotEmpty(t, buffer.String())
	return buffer.String()
}
