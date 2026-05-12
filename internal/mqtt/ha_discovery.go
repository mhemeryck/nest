package mqtt

import (
	"encoding/json"

	"github.com/mhemeryck/nest/internal/entity"
)

type HomeAssistantDevice struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
}

type HomeAssistantLightDiscovery struct {
	Name               string              `json:"name"`
	UniqueID           string              `json:"unique_id"`
	StateTopic         string              `json:"state_topic"`
	CommandTopic       string              `json:"command_topic"`
	StateValueTemplate string              `json:"state_value_template"`
	PayloadOn          string              `json:"payload_on"`
	PayloadOff         string              `json:"payload_off"`
	AvailabilityTopic  string              `json:"availability_topic"`
	Device             HomeAssistantDevice `json:"device"`
}

func BuildHomeAssistantLightDiscovery(light entity.Light, topics Topics) HomeAssistantLightDiscovery {
	return HomeAssistantLightDiscovery{
		Name:               light.Name,
		UniqueID:           homeAssistantUniqueID(topics, string(light.ID)),
		StateTopic:         LightStateTopic(topics, light.ID),
		CommandTopic:       LightCommandTopic(topics, light.ID),
		StateValueTemplate: "{{ value_json.state }}",
		PayloadOn:          "ON",
		PayloadOff:         "OFF",
		AvailabilityTopic:  AvailabilityTopic(topics),
		Device:             homeAssistantDevice(topics),
	}
}

func HomeAssistantLightDiscoveryPayload(light entity.Light, topics Topics) ([]byte, error) {
	payload, err := json.Marshal(BuildHomeAssistantLightDiscovery(light, topics))
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func HomeAssistantLightDiscoveryMessage(light entity.Light, topics Topics) (PublishMessage, error) {
	payload, err := HomeAssistantLightDiscoveryPayload(light, topics)
	if err != nil {
		return PublishMessage{}, err
	}

	return PublishMessage{
		Topic:   HomeAssistantLightDiscoveryTopic(topics, light.ID),
		Payload: payload,
		Retain:  true,
		QoS:     0,
	}, nil
}

func homeAssistantDevice(topics Topics) HomeAssistantDevice {
	return HomeAssistantDevice{
		Identifiers:  []string{homeAssistantUniqueID(topics, "unit")},
		Name:         "nest " + topics.UnitID,
		Manufacturer: "nest",
	}
}
