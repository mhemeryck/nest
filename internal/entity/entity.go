package entity

import "github.com/mhemeryck/nest/internal/config"

type Root struct {
	SysfsRoot     string
	DigitalInputs []DigitalInput
	PushButtons   []PushButton
	Relays        []Relay
}

type DigitalInput struct {
	ID     string
	Device string
}

type PushButton struct {
	ID    string
	Name  string
	Input string
}

type Relay struct {
	ID     string
	Name   string
	Device string
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
			ID:     input.ID,
			Device: input.Device,
		})
	}

	for _, button := range root.PushButtons {
		entities.PushButtons = append(entities.PushButtons, PushButton{
			ID:    button.ID,
			Name:  button.Name,
			Input: button.Input,
		})
	}

	for _, relay := range root.Relays {
		entities.Relays = append(entities.Relays, Relay{
			ID:     relay.ID,
			Name:   relay.Name,
			Device: relay.Device,
		})
	}

	return entities
}
