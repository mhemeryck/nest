package controller

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	go Run(ctx, index, commands, nil, stateChanges, done)

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
	go Run(ctx, index, commands, nil, stateChanges, done)

	close(stateChanges)

	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "Run did not return after poll event channel closed")
	}
}

func TestHandleStateChangeTogglesLightRelay(t *testing.T) {
	relayPath := filepath.Join(t.TempDir(), "ro_value")
	require.NoError(t, os.WriteFile(relayPath, []byte("0\n"), 0o644))

	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays:        []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", Device: entity.DeviceID("ro_3_14")}},
		Bindings:      []entity.Binding{{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle}},
	})
	commands := make(chan sysfs.Command, 1)
	commandDone := make(chan struct{})
	go func() {
		defer close(commandDone)
		cmd := <-commands
		require.Equal(t, sysfs.ToggleCommand, cmd.Kind)
		require.Equal(t, "ro_3_14", cmd.DeviceID)
		require.NoError(t, os.WriteFile(relayPath, []byte("1\n"), 0o644))
	}()

	logs := captureLogs(t, func() {
		handleStateChange(t.Context(), index, commands, nil, sysfs.StateChange{
			Device:   sysfs.Device{Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
			OldValue: sysfs.Off,
			NewValue: sysfs.On,
			IsRising: true,
		})
	})

	assert.Contains(t, logs, "digital input event")
	assert.Contains(t, logs, "push button event")
	assert.Contains(t, logs, "light toggled")
	<-commandDone

	data, err := os.ReadFile(relayPath)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(data))
}

func TestHandleStateChangeLogsRawStateChangeForUnknownDevice(t *testing.T) {
	index := registry.Build(&entity.Root{})

	logs := captureLogs(t, func() {
		handleStateChange(t.Context(), index, nil, nil, sysfs.StateChange{
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

func TestHandleStateChangePublishesMQTTInputStates(t *testing.T) {
	index := registry.Build(&entity.Root{
		MQTT:          entity.MQTT{UnitID: "tesla"},
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
	})
	mqttStates := make(chan mqtt.State, 4)

	handleStateChange(t.Context(), index, nil, mqttStates, sysfs.StateChange{
		Device:   sysfs.Device{Type: sysfs.DigitalInput, Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})

	assertPublished(t, mqttStates, mqtt.SysfsState, "tesla/sysfs/di_3_16/state", true, `{"device_id":"di_3_16","type":"digital_input","old":"OFF","new":"ON","rising":true,"falling":false}`)
	assertPublished(t, mqttStates, mqtt.InputState, "tesla/input/3_16/state", true, "ON")
	assertPublished(t, mqttStates, mqtt.InputState, "tesla/input/3_16/event", false, `{"input_id":"office_button_input","device_id":"di_3_16","state":"ON","rising":true,"falling":false}`)
	assertPublished(t, mqttStates, mqtt.PushButtonState, "tesla/push_button/office_button/event", false, `{"button_id":"office_button","name":"Office button","kind":"pressed"}`)
}

func TestHandleStateChangePublishesMQTTRelayState(t *testing.T) {
	index := registry.Build(&entity.Root{
		MQTT:   entity.MQTT{UnitID: "edison"},
		Relays: []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", Device: entity.DeviceID("ro_3_14")}},
	})
	mqttStates := make(chan mqtt.State, 2)

	handleStateChange(t.Context(), index, nil, mqttStates, sysfs.StateChange{
		Device:   sysfs.Device{Type: sysfs.RelayOutput, Identifier: "ro_3_14", Path: "/sys/ro_3_14/ro_value"},
		OldValue: sysfs.On,
		NewValue: sysfs.Off,
	})

	assertPublished(t, mqttStates, mqtt.SysfsState, "edison/sysfs/ro_3_14/state", true, `{"device_id":"ro_3_14","type":"relay","old":"ON","new":"OFF","rising":false,"falling":true}`)
	assertPublished(t, mqttStates, mqtt.RelayState, "edison/relay/3_14/state", true, "OFF")
}

func TestHandleStateChangeDoesNotBlockCommandSendAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays:        []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", Device: entity.DeviceID("ro_3_14")}},
		Bindings:      []entity.Binding{{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle}},
	})
	commands := make(chan sysfs.Command)
	done := make(chan struct{})

	go func() {
		defer close(done)
		handleStateChange(ctx, index, commands, nil, sysfs.StateChange{
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

func assertPublished(t *testing.T, states <-chan mqtt.State, kind mqtt.StateKind, topic string, retain bool, payload string) {
	t.Helper()

	select {
	case state := <-states:
		assert.Equal(t, kind, state.Kind)
		assert.Equal(t, topic, state.Topic)
		assert.Equal(t, retain, state.Retain)
		if len(payload) > 0 && payload[0] == '{' {
			assert.JSONEq(t, payload, string(state.Payload))
		} else {
			assert.Equal(t, payload, string(state.Payload))
		}
	case <-time.After(time.Second):
		require.Fail(t, "expected mqtt state")
	}
}
