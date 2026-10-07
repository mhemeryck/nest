package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
)

func publishPushButtonSourceEvent(ctx context.Context, index *registry.Registry, commands chan<- mqtt.Command, topics mqtt.Topics, eventKind event.Kind, pushButton event.PushButton, handoff *mqtt.Handoff) {
	if len(registry.RemoteSourceBindingsByButton(index, pushButton.ButtonID)) == 0 {
		return
	}

	message, err := mqtt.SemanticSourceEventMessage(topics, mqtt.SemanticSourceEventObservation{
		SourceID: entity.ID(pushButton.ButtonID),
		Event:    pushButtonSourceEventName(eventKind),
	})
	if err != nil {
		slog.Error("build mqtt source event failed", "source_id", pushButton.ButtonID, "error", err)
		return
	}

	publishMQTT(ctx, commands, message, handoff)
}

func publishLightState(ctx context.Context, commands chan<- mqtt.Command, topics mqtt.Topics, light event.LightState, handoff *mqtt.Handoff) {
	message, err := mqtt.LightStateMessage(topics, mqtt.LightObservation{
		LightID: light.LightID,
		State:   lightState(light.Value),
	})
	if err != nil {
		slog.Error("build mqtt light state failed", "light_id", light.LightID, "error", err)
		return
	}

	publishMQTT(ctx, commands, message, handoff)
}

func publishMQTT(ctx context.Context, commands chan<- mqtt.Command, message mqtt.PublishMessage, handoff *mqtt.Handoff) bool {
	if commands == nil {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	if handoff != nil {
		if err := mqtt.QueueMessage(handoff, message); err != nil {
			slog.Error("mqtt integration failure", "error", err)
			return false
		}
		return true
	}

	select {
	case <-ctx.Done():
		return false
	case commands <- mqtt.PublishCommand(message):
		return true
	default:
		slog.Warn("mqtt publish command dropped", "topic", message.Topic)
		return false
	}
}

func lightState(value int) string {
	if value == 1 {
		return "ON"
	}

	return "OFF"
}

func pushButtonSourceEventName(eventKind event.Kind) string {
	if eventKind == event.PushButtonPressedKind {
		return "pressed"
	}

	return "released"
}
