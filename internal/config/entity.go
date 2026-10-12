package config

import "github.com/mhemeryck/nest/internal/entity"

func ToEntityRoot(root *Root) *entity.Root {
	if root == nil {
		return nil
	}

	return &entity.Root{
		SysfsRoot:            root.Sysfs.Root,
		SysfsPollIntervals:   pollIntervalsFromConfig(root.Sysfs.PollIntervals),
		MQTT:                 mqttFromConfig(root.MQTT),
		Modbus:               modbusFromConfig(root.Modbus),
		DigitalInputs:        digitalInputsFromConfig(root.DigitalInputs),
		PushButtons:          pushButtonsFromConfig(root.PushButtons),
		Lights:               lightsFromConfig(root.Lights),
		Covers:               coversFromConfig(root.Covers),
		CoverControl:         entity.CoverControl(root.CoverControl),
		PersistencePath:      root.Persistence.Path,
		Relays:               relaysFromConfig(root.Relays),
		Bindings:             bindingsFromConfig(root.Bindings),
		RemoteSourceBindings: bindingsFromConfig(root.RemoteSourceBindings),
		RemoteTargetBindings: bindingsFromConfig(root.RemoteTargetBindings),
	}
}

func digitalInputsFromConfig(inputs []DigitalInputConfig) []entity.DigitalInput {
	converted := make([]entity.DigitalInput, 0, len(inputs))
	for _, input := range inputs {
		converted = append(converted, entity.DigitalInput{
			ID:          entity.DigitalInputID(input.ID),
			SysfsDevice: entity.SysfsDeviceID(input.Device),
		})
	}
	return converted
}

func pushButtonsFromConfig(buttons []PushButtonConfig) []entity.PushButton {
	converted := make([]entity.PushButton, 0, len(buttons))
	for _, button := range buttons {
		converted = append(converted, entity.PushButton{
			ID:    entity.PushButtonID(button.ID),
			Name:  button.Name,
			Input: entity.DigitalInputID(button.Input),
		})
	}
	return converted
}

func relaysFromConfig(relays []RelayConfig) []entity.Relay {
	converted := make([]entity.Relay, 0, len(relays))
	for _, relay := range relays {
		converted = append(converted, entity.Relay{
			ID:          entity.RelayID(relay.ID),
			Name:        relay.Name,
			SysfsDevice: entity.SysfsDeviceID(relay.Device),
		})
	}
	return converted
}

func lightsFromConfig(lights []LightConfig) []entity.Light {
	converted := make([]entity.Light, 0, len(lights))
	for _, light := range lights {
		converted = append(converted, entity.Light{
			ID:    entity.LightID(light.ID),
			Name:  light.Name,
			Relay: entity.RelayID(light.Relay),
		})
	}
	return converted
}

func coversFromConfig(covers []CoverConfig) []entity.Cover {
	converted := make([]entity.Cover, 0, len(covers))
	for _, cover := range covers {
		converted = append(converted, entity.Cover{
			ID:         entity.CoverID(cover.ID),
			Name:       cover.Name,
			OpenRelay:  entity.RelayID(cover.OpenRelay),
			CloseRelay: entity.RelayID(cover.CloseRelay),
		})
	}
	return converted
}

func bindingsFromConfig(bindings []BindingConfig) []entity.Binding {
	converted := make([]entity.Binding, 0, len(bindings))
	for _, binding := range bindings {
		converted = append(converted, bindingFromConfig(binding))
	}
	return converted
}

func modbusFromConfig(modbus ModbusConfig) entity.Modbus {
	converted := entity.Modbus{
		Mode:              entity.ModbusMode(modbus.Mode),
		Port:              modbus.Port,
		BaudRate:          modbus.BaudRate,
		Timeout:           modbus.Timeout,
		PollInterval:      modbus.PollInterval,
		UnitID:            modbus.UnitID,
		EventSignals:      modbusEventSignalsFromConfig(modbus.EventSignals),
		StatePoints:       modbusStatePointsFromConfig(modbus.StatePoints),
		EventSignalWrites: modbusEventSignalWritesFromConfig(modbus.EventSignalWrites),
		StatePolls:        modbusStatePollsFromConfig(modbus.StatePolls),
	}
	return converted
}

func modbusEventSignalsFromConfig(signals []ModbusEventSignalConfig) []entity.ModbusEventSignal {
	converted := make([]entity.ModbusEventSignal, 0, len(signals))
	for _, signal := range signals {
		converted = append(converted, entity.ModbusEventSignal{
			ID:     entity.ModbusEventSignalID(signal.ID),
			Coil:   signal.Coil,
			Source: entity.ID(signal.Source),
			Target: entity.ID(signal.Target),
			Action: entity.Action(signal.Action),
		})
	}
	return converted
}

func modbusStatePointsFromConfig(points []ModbusStatePointConfig) []entity.ModbusStatePoint {
	converted := make([]entity.ModbusStatePoint, 0, len(points))
	for _, point := range points {
		converted = append(converted, entity.ModbusStatePoint{
			ID:     entity.ModbusStatePointID(point.ID),
			Coil:   point.Coil,
			Entity: entity.ID(point.Entity),
		})
	}
	return converted
}

func modbusEventSignalWritesFromConfig(writes []ModbusEventSignalWriteConfig) []entity.ModbusEventSignalWrite {
	converted := make([]entity.ModbusEventSignalWrite, 0, len(writes))
	for _, write := range writes {
		converted = append(converted, entity.ModbusEventSignalWrite{
			Unit:   write.Unit,
			Signal: entity.ModbusEventSignalID(write.Signal),
			Source: entity.ID(write.Source),
			Target: entity.ID(write.Target),
			Action: entity.Action(write.Action),
			UnitID: write.UnitID,
			Coil:   write.Coil,
		})
	}
	return converted
}

func modbusStatePollsFromConfig(polls []ModbusStatePollConfig) []entity.ModbusStatePoll {
	converted := make([]entity.ModbusStatePoll, 0, len(polls))
	for _, poll := range polls {
		converted = append(converted, entity.ModbusStatePoll{
			Unit:   poll.Unit,
			Point:  entity.ModbusStatePointID(poll.Point),
			Entity: entity.ID(poll.Entity),
			UnitID: poll.UnitID,
			Coil:   poll.Coil,
		})
	}

	return converted
}

func bindingFromConfig(binding BindingConfig) entity.Binding {
	return entity.Binding{
		Source:             entity.ID(binding.Source),
		Target:             entity.ID(binding.Target),
		Action:             entity.Action(binding.Action),
		ExecutionTransport: entity.ExecutionTransport(binding.ExecutionTransport),
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
