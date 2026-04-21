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
	devices []*sysfs.Device,
	pollEvents <-chan sysfs.PollEvent,
	sigCh <-chan os.Signal,
) {
	devicesByID := buildDevicesByID(devices)

	for {
		select {
		case <-sigCh:
			return
		case pollEvent, ok := <-pollEvents:
			if !ok {
				return
			}

			handlePollEvent(index, devicesByID, pollEvent)
		}
	}
}

func handlePollEvent(index *registry.Index, devicesByID map[entity.DeviceID]*sysfs.Device, pollEvent sysfs.PollEvent) {
	digitalInputEvent, ok := event.PollEventToDigitalInputEvent(index, pollEvent)
	if ok {
		handleEvent(index, devicesByID, digitalInputEvent)
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

func handleEvent(index *registry.Index, devicesByID map[entity.DeviceID]*sysfs.Device, busEvent event.Event) {
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
			handleEvent(index, devicesByID, pushButtonEvent)
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

		handlePushButtonEvent(index, devicesByID, *busEvent.PushButton)
	}
}

func handlePushButtonEvent(index *registry.Index, devicesByID map[entity.DeviceID]*sysfs.Device, pushButtonEvent event.PushButtonEvent) {
	for _, binding := range index.BindingsByButtonID[pushButtonEvent.ButtonID] {
		handleBinding(index, devicesByID, binding)
	}
}

func handleBinding(index *registry.Index, devicesByID map[entity.DeviceID]*sysfs.Device, binding entity.Binding) {
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

	device, ok := devicesByID[relay.Device]
	if !ok {
		slog.Error("relay device not available", "light_id", light.ID, "relay_id", relay.ID, "device_id", relay.Device)
		return
	}

	newValue, err := sysfs.ToggleDevice(device)
	if err != nil {
		slog.Error("toggle light failed", "light_id", light.ID, "relay_id", relay.ID, "device_id", relay.Device, "error", err)
		return
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
		"value",
		sysfs.PrintableValue(newValue),
	)
}

func buildDevicesByID(devices []*sysfs.Device) map[entity.DeviceID]*sysfs.Device {
	devicesByID := make(map[entity.DeviceID]*sysfs.Device, len(devices))
	for _, device := range devices {
		devicesByID[entity.DeviceID(device.Identifier)] = device
	}

	return devicesByID
}
