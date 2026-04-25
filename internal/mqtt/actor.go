package mqtt

import (
	"context"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
)

const publishTimeout = 5 * time.Second

func Run(ctx context.Context, root *entity.Root, index *registry.Index, states <-chan State, commands chan<- Command, done chan<- struct{}) {
	defer close(done)
	if !root.MQTT.Enabled {
		return
	}

	cfg := configFromEntity(root.MQTT)

	client := paho.NewClient(clientOptions(cfg))
	if !waitToken(ctx, client.Connect()) {
		return
	}
	defer client.Disconnect(250)

	publish(ctx, client, AvailabilityOnline(cfg))
	for _, state := range DiscoveryStates(cfg, root, index) {
		publish(ctx, client, state)
	}
	defer publish(context.Background(), client, AvailabilityOffline(cfg))
	_ = commands

	for {
		select {
		case <-ctx.Done():
			return
		case state, ok := <-states:
			if !ok {
				return
			}
			publish(ctx, client, state)
		}
	}
}

func configFromEntity(cfg entity.MQTT) Config {
	return Config{
		Enabled:         cfg.Enabled,
		Broker:          cfg.Broker,
		UnitID:          cfg.UnitID,
		ClientID:        cfg.ClientID,
		Username:        cfg.Username,
		Password:        cfg.Password,
		DiscoveryPrefix: cfg.DiscoveryPrefix,
	}
}

func clientOptions(cfg Config) *paho.ClientOptions {
	options := paho.NewClientOptions()
	options.AddBroker(cfg.Broker)
	options.SetClientID(cfg.ClientID)
	options.SetWill(AvailabilityTopic(cfg), payloadOff, 0, true)
	options.SetAutoReconnect(true)
	options.SetConnectRetry(true)
	options.SetConnectionLostHandler(func(_ paho.Client, err error) {
		slog.Error("mqtt connection lost", "error", err)
	})
	options.SetOnConnectHandler(func(client paho.Client) {
		publish(context.Background(), client, AvailabilityOnline(cfg))
	})
	if cfg.Username != "" {
		options.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		options.SetPassword(cfg.Password)
	}

	return options
}

func publish(ctx context.Context, client paho.Client, state State) bool {
	if state.Topic == "" || state.Payload == nil {
		return true
	}

	token := client.Publish(state.Topic, 0, state.Retain, state.Payload)
	if waitToken(ctx, token) {
		return true
	}

	slog.Error("mqtt publish failed", "topic", state.Topic, "kind", state.Kind, "error", token.Error())
	return false
}

func waitToken(ctx context.Context, token paho.Token) bool {
	done := make(chan bool, 1)
	go func() {
		done <- token.WaitTimeout(publishTimeout)
	}()

	select {
	case <-ctx.Done():
		return false
	case ok := <-done:
		if !ok {
			slog.Error("mqtt operation timed out")
			return false
		}
		if token.Error() != nil {
			slog.Error("mqtt operation failed", "error", token.Error())
			return false
		}
		return true
	}
}
