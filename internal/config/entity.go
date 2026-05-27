package config

import "github.com/mhemeryck/nest/internal/entity"

func ToEntityRoot(root *Root) *entity.Root {
	if root == nil {
		return nil
	}

	entities := &entity.Root{
		SysfsRoot:          root.Sysfs.Root,
		SysfsPollIntervals: pollIntervalsFromConfig(root.Sysfs.PollIntervals),
		MQTT:               mqttFromConfig(root.MQTT),
		DigitalInputs:      make([]entity.DigitalInput, 0, len(root.DigitalInputs)),
		PushButtons:        make([]entity.PushButton, 0, len(root.PushButtons)),
		Lights:             make([]entity.Light, 0, len(root.Lights)),
		Relays:             make([]entity.Relay, 0, len(root.Relays)),
		Bindings:           make([]entity.Binding, 0, len(root.Bindings)),
	}

	for _, input := range root.DigitalInputs {
		entities.DigitalInputs = append(entities.DigitalInputs, entity.DigitalInput{
			ID:          entity.DigitalInputID(input.ID),
			SysfsDevice: entity.SysfsDeviceID(input.Device),
		})
	}

	for _, button := range root.PushButtons {
		entities.PushButtons = append(entities.PushButtons, entity.PushButton{
			ID:    entity.PushButtonID(button.ID),
			Name:  button.Name,
			Input: entity.DigitalInputID(button.Input),
		})
	}

	for _, relay := range root.Relays {
		entities.Relays = append(entities.Relays, entity.Relay{
			ID:          entity.RelayID(relay.ID),
			Name:        relay.Name,
			SysfsDevice: entity.SysfsDeviceID(relay.Device),
		})
	}

	for _, light := range root.Lights {
		entities.Lights = append(entities.Lights, entity.Light{
			ID:    entity.LightID(light.ID),
			Name:  light.Name,
			Relay: entity.RelayID(light.Relay),
		})
	}

	for _, binding := range root.Bindings {
		entities.Bindings = append(entities.Bindings, entity.Binding{
			Button: entity.PushButtonID(binding.Button),
			Light:  entity.LightID(binding.Light),
			Action: entity.LightAction(binding.Action),
		})
	}

	return entities
}

func pollIntervalsFromConfig(intervals PollIntervalsConfig) entity.PollIntervals {
	return entity.PollIntervals{
		DigitalInput:  intervals.DigitalInput,
		DigitalOutput: intervals.DigitalOutput,
		RelayOutput:   intervals.RelayOutput,
	}
}

func mqttFromConfig(mqtt MQTTConfig) entity.MQTT {
	return entity.MQTT{
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
