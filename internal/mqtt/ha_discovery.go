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
	AvailabilityTopic string                                     `json:"availability_topic,omitempty"`
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
	Platform            string                      `json:"p"`
	Name                string                      `json:"name"`
	UniqueID            string                      `json:"unique_id"`
	DefaultEntityID     string                      `json:"default_entity_id"`
	StateTopic          string                      `json:"state_topic"`
	CommandTopic        string                      `json:"command_topic"`
	StateValueTemplate  string                      `json:"state_value_template,omitempty"`
	ValueTemplate       string                      `json:"value_template,omitempty"`
	PayloadOn           string                      `json:"payload_on,omitempty"`
	PayloadOff          string                      `json:"payload_off,omitempty"`
	PayloadOpen         string                      `json:"payload_open,omitempty"`
	PayloadClose        string                      `json:"payload_close,omitempty"`
	PayloadStop         string                      `json:"payload_stop,omitempty"`
	JSONAttributesTopic string                      `json:"json_attributes_topic,omitempty"`
	AvailabilityTopic   string                      `json:"availability_topic,omitempty"`
	Availability        []HomeAssistantAvailability `json:"availability,omitempty"`
	AvailabilityMode    string                      `json:"availability_mode,omitempty"`
}

type HomeAssistantAvailability struct {
	Topic         string `json:"topic"`
	ValueTemplate string `json:"value_template,omitempty"`
}

func BuildHomeAssistantDeviceDiscovery(lights []entity.Light, topics Topics, coverLists ...[]entity.Cover) HomeAssistantDeviceDiscovery {
	var covers []entity.Cover
	if len(coverLists) > 0 {
		covers = coverLists[0]
	}
	doc := HomeAssistantDeviceDiscovery{
		Device:            homeAssistantDevice(topics),
		Origin:            HomeAssistantOrigin{Name: "nest"},
		Components:        make(map[string]HomeAssistantComponentDiscovery, len(lights)),
		AvailabilityTopic: AvailabilityTopic(topics),
	}

	for _, light := range lights {
		doc.Components[lightTopicSegment(light.ID)] = homeAssistantLightComponent(light, topics)
	}
	if len(covers) > 0 {
		doc.AvailabilityTopic = ""
		for key, component := range doc.Components {
			component.AvailabilityTopic = AvailabilityTopic(topics)
			doc.Components[key] = component
		}
		for _, cover := range covers {
			doc.Components["cover."+coverTopicSegment(cover.ID)] = homeAssistantCoverComponent(cover, topics)
		}
	}

	return doc
}

func HomeAssistantDeviceDiscoveryPayload(lights []entity.Light, topics Topics, covers ...[]entity.Cover) ([]byte, error) {
	payload, err := json.Marshal(BuildHomeAssistantDeviceDiscovery(lights, topics, covers...))
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func HomeAssistantDeviceDiscoveryMessage(lights []entity.Light, topics Topics, covers ...[]entity.Cover) (PublishMessage, error) {
	payload, err := HomeAssistantDeviceDiscoveryPayload(lights, topics, covers...)
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

func homeAssistantCoverComponent(cover entity.Cover, topics Topics) HomeAssistantComponentDiscovery {
	stateTopic := CoverStateTopic(topics, cover.ID)
	return HomeAssistantComponentDiscovery{
		Platform: "cover", Name: cover.Name,
		UniqueID:        homeAssistantUniqueID(topics, "cover_"+coverTopicSegment(cover.ID)),
		DefaultEntityID: fmt.Sprintf("cover.%s_%s", topics.UnitID, coverTopicSegment(cover.ID)),
		StateTopic:      stateTopic, CommandTopic: CoverCommandTopic(topics, cover.ID),
		ValueTemplate: "{{ value_json.ha_state }}", JSONAttributesTopic: stateTopic,
		PayloadOpen: "OPEN", PayloadClose: "CLOSE", PayloadStop: "STOP",
		AvailabilityMode: "all",
		Availability: []HomeAssistantAvailability{
			{Topic: AvailabilityTopic(topics)},
			{Topic: stateTopic, ValueTemplate: "{{ 'online' if value_json.available else 'offline' }}"},
		},
	}
}
