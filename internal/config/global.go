package config

import (
	"fmt"
	"strings"
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
	MQTT  UnitMQTTConfig  `yaml:"mqtt"`
	Sysfs UnitSysfsConfig `yaml:"sysfs"`
}

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
	Buttons []PushButtonConfig `yaml:"buttons"`
	Lights  []LightConfig      `yaml:"lights"`
}

type GlobalBindingConfig struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Action string `yaml:"action"`
}

func ProjectUnit(global *GlobalRoot, unitID string) (*Root, error) {
	if err := validateID("unit_id", unitID); err != nil {
		return nil, err
	}

	unit, ok := global.Units[unitID]
	if !ok {
		return nil, fmt.Errorf("unit_id: unknown unit %q", unitID)
	}

	mqtt := global.Actors.MQTT.Broker
	mqtt.Enabled = unit.Actors.MQTT.Enabled
	mqtt.UnitID = unitID

	localLights := make([]LightConfig, 0, len(unit.Entities.Lights))
	for _, light := range unit.Entities.Lights {
		if light.Relay == "" {
			light.Relay = light.Actuator
		}
		light.Actuator = ""
		localLights = append(localLights, light)
	}

	local := &Root{
		Sysfs: SysfsConfig{
			Root:          unit.Actors.Sysfs.Root,
			PollIntervals: unit.Actors.Sysfs.PollIntervals,
		},
		MQTT:          mqtt,
		DigitalInputs: append([]DigitalInputConfig(nil), unit.Actors.Sysfs.DigitalInputs...),
		PushButtons:   append([]PushButtonConfig(nil), unit.Entities.Buttons...),
		Lights:        localLights,
		Relays:        append([]RelayConfig(nil), unit.Actors.Sysfs.Relays...),
	}

	for _, binding := range global.Bindings {
		buttonID, ok := localSemanticID(unitID, "button", binding.Source)
		if !ok {
			continue
		}

		lightID, ok := localSemanticID(unitID, "light", binding.Target)
		if !ok {
			continue
		}

		local.Bindings = append(local.Bindings, BindingConfig{
			Button: buttonID,
			Light:  lightID,
			Action: binding.Action,
		})
	}

	return local, nil
}

func localSemanticID(unitID string, entityType string, value string) (string, bool) {
	prefix := unitID + "." + entityType + "."
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}

	return strings.TrimPrefix(value, prefix), true
}
