package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/event"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

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
