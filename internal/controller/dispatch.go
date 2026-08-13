package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func dispatchEvent(
	ctx context.Context,
	reg *registry.Registry,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	modbusCommands chan<- modbus.Command,
	mqttTopics mqtt.Topics,
	busEvent event.Event,
) []event.Event {
	logSemanticEvent(busEvent)
	derivedEvents := bindingEventsFromEvent(reg, busEvent)
	dispatchSysfsCommand(ctx, reg, sysfsCommands, busEvent)
	dispatchMQTTCommand(ctx, reg, mqttCommands, mqttTopics, busEvent)
	dispatchModbusCommand(ctx, reg, modbusCommands, busEvent)

	return derivedEvents
}

func dispatchSysfsCommand(ctx context.Context, index *registry.Registry, commands chan<- sysfs.Command, busEvent event.Event) {
	switch busEvent.Kind {
	case event.LightKind:
		dispatchLightEvent(ctx, index, commands, *busEvent.Light)
	}
}

func dispatchMQTTCommand(ctx context.Context, reg *registry.Registry, commands chan<- mqtt.Command, topics mqtt.Topics, busEvent event.Event) {
	switch busEvent.Kind {
	case event.PushButtonPressedKind, event.PushButtonReleasedKind:
		publishPushButtonSourceEvent(ctx, reg, commands, topics, busEvent.Kind, *busEvent.PushButton)
	case event.LightStateKind:
		publishLightState(ctx, commands, topics, *busEvent.LightState)
	case event.MQTTConnectedKind:
		if err := publishMQTTStartup(ctx, reg, commands); err != nil {
			slog.Error("mqtt startup publish failed", "error", err)
		}
	}
}

func dispatchModbusCommand(ctx context.Context, reg *registry.Registry, commands chan<- modbus.Command, busEvent event.Event) {
	if commands == nil {
		return
	}

	switch busEvent.Kind {
	case event.PushButtonPressedKind:
		dispatchModbusEventSignalWrites(ctx, reg, commands, *busEvent.PushButton)
	case event.LightStateKind:
		dispatchModbusStatePoints(ctx, reg, commands, *busEvent.LightState)
	}
}

func dispatchModbusEventSignalWrites(ctx context.Context, reg *registry.Registry, commands chan<- modbus.Command, pushButton event.PushButton) {
	for _, write := range registry.ModbusEventSignalWritesBySource(reg, entity.ID(pushButton.ButtonID)) {
		select {
		case <-ctx.Done():
			return
		case commands <- modbus.WriteCoilCommand(write.UnitID, write.Coil, true):
		}
	}
}

func dispatchModbusStatePoints(ctx context.Context, reg *registry.Registry, commands chan<- modbus.Command, lightState event.LightState) {
	for _, point := range registry.ModbusStatePointsByEntity(reg, entity.ID(lightState.LightID)) {
		select {
		case <-ctx.Done():
			return
		case commands <- modbus.SetCoilStateCommand(uint16(point.Coil), lightState.Value != 0):
		}
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

func dispatchLightEvent(ctx context.Context, index *registry.Registry, sysfsCommands chan<- sysfs.Command, lightEvent event.Light) {
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
