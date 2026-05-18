package nest

import (
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
