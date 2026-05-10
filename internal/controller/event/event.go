package event

import "github.com/mhemeryck/nest/internal/entity"

type Kind string

const (
	DigitalInputStateKind  Kind = "digital_input_state"
	PushButtonPressedKind  Kind = "push_button_pressed"
	PushButtonReleasedKind Kind = "push_button_released"
	RelayStateKind         Kind = "relay_state"
	LightStateKind         Kind = "light_state"
	LightKind              Kind = "light"
)

type Event struct {
	Kind         Kind
	DigitalInput *DigitalInput
	PushButton   *PushButton
	Relay        *Relay
	LightState   *LightState
	Light        *Light
}

type DigitalInput struct {
	InputID     entity.DigitalInputID
	SysfsDevice entity.SysfsDeviceID
	Value       int
}

type PushButton struct {
	ButtonID entity.PushButtonID
	Name     string
}

type Light struct {
	LightID entity.LightID
	Name    string
	Action  entity.LightAction
}

type LightState struct {
	LightID entity.LightID
	Name    string
	RelayID entity.RelayID
	Value   int
}

type Relay struct {
	RelayID     entity.RelayID
	Name        string
	SysfsDevice entity.SysfsDeviceID
	Value       int
}
