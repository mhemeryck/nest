package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func dispatchEvent(
	ctx context.Context,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	mqttTopics mqtt.Topics,
	busEvent event.Event,
) {
	switch busEvent.Kind {
	case event.DigitalInputStateKind:
		publishDigitalInputState(ctx, mqttCommands, mqttTopics, *busEvent.DigitalInput)
	case event.PushButtonPressedKind, event.PushButtonReleasedKind:
		logPushButtonEvent(busEvent.Kind, *busEvent.PushButton)
		publishPushButtonState(ctx, mqttCommands, mqttTopics, busEvent.Kind, *busEvent.PushButton)
		dispatchLightEventsFromPushButton(ctx, index, sysfsCommands, mqttCommands, mqttTopics, busEvent.Kind, *busEvent.PushButton)
	case event.RelayStateKind:
		publishRelayState(ctx, mqttCommands, mqttTopics, *busEvent.Relay)
	case event.LightKind:
		dispatchLightEvent(ctx, index, sysfsCommands, *busEvent.Light)
	}
}

func logPushButtonEvent(eventKind event.Kind, pushButton event.PushButton) {
	slog.Info(
		"push button event",
		"button_id",
		pushButton.ButtonID,
		"name",
		pushButton.Name,
		"event_kind",
		eventKind,
	)
}

func dispatchLightEventsFromPushButton(
	ctx context.Context,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	mqttTopics mqtt.Topics,
	eventKind event.Kind,
	pushButton event.PushButton,
) {
	if eventKind != event.PushButtonPressedKind {
		return
	}

	for _, lightEvent := range lightEventsFromPushButton(index, pushButton) {
		dispatchEvent(ctx, index, sysfsCommands, mqttCommands, mqttTopics, lightEvent)
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
