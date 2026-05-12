package mqtt

import "github.com/mhemeryck/nest/internal/entity"

func StartupCommands(root *entity.Root) ([]Command, error) {
	topics := NewTopics(root.MQTT.TopicPrefix, root.MQTT.UnitID)
	discovery, err := DiscoveryMessage(root, topics)
	if err != nil {
		return nil, err
	}

	homeAssistantDiscovery, err := HomeAssistantDeviceDiscoveryMessage(root, topics)
	if err != nil {
		return nil, err
	}

	return []Command{
		PublishCommand(discovery),
		PublishCommand(homeAssistantDiscovery),
		PublishCommand(AvailabilityMessage(topics, AvailabilityOnline)),
	}, nil
}
