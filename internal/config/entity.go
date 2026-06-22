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
		Modbus:             modbusFromConfig(root.Modbus),
		DigitalInputs:      make([]entity.DigitalInput, 0, len(root.DigitalInputs)),
		PushButtons:        make([]entity.PushButton, 0, len(root.PushButtons)),
		Lights:             make([]entity.Light, 0, len(root.Lights)),
		Relays:             make([]entity.Relay, 0, len(root.Relays)),
		Bindings:           make([]entity.Binding, 0, len(root.Bindings)),
		RemoteSourceBindings: make(
			[]entity.Binding,
			0,
			len(root.RemoteSourceBindings),
		),
		RemoteTargetBindings: make(
			[]entity.Binding,
			0,
			len(root.RemoteTargetBindings),
		),
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
		entities.Bindings = append(entities.Bindings, bindingFromConfig(binding))
	}

	for _, binding := range root.RemoteSourceBindings {
		entities.RemoteSourceBindings = append(entities.RemoteSourceBindings, bindingFromConfig(binding))
	}

	for _, binding := range root.RemoteTargetBindings {
		entities.RemoteTargetBindings = append(entities.RemoteTargetBindings, bindingFromConfig(binding))
	}

	return entities
}

func modbusFromConfig(modbus ModbusConfig) entity.Modbus {
	converted := entity.Modbus{
		Mode:              entity.ModbusMode(modbus.Mode),
		EventSignals:      make([]entity.ModbusEventSignal, 0, len(modbus.EventSignals)),
		StatePoints:       make([]entity.ModbusStatePoint, 0, len(modbus.StatePoints)),
		EventSignalWrites: make([]entity.ModbusEventSignalWrite, 0, len(modbus.EventSignalWrites)),
		StatePolls:        make([]entity.ModbusStatePoll, 0, len(modbus.StatePolls)),
	}

	for _, signal := range modbus.EventSignals {
		converted.EventSignals = append(converted.EventSignals, entity.ModbusEventSignal{
			ID:     entity.ModbusEventSignalID(signal.ID),
			Source: entity.ID(signal.Source),
			Target: entity.ID(signal.Target),
			Action: entity.Action(signal.Action),
		})
	}

	for _, point := range modbus.StatePoints {
		converted.StatePoints = append(converted.StatePoints, entity.ModbusStatePoint{
			ID:     entity.ModbusStatePointID(point.ID),
			Entity: entity.ID(point.Entity),
		})
	}

	for _, write := range modbus.EventSignalWrites {
		converted.EventSignalWrites = append(converted.EventSignalWrites, entity.ModbusEventSignalWrite{
			Unit:   write.Unit,
			Signal: entity.ModbusEventSignalID(write.Signal),
			Source: entity.ID(write.Source),
			Target: entity.ID(write.Target),
			Action: entity.Action(write.Action),
		})
	}

	for _, poll := range modbus.StatePolls {
		converted.StatePolls = append(converted.StatePolls, entity.ModbusStatePoll{
			Unit:   poll.Unit,
			Point:  entity.ModbusStatePointID(poll.Point),
			Entity: entity.ID(poll.Entity),
		})
	}

	return converted
}

func bindingFromConfig(binding BindingConfig) entity.Binding {
	return entity.Binding{
		Source: entity.ID(binding.Source),
		Target: entity.ID(binding.Target),
		Action: entity.Action(binding.Action),
	}
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
