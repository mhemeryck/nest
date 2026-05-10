package entity

import "github.com/mhemeryck/nest/internal/config"

type (
	SysfsDeviceID  string
	DigitalInputID string
	PushButtonID   string
	LightID        string
	RelayID        string
	LightAction    string
)

const LightActionToggle LightAction = "toggle"

type Root struct {
	SysfsRoot     string
	MQTT          MQTT
	DigitalInputs []DigitalInput
	PushButtons   []PushButton
	Lights        []Light
	Relays        []Relay
	Bindings      []Binding
}

type MQTT struct {
	Enabled     bool
	Host        string
	Port        int
	ClientID    string
	Username    string
	Password    string
	TopicPrefix string
	UnitID      string
}

type DigitalInput struct {
	ID          DigitalInputID
	SysfsDevice SysfsDeviceID
}

type PushButton struct {
	ID    PushButtonID
	Name  string
	Input DigitalInputID
}

type Relay struct {
	ID          RelayID
	Name        string
	SysfsDevice SysfsDeviceID
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
		MQTT:          mqttFromConfig(root.MQTT),
		DigitalInputs: make([]DigitalInput, 0, len(root.DigitalInputs)),
		PushButtons:   make([]PushButton, 0, len(root.PushButtons)),
		Lights:        make([]Light, 0, len(root.Lights)),
		Relays:        make([]Relay, 0, len(root.Relays)),
		Bindings:      make([]Binding, 0, len(root.Bindings)),
	}

	for _, input := range root.DigitalInputs {
		entities.DigitalInputs = append(entities.DigitalInputs, DigitalInput{
			ID:          DigitalInputID(input.ID),
			SysfsDevice: SysfsDeviceID(input.Device),
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
			ID:          RelayID(relay.ID),
			Name:        relay.Name,
			SysfsDevice: SysfsDeviceID(relay.Device),
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

func mqttFromConfig(mqtt config.MQTTConfig) MQTT {
	return MQTT{
		Enabled:     mqtt.Enabled,
		Host:        mqtt.Host,
		Port:        mqtt.Port,
		ClientID:    mqtt.ClientID,
		Username:    mqtt.Username,
		Password:    mqtt.Password,
		TopicPrefix: mqtt.TopicPrefix,
		UnitID:      mqtt.UnitID,
	}
}
