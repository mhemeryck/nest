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

	client := newRuntimeClient(ctx, cfg, sourceEventTopics, events)
	if !connectClient(ctx, client, cfg, events) {
		return
	}
	defer disconnectClient(ctx, client, cfg, events)
	processCommands(ctx, client, commands, events)
}

func newRuntimeClient(ctx context.Context, cfg entity.MQTT, sourceEventTopics []string, events chan<- Event) paho.Client {
	topics := NewTopics(cfg.TopicPrefix, cfg.UnitID)
	return paho.NewClient(clientOptions(ctx, cfg, topics, sourceEventTopics, events))
}

func connectClient(ctx context.Context, client paho.Client, cfg entity.MQTT, events chan<- Event) bool {
	if err := waitToken(ctx, client.Connect()); err != nil {
		client.Disconnect(0)
		slog.Error("mqtt connect failed", "broker", brokerURL(cfg), "error", err)
		publishEvent(ctx, events, ConnectFailedEvent(err))
		return false
	}
	return true
}

func disconnectClient(ctx context.Context, client paho.Client, cfg entity.MQTT, events chan<- Event) {
	offlineCtx, cancel := context.WithTimeout(context.Background(), gracefulOfflineTimeout)
	defer cancel()
	publishGracefulOffline(offlineCtx, client, cfg)
	client.Disconnect(uint(disconnectQuiesce / time.Millisecond))
	publishEvent(ctx, events, DisconnectedEvent())
}

func processCommands(ctx context.Context, client client, commands <-chan Command, events chan<- Event) {
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
	configureBrokerClient(options, cfg)
	configureClientWill(options, topics)
	configureClientSubscriptions(ctx, options, cfg, topics, sourceEventTopics, events)
	configureClientAuthentication(options, cfg)
	configureClientDelivery(options)
	return options
}

func configureBrokerClient(options *paho.ClientOptions, cfg entity.MQTT) {
	options.AddBroker(brokerURL(cfg))
	options.SetClientID(cfg.ClientID)
}

func configureClientWill(options *paho.ClientOptions, topics Topics) {
	offline := AvailabilityMessage(topics, AvailabilityOffline)
	options.SetWill(offline.Topic, string(offline.Payload), offline.QoS, offline.Retain)
}

func configureClientSubscriptions(ctx context.Context, options *paho.ClientOptions, cfg entity.MQTT, topics Topics, sourceEventTopics []string, events chan<- Event) {
	options.SetOnConnectHandler(func(client paho.Client) {
		subscribeRuntimeTopics(ctx, client, topics, sourceEventTopics, events)
		slog.Info("mqtt connected", "broker", brokerURL(cfg), "client_id", cfg.ClientID)
		publishEvent(ctx, events, ConnectedEvent())
	})
}

func configureClientAuthentication(options *paho.ClientOptions, cfg entity.MQTT) {
	if cfg.Username != "" {
		options.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		options.SetPassword(cfg.Password)
	}
}

func configureClientDelivery(options *paho.ClientOptions) {
	options.SetAutoReconnect(true)
	options.SetConnectRetry(true)
	options.SetOrderMatters(true)
}

func subscribeRuntimeTopics(ctx context.Context, client client, topics Topics, sourceEventTopics []string, events chan<- Event) {
	if err := subscribeLightCommands(ctx, client, topics, events); err != nil {
		slog.Error("mqtt subscribe failed", "topic", LightCommandSubscriptionTopic(topics), "error", err)
	}
	if err := subscribeSourceEvents(ctx, client, sourceEventTopics, events); err != nil {
		slog.Error("mqtt source event subscribe failed", "error", err)
	}
	if err := subscribeCoverCommands(ctx, client, topics, events); err != nil {
		slog.Error("mqtt cover subscribe failed", "error", err)
	}
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

func subscribeCoverCommands(ctx context.Context, client client, topics Topics, events chan<- Event) error {
	if client == nil {
		return nil
	}
	token := client.Subscribe(CoverCommandSubscriptionTopic(topics), 0, func(_ paho.Client, message paho.Message) {
		publishEvent(ctx, events, ReceivedEvent(ReceivedMessage{Topic: message.Topic(), Payload: bytes.Clone(message.Payload()), Retained: message.Retained()}))
	})
	return waitToken(ctx, token)
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
