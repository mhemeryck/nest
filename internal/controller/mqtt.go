package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/mqtt"
)

func publishDigitalInputState(ctx context.Context, commands chan<- mqtt.Command, topics mqtt.Topics, input event.DigitalInput) {
	message, err := mqtt.DigitalInputStateMessage(topics, mqtt.DigitalInputObservation{
		InputID:     input.InputID,
		SysfsDevice: input.SysfsDevice,
		Value:       input.Value,
	})
	if err != nil {
		slog.Error("build mqtt digital input state failed", "input_id", input.InputID, "error", err)
		return
	}

	publishMQTT(ctx, commands, message)
}

func publishPushButtonState(
	ctx context.Context,
	commands chan<- mqtt.Command,
	topics mqtt.Topics,
	eventKind event.Kind,
	pushButton event.PushButton,
) {
	message, err := mqtt.PushButtonStateMessage(topics, mqtt.PushButtonObservation{
		ButtonID: pushButton.ButtonID,
		Name:     pushButton.Name,
		State:    pushButtonState(eventKind),
	})
	if err != nil {
		slog.Error("build mqtt push button state failed", "button_id", pushButton.ButtonID, "error", err)
		return
	}

	publishMQTT(ctx, commands, message)
}

func publishRelayState(ctx context.Context, commands chan<- mqtt.Command, topics mqtt.Topics, relay event.Relay) {
	message, err := mqtt.RelayStateMessage(topics, mqtt.RelayObservation{
		RelayID:     relay.RelayID,
		Name:        relay.Name,
		SysfsDevice: relay.SysfsDevice,
		Value:       relay.Value,
	})
	if err != nil {
		slog.Error("build mqtt relay state failed", "relay_id", relay.RelayID, "error", err)
		return
	}

	publishMQTT(ctx, commands, message)
}

func publishLightState(ctx context.Context, commands chan<- mqtt.Command, topics mqtt.Topics, light event.LightState) {
	message, err := mqtt.LightStateMessage(topics, mqtt.LightObservation{
		LightID: light.LightID,
		Name:    light.Name,
		RelayID: light.RelayID,
		Value:   light.Value,
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

func pushButtonState(eventKind event.Kind) string {
	if eventKind == event.PushButtonPressedKind {
		return "pressed"
	}

	return "released"
}
