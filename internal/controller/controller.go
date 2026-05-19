package controller

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func Run(
	ctx context.Context,
	root *entity.Root,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	mqttTopics mqtt.Topics,
	stateChanges <-chan sysfs.StateChange,
	mqttEvents <-chan mqtt.Event,
	done chan<- struct{},
) {
	defer close(done)
	semanticEvents := make(chan event.Event, 32)
	normalizerDone := make(chan struct{})
	go normalizeEvents(ctx, index, mqttTopics, stateChanges, mqttEvents, semanticEvents, normalizerDone)

	dispatchEvents(ctx, root, index, sysfsCommands, mqttCommands, mqttTopics, semanticEvents)
	<-normalizerDone
}

func normalizeEvents(
	ctx context.Context,
	index *registry.Index,
	mqttTopics mqtt.Topics,
	stateChanges <-chan sysfs.StateChange,
	mqttEvents <-chan mqtt.Event,
	semanticEvents chan<- event.Event,
	done chan<- struct{},
) {
	defer close(done)
	defer close(semanticEvents)
	normalizerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	normalizerDone := make(chan struct{}, 2)

	go func() {
		normalizeStateChanges(normalizerCtx, index, stateChanges, semanticEvents)
		normalizerDone <- struct{}{}
	}()
	go func() {
		normalizeMQTTEvents(normalizerCtx, index, mqttTopics, mqttEvents, semanticEvents)
		normalizerDone <- struct{}{}
	}()

	completed := 0
	select {
	case <-ctx.Done():
	case <-normalizerDone:
		completed++
	}
	cancel()
	for completed < 2 {
		<-normalizerDone
		completed++
	}
}

func normalizeStateChanges(
	ctx context.Context,
	index *registry.Index,
	stateChanges <-chan sysfs.StateChange,
	semanticEvents chan<- event.Event,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case stateChange, ok := <-stateChanges:
			if !ok {
				return
			}

			normalizeStateChange(ctx, index, semanticEvents, stateChange)
		}
	}
}

func normalizeMQTTEvents(ctx context.Context, index *registry.Index, mqttTopics mqtt.Topics, mqttEvents <-chan mqtt.Event, semanticEvents chan<- event.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case mqttEvent, ok := <-mqttEvents:
			if !ok {
				return
			}

			semanticEvent, handled := semanticEventFromMQTTEvent(index, mqttTopics, mqttEvent)
			if handled {
				publishSemanticEvent(ctx, semanticEvents, semanticEvent)
			}
		}
	}
}

func dispatchEvents(
	ctx context.Context,
	root *entity.Root,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	mqttTopics mqtt.Topics,
	semanticEvents <-chan event.Event,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case semanticEvent, ok := <-semanticEvents:
			if !ok {
				return
			}

			dispatchEvent(ctx, root, index, sysfsCommands, mqttCommands, mqttTopics, semanticEvent)
		}
	}
}

func publishMQTTStartup(ctx context.Context, root *entity.Root, commands chan<- mqtt.Command) error {
	startupCommands, err := mqtt.StartupCommands(root)
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

func semanticEventFromMQTTEvent(index *registry.Index, mqttTopics mqtt.Topics, mqttEvent mqtt.Event) (event.Event, bool) {
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
		return semanticLightEventFromMQTTMessage(index, mqttTopics, mqttEvent.Message)
	default:
		return event.Event{}, false
	}

	return semanticEvent, true
}

func semanticLightEventFromMQTTMessage(index *registry.Index, mqttTopics mqtt.Topics, message mqtt.ReceivedMessage) (event.Event, bool) {
	if message.Retained {
		slog.Warn("ignoring retained mqtt light command", "topic", message.Topic)
		return event.Event{}, false
	}

	lightID, ok := mqtt.ParseLightCommandTopic(mqttTopics, message.Topic)
	if !ok {
		slog.Warn("unhandled mqtt command topic", "topic", message.Topic)
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

func normalizeStateChange(ctx context.Context, index *registry.Index, semanticEvents chan<- event.Event, stateChange sysfs.StateChange) {
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
