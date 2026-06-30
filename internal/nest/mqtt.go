package nest

import (
	"context"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
)

type mqttActor struct {
	commands          chan mqtt.Command
	events            chan mqtt.Event
	done              chan struct{}
	topics            mqtt.Topics
	sourceEventTopics []string
	cfg               entity.MQTT
}

func newMQTTActor(root *entity.Root) mqttActor {
	if !root.MQTT.Enabled {
		return mqttActor{}
	}

	topics := mqtt.NewTopics(root.MQTT.TopicPrefix, root.MQTT.UnitID)

	return mqttActor{
		commands:          make(chan mqtt.Command, 32),
		events:            make(chan mqtt.Event, 32),
		done:              make(chan struct{}),
		topics:            topics,
		sourceEventTopics: mqtt.SemanticSourceEventSubscriptionTopics(topics, root),
		cfg:               root.MQTT,
	}
}

func startMQTTActor(ctx context.Context, actor mqttActor) {
	if actor.commands == nil {
		return
	}

	go mqtt.Run(ctx, actor.cfg, actor.sourceEventTopics, actor.commands, actor.events, actor.done)
}

func waitForMQTTActor(actor mqttActor) {
	if actor.done == nil {
		return
	}

	<-actor.done
	close(actor.events)
}
