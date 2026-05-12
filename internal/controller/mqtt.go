package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/mqtt"
)

func publishLightState(ctx context.Context, commands chan<- mqtt.Command, topics mqtt.Topics, light event.LightState) {
	message, err := mqtt.LightStateMessage(topics, mqtt.LightObservation{
		LightID: light.LightID,
		State:   lightState(light.Value),
	})
	if err != nil {
		slog.Error("build mqtt light state failed", "light_id", light.LightID, "error", err)
		return
	}

	publishMQTT(ctx, commands, message)
}

func publishMQTT(ctx context.Context, commands chan<- mqtt.Command, message mqtt.PublishMessage) bool {
	if commands == nil {
		return false
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
