package mqtt

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/mhemeryck/nest/internal/entity"
)

type client interface {
	Publish(topic string, qos byte, retained bool, payload any) paho.Token
	Subscribe(topic string, qos byte, callback paho.MessageHandler) paho.Token
}

const (
	disconnectQuiesce      = 250 * time.Millisecond
	gracefulOfflineTimeout = 2 * time.Second
)

func Run(ctx context.Context, cfg entity.MQTT, sourceEventTopics []string, commands <-chan Command, events chan<- Event, done chan<- struct{}) {
	defer close(done)

	topics := NewTopics(cfg.TopicPrefix, cfg.UnitID)
	client := paho.NewClient(clientOptions(ctx, cfg, topics, sourceEventTopics, events))
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

func handleCommand(ctx context.Context, client client, events chan<- Event, command Command) {
	switch command.Kind {
	case PublishCommandKind:
		publish(ctx, client, events, command.Publish)
	default:
		return
	}
}

func publish(ctx context.Context, client client, events chan<- Event, message PublishMessage) {
	token := client.Publish(message.Topic, message.QoS, message.Retain, message.Payload)
	if err := waitToken(ctx, token); err != nil {
		slog.Error("mqtt publish failed", "topic", message.Topic, "error", err)
		publishEvent(ctx, events, PublishFailedEvent(message, err))
		return
	}

	publishEvent(ctx, events, PublishedEvent(message))
}

func clientOptions(ctx context.Context, cfg entity.MQTT, topics Topics, sourceEventTopics []string, events chan<- Event) *paho.ClientOptions {
	options := paho.NewClientOptions()
	options.AddBroker(brokerURL(cfg))
	options.SetClientID(cfg.ClientID)
	offline := AvailabilityMessage(topics, AvailabilityOffline)
	options.SetWill(offline.Topic, string(offline.Payload), offline.QoS, offline.Retain)
	options.SetOnConnectHandler(func(client paho.Client) {
		if err := subscribeLightCommands(ctx, client, topics, events); err != nil {
			slog.Error("mqtt subscribe failed", "topic", LightCommandSubscriptionTopic(topics), "error", err)
		}
		if err := subscribeSourceEvents(ctx, client, sourceEventTopics, events); err != nil {
			slog.Error("mqtt source event subscribe failed", "error", err)
		}
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

func subscribeLightCommands(ctx context.Context, client client, topics Topics, events chan<- Event) error {
	if client == nil {
		return nil
	}

	topic := LightCommandSubscriptionTopic(topics)
	token := client.Subscribe(topic, 0, func(_ paho.Client, message paho.Message) {
		if message.Retained() {
			slog.Warn("ignoring retained mqtt light command", "topic", message.Topic())
			return
		}

		publishEvent(ctx, events, ReceivedEvent(ReceivedMessage{
			Topic:   message.Topic(),
			Payload: bytes.Clone(message.Payload()),
		}))
	})
	if err := waitToken(ctx, token); err != nil {
		return err
	}

	slog.Info("mqtt subscribed", "topic", topic)
	return nil
}

func subscribeSourceEvents(ctx context.Context, client client, topics []string, events chan<- Event) error {
	if client == nil {
		return nil
	}

	for _, topic := range topics {
		token := client.Subscribe(topic, 0, func(_ paho.Client, message paho.Message) {
			if message.Retained() {
				slog.Warn("ignoring retained mqtt source event", "topic", message.Topic())
				return
			}

			publishEvent(ctx, events, ReceivedEvent(ReceivedMessage{
				Topic:   message.Topic(),
				Payload: bytes.Clone(message.Payload()),
			}))
		})
		if err := waitToken(ctx, token); err != nil {
			return err
		}

		slog.Info("mqtt subscribed", "topic", topic)
	}

	return nil
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
