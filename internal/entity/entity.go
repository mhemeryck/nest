package entity

import "github.com/mhemeryck/nest/internal/config"

type (
	DeviceID       string
	DigitalInputID string
	PushButtonID   string
	RelayID        string
)

type Root struct {
	SysfsRoot     string
	DigitalInputs []DigitalInput
	PushButtons   []PushButton
	Relays        []Relay
}

type DigitalInput struct {
	ID     DigitalInputID
	Device DeviceID
}

type PushButton struct {
	ID    PushButtonID
	Name  string
	Input DigitalInputID
}

type Relay struct {
	ID     RelayID
	Name   string
	Device DeviceID
}

func FromConfig(root *config.Root) *Root {
	if root == nil {
		return nil
	}

	entities := &Root{
		SysfsRoot:     root.Sysfs.Root,
		DigitalInputs: make([]DigitalInput, 0, len(root.DigitalInputs)),
		PushButtons:   make([]PushButton, 0, len(root.PushButtons)),
		Relays:        make([]Relay, 0, len(root.Relays)),
	}

	for _, input := range root.DigitalInputs {
		entities.DigitalInputs = append(entities.DigitalInputs, DigitalInput{
			ID:     DigitalInputID(input.ID),
			Device: DeviceID(input.Device),
		})
	}

	for _, button := range root.PushButtons {
		entities.PushButtons = append(entities.PushButtons, PushButton{
			ID:    PushButtonID(button.ID),
			Name:  button.Name,
			Input: DigitalInputID(button.Input),
		})
	}

	for _, relay := range root.Relays {
		entities.Relays = append(entities.Relays, Relay{
			ID:     RelayID(relay.ID),
			Name:   relay.Name,
			Device: DeviceID(relay.Device),
		})
	}

	return entities
}
