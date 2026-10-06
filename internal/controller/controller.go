package controller

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/persistence"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type RuntimeOptions struct {
	Persistence       *persistence.Store
	RestoredPositions map[entity.CoverID]*float64
	FeedbackContext   context.Context
}

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
	options ...RuntimeOptions,
) {
	defer close(done)
	var runtimeOptions RuntimeOptions
	if len(options) > 0 {
		runtimeOptions = options[0]
	}
	feedbackContext := runtimeOptions.FeedbackContext
	if feedbackContext == nil {
		feedbackContext = context.WithoutCancel(ctx)
	}
	normalizerContext, cancelNormalizer := context.WithCancel(feedbackContext)
	defer cancelNormalizer()
	runtimeOptions.FeedbackContext = normalizerContext
	semanticEvents := make(chan event.Event, 32)
	normalizerDone := make(chan struct{})
	go normalizeEvents(normalizerContext, reg, mqttTopics, stateChanges, mqttEvents, modbusEvents, semanticEvents, normalizerDone)

	dispatchEvents(ctx, reg, sysfsCommands, mqttCommands, modbusCommands, mqttTopics, semanticEvents, runtimeOptions)
	cancelNormalizer()
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
				mqttEvents = nil
				continue
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
	options ...RuntimeOptions,
) {
	var runtimeOptions RuntimeOptions
	if len(options) > 0 {
		runtimeOptions = options[0]
	}
	feedbackContext := runtimeOptions.FeedbackContext
	if feedbackContext == nil {
		feedbackContext = context.WithoutCancel(ctx)
	}
	handoffContext, cancelHandoff := context.WithCancel(feedbackContext)
	defer cancelHandoff()
	modbusCapacity := max(32, len(registry.Modbus(reg).StatePoints))
	handoff := modbus.NewHandoff(modbusCapacity)
	handoffDone := make(chan struct{})
	go modbus.RunHandoff(handoffContext, handoff, modbusCommands, handoffDone)
	defer func() { modbus.FlushAvailable(handoff, modbusCommands); cancelHandoff(); <-handoffDone }()
	mqttHandoff := mqtt.NewHandoff(max(32, len(registry.Covers(reg))+len(registry.Lights(reg))+2))
	mqttHandoffDone := make(chan struct{})
	go mqtt.RunHandoff(handoffContext, mqttHandoff, mqttCommands, mqttHandoffDone)
	defer func() { mqtt.FlushAvailable(mqttHandoff, mqttCommands); cancelHandoff(); <-mqttHandoffDone }()
	covers := newCoverController(reg, runtimeOptions.RestoredPositions)
	dispatchQueue := initializeCovers(covers, time.Now())
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	semanticEventsOpen := true
	shutdownSignal := ctx.Done()
	var shutdownDeadline time.Time

	for {
		now := time.Now()
		select {
		case <-shutdownSignal:
			shutdownSignal = nil
			shutdownDeadline = now.Add(covers.settings.ShutdownPeriod)
			dispatchQueue = append(beginCoverShutdown(covers, now), dispatchQueue...)
		default:
		}
		if covers.shuttingDown && !now.Before(shutdownDeadline) {
			return
		}
		due := processCoverDeadlines(covers, time.Now())
		dispatchQueue = append(due, dispatchQueue...)
		if semanticEvent, remainingEvents, ok := popEvent(dispatchQueue); ok {
			if semanticEvent.Kind == event.OutputCommandKind && !currentCoverCommand(covers, *semanticEvent.OutputCommand) {
				dispatchQueue = remainingEvents
				continue
			}
			derived := handleCoverEvent(covers, semanticEvent, time.Now())
			derived = append(derived, dispatchPersistenceEvent(runtimeOptions.Persistence, semanticEvent)...)
			derived = append(derived, dispatchEvent(feedbackContext, reg, sysfsCommands, mqttCommands, modbusCommands, mqttTopics, semanticEvent, dispatchHandoffs{modbus: handoff, mqtt: mqttHandoff})...)
			dispatchQueue = append(remainingEvents, derived...)
			continue
		}

		if covers.shuttingDown && coverOutputsStopped(covers) {
			dispatchPersistenceEvent(runtimeOptions.Persistence, event.Event{Kind: event.SessionStoppedKind})
			if runtimeOptions.Persistence != nil {
				flushContext, cancelFlush := context.WithDeadline(feedbackContext, shutdownDeadline)
				if !persistence.FlushWithin(flushContext, runtimeOptions.Persistence) {
					slog.Error("persistence shutdown flush incomplete")
				}
				cancelFlush()
			}
			return
		}
		if !semanticEventsOpen && !covers.shuttingDown {
			shutdownSignal = nil
			shutdownDeadline = time.Now().Add(covers.settings.ShutdownPeriod)
			dispatchQueue = beginCoverShutdown(covers, time.Now())
			continue
		}

		var deadline <-chan time.Time
		next := nextCoverDeadline(covers)
		if covers.shuttingDown && (next.IsZero() || shutdownDeadline.Before(next)) {
			next = shutdownDeadline
		}
		if !next.IsZero() {
			timer.Reset(max(0, time.Until(next)))
			deadline = timer.C
		}
		select {
		case <-shutdownSignal:
			shutdownSignal = nil
			shutdownDeadline = time.Now().Add(covers.settings.ShutdownPeriod)
			dispatchQueue = beginCoverShutdown(covers, time.Now())
		case <-deadline:
		case semanticEvent, ok := <-semanticEvents:
			if !ok {
				semanticEventsOpen = false
				semanticEvents = nil
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

func publishMQTTStartup(ctx context.Context, reg *registry.Registry, commands chan<- mqtt.Command, handoffs ...*mqtt.Handoff) error {
	startupCommands, err := mqtt.StartupCommands(registry.MQTT(reg), registry.Lights(reg), registry.Covers(reg))
	if err != nil {
		return fmt.Errorf("build mqtt startup commands: %w", err)
	}

	for _, command := range startupCommands {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !publishMQTT(ctx, commands, command.Publish, handoffs...) {
			return fmt.Errorf("mqtt startup handoff rejected")
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
	if _, isCoverCommand := mqtt.ParseCoverCommandTopic(mqttTopics, message.Topic); isCoverCommand {
		return semanticCoverEventFromMQTTMessage(index, mqttTopics, message)
	}
	if semanticEvent, ok := semanticLightEventFromMQTTMessage(index, mqttTopics, message); ok {
		return semanticEvent, true
	}

	return semanticSourceEventFromMQTTMessage(index, mqttTopics, message)
}

func semanticCoverEventFromMQTTMessage(index *registry.Registry, topics mqtt.Topics, message mqtt.ReceivedMessage) (event.Event, bool) {
	id, ok := mqtt.ParseCoverCommandTopic(topics, message.Topic)
	if !ok {
		return event.Event{}, false
	}
	cover, ok := registry.CoverByID(index, id)
	if !ok {
		slog.Warn("mqtt command references unknown cover", "cover_id", id)
		return event.Event{}, false
	}
	var action entity.CoverAction
	switch strings.ToUpper(string(bytes.TrimSpace(message.Payload))) {
	case "OPEN":
		action = entity.CoverActionOpen
	case "CLOSE":
		action = entity.CoverActionClose
	case "STOP":
		action = entity.CoverActionStop
	default:
		slog.Warn("invalid mqtt cover command", "cover_id", id)
		return event.Event{}, false
	}
	if message.Retained && action != entity.CoverActionStop {
		return event.Event{}, false
	}
	return event.Event{Kind: event.CoverKind, Cover: &event.Cover{CoverID: cover.ID, Name: cover.Name, Action: action}}, true
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
