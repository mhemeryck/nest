package config

import (
	"errors"
	"fmt"

	"github.com/mhemeryck/nest/internal/entity"
)

const (
	endpointActorSysfs            = "sysfs"
	endpointKindSysfsDigitalInput = "digital_input"
	endpointKindSysfsRelay        = "relay"
)

type GlobalRoot struct {
	Actors   GlobalActorsConfig    `yaml:"actors"`
	Units    map[string]UnitConfig `yaml:"units"`
	Bindings []GlobalBindingConfig `yaml:"bindings"`
}

type GlobalActorsConfig struct {
	MQTT GlobalMQTTConfig `yaml:"mqtt"`
}

type GlobalMQTTConfig struct {
	Broker MQTTConfig `yaml:"broker"`
}

type UnitConfig struct {
	Actors   UnitActorsConfig   `yaml:"actors"`
	Entities UnitEntitiesConfig `yaml:"entities"`
}

type UnitActorsConfig struct {
	MQTT   UnitMQTTConfig   `yaml:"mqtt"`
	Sysfs  UnitSysfsConfig  `yaml:"sysfs"`
	Modbus UnitModbusConfig `yaml:"modbus"`
}

type UnitModbusConfig = ModbusConfig

type UnitMQTTConfig struct {
	Enabled bool `yaml:"enabled"`
}

type UnitSysfsConfig struct {
	Root          string               `yaml:"root"`
	PollIntervals PollIntervalsConfig  `yaml:"poll_intervals"`
	DigitalInputs []DigitalInputConfig `yaml:"digital_inputs"`
	Relays        []RelayConfig        `yaml:"relays"`
}

type UnitEntitiesConfig struct {
	Buttons []UnitPushButtonConfig `yaml:"buttons"`
	Lights  []UnitLightConfig      `yaml:"lights"`
}

type GlobalBindingConfig struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Action string `yaml:"action"`
}

type EndpointRefConfig struct {
	Actor string `yaml:"actor"`
	Kind  string `yaml:"kind"`
	ID    string `yaml:"id"`
}

type UnitPushButtonConfig struct {
	ID    string            `yaml:"id"`
	Name  string            `yaml:"name"`
	Input EndpointRefConfig `yaml:"input"`
}

type UnitLightConfig struct {
	ID       string            `yaml:"id"`
	Name     string            `yaml:"name"`
	Actuator EndpointRefConfig `yaml:"actuator"`
}

func ProjectUnit(global *GlobalRoot, unitID string) (*Root, error) {
	if err := validateID("unit_id", unitID); err != nil {
		return nil, err
	}

	unit, ok := global.Units[unitID]
	if !ok {
		return nil, fmt.Errorf("unit_id: unknown unit %q", unitID)
	}
	if err := errors.Join(validateGlobalBindings(global), validateGlobalModbus(global)); err != nil {
		return nil, err
	}

	mqtt := global.Actors.MQTT.Broker
	mqtt.Enabled = unit.Actors.MQTT.Enabled
	mqtt.UnitID = unitID
	mqtt.ClientID = projectedMQTTClientID(mqtt.ClientID, unitID)

	localButtons, buttonErr := projectPushButtons(unitID, unit.Entities.Buttons)
	localLights, lightErr := projectLights(unitID, unit.Entities.Lights)
	if err := errors.Join(buttonErr, lightErr); err != nil {
		return nil, err
	}

	local := &Root{
		Sysfs: SysfsConfig{
			Root:          unit.Actors.Sysfs.Root,
			PollIntervals: unit.Actors.Sysfs.PollIntervals,
		},
		MQTT:          mqtt,
		Modbus:        cloneModbusConfig(unit.Actors.Modbus),
		DigitalInputs: append([]DigitalInputConfig(nil), unit.Actors.Sysfs.DigitalInputs...),
		PushButtons:   localButtons,
		Lights:        localLights,
		Relays:        append([]RelayConfig(nil), unit.Actors.Sysfs.Relays...),
	}

	for _, binding := range global.Bindings {
		sourceLocal := entity.IsIDForUnit(binding.Source, unitID, entity.TypeButton)
		targetLocal := entity.IsIDForUnit(binding.Target, unitID, entity.TypeLight)

		switch {
		case sourceLocal && targetLocal:
			local.Bindings = append(local.Bindings, BindingConfig(binding))
		case sourceLocal:
			local.RemoteSourceBindings = append(local.RemoteSourceBindings, BindingConfig(binding))
		case targetLocal:
			local.RemoteTargetBindings = append(local.RemoteTargetBindings, BindingConfig(binding))
		}
	}

	return local, nil
}

func cloneModbusConfig(modbus ModbusConfig) ModbusConfig {
	return ModbusConfig{
		Mode:              modbus.Mode,
		EventSignals:      append([]ModbusEventSignalConfig(nil), modbus.EventSignals...),
		StatePoints:       append([]ModbusStatePointConfig(nil), modbus.StatePoints...),
		EventSignalWrites: append([]ModbusEventSignalWriteConfig(nil), modbus.EventSignalWrites...),
		StatePolls:        append([]ModbusStatePollConfig(nil), modbus.StatePolls...),
	}
}

func projectedMQTTClientID(clientID string, unitID string) string {
	if clientID == "" {
		return ""
	}

	return fmt.Sprintf("%s-%s", clientID, unitID)
}

func projectPushButtons(unitID string, buttons []UnitPushButtonConfig) ([]PushButtonConfig, error) {
	projected := make([]PushButtonConfig, 0, len(buttons))
	var errs error
	for i, button := range buttons {
		inputID, err := projectEndpointRef(
			fmt.Sprintf("entities.buttons[%d].input", i),
			button.Input,
			endpointActorSysfs,
			endpointKindSysfsDigitalInput,
		)
		errs = errors.Join(errs, err)

		projected = append(projected, PushButtonConfig{
			ID:    string(entity.NewID(unitID, entity.TypeButton, button.ID)),
			Name:  button.Name,
			Input: inputID,
		})
	}

	return projected, errs
}

func projectLights(unitID string, lights []UnitLightConfig) ([]LightConfig, error) {
	projected := make([]LightConfig, 0, len(lights))
	var errs error
	for i, light := range lights {
		relayID, err := projectEndpointRef(
			fmt.Sprintf("entities.lights[%d].actuator", i),
			light.Actuator,
			endpointActorSysfs,
			endpointKindSysfsRelay,
		)
		errs = errors.Join(errs, err)

		projected = append(projected, LightConfig{
			ID:    string(entity.NewID(unitID, entity.TypeLight, light.ID)),
			Name:  light.Name,
			Relay: relayID,
		})
	}

	return projected, errs
}

func projectEndpointRef(field string, endpoint EndpointRefConfig, actor string, kind string) (string, error) {
	var errs error
	if endpoint.Actor != actor {
		errs = errors.Join(errs, fmt.Errorf("%s.actor: unsupported endpoint actor %q, expected %q", field, endpoint.Actor, actor))
	}
	if endpoint.Kind != kind {
		errs = errors.Join(errs, fmt.Errorf("%s.kind: unsupported endpoint kind %q, expected %q", field, endpoint.Kind, kind))
	}

	errs = errors.Join(errs, validateID(field+".id", endpoint.ID))
	return endpoint.ID, errs
}
