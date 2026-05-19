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
	root *entity.Root,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	mqttTopics mqtt.Topics,
	busEvent event.Event,
) {
	logSemanticEvent(busEvent)
	dispatchSysfsCommand(ctx, index, sysfsCommands, busEvent)
	dispatchMQTTCommand(ctx, root, mqttCommands, mqttTopics, busEvent)
}

func dispatchSysfsCommand(ctx context.Context, index *registry.Index, commands chan<- sysfs.Command, busEvent event.Event) {
	switch busEvent.Kind {
	case event.PushButtonPressedKind:
		dispatchLightEventsFromPushButton(ctx, index, commands, *busEvent.PushButton)
	case event.LightKind:
		dispatchLightEvent(ctx, index, commands, *busEvent.Light)
	}
}

func dispatchMQTTCommand(ctx context.Context, root *entity.Root, commands chan<- mqtt.Command, topics mqtt.Topics, busEvent event.Event) {
	switch busEvent.Kind {
	case event.LightStateKind:
		publishLightState(ctx, commands, topics, *busEvent.LightState)
	case event.MQTTConnectedKind:
		if err := publishMQTTStartup(ctx, root, commands); err != nil {
			slog.Error("mqtt startup publish failed", "error", err)
		}
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
	case event.MQTTConnectedKind:
		slog.Info("mqtt actor connected")
	case event.MQTTConnectFailedKind:
		slog.Error("mqtt actor connect failed", "error", busEvent.MQTT.Error)
	case event.MQTTDisconnectedKind:
		slog.Info("mqtt actor disconnected")
	case event.MQTTPublishedKind:
		slog.Debug("mqtt message published", "topic", busEvent.MQTT.PublishTopic)
	case event.MQTTPublishFailedKind:
		slog.Error("mqtt message publish failed", "topic", busEvent.MQTT.PublishTopic, "error", busEvent.MQTT.Error)
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
	switch lightEvent.Action {
	case entity.LightActionToggle:
		handleLightToggle(ctx, index, sysfsCommands, lightEvent)
	case entity.LightActionOn:
		handleLightSet(ctx, index, sysfsCommands, lightEvent, sysfs.OnCommand)
	case entity.LightActionOff:
		handleLightSet(ctx, index, sysfsCommands, lightEvent, sysfs.OffCommand)
	default:
		slog.Error("unsupported light action", "action", lightEvent.Action)
	}
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
