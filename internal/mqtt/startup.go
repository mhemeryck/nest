package mqtt

import "github.com/mhemeryck/nest/internal/entity"

func StartupCommands(cfg entity.MQTT, lights []entity.Light) ([]Command, error) {
	topics := NewTopics(cfg.TopicPrefix, cfg.UnitID)
	homeAssistantDiscovery, err := HomeAssistantDeviceDiscoveryMessage(lights, topics)
	if err != nil {
		return nil, err
	}

	return []Command{
		PublishCommand(homeAssistantDiscovery),
		PublishCommand(AvailabilityMessage(topics, AvailabilityOnline)),
	}, nil
}
