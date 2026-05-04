package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

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
