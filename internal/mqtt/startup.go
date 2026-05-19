package mqtt

import "github.com/mhemeryck/nest/internal/entity"

func StartupCommands(root *entity.Root) ([]Command, error) {
	topics := NewTopics(root.MQTT.TopicPrefix, root.MQTT.UnitID)
	homeAssistantDiscovery, err := HomeAssistantDeviceDiscoveryMessage(root, topics)
	if err != nil {
		return nil, err
	}

	return []Command{
		PublishCommand(homeAssistantDiscovery),
		PublishCommand(AvailabilityMessage(topics, AvailabilityOnline)),
	}, nil
}
