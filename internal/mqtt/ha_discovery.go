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
	PositionTopic      string `json:"position_topic,omitempty"`
	StateTopic         string `json:"state_topic"`
	CommandTopic       string `json:"command_topic"`
	StateValueTemplate string `json:"state_value_template,omitempty"`
	PayloadOn          string `json:"payload_on,omitempty"`
	PayloadOff         string `json:"payload_off,omitempty"`
}

func BuildHomeAssistantDeviceDiscovery(lights []entity.Light, covers []entity.Cover, topics Topics) HomeAssistantDeviceDiscovery {
	components := make(map[string]HomeAssistantComponentDiscovery, len(lights)+len(covers))

	// Add light components.
	for _, light := range lights {
		localID, ok := entity.LocalID(string(light.ID), entity.TypeLight)
		if !ok {
			localID = string(light.ID)
		}
		entityID := homeAssistantLightEntityID(light.ID)
		components["light-"+localID] = HomeAssistantComponentDiscovery{
			Platform:           "light",
			Name:               light.Name,
			UniqueID:           homeAssistantUniqueID(topics, entityID),
			DefaultEntityID:    fmt.Sprintf("light.%s_%s", topics.UnitID, localID),
			StateTopic:         lightTopic(topics, light.ID, stateSuffix),
			CommandTopic:       lightTopic(topics, light.ID, commandSuffix),
			StateValueTemplate: "{{ value_json.state }}",
			PayloadOn:          "ON",
			PayloadOff:         "OFF",
		}
	}

	// Add cover components.
	for _, cover := range covers {
		localID, ok := entity.LocalID(string(cover.ID), entity.TypeCover)
		if !ok {
			localID = string(cover.ID)
		}
		components["cover-"+localID] = HomeAssistantComponentDiscovery{
			Platform:        "cover",
			Name:            cover.Name,
			UniqueID:        homeAssistantUniqueID(topics, "cover_"+localID),
			DefaultEntityID: fmt.Sprintf("cover.%s_%s", topics.UnitID, localID),
			StateTopic:      coverTopic(topics, cover.ID, stateSuffix),
			PositionTopic:   coverTopic(topics, cover.ID, positionSuffix),
			CommandTopic:    coverTopic(topics, cover.ID, commandSuffix),
		}
	}

	return HomeAssistantDeviceDiscovery{
		Device:            homeAssistantDevice(topics),
		Origin:            HomeAssistantOrigin{Name: "nest"},
		Components:        components,
		AvailabilityTopic: AvailabilityTopic(topics),
	}
}

func HomeAssistantDeviceDiscoveryMessage(lights []entity.Light, covers []entity.Cover, topics Topics) (PublishMessage, error) {
	payload, err := json.Marshal(BuildHomeAssistantDeviceDiscovery(lights, covers, topics))
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
