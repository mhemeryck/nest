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
			dispatchEvent(ctx, index, sysfsCommands, pushButtonEvent)
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

func dispatchEvent(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, busEvent event.Event) {
	switch busEvent.Kind {
	case event.PushButtonKind:
		handlePushButtonEvent(ctx, index, sysfsCommands, *busEvent.PushButton)
	case event.LightKind:
		dispatchLightEvent(ctx, index, sysfsCommands, *busEvent.Light)
	}
}

func handlePushButtonEvent(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, pushButton event.PushButton) {
	slog.Info(
		"push button event",
		"button_id",
		pushButton.ButtonID,
		"name",
		pushButton.Name,
		"kind",
		pushButton.Kind,
	)

	for _, lightEvent := range lightEventsFromPushButton(index, pushButton) {
		dispatchEvent(ctx, index, sysfsCommands, lightEvent)
	}
}

func dispatchLightEvent(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, lightEvent event.Light) {
	slog.Info(
		"light event",
		"light_id",
		lightEvent.LightID,
		"name",
		lightEvent.Name,
		"action",
		lightEvent.Action,
	)

	if lightEvent.Action != entity.LightActionToggle {
		slog.Error("unsupported light action", "action", lightEvent.Action)
		return
	}

	handleLightToggle(ctx, index, sysfsCommands, lightEvent)
}

func handleLightToggle(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, lightEvent event.Light) {
	light, ok := registry.LightByID(index, lightEvent.LightID)
	if !ok {
		slog.Error("unknown light", "light_id", lightEvent.LightID)
		return
	}

	cmd, ok := relayToggleCommandForLight(index, light)
	if !ok {
		return
	}

	select {
	case <-ctx.Done():
		return
	case sysfsCommands <- cmd:
	}

	slog.Info(
		"light toggled",
		"light_id",
		lightEvent.LightID,
		"name",
		lightEvent.Name,
		"device_id",
		cmd.DeviceID,
	)
}

func relayToggleCommandForLight(index *registry.Index, light entity.Light) (sysfs.Command, bool) {
	relay, ok := registry.RelayByID(index, light.Relay)
	if !ok {
		slog.Error("light references unknown relay", "light_id", light.ID, "relay_id", light.Relay)
		return sysfs.Command{}, false
	}

	return sysfs.Command{
		Kind:     sysfs.ToggleCommand,
		DeviceID: string(relay.SysfsDevice),
	}, true
}
