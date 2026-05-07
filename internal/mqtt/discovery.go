package mqtt

import (
	"encoding/json"

	"github.com/mhemeryck/nest/internal/entity"
)

const DiscoverySchemaVersion = 1

type DiscoveryDocument struct {
	SchemaVersion int                    `json:"schema_version"`
	UnitID        string                 `json:"unit_id"`
	Availability  string                 `json:"availability_topic"`
	RawSysfs      []RawSysfsDiscovery    `json:"raw_sysfs"`
	DigitalInputs []DigitalInputDiscovery `json:"digital_inputs"`
	PushButtons   []PushButtonDiscovery   `json:"push_buttons"`
	Relays        []RelayDiscovery        `json:"relays"`
	Lights        []LightDiscovery        `json:"lights"`
}

type RawSysfsDiscovery struct {
	DeviceID   string `json:"device_id"`
	StateTopic string `json:"state_topic"`
}

type DigitalInputDiscovery struct {
	ID          string `json:"id"`
	SysfsDevice string `json:"sysfs_device"`
	StateTopic  string `json:"state_topic"`
}

type PushButtonDiscovery struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Input      string `json:"input"`
	StateTopic string `json:"state_topic"`
}

type RelayDiscovery struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	SysfsDevice string `json:"sysfs_device"`
	StateTopic  string `json:"state_topic"`
}

type LightDiscovery struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Relay          string   `json:"relay"`
	Capabilities   []string `json:"capabilities"`
	StateTopic     string   `json:"state_topic"`
	CommandTopic   string   `json:"command_topic"`
	CommandsEnabled bool     `json:"commands_enabled"`
}

func BuildDiscovery(root *entity.Root, topics Topics) DiscoveryDocument {
	doc := DiscoveryDocument{
		SchemaVersion: DiscoverySchemaVersion,
		UnitID:        topics.UnitID,
		Availability:  AvailabilityTopic(topics),
		RawSysfs:      make([]RawSysfsDiscovery, 0, len(root.DigitalInputs)+len(root.Relays)),
		DigitalInputs: make([]DigitalInputDiscovery, 0, len(root.DigitalInputs)),
		PushButtons:   make([]PushButtonDiscovery, 0, len(root.PushButtons)),
		Relays:        make([]RelayDiscovery, 0, len(root.Relays)),
		Lights:        make([]LightDiscovery, 0, len(root.Lights)),
	}

	for _, input := range root.DigitalInputs {
		doc.RawSysfs = append(doc.RawSysfs, RawSysfsDiscovery{
			DeviceID:   string(input.SysfsDevice),
			StateTopic: RawSysfsStateTopic(topics, input.SysfsDevice),
		})
		doc.DigitalInputs = append(doc.DigitalInputs, DigitalInputDiscovery{
			ID:          string(input.ID),
			SysfsDevice: string(input.SysfsDevice),
			StateTopic:  DigitalInputStateTopic(topics, input.ID),
		})
	}

	for _, button := range root.PushButtons {
		doc.PushButtons = append(doc.PushButtons, PushButtonDiscovery{
			ID:         string(button.ID),
			Name:       button.Name,
			Input:      string(button.Input),
			StateTopic: PushButtonStateTopic(topics, button.ID),
		})
	}

	for _, relay := range root.Relays {
		doc.RawSysfs = append(doc.RawSysfs, RawSysfsDiscovery{
			DeviceID:   string(relay.SysfsDevice),
			StateTopic: RawSysfsStateTopic(topics, relay.SysfsDevice),
		})
		doc.Relays = append(doc.Relays, RelayDiscovery{
			ID:          string(relay.ID),
			Name:        relay.Name,
			SysfsDevice: string(relay.SysfsDevice),
			StateTopic:  RelayStateTopic(topics, relay.ID),
		})
	}

	for _, light := range root.Lights {
		doc.Lights = append(doc.Lights, LightDiscovery{
			ID:             string(light.ID),
			Name:           light.Name,
			Relay:          string(light.Relay),
			Capabilities:   []string{"toggle"},
			StateTopic:     LightStateTopic(topics, light.ID),
			CommandTopic:   LightCommandTopic(topics, light.ID),
			CommandsEnabled: false,
		})
	}

	return doc
}

func DiscoveryPayload(root *entity.Root, topics Topics) ([]byte, error) {
	payload, err := json.Marshal(BuildDiscovery(root, topics))
	if err != nil {
		return nil, err
	}

	return payload, nil
}
