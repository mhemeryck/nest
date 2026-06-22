package entity

import (
	"time"
)

type (
	SysfsDeviceID       string
	DigitalInputID      string
	PushButtonID        string
	LightID             string
	RelayID             string
	Action              string
	LightAction         string
	ModbusMode          string
	ModbusEventSignalID string
	ModbusStatePointID  string
)

const (
	ActionToggle      Action      = "toggle"
	LightActionToggle LightAction = "toggle"
	LightActionOn     LightAction = "on"
	LightActionOff    LightAction = "off"
	ModbusModeMaster  ModbusMode  = "master"
	ModbusModeSlave   ModbusMode  = "slave"
)

type Root struct {
	SysfsRoot            string
	SysfsPollIntervals   PollIntervals
	MQTT                 MQTT
	Modbus               Modbus
	DigitalInputs        []DigitalInput
	PushButtons          []PushButton
	Lights               []Light
	Relays               []Relay
	Bindings             []Binding
	RemoteSourceBindings []Binding
	RemoteTargetBindings []Binding
}

type PollIntervals struct {
	DigitalInput  time.Duration
	DigitalOutput time.Duration
	RelayOutput   time.Duration
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

type Modbus struct {
	Mode              ModbusMode
	EventSignals      []ModbusEventSignal
	StatePoints       []ModbusStatePoint
	EventSignalWrites []ModbusEventSignalWrite
	StatePolls        []ModbusStatePoll
}

type ModbusEventSignal struct {
	ID     ModbusEventSignalID
	Source ID
	Target ID
	Action Action
}

type ModbusStatePoint struct {
	ID     ModbusStatePointID
	Entity ID
}

type ModbusEventSignalWrite struct {
	Unit   string
	Signal ModbusEventSignalID
	Source ID
	Target ID
	Action Action
}

type ModbusStatePoll struct {
	Unit   string
	Point  ModbusStatePointID
	Entity ID
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
	Source ID
	Target ID
	Action Action
}
