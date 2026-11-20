package event

import (
	"github.com/mhemeryck/nest/internal/controller/cover"
	"github.com/mhemeryck/nest/internal/entity"
)

type Kind string

const (
	DigitalInputStateKind  Kind = "digital_input_state"
	PushButtonPressedKind  Kind = "push_button_pressed"
	PushButtonReleasedKind Kind = "push_button_released"
	RelayStateKind         Kind = "relay_state"
	LightStateKind         Kind = "light_state"
	LightKind              Kind = "light"
	CoverKind              Kind = "cover"
	CoverStateKind         Kind = "cover_state"
	MQTTConnectedKind      Kind = "mqtt_connected"
	MQTTConnectFailedKind  Kind = "mqtt_connect_failed"
	MQTTDisconnectedKind   Kind = "mqtt_disconnected"
	MQTTPublishedKind      Kind = "mqtt_published"
	MQTTPublishFailedKind  Kind = "mqtt_publish_failed"
)

type Event struct {
	Kind         Kind
	DigitalInput *DigitalInput
	PushButton   *PushButton
	Relay        *Relay
	LightState   *LightState
	Light        *Light
	Cover        *Cover
	CoverState   *CoverState
	MQTT         *MQTT
}

type MQTT struct {
	PublishTopic string
	Error        string
}

type DigitalInput struct {
	InputID     entity.DigitalInputID
	SysfsDevice entity.SysfsDeviceID
	Value       int
}

type PushButton struct {
	ButtonID entity.PushButtonID
	Name     string
	Delivery entity.ExecutionTransport
}

type Light struct {
	LightID entity.LightID
	Name    string
	Action  entity.LightAction
}

type Cover struct {
	CoverID entity.CoverID
	Action  entity.CoverAction
}

type CoverState struct {
	CoverID  entity.CoverID
	Movement cover.Movement
	Position *int
	Error    error
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
