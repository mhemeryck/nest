package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhemeryck/nest/internal/entity"
)

type HomeAssistantDeviceDiscovery struct {
	Device            HomeAssistantDevice                        `json:"dev"`
	Origin            HomeAssistantOrigin                        `json:"o"`
	Components        map[string]HomeAssistantComponentDiscovery `json:"cmps"`
	AvailabilityTopic string                                     `json:"availability_topic"`
}

type HomeAssistantDevice struct {
	Identifiers  []string `json:"ids"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"mf"`
}

type HomeAssistantOrigin struct {
	Name string `json:"name"`
}

type HomeAssistantComponentDiscovery struct {
	Platform           string `json:"p"`
	Name               string `json:"name"`
	UniqueID           string `json:"unique_id"`
	DefaultEntityID    string `json:"default_entity_id"`
	StateTopic         string `json:"state_topic"`
	CommandTopic       string `json:"command_topic"`
	StateValueTemplate string `json:"state_value_template"`
	PayloadOn          string `json:"payload_on"`
	PayloadOff         string `json:"payload_off"`
}

func BuildHomeAssistantDeviceDiscovery(root *entity.Root, topics Topics) HomeAssistantDeviceDiscovery {
	doc := HomeAssistantDeviceDiscovery{
		Device:            homeAssistantDevice(topics),
		Origin:            HomeAssistantOrigin{Name: "nest"},
		Components:        make(map[string]HomeAssistantComponentDiscovery, len(root.Lights)),
		AvailabilityTopic: AvailabilityTopic(topics),
	}

	for _, light := range root.Lights {
		doc.Components[lightTopicSegment(light.ID)] = homeAssistantLightComponent(light, topics)
	}

	return doc
}

func HomeAssistantDeviceDiscoveryPayload(root *entity.Root, topics Topics) ([]byte, error) {
	payload, err := json.Marshal(BuildHomeAssistantDeviceDiscovery(root, topics))
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func HomeAssistantDeviceDiscoveryMessage(root *entity.Root, topics Topics) (PublishMessage, error) {
	payload, err := HomeAssistantDeviceDiscoveryPayload(root, topics)
	if err != nil {
		return PublishMessage{}, err
	}

	return PublishMessage{
		Topic:   HomeAssistantDeviceDiscoveryTopic(topics),
		Payload: payload,
		Retain:  true,
		QoS:     0,
	}, nil
}

func homeAssistantLightComponent(light entity.Light, topics Topics) HomeAssistantComponentDiscovery {
	entityID := homeAssistantLightEntityID(light.ID)
	return HomeAssistantComponentDiscovery{
		Platform:           "light",
		Name:               light.Name,
		UniqueID:           homeAssistantUniqueID(topics, entityID),
		DefaultEntityID:    fmt.Sprintf("light.%s_%s", topics.UnitID, lightTopicSegment(light.ID)),
		StateTopic:         LightStateTopic(topics, light.ID),
		CommandTopic:       LightCommandTopic(topics, light.ID),
		StateValueTemplate: "{{ value_json.state }}",
		PayloadOn:          "ON",
		PayloadOff:         "OFF",
	}
}

func homeAssistantLightEntityID(lightID entity.LightID) string {
	if entity.IsID(string(lightID), entity.TypeLight) {
		parts := strings.Split(string(lightID), ".")
		return strings.Join(parts[1:], "_")
	}

	return string(lightID)
}

func homeAssistantDevice(topics Topics) HomeAssistantDevice {
	return HomeAssistantDevice{
		Identifiers:  []string{homeAssistantUniqueID(topics, "unit")},
		Name:         "nest " + topics.UnitID,
		Manufacturer: "nest",
	}
}
