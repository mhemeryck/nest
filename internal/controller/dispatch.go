package controller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/persistence"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type dispatchHandoffs struct {
	modbus *modbus.Handoff
	mqtt   *mqtt.Handoff
}

type dispatchTargets struct {
	reg            *registry.Registry
	sysfsCommands  chan<- sysfs.Command
	mqttCommands   chan<- mqtt.Command
	modbusCommands chan<- modbus.Command
	mqttTopics     mqtt.Topics
	persistence    *persistence.Store
	handoffs       dispatchHandoffs
}

func startDispatchHandoffs(ctx context.Context, reg *registry.Registry, mqttCommands chan<- mqtt.Command, modbusCommands chan<- modbus.Command) (dispatchHandoffs, func()) {
	handoffContext, cancel := context.WithCancel(ctx)
	handoffs := dispatchHandoffs{
		modbus: modbus.NewHandoff(max(32, len(registry.Modbus(reg).StatePoints))),
		mqtt:   mqtt.NewHandoff(mqttHandoffCapacity(reg)),
	}
	modbusDone := make(chan struct{})
	mqttDone := make(chan struct{})
	go modbus.RunHandoff(handoffContext, handoffs.modbus, modbusCommands, modbusDone)
	go mqtt.RunHandoff(handoffContext, handoffs.mqtt, mqttCommands, mqttDone)
	return handoffs, func() {
		mqtt.FlushAvailable(handoffs.mqtt, mqttCommands)
		cancel()
		<-mqttDone
		modbus.FlushAvailable(handoffs.modbus, modbusCommands)
		<-modbusDone
	}
}

func popEvent(events []event.Event) (event.Event, []event.Event, bool) {
	if len(events) == 0 {
		return event.Event{}, events, false
	}
	return events[0], events[1:], true
}

func dispatchEvent(
	ctx context.Context,
	reg *registry.Registry,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	modbusCommands chan<- modbus.Command,
	mqttTopics mqtt.Topics,
	busEvent event.Event,
	handoffs dispatchHandoffs,
) []event.Event {
	logSemanticEvent(busEvent)
	derivedEvents := bindingEventsFromEvent(reg, busEvent)
	if busEvent.Kind == event.OutputCommandKind {
		derivedEvents = append(derivedEvents, dispatchCoverOutputCommand(ctx, reg, sysfsCommands, *busEvent.OutputCommand)...)
	}
	dispatchSysfsCommand(ctx, reg, sysfsCommands, busEvent)
	dispatchMQTTCommand(ctx, reg, mqttCommands, mqttTopics, busEvent, handoffs.mqtt)
	derivedEvents = append(derivedEvents, dispatchModbusCommand(ctx, reg, modbusCommands, busEvent, handoffs.modbus)...)

	return derivedEvents
}

func dispatchCoverOutputCommand(ctx context.Context, reg *registry.Registry, commands chan<- sysfs.Command, output event.OutputCommand) []event.Event {
	if ctx.Err() != nil {
		return nil
	}
	relay, found := registry.RelayByID(reg, output.RelayID)
	if !found {
		return []event.Event{{Kind: event.OutputResultKind, OutputResult: &event.OutputResult{
			CommandID: output.CommandID, Action: output.Action, CompletedAt: time.Now(), Error: fmt.Errorf("unknown cover relay %q", output.RelayID),
		}}}
	}
	command := sysfs.Command{ID: output.CommandID, DeviceID: string(relay.SysfsDevice), Kind: sysfs.CommandKind(output.Action)}
	if failure := sysfs.AdmitCommand(commands, command); failure != nil {
		return []event.Event{{Kind: event.OutputResultKind, OutputResult: &event.OutputResult{
			CommandID: command.ID, SysfsDevice: relay.SysfsDevice, Action: output.Action, CompletedAt: failure.CompletedAt, Error: failure.Error,
		}}}
	}
	return nil
}

func dispatchSysfsCommand(ctx context.Context, index *registry.Registry, commands chan<- sysfs.Command, busEvent event.Event) {
	switch busEvent.Kind {
	case event.LightKind:
		dispatchLightEvent(ctx, index, commands, *busEvent.Light)
	}
}

func dispatchMQTTCommand(ctx context.Context, reg *registry.Registry, commands chan<- mqtt.Command, topics mqtt.Topics, busEvent event.Event, handoff *mqtt.Handoff) {
	if commands == nil {
		return
	}
	switch busEvent.Kind {
	case event.CoverKind:
		if _, local := registry.CoverByID(reg, busEvent.Cover.CoverID); local {
			return
		}
		message, err := mqtt.CoverCommandMessage(topics, busEvent.Cover.CoverID, busEvent.Cover.Action)
		if err != nil {
			slog.Error("build remote cover command failed", "error", err)
			return
		}
		publishMQTT(ctx, commands, message, handoff)
	case event.PushButtonPressedKind, event.PushButtonReleasedKind:
		publishPushButtonSourceEvent(ctx, reg, commands, topics, busEvent.Kind, *busEvent.PushButton, handoff)
	case event.LightStateKind:
		publishLightState(ctx, commands, topics, *busEvent.LightState, handoff)
	case event.MQTTConnectedKind:
		if err := publishMQTTStartup(ctx, reg, commands, handoff); err != nil {
			slog.Error("mqtt startup publish failed", "error", err)
		}
	case event.CoverObservationKind, event.CoverStoppedKind, event.CoverStartIntentKind:
		observation := busEvent.CoverObservation
		message, err := mqtt.CoverStateMessage(topics, mqtt.CoverObservation{CoverID: observation.CoverID, State: observation.State, EstimatedPosition: observation.EstimatedPosition, Available: observation.Available})
		if err != nil {
			slog.Error("build mqtt cover observation failed", "error", err)
			return
		}
		publishMQTT(ctx, commands, message, handoff)
	}
}

func dispatchModbusCommand(ctx context.Context, reg *registry.Registry, commands chan<- modbus.Command, busEvent event.Event, handoff *modbus.Handoff) []event.Event {
	if commands == nil {
		return nil
	}

	switch busEvent.Kind {
	case event.PushButtonPressedKind:
		return dispatchModbusEventSignalWrites(ctx, reg, commands, *busEvent.PushButton, handoff)
	case event.LightStateKind:
		return dispatchModbusStatePoints(ctx, reg, commands, *busEvent.LightState, handoff)
	}
	return nil
}

func dispatchModbusEventSignalWrites(ctx context.Context, reg *registry.Registry, commands chan<- modbus.Command, pushButton event.PushButton, handoff *modbus.Handoff) []event.Event {
	var failures []event.Event
	for _, write := range registry.ModbusEventSignalWritesBySource(reg, entity.ID(pushButton.ButtonID)) {
		failures = append(failures, submitModbusCommand(ctx, commands, modbus.WriteCoilCommand(write.UnitID, write.Coil, true), handoff)...)
	}
	return failures
}

func dispatchModbusStatePoints(ctx context.Context, reg *registry.Registry, commands chan<- modbus.Command, lightState event.LightState, handoff *modbus.Handoff) []event.Event {
	var failures []event.Event
	for _, point := range registry.ModbusStatePointsByEntity(reg, entity.ID(lightState.LightID)) {
		failures = append(failures, submitModbusCommand(ctx, commands, modbus.SetCoilStateCommand(uint16(point.Coil), lightState.Value != 0), handoff)...)
	}
	return failures
}

func submitModbusCommand(ctx context.Context, commands chan<- modbus.Command, command modbus.Command, handoff *modbus.Handoff) []event.Event {
	if ctx.Err() != nil {
		return nil
	}
	errorMessage := ""
	if handoff != nil {
		if failure := modbus.QueueCommand(handoff, command); failure != nil {
			errorMessage = failure.Error
		}
	} else {
		select {
		case commands <- command:
		default:
			errorMessage = "command admission exhausted"
		}
	}
	if errorMessage == "" {
		return nil
	}
	slog.Error("modbus integration failure", "unit_id", command.UnitID, "coil", command.Coil, "error", errorMessage)
	return []event.Event{{Kind: event.IntegrationFailureKind, IntegrationFailure: &event.IntegrationFailure{Integration: "modbus", Error: errorMessage}}}
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
