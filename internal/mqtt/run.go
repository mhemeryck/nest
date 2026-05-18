package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/mhemeryck/nest/internal/entity"
)

const (
	disconnectQuiesce      = 250 * time.Millisecond
	gracefulOfflineTimeout = 2 * time.Second
)

func Run(ctx context.Context, cfg entity.MQTT, commands <-chan Command, events chan<- Event, done chan<- struct{}) {
	defer close(done)

	client := paho.NewClient(clientOptions(ctx, cfg, events))
	if err := waitToken(ctx, client.Connect()); err != nil {
		slog.Error("mqtt connect failed", "broker", brokerURL(cfg), "error", err)
		publishEvent(ctx, events, ConnectFailedEvent(err))
		return
	}

	defer func() {
		offlineCtx, cancel := context.WithTimeout(context.Background(), gracefulOfflineTimeout)
		defer cancel()
		publishGracefulOffline(offlineCtx, client, cfg)
		client.Disconnect(uint(disconnectQuiesce / time.Millisecond))
		publishEvent(ctx, events, DisconnectedEvent())
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case command, ok := <-commands:
			if !ok {
				return
			}
			handleCommand(ctx, client, events, command)
		}
	}
}

func handleCommand(ctx context.Context, client paho.Client, events chan<- Event, command Command) {
	switch command.Kind {
	case PublishCommandKind:
		publish(ctx, client, events, command.Publish)
	default:
		return
	}
}

func publish(ctx context.Context, client paho.Client, events chan<- Event, message PublishMessage) {
	token := client.Publish(message.Topic, message.QoS, message.Retain, message.Payload)
	if err := waitToken(ctx, token); err != nil {
		slog.Error("mqtt publish failed", "topic", message.Topic, "error", err)
		publishEvent(ctx, events, PublishFailedEvent(message, err))
		return
	}

	publishEvent(ctx, events, PublishedEvent(message))
}

func clientOptions(ctx context.Context, cfg entity.MQTT, events chan<- Event) *paho.ClientOptions {
	options := paho.NewClientOptions()
	options.AddBroker(brokerURL(cfg))
	options.SetClientID(cfg.ClientID)
	offline := AvailabilityMessage(NewTopics(cfg.TopicPrefix, cfg.UnitID), AvailabilityOffline)
	options.SetWill(offline.Topic, string(offline.Payload), offline.QoS, offline.Retain)
	options.SetOnConnectHandler(func(_ paho.Client) {
		slog.Info("mqtt connected", "broker", brokerURL(cfg), "client_id", cfg.ClientID)
		publishEvent(ctx, events, ConnectedEvent())
	})
	if cfg.Username != "" {
		options.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		options.SetPassword(cfg.Password)
	}
	options.SetAutoReconnect(true)
	options.SetConnectRetry(true)
	options.SetOrderMatters(true)

	return options
}

func publishGracefulOffline(ctx context.Context, client paho.Client, cfg entity.MQTT) {
	message := AvailabilityMessage(NewTopics(cfg.TopicPrefix, cfg.UnitID), AvailabilityOffline)
	token := client.Publish(message.Topic, message.QoS, message.Retain, message.Payload)
	if err := waitToken(ctx, token); err != nil {
		slog.Error("mqtt graceful offline publish failed", "topic", message.Topic, "error", err)
		return
	}

	slog.Info("mqtt graceful offline published", "topic", message.Topic)
}

func brokerURL(cfg entity.MQTT) string {
	return fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)
}

func waitToken(ctx context.Context, token paho.Token) error {
	done := make(chan struct{})
	go func() {
		token.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return token.Error()
	}
}

func publishEvent(ctx context.Context, events chan<- Event, event Event) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}
