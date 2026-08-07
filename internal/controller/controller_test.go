package controller

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
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
	go Run(ctx, index, commands, nil, nil, mqtt.Topics{}, stateChanges, nil, nil, done)

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
	go Run(ctx, index, commands, nil, nil, mqtt.Topics{}, stateChanges, nil, nil, done)

	close(stateChanges)

	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "Run did not return after poll event channel closed")
	}
}

func TestRunContinuesWhenModbusEventsClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	index := registry.Build(&entity.Root{})
	stateChanges := make(chan sysfs.StateChange)
	modbusEvents := make(chan modbus.Event)
	commands := make(chan sysfs.Command)
	done := make(chan struct{})
	go Run(ctx, index, commands, nil, nil, mqtt.Topics{}, stateChanges, nil, modbusEvents, done)

	close(modbusEvents)

	select {
	case <-done:
		require.Fail(t, "Run returned after Modbus event channel closed")
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "Run did not return after signal")
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
		Bindings:      []entity.Binding{{Source: entity.ID("office_button"), Target: entity.ID("office_light"), Action: entity.ActionToggle}},
	})
	commands := make(chan sysfs.Command, 1)

	logs := captureLogs(t, func() {
		dispatchQueuedTestEvents(t.Context(), index, commands, nil, mqtt.Topics{}, event.Event{
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

func TestDispatchPushButtonEventDerivesLightEvent(t *testing.T) {
	index := registry.Build(&entity.Root{
		PushButtons: []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:      []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Bindings:    []entity.Binding{{Source: entity.ID("office_button"), Target: entity.ID("office_light"), Action: entity.ActionToggle}},
	})
	sysfsCommands := make(chan sysfs.Command, 1)

	derivedEvents := dispatchEvent(t.Context(), index, sysfsCommands, nil, mqtt.Topics{}, event.Event{
		Kind: event.PushButtonPressedKind,
		PushButton: &event.PushButton{
			ButtonID: entity.PushButtonID("office_button"),
			Name:     "Office button",
		},
	})

	require.Len(t, derivedEvents, 1)
	assert.Equal(t, event.LightKind, derivedEvents[0].Kind)
	assert.Equal(t, entity.LightID("office_light"), derivedEvents[0].Light.LightID)
	assert.Equal(t, entity.LightActionToggle, derivedEvents[0].Light.Action)
	assert.Empty(t, sysfsCommands)
}

func TestHandleStateChangeDoesNotPublishRawInputOrButtonState(t *testing.T) {
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays:        []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
		Bindings:      []entity.Binding{{Source: entity.ID("office_button"), Target: entity.ID("office_light"), Action: entity.ActionToggle}},
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
	dispatchQueuedTestEvents(t.Context(), index, sysfsCommands, mqttCommands, topics, <-semanticEvents, <-semanticEvents)

	assert.Empty(t, mqttCommands)

	sysfsCommand := <-sysfsCommands
	assert.Equal(t, sysfs.ToggleCommand, sysfsCommand.Kind)
	assert.Equal(t, "ro_3_14", sysfsCommand.DeviceID)
}

func TestProjectedConfigStateChangeTogglesLocalLight(t *testing.T) {
	configRoot, err := config.Load(filepath.Join("..", "..", "test", "fixtures", "config.local.yaml"), "controller_1")
	require.NoError(t, err)
	root := config.ToEntityRoot(configRoot)
	index := registry.Build(root)
	sysfsCommands := make(chan sysfs.Command, 1)
	mqttCommands := make(chan mqtt.Command, 2)
	semanticEvents := make(chan event.Event, 2)

	normalizeStateChange(t.Context(), index, semanticEvents, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})
	inputEvent := <-semanticEvents
	buttonEvent := <-semanticEvents

	assert.Equal(t, event.DigitalInputStateKind, inputEvent.Kind)
	assert.Equal(t, event.PushButtonPressedKind, buttonEvent.Kind)
	assert.Equal(t, entity.PushButtonID("controller_1.button.office_button"), buttonEvent.PushButton.ButtonID)

	dispatchQueuedTestEvents(t.Context(), index, sysfsCommands, mqttCommands, mqtt.NewTopics("nest", "controller_1"), inputEvent, buttonEvent)

	assert.Empty(t, mqttCommands)
	sysfsCommand := <-sysfsCommands
	assert.Equal(t, sysfs.ToggleCommand, sysfsCommand.Kind)
	assert.Equal(t, "ro_3_14", sysfsCommand.DeviceID)
}

func TestDispatchPushButtonEventPublishesRemoteSourceEvent(t *testing.T) {
	root := &entity.Root{
		PushButtons: []entity.PushButton{{ID: entity.PushButtonID("controller_1.button.office_button"), Name: "Office button", Input: entity.DigitalInputID("controller_1.digital_input.office_button_input")}},
		RemoteSourceBindings: []entity.Binding{{
			Source: entity.ID("controller_1.button.office_button"),
			Target: entity.ID("controller_2.light.hall_light"),
			Action: entity.ActionToggle,
		}},
	}
	index := registry.Build(root)
	mqttCommands := make(chan mqtt.Command, 1)

	dispatchEvent(t.Context(), index, nil, mqttCommands, mqtt.NewTopics("nest", "controller_1"), event.Event{
		Kind: event.PushButtonPressedKind,
		PushButton: &event.PushButton{
			ButtonID: entity.PushButtonID("controller_1.button.office_button"),
			Name:     "Office button",
		},
	})

	command := <-mqttCommands
	assert.Equal(t, mqtt.PublishCommandKind, command.Kind)
	assert.Equal(t, "nest/units/controller_1/sources/controller_1.button.office_button/event", command.Publish.Topic)
	assert.JSONEq(t, `{"source":"controller_1.button.office_button","event":"pressed"}`, string(command.Publish.Payload))
	assert.False(t, command.Publish.Retain)
}

func TestDispatchRemoteSourceEventTogglesTargetLocalLight(t *testing.T) {
	index := registry.Build(&entity.Root{
		Lights: []entity.Light{{ID: entity.LightID("controller_1.light.office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays: []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
		RemoteTargetBindings: []entity.Binding{{
			Source: entity.ID("controller_2.button.hall_button"),
			Target: entity.ID("controller_1.light.office_light"),
			Action: entity.ActionToggle,
		}},
	})
	sysfsCommands := make(chan sysfs.Command, 1)

	derivedEvents := dispatchEvent(t.Context(), index, sysfsCommands, nil, mqtt.Topics{}, event.Event{
		Kind: event.PushButtonPressedKind,
		PushButton: &event.PushButton{
			ButtonID: entity.PushButtonID("controller_2.button.hall_button"),
		},
	})
	require.Len(t, derivedEvents, 1)
	dispatchEvent(t.Context(), index, sysfsCommands, nil, mqtt.Topics{}, derivedEvents[0])

	command := <-sysfsCommands
	assert.Equal(t, sysfs.ToggleCommand, command.Kind)
	assert.Equal(t, "ro_3_14", command.DeviceID)
}

func TestHandleStateChangePublishesMappedLightState(t *testing.T) {
	index := registry.Build(&entity.Root{
		Lights: []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays: []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
	})
	mqttCommands := make(chan mqtt.Command, 2)
	topics := mqtt.NewTopics("nest", "controller_1")
	semanticEvents := make(chan event.Event, 2)

	normalizeStateChange(t.Context(), index, semanticEvents, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "ro_3_14", Path: "/sys/ro_3_14/ro_value"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})
	dispatchEvent(t.Context(), index, nil, mqttCommands, topics, <-semanticEvents)
	dispatchEvent(t.Context(), index, nil, mqttCommands, topics, <-semanticEvents)

	lightCommand := <-mqttCommands
	assert.Equal(t, mqtt.PublishCommandKind, lightCommand.Kind)
	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", lightCommand.Publish.Topic)
	assert.JSONEq(t, `{"state":"ON"}`, string(lightCommand.Publish.Payload))
	assert.True(t, lightCommand.Publish.Retain)
	assert.Empty(t, mqttCommands)
}

func TestPublishMQTTDoesNotBlockWhenCommandChannelIsFull(t *testing.T) {
	commands := make(chan mqtt.Command, 1)
	commands <- mqtt.PublishCommand(mqtt.PublishMessage{Topic: "nest/full"})

	published := publishMQTT(t.Context(), commands, mqtt.PublishMessage{Topic: "nest/dropped"})

	assert.False(t, published)
	assert.Len(t, commands, 1)
}

func TestNormalizeMQTTEventPublishesStartupCommandsOnConnect(t *testing.T) {
	root := &entity.Root{
		MQTT: entity.MQTT{
			TopicPrefix: "nest",
			UnitID:      "controller_1",
		},
		Lights: []entity.Light{{
			ID:    entity.LightID("office_light"),
			Name:  "Office light",
			Relay: entity.RelayID("office_light_relay"),
		}},
	}
	commands := make(chan mqtt.Command, 2)
	semanticEvent, handled := semanticEventFromMQTTEvent(registry.Build(root), mqtt.Topics{}, mqtt.ConnectedEvent())
	require.True(t, handled)

	dispatchEvent(t.Context(), registry.Build(root), nil, commands, mqtt.Topics{}, semanticEvent)

	require.Len(t, commands, 2)
	assert.Equal(t, "homeassistant/device/nest_controller_1_unit/config", (<-commands).Publish.Topic)
	assert.Equal(t, "nest/units/controller_1/availability", (<-commands).Publish.Topic)
}

func TestNormalizeMQTTEventIgnoresNonConnectEvents(t *testing.T) {
	commands := make(chan mqtt.Command, 1)
	semanticEvent, handled := semanticEventFromMQTTEvent(registry.Build(&entity.Root{}), mqtt.Topics{}, mqtt.PublishedEvent(mqtt.PublishMessage{Topic: "nest/topic"}))
	require.True(t, handled)

	dispatchEvent(t.Context(), registry.Build(&entity.Root{}), nil, commands, mqtt.Topics{}, semanticEvent)

	assert.Empty(t, commands)
}

func TestDispatchMQTTLightCommandTurnsLightOn(t *testing.T) {
	index := registry.Build(&entity.Root{
		Lights: []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays: []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
	})
	commands := make(chan sysfs.Command, 1)

	dispatchEvent(t.Context(), index, commands, nil, mqtt.Topics{}, event.Event{
		Kind: event.LightKind,
		Light: &event.Light{
			LightID: entity.LightID("office_light"),
			Name:    "Office light",
			Action:  entity.LightActionOn,
		},
	})

	command := <-commands
	assert.Equal(t, sysfs.OnCommand, command.Kind)
	assert.Equal(t, "ro_3_14", command.DeviceID)
}

func TestProjectedConfigMQTTLightCommandTurnsLightOn(t *testing.T) {
	configRoot, err := config.Load(filepath.Join("..", "..", "test", "fixtures", "config.local.yaml"), "controller_1")
	require.NoError(t, err)
	root := config.ToEntityRoot(configRoot)
	commands := make(chan sysfs.Command, 1)

	semanticEvent, handled := semanticEventFromMQTTEvent(
		registry.Build(root),
		mqtt.NewTopics("nest", "controller_1"),
		mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: "nest/units/controller_1/lights/office_light/command", Payload: []byte("ON")}),
	)
	require.True(t, handled)
	assert.Equal(t, entity.LightID("controller_1.light.office_light"), semanticEvent.Light.LightID)

	dispatchEvent(t.Context(), registry.Build(root), commands, nil, mqtt.Topics{}, semanticEvent)

	command := <-commands
	assert.Equal(t, sysfs.OnCommand, command.Kind)
	assert.Equal(t, "ro_3_14", command.DeviceID)
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
		Bindings:      []entity.Binding{{Source: entity.ID("office_button"), Target: entity.ID("office_light"), Action: entity.ActionToggle}},
	})
	semanticEvents := make(chan event.Event, 2)

	logs := captureLogs(t, func() {
		normalizeStateChange(t.Context(), index, semanticEvents, sysfs.StateChange{
			Device:   sysfs.Device{Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
			OldValue: sysfs.On,
			NewValue: sysfs.Off,
			IsRising: false,
		})
		dispatchQueuedTestEvents(t.Context(), index, nil, nil, mqtt.Topics{}, <-semanticEvents, <-semanticEvents)
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
		Bindings:      []entity.Binding{{Source: entity.ID("office_button"), Target: entity.ID("office_light"), Action: entity.ActionToggle}},
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

func dispatchQueuedTestEvents(
	ctx context.Context,
	index *registry.Registry,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	mqttTopics mqtt.Topics,
	events ...event.Event,
) {
	semanticEvents := make(chan event.Event, len(events))
	for _, semanticEvent := range events {
		semanticEvents <- semanticEvent
	}
	close(semanticEvents)

	dispatchEvents(ctx, index, sysfsCommands, mqttCommands, mqttTopics, semanticEvents)
}
