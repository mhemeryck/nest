package nest

import (
	"context"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
)

type mqttActor struct {
	enabled           bool
	commands          chan mqtt.Command
	events            chan mqtt.Event
	done              chan struct{}
	topics            mqtt.Topics
	sourceEventTopics []string
	startupMessages   []mqtt.PublishMessage
	cfg               entity.MQTT
}

func newMQTTActor(reg *registry.Registry) mqttActor {
	cfg := registry.MQTT(reg)
	if !cfg.Enabled {
		return mqttActor{}
	}

	topics := mqtt.NewTopics(cfg.TopicPrefix, cfg.UnitID)
	covers := registry.Covers(reg)

	// Prepare startup messages to clear retained cover positions and report unknown state.
	startupMessages := make([]mqtt.PublishMessage, 0, 2*len(covers))
	for _, cover := range covers {
		reset := mqtt.CoverStateMessages(topics, cover.ID, mqtt.CoverStateUnknown, nil)
		startupMessages = append(startupMessages, reset[:]...)
	}

	return mqttActor{
		enabled:           true,
		commands:          make(chan mqtt.Command, 32+len(registry.Lights(reg))+2*len(covers)),
		events:            make(chan mqtt.Event, 32),
		done:              make(chan struct{}),
		topics:            topics,
		sourceEventTopics: mqtt.SemanticSourceEventSubscriptionTopics(topics, registry.RemoteTargetBindings(reg)),
		startupMessages:   startupMessages,
		cfg:               cfg,
	}
}

func startMQTTActor(ctx context.Context, actor mqttActor) {
	if !actor.enabled {
		return
	}

	go mqtt.Run(
		ctx,
		actor.cfg,
		actor.startupMessages,
		actor.sourceEventTopics,
		actor.commands,
		actor.events,
		actor.done,
	)
}

func waitForMQTTActor(actor mqttActor) {
	if !actor.enabled {
		return
	}

	<-actor.done
	close(actor.events)
}
