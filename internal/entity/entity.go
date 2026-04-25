package entity

import "github.com/mhemeryck/nest/internal/config"

type (
	DeviceID       string
	DigitalInputID string
	PushButtonID   string
	LightID        string
	RelayID        string
	LightAction    string
)

const LightActionToggle LightAction = "toggle"

type Root struct {
	SysfsRoot     string
	DigitalInputs []DigitalInput
	PushButtons   []PushButton
	Lights        []Light
	Relays        []Relay
	Bindings      []Binding
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

type Light struct {
	ID    LightID
	Name  string
	Relay RelayID
}

type Binding struct {
	Button PushButtonID
	Light  LightID
	Action LightAction
}

func FromConfig(root *config.Root) *Root {
	if root == nil {
		return nil
	}

	entities := &Root{
		SysfsRoot:     root.Sysfs.Root,
		DigitalInputs: make([]DigitalInput, 0, len(root.DigitalInputs)),
		PushButtons:   make([]PushButton, 0, len(root.PushButtons)),
		Lights:        make([]Light, 0, len(root.Lights)),
		Relays:        make([]Relay, 0, len(root.Relays)),
		Bindings:      make([]Binding, 0, len(root.Bindings)),
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

	for _, light := range root.Lights {
		entities.Lights = append(entities.Lights, Light{
			ID:    LightID(light.ID),
			Name:  light.Name,
			Relay: RelayID(light.Relay),
		})
	}

	for _, binding := range root.Bindings {
		entities.Bindings = append(entities.Bindings, Binding{
			Button: PushButtonID(binding.Button),
			Light:  LightID(binding.Light),
			Action: LightAction(binding.Action),
		})
	}

	return entities
}
