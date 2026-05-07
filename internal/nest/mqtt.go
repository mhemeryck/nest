package nest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
)

func mqttChannels(root *entity.Root) (chan mqtt.Command, chan mqtt.Event, chan struct{}) {
	if !root.MQTT.Enabled {
		return nil, nil, nil
	}

	commands := make(chan mqtt.Command, 32)
	events := make(chan mqtt.Event, 32)
	done := make(chan struct{})

	return commands, events, done
}

func mqttTopics(root *entity.Root) mqtt.Topics {
	if !root.MQTT.Enabled {
		return mqtt.Topics{}
	}

	return mqtt.NewTopics(root.MQTT.TopicPrefix, root.MQTT.UnitID)
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

func logMQTTEvents(ctx context.Context, events <-chan mqtt.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			logMQTTEvent(event)
		}
	}
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
