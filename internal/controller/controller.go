package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func Run(
	ctx context.Context,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	mqttTopics mqtt.Topics,
	stateChanges <-chan sysfs.StateChange,
	done chan<- struct{},
) {
	defer close(done)
	semanticEvents := make(chan event.Event, 32)
	normalizerDone := make(chan struct{})
	go normalizeStateChanges(ctx, index, stateChanges, semanticEvents, normalizerDone)

	dispatchEvents(ctx, index, sysfsCommands, mqttCommands, mqttTopics, semanticEvents)
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

			dispatchEvent(ctx, index, sysfsCommands, mqttCommands, mqttTopics, semanticEvent)
		}
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
