package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/event"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func Run(
	ctx context.Context,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	mqttStates chan<- mqtt.State,
	stateChanges <-chan sysfs.StateChange,
	done chan<- struct{},
) {
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			return
		case stateChange, ok := <-stateChanges:
			if !ok {
				return
			}

			handleStateChange(ctx, index, sysfsCommands, mqttStates, stateChange)
		}
	}
}

func handleStateChange(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, mqttStates chan<- mqtt.State, stateChange sysfs.StateChange) {
	publishMQTTState(ctx, mqttStates, mqtt.SysfsStateChange(mqttConfig(index), stateChange))

	digitalInputEvent, ok := event.StateChangeToDigitalInputEvent(index, stateChange)
	if ok {
		for _, state := range mqtt.DigitalInputStates(mqttConfig(index), *digitalInputEvent.DigitalInput, stateChange.NewValue) {
			publishMQTTState(ctx, mqttStates, state)
		}
		handleEvent(ctx, index, sysfsCommands, mqttStates, digitalInputEvent)
		return
	}
	if relay, ok := index.RelaysByDevice[entity.DeviceID(stateChange.Device.Identifier)]; ok {
		publishMQTTState(ctx, mqttStates, mqtt.RelayStateChange(mqttConfig(index), relay, stateChange.NewValue))
		slog.Info(
			"relay state change",
			"relay_id",
			relay.ID,
			"device_id",
			relay.Device,
			"old_value",
			sysfs.PrintableValue(stateChange.OldValue),
			"new_value",
			sysfs.PrintableValue(stateChange.NewValue),
		)
		return
	}

	slog.Info(
		"state change",
		"identifier",
		stateChange.Device.Identifier,
		"path",
		stateChange.Device.Path,
		"old_value",
		sysfs.PrintableValue(stateChange.OldValue),
		"new_value",
		sysfs.PrintableValue(stateChange.NewValue),
		"rising",
		stateChange.IsRising,
	)
}

func handleEvent(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, mqttStates chan<- mqtt.State, busEvent event.Event) {
	switch busEvent.Kind {
	case event.DigitalInputKind:
		slog.Info(
			"digital input event",
			"input_id",
			busEvent.DigitalInput.InputID,
			"device_id",
			busEvent.DigitalInput.DeviceID,
			"rising",
			busEvent.DigitalInput.IsRising,
			"falling",
			busEvent.DigitalInput.IsFalling,
		)

		for _, pushButtonEvent := range event.DigitalInputEventToPushButtonEvents(index, *busEvent.DigitalInput) {
			handleEvent(ctx, index, sysfsCommands, mqttStates, pushButtonEvent)
		}
	case event.PushButtonKind:
		publishMQTTState(ctx, mqttStates, mqtt.PushButtonEvent(mqttConfig(index), *busEvent.PushButton))

		slog.Info(
			"push button event",
			"button_id",
			busEvent.PushButton.ButtonID,
			"name",
			busEvent.PushButton.Name,
			"kind",
			busEvent.PushButton.Kind,
		)

		handlePushButtonEvent(ctx, index, sysfsCommands, *busEvent.PushButton)
	}
}

func publishMQTTState(ctx context.Context, states chan<- mqtt.State, state mqtt.State) {
	if states == nil {
		return
	}

	select {
	case <-ctx.Done():
	case states <- state:
	default:
		slog.Warn("mqtt state dropped", "topic", state.Topic, "kind", state.Kind)
	}
}

func mqttConfig(index *registry.Index) mqtt.Config {
	return mqtt.Config{UnitID: index.MQTT.UnitID, DiscoveryPrefix: index.MQTT.DiscoveryPrefix}
}

func handlePushButtonEvent(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, pushButtonEvent event.PushButtonEvent) {
	for _, binding := range index.BindingsByButtonID[pushButtonEvent.ButtonID] {
		handleBinding(ctx, index, sysfsCommands, binding)
	}
}

func handleBinding(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, binding entity.Binding) {
	if binding.Action != entity.LightActionToggle {
		slog.Error("unsupported light action", "action", binding.Action)
		return
	}

	light, ok := index.LightsByID[binding.Light]
	if !ok {
		slog.Error("binding references unknown light", "light_id", binding.Light)
		return
	}

	relay, ok := index.RelaysByID[light.Relay]
	if !ok {
		slog.Error("light references unknown relay", "light_id", light.ID, "relay_id", light.Relay)
		return
	}

	select {
	case <-ctx.Done():
		return
	case sysfsCommands <- sysfs.Command{
		Kind:     sysfs.ToggleCommand,
		DeviceID: string(relay.Device),
	}:
	}

	slog.Info(
		"light toggled",
		"light_id",
		light.ID,
		"name",
		light.Name,
		"relay_id",
		relay.ID,
		"device_id",
		relay.Device,
	)
}
