package config

import "time"

type Root struct {
	Sysfs         SysfsConfig          `yaml:"sysfs"`
	MQTT          MQTTConfig           `yaml:"mqtt"`
	DigitalInputs []DigitalInputConfig `yaml:"digital_inputs"`
	PushButtons   []PushButtonConfig   `yaml:"push_buttons"`
	Lights        []LightConfig        `yaml:"lights"`
	Relays        []RelayConfig        `yaml:"relays"`
	Bindings      []BindingConfig      `yaml:"bindings"`
}

type SysfsConfig struct {
	Root          string              `yaml:"root"`
	PollIntervals PollIntervalsConfig `yaml:"poll_intervals"`
}

type PollIntervalsConfig struct {
	DigitalInput  time.Duration `yaml:"digital_input"`
	DigitalOutput time.Duration `yaml:"digital_output"`
	RelayOutput   time.Duration `yaml:"relay_output"`
}

type MQTTConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	ClientID    string `yaml:"client_id"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	TopicPrefix string `yaml:"topic_prefix"`
	UnitID      string `yaml:"unit_id"`
}

type DigitalInputConfig struct {
	ID     string `yaml:"id"`
	Device string `yaml:"device"`
}

type PushButtonConfig struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Input string `yaml:"input"`
}

type RelayConfig struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Device string `yaml:"device"`
}

type LightConfig struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Relay    string `yaml:"relay"`
	Actuator string `yaml:"actuator"`
}

type BindingConfig struct {
	Button string `yaml:"button"`
	Light  string `yaml:"light"`
	Action string `yaml:"action"`
}

const BindingActionToggle = "toggle"
