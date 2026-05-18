package controller

import (
	"context"
	"fmt"
	"log/slog"

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
	go normalizeStateChanges(ctx, index, stateChanges, semanticEvents, normalizerDone)

	dispatchEvents(ctx, root, index, sysfsCommands, mqttCommands, mqttTopics, semanticEvents, mqttEvents)
	<-normalizerDone
}

func normalizeStateChanges(
	ctx context.Context,
	index *registry.Index,
	stateChanges <-chan sysfs.StateChange,
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
	mqttEvents <-chan mqtt.Event,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case semanticEvent, ok := <-semanticEvents:
			if !ok {
				return
			}

			dispatchEvent(ctx, index, sysfsCommands, mqttCommands, mqttTopics, semanticEvent)
		case mqttEvent, ok := <-mqttEvents:
			if !ok {
				return
			}

			dispatchMQTTActorEvent(ctx, root, mqttCommands, mqttEvent)
		}
	}
}

func dispatchMQTTActorEvent(ctx context.Context, root *entity.Root, commands chan<- mqtt.Command, event mqtt.Event) {
	logMQTTEvent(event)

	if event.Kind != mqtt.ConnectedEventKind {
		return
	}

	if err := publishMQTTStartup(ctx, root, commands); err != nil {
		slog.Error("mqtt startup publish failed", "error", err)
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

func logMQTTEvent(event mqtt.Event) {
	switch event.Kind {
	case mqtt.ConnectedEventKind:
		slog.Info("mqtt actor connected")
	case mqtt.ConnectFailedKind:
		slog.Error("mqtt actor connect failed", "error", event.Error)
	case mqtt.DisconnectedEventKind:
		slog.Info("mqtt actor disconnected")
	case mqtt.PublishedEventKind:
		slog.Debug("mqtt message published", "topic", event.Publish.Topic)
	case mqtt.PublishFailedKind:
		slog.Error("mqtt message publish failed", "topic", event.Publish.Topic, "error", event.Error)
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
