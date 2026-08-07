package config

import "time"

type Root struct {
	Sysfs         SysfsConfig          `yaml:"sysfs"`
	MQTT          MQTTConfig           `yaml:"mqtt"`
	Modbus        ModbusConfig         `yaml:"modbus"`
	DigitalInputs []DigitalInputConfig `yaml:"digital_inputs"`
	PushButtons   []PushButtonConfig   `yaml:"push_buttons"`
	Lights        []LightConfig        `yaml:"lights"`
	Relays        []RelayConfig        `yaml:"relays"`
	Bindings      []BindingConfig      `yaml:"bindings"`

	RemoteSourceBindings []BindingConfig `yaml:"remote_source_bindings"`
	RemoteTargetBindings []BindingConfig `yaml:"remote_target_bindings"`
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

type ModbusConfig struct {
	Mode              string                         `yaml:"mode"`
	Port              string                         `yaml:"port"`
	BaudRate          int                            `yaml:"baudrate"`
	Timeout           time.Duration                  `yaml:"timeout"`
	UnitID            int                            `yaml:"unit_id"`
	EventSignals      []ModbusEventSignalConfig      `yaml:"event_signals"`
	StatePoints       []ModbusStatePointConfig       `yaml:"state_points"`
	EventSignalWrites []ModbusEventSignalWriteConfig `yaml:"event_signal_writes"`
	StatePolls        []ModbusStatePollConfig        `yaml:"state_polls"`
}

type ModbusEventSignalConfig struct {
	ID     string `yaml:"id"`
	Coil   int    `yaml:"coil"`
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Action string `yaml:"action"`
}

type ModbusStatePointConfig struct {
	ID     string `yaml:"id"`
	Coil   int    `yaml:"coil"`
	Entity string `yaml:"entity"`
}

type ModbusEventSignalWriteConfig struct {
	Unit   string `yaml:"unit"`
	Signal string `yaml:"signal"`
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Action string `yaml:"action"`
	UnitID uint8  `yaml:"-"`
	Coil   uint16 `yaml:"-"`
}

type ModbusStatePollConfig struct {
	Unit   string `yaml:"unit"`
	Point  string `yaml:"point"`
	Entity string `yaml:"entity"`
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
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Relay string `yaml:"relay"`
}

type BindingConfig struct {
	Source             string `yaml:"source"`
	Target             string `yaml:"target"`
	Action             string `yaml:"action"`
	ExecutionTransport string `yaml:"execution_transport"`
}

const BindingActionToggle = "toggle"

const (
	ModbusModeMaster = "master"
	ModbusModeSlave  = "slave"
)
