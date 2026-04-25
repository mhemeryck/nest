package mqtt

import (
	"encoding/json"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
)

type haDiscoveryPayload struct {
	Name                string       `json:"name"`
	ObjectID            string       `json:"object_id"`
	UniqueID            string       `json:"unique_id"`
	StateTopic          string       `json:"state_topic"`
	CommandTopic        string       `json:"command_topic"`
	PayloadOn           string       `json:"payload_on"`
	PayloadOff          string       `json:"payload_off"`
	AvailabilityTopic   string       `json:"availability_topic"`
	PayloadAvailable    string       `json:"payload_available"`
	PayloadNotAvailable string       `json:"payload_not_available"`
	Device              deviceConfig `json:"device"`
}

type deviceConfig struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
}

type unitDiscoveryPayload struct {
	SchemaVersion     int               `json:"schema_version"`
	UnitID            string            `json:"unit_id"`
	AvailabilityTopic string            `json:"availability_topic"`
	CommandsEnabled   bool              `json:"commands_enabled"`
	Inputs            []inputDiscovery  `json:"inputs"`
	PushButtons       []buttonDiscovery `json:"push_buttons"`
	Relays            []relayDiscovery  `json:"relays"`
	Lights            []lightDiscovery  `json:"lights"`
}

type inputDiscovery struct {
	ID           string `json:"id"`
	DeviceID     string `json:"device_id"`
	StateTopic   string `json:"state_topic"`
	EventTopic   string `json:"event_topic"`
	CommandTopic string `json:"command_topic"`
}

type buttonDiscovery struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	InputID      string `json:"input_id"`
	EventTopic   string `json:"event_topic"`
	CommandTopic string `json:"command_topic"`
}

type relayDiscovery struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DeviceID     string `json:"device_id"`
	StateTopic   string `json:"state_topic"`
	CommandTopic string `json:"command_topic"`
}

type lightDiscovery struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RelayID      string `json:"relay_id"`
	StateTopic   string `json:"state_topic"`
	CommandTopic string `json:"command_topic"`
}

func DiscoveryStates(cfg Config, root *entity.Root, index *registry.Index) []State {
	states := []State{unitDiscoveryState(cfg, root, index)}
	device := haDevice(cfg)

	for _, button := range root.PushButtons {
		input, ok := inputByID(root, button.Input)
		if !ok {
			continue
		}

		payload := haDiscoveryPayload{
			Name:                button.Name,
			ObjectID:            string(button.ID),
			UniqueID:            cfg.UnitID + "_" + string(button.ID),
			StateTopic:          InputStateTopic(cfg, input.Device),
			CommandTopic:        InputCommandTopic(cfg, input.Device),
			PayloadOn:           payloadOn,
			PayloadOff:          payloadOff,
			AvailabilityTopic:   AvailabilityTopic(cfg),
			PayloadAvailable:    payloadOn,
			PayloadNotAvailable: payloadOff,
			Device:              device,
		}
		states = append(states, State{Kind: DiscoveryState, Topic: HASwitchDiscoveryTopic(cfg, button.ID), Payload: marshalJSON(payload), Retain: true})
	}

	for _, light := range root.Lights {
		relay, ok := index.RelaysByID[light.Relay]
		if !ok {
			continue
		}

		payload := haDiscoveryPayload{
			Name:                light.Name,
			ObjectID:            string(light.ID),
			UniqueID:            cfg.UnitID + "_" + string(light.ID),
			StateTopic:          RelayStateTopic(cfg, relay.Device),
			CommandTopic:        RelayCommandTopic(cfg, relay.Device),
			PayloadOn:           payloadOn,
			PayloadOff:          payloadOff,
			AvailabilityTopic:   AvailabilityTopic(cfg),
			PayloadAvailable:    payloadOn,
			PayloadNotAvailable: payloadOff,
			Device:              device,
		}
		states = append(states, State{Kind: DiscoveryState, Topic: HALightDiscoveryTopic(cfg, light.ID), Payload: marshalJSON(payload), Retain: true})
	}

	return states
}

func unitDiscoveryState(cfg Config, root *entity.Root, index *registry.Index) State {
	payload := unitDiscoveryPayload{
		SchemaVersion:     1,
		UnitID:            cfg.UnitID,
		AvailabilityTopic: AvailabilityTopic(cfg),
		CommandsEnabled:   false,
		Inputs:            make([]inputDiscovery, 0, len(root.DigitalInputs)),
		PushButtons:       make([]buttonDiscovery, 0, len(root.PushButtons)),
		Relays:            make([]relayDiscovery, 0, len(root.Relays)),
		Lights:            make([]lightDiscovery, 0, len(root.Lights)),
	}

	for _, input := range root.DigitalInputs {
		payload.Inputs = append(payload.Inputs, inputDiscovery{
			ID:           string(input.ID),
			DeviceID:     string(input.Device),
			StateTopic:   InputStateTopic(cfg, input.Device),
			EventTopic:   InputEventTopic(cfg, input.Device),
			CommandTopic: InputCommandTopic(cfg, input.Device),
		})
	}

	for _, button := range root.PushButtons {
		input, ok := inputByID(root, button.Input)
		if !ok {
			continue
		}
		payload.PushButtons = append(payload.PushButtons, buttonDiscovery{
			ID:           string(button.ID),
			Name:         button.Name,
			InputID:      string(button.Input),
			EventTopic:   PushButtonEventTopic(cfg, button.ID),
			CommandTopic: InputCommandTopic(cfg, input.Device),
		})
	}

	for _, relay := range root.Relays {
		payload.Relays = append(payload.Relays, relayDiscovery{
			ID:           string(relay.ID),
			Name:         relay.Name,
			DeviceID:     string(relay.Device),
			StateTopic:   RelayStateTopic(cfg, relay.Device),
			CommandTopic: RelayCommandTopic(cfg, relay.Device),
		})
	}

	for _, light := range root.Lights {
		relay, ok := index.RelaysByID[light.Relay]
		if !ok {
			continue
		}
		payload.Lights = append(payload.Lights, lightDiscovery{
			ID:           string(light.ID),
			Name:         light.Name,
			RelayID:      string(light.Relay),
			StateTopic:   RelayStateTopic(cfg, relay.Device),
			CommandTopic: RelayCommandTopic(cfg, relay.Device),
		})
	}

	return State{Kind: DiscoveryState, Topic: UnitDiscoveryTopic(cfg), Payload: marshalJSON(payload), Retain: true}
}

func inputByID(root *entity.Root, inputID entity.DigitalInputID) (entity.DigitalInput, bool) {
	for _, input := range root.DigitalInputs {
		if input.ID == inputID {
			return input, true
		}
	}

	return entity.DigitalInput{}, false
}

func haDevice(cfg Config) deviceConfig {
	return deviceConfig{
		Identifiers:  []string{"nest_" + cfg.UnitID},
		Name:         cfg.UnitID,
		Manufacturer: "Unipi",
		Model:        "nest",
	}
}

func marshalJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}

	return data
}
