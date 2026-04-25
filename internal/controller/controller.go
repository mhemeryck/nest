package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/event"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func Run(
	ctx context.Context,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
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

			handleStateChange(index, sysfsCommands, stateChange)
		}
	}
}

func handleStateChange(index *registry.Index, sysfsCommands chan<- sysfs.Command, stateChange sysfs.StateChange) {
	digitalInputEvent, ok := event.StateChangeToDigitalInputEvent(index, stateChange)
	if ok {
		handleEvent(index, sysfsCommands, digitalInputEvent)
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

func handleEvent(index *registry.Index, sysfsCommands chan<- sysfs.Command, busEvent event.Event) {
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
			handleEvent(index, sysfsCommands, pushButtonEvent)
		}
	case event.PushButtonKind:
		slog.Info(
			"push button event",
			"button_id",
			busEvent.PushButton.ButtonID,
			"name",
			busEvent.PushButton.Name,
			"kind",
			busEvent.PushButton.Kind,
		)

		handlePushButtonEvent(index, sysfsCommands, *busEvent.PushButton)
	}
}

func handlePushButtonEvent(index *registry.Index, sysfsCommands chan<- sysfs.Command, pushButtonEvent event.PushButtonEvent) {
	for _, binding := range index.BindingsByButtonID[pushButtonEvent.ButtonID] {
		handleBinding(index, sysfsCommands, binding)
	}
}

func handleBinding(index *registry.Index, sysfsCommands chan<- sysfs.Command, binding entity.Binding) {
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

	sysfsCommands <- sysfs.Command{
		Kind:     sysfs.ToggleCommand,
		DeviceID: string(relay.Device),
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
