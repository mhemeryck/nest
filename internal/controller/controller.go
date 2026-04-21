package controller

import (
	"log/slog"
	"os"

	"github.com/mhemeryck/nest/internal/event"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func Run(
	index *registry.Index,
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

			handlePollEvent(index, pollEvent)
		}
	}
}

func handlePollEvent(index *registry.Index, pollEvent sysfs.PollEvent) {
	digitalInputEvent, ok := event.PollEventToDigitalInputEvent(index, pollEvent)
	if ok {
		handleEvent(index, digitalInputEvent)
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

func handleEvent(index *registry.Index, busEvent event.Event) {
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
			handleEvent(index, pushButtonEvent)
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
	}
}
