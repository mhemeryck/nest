package controller

import (
	"log/slog"
	"os"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/event"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func Run(
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	pollEvents <-chan sysfs.PollEvent,
	sigCh <-chan os.Signal,
) {
	for {
		select {
		case <-sigCh:
			return
		case pollEvent, ok := <-pollEvents:
			if !ok {
				return
			}

			handlePollEvent(index, sysfsCommands, pollEvent)
		}
	}
}

func handlePollEvent(index *registry.Index, sysfsCommands chan<- sysfs.Command, pollEvent sysfs.PollEvent) {
	digitalInputEvent, ok := event.PollEventToDigitalInputEvent(index, pollEvent)
	if ok {
		handleEvent(index, sysfsCommands, digitalInputEvent)
		return
	}

	slog.Info(
		"poll event",
		"identifier",
		pollEvent.Device.Identifier,
		"path",
		pollEvent.Device.Path,
		"old_value",
		sysfs.PrintableValue(pollEvent.OldValue),
		"new_value",
		sysfs.PrintableValue(pollEvent.NewValue),
		"rising",
		pollEvent.IsRising,
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
