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

			handleStateChange(ctx, index, sysfsCommands, stateChange)
		}
	}
}

func handleStateChange(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, stateChange sysfs.StateChange) {
	pushButtonEvents, handled := pushButtonEventsFromStateChange(index, stateChange)
	if handled {
		for _, pushButtonEvent := range pushButtonEvents {
			handleEvent(ctx, index, sysfsCommands, pushButtonEvent)
		}
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

func handleEvent(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, busEvent event.Event) {
	switch busEvent.Kind {
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

		for _, lightEvent := range lightEventsFromPushButton(index, *busEvent.PushButton) {
			handleEvent(ctx, index, sysfsCommands, lightEvent)
		}
	case event.LightKind:
		slog.Info(
			"light event",
			"light_id",
			busEvent.Light.LightID,
			"name",
			busEvent.Light.Name,
			"action",
			busEvent.Light.Action,
		)

		handleLightEvent(ctx, index, sysfsCommands, *busEvent.Light)
	}
}

func handleLightEvent(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, lightEvent event.Light) {
	if lightEvent.Action != entity.LightActionToggle {
		slog.Error("unsupported light action", "action", lightEvent.Action)
		return
	}

	light, ok := index.LightsByID[lightEvent.LightID]
	if !ok {
		slog.Error("unknown light", "light_id", lightEvent.LightID)
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
		DeviceID: string(relay.SysfsDevice),
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
		relay.SysfsDevice,
	)
}
