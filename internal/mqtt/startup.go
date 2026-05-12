package mqtt

import "github.com/mhemeryck/nest/internal/entity"

func StartupCommands(root *entity.Root) ([]Command, error) {
	topics := NewTopics(root.MQTT.TopicPrefix, root.MQTT.UnitID)
	discovery, err := DiscoveryMessage(root, topics)
	if err != nil {
		return nil, err
	}

	commands := []Command{
		PublishCommand(discovery),
	}

	for _, light := range root.Lights {
		message, err := HomeAssistantLightDiscoveryMessage(light, topics)
		if err != nil {
			return nil, err
		}
		commands = append(commands, PublishCommand(message))
	}

	commands = append(commands, PublishCommand(AvailabilityMessage(topics, AvailabilityOnline)))

	return commands, nil
}
