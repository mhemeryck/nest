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
	logSemanticEvent(busEvent)
	dispatchSysfsCommand(ctx, index, sysfsCommands, busEvent)
	dispatchMQTTCommand(ctx, mqttCommands, mqttTopics, busEvent)
}

func dispatchSysfsCommand(ctx context.Context, index *registry.Index, commands chan<- sysfs.Command, busEvent event.Event) {
	switch busEvent.Kind {
	case event.PushButtonPressedKind:
		dispatchLightEventsFromPushButton(ctx, index, commands, *busEvent.PushButton)
	case event.LightKind:
		dispatchLightEvent(ctx, index, commands, *busEvent.Light)
	}
}

func dispatchMQTTCommand(ctx context.Context, commands chan<- mqtt.Command, topics mqtt.Topics, busEvent event.Event) {
	switch busEvent.Kind {
	case event.DigitalInputStateKind:
		publishDigitalInputState(ctx, commands, topics, *busEvent.DigitalInput)
	case event.PushButtonPressedKind, event.PushButtonReleasedKind:
		publishPushButtonState(ctx, commands, topics, busEvent.Kind, *busEvent.PushButton)
	case event.RelayStateKind:
		publishRelayState(ctx, commands, topics, *busEvent.Relay)
	}
}

func dispatchLightEventsFromPushButton(
	ctx context.Context,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	pushButton event.PushButton,
) {
	for _, lightEvent := range lightEventsFromPushButton(index, pushButton) {
		logSemanticEvent(lightEvent)
		dispatchSysfsCommand(ctx, index, sysfsCommands, lightEvent)
	}
}

func logSemanticEvent(busEvent event.Event) {
	switch busEvent.Kind {
	case event.PushButtonPressedKind, event.PushButtonReleasedKind:
		logPushButtonEvent(busEvent.Kind, *busEvent.PushButton)
	case event.LightKind:
		logLightEvent(*busEvent.Light)
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

func dispatchLightEvent(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, lightEvent event.Light) {
	if lightEvent.Action != entity.LightActionToggle {
		slog.Error("unsupported light action", "action", lightEvent.Action)
		return
	}

	handleLightToggle(ctx, index, sysfsCommands, lightEvent)
}

func logLightEvent(lightEvent event.Light) {
	slog.Info(
		"light event",
		"light_id",
		lightEvent.LightID,
		"name",
		lightEvent.Name,
		"action",
		lightEvent.Action,
	)
}
