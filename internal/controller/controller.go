package controller

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func Run(
	ctx context.Context,
	reg *registry.Registry,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	modbusCommands chan<- modbus.Command,
	mqttTopics mqtt.Topics,
	stateChanges <-chan sysfs.StateChange,
	mqttEvents <-chan mqtt.Event,
	modbusEvents <-chan modbus.Event,
	done chan<- struct{},
) {
	defer close(done)
	semanticEvents := make(chan event.Event, 32)
	normalizerDone := make(chan struct{})
	go normalizeEvents(ctx, reg, mqttTopics, stateChanges, mqttEvents, modbusEvents, semanticEvents, normalizerDone)

	dispatchEvents(ctx, reg, sysfsCommands, mqttCommands, modbusCommands, mqttTopics, semanticEvents)
	<-normalizerDone
}

func normalizeEvents(
	ctx context.Context,
	index *registry.Registry,
	mqttTopics mqtt.Topics,
	stateChanges <-chan sysfs.StateChange,
	mqttEvents <-chan mqtt.Event,
	modbusEvents <-chan modbus.Event,
	semanticEvents chan<- event.Event,
	done chan<- struct{},
) {
	defer close(done)
	defer close(semanticEvents)
	for {
		select {
		case <-ctx.Done():
			return
		case stateChange, ok := <-stateChanges:
			if !ok {
				return
			}

			normalizeStateChange(ctx, index, semanticEvents, stateChange)
		case mqttEvent, ok := <-mqttEvents:
			if !ok {
				return
			}

			semanticEvent, handled := semanticEventFromMQTTEvent(index, mqttTopics, mqttEvent)
			if handled {
				publishSemanticEvent(ctx, semanticEvents, semanticEvent)
			}
		case modbusEvent, ok := <-modbusEvents:
			if !ok {
				modbusEvents = nil
				continue
			}

			semanticEvent, handled := semanticEventFromModbusEvent(index, modbusEvent)
			if handled {
				publishSemanticEvent(ctx, semanticEvents, semanticEvent)
			}
		}
	}
}

func dispatchEvents(
	ctx context.Context,
	reg *registry.Registry,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	modbusCommands chan<- modbus.Command,
	mqttTopics mqtt.Topics,
	semanticEvents <-chan event.Event,
) {
	var dispatchQueue []event.Event
	semanticEventsOpen := true

	for {
		if semanticEvent, remainingEvents, ok := popEvent(dispatchQueue); ok {
			dispatchQueue = append(remainingEvents, dispatchEvent(ctx, reg, sysfsCommands, mqttCommands, modbusCommands, mqttTopics, semanticEvent)...)
			continue
		}

		if !semanticEventsOpen {
			return
		}

		select {
		case <-ctx.Done():
			return
		case semanticEvent, ok := <-semanticEvents:
			if !ok {
				semanticEventsOpen = false
				continue
			}

			dispatchQueue = append(dispatchQueue, semanticEvent)
		}
	}
}

func popEvent(events []event.Event) (event.Event, []event.Event, bool) {
	if len(events) == 0 {
		return event.Event{}, events, false
	}

	return events[0], events[1:], true
}

func publishMQTTStartup(ctx context.Context, reg *registry.Registry, commands chan<- mqtt.Command) error {
	startupCommands, err := mqtt.StartupCommands(registry.MQTT(reg), registry.Lights(reg))
	if err != nil {
		return fmt.Errorf("build mqtt startup commands: %w", err)
	}

	for _, command := range startupCommands {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case commands <- command:
		}
	}

	return nil
}

func semanticEventFromMQTTEvent(index *registry.Registry, mqttTopics mqtt.Topics, mqttEvent mqtt.Event) (event.Event, bool) {
	semanticEvent := event.Event{MQTT: &event.MQTT{PublishTopic: mqttEvent.Publish.Topic, Error: mqttEvent.Error}}
	switch mqttEvent.Kind {
	case mqtt.ConnectedEventKind:
		semanticEvent.Kind = event.MQTTConnectedKind
	case mqtt.ConnectFailedKind:
		semanticEvent.Kind = event.MQTTConnectFailedKind
	case mqtt.DisconnectedEventKind:
		semanticEvent.Kind = event.MQTTDisconnectedKind
	case mqtt.PublishedEventKind:
		semanticEvent.Kind = event.MQTTPublishedKind
	case mqtt.PublishFailedKind:
		semanticEvent.Kind = event.MQTTPublishFailedKind
	case mqtt.ReceivedEventKind:
		return semanticEventFromMQTTMessage(index, mqttTopics, mqttEvent.Message)
	default:
		return event.Event{}, false
	}

	return semanticEvent, true
}

func semanticEventFromMQTTMessage(index *registry.Registry, mqttTopics mqtt.Topics, message mqtt.ReceivedMessage) (event.Event, bool) {
	if semanticEvent, ok := semanticLightEventFromMQTTMessage(index, mqttTopics, message); ok {
		return semanticEvent, true
	}

	return semanticSourceEventFromMQTTMessage(index, mqttTopics, message)
}

func semanticLightEventFromMQTTMessage(index *registry.Registry, mqttTopics mqtt.Topics, message mqtt.ReceivedMessage) (event.Event, bool) {
	lightID, ok := mqtt.ParseLightCommandTopic(mqttTopics, message.Topic)
	if !ok {
		return event.Event{}, false
	}

	action, ok := lightActionFromMQTTPayload(message.Payload)
	if !ok {
		slog.Warn("invalid mqtt light command payload", "topic", message.Topic, "payload", string(bytes.TrimSpace(message.Payload)))
		return event.Event{}, false
	}

	light, ok := registry.LightByID(index, lightID)
	if !ok {
		slog.Error("mqtt command references unknown light", "topic", message.Topic, "light_id", lightID)
		return event.Event{}, false
	}

	return event.Event{
		Kind: event.LightKind,
		Light: &event.Light{
			LightID: light.ID,
			Name:    light.Name,
			Action:  action,
		},
	}, true
}

func semanticSourceEventFromMQTTMessage(index *registry.Registry, mqttTopics mqtt.Topics, message mqtt.ReceivedMessage) (event.Event, bool) {
	sourceEvent, ok := mqtt.ParseSemanticSourceEventMessage(message, mqttTopics.Prefix)
	if !ok {
		slog.Warn("unhandled mqtt message topic", "topic", message.Topic)
		return event.Event{}, false
	}
	semanticEvent, handled := semanticEventFromSourceEvent(index, sourceEvent.SourceID, sourceEvent.Event, entity.ExecutionTransportMQTT)
	if !handled && len(registry.RemoteTargetBindingsBySource(index, sourceEvent.SourceID)) == 0 {
		slog.Warn("mqtt source event has no target-local binding", "source_id", sourceEvent.SourceID)
	}

	return semanticEvent, handled
}

func semanticEventFromModbusEvent(index *registry.Registry, modbusEvent modbus.Event) (event.Event, bool) {
	switch modbusEvent.Kind {
	case modbus.WriteFailedEventKind:
		slog.Error("modbus coil write failed", "unit_id", modbusEvent.UnitID, "coil", modbusEvent.Coil, "error", modbusEvent.Error)
		return event.Event{}, false
	case modbus.ReadFailedEventKind:
		slog.Error("modbus coil read failed", "unit_id", modbusEvent.UnitID, "coil", modbusEvent.Coil, "error", modbusEvent.Error)
		return event.Event{}, false
	case modbus.CoilReadEventKind:
		poll, ok := registry.ModbusStatePollByCoil(index, modbusEvent.UnitID, modbusEvent.Coil)
		if !ok {
			return event.Event{}, false
		}
		value := 0
		if modbusEvent.Value {
			value = 1
		}
		return event.Event{
			Kind: event.LightStateKind,
			LightState: &event.LightState{
				LightID: entity.LightID(poll.Entity),
				Value:   value,
			},
		}, true
	case modbus.WriteSucceededEventKind:
		if !modbusEvent.Value {
			return event.Event{}, false
		}
	default:
		return event.Event{}, false
	}

	signal, ok := registry.ModbusEventSignalByCoil(index, modbusEvent.Coil)
	if !ok {
		return event.Event{}, false
	}

	// Initial Modbus event signals represent remote button presses.
	return semanticEventFromSourceEvent(index, signal.Source, "pressed", entity.ExecutionTransportModbus)
}

func semanticEventFromSourceEvent(index *registry.Registry, sourceID entity.ID, sourceEvent string, delivery entity.ExecutionTransport) (event.Event, bool) {
	if len(remoteTargetBindingsBySourceAndTransport(index, sourceID, delivery)) == 0 {
		return event.Event{}, false
	}

	semanticEvent := event.Event{
		PushButton: &event.PushButton{
			ButtonID: entity.PushButtonID(sourceID),
			Delivery: delivery,
		},
	}
	switch sourceEvent {
	case "pressed":
		semanticEvent.Kind = event.PushButtonPressedKind
	case "released":
		semanticEvent.Kind = event.PushButtonReleasedKind
	default:
		return event.Event{}, false
	}

	return semanticEvent, true
}

func lightActionFromMQTTPayload(payload []byte) (entity.LightAction, bool) {
	switch strings.ToUpper(string(bytes.TrimSpace(payload))) {
	case "ON":
		return entity.LightActionOn, true
	case "OFF":
		return entity.LightActionOff, true
	default:
		return "", false
	}
}

func normalizeStateChange(ctx context.Context, index *registry.Registry, semanticEvents chan<- event.Event, stateChange sysfs.StateChange) {
	events, handled := semanticEventsFromStateChange(index, stateChange)
	if handled {
		for _, semanticEvent := range events {
			if !publishSemanticEvent(ctx, semanticEvents, semanticEvent) {
				return
			}
		}
		return
	}

	logUnmappedStateChange(stateChange)
}

func logUnmappedStateChange(stateChange sysfs.StateChange) {
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
		"initial",
		stateChange.Initial,
	)
}

func publishSemanticEvent(ctx context.Context, semanticEvents chan<- event.Event, semanticEvent event.Event) bool {
	select {
	case <-ctx.Done():
		return false
	case semanticEvents <- semanticEvent:
		return true
	}
}
