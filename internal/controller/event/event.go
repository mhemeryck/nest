package event

import (
	"time"

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
	OutputResultKind       Kind = "output_result"
	InputFailureKind       Kind = "input_failure"
	IntegrationFailureKind Kind = "integration_failure"
	OutputCommandKind      Kind = "output_command"
	CoverObservationKind   Kind = "cover_observation"
	CoverStartIntentKind   Kind = "cover_start_intent"
	CoverStoppedKind       Kind = "cover_stopped"
	SessionStartedKind     Kind = "session_started"
	SessionStoppedKind     Kind = "session_stopped"
	MQTTConnectedKind      Kind = "mqtt_connected"
	MQTTConnectFailedKind  Kind = "mqtt_connect_failed"
	MQTTDisconnectedKind   Kind = "mqtt_disconnected"
	MQTTPublishedKind      Kind = "mqtt_published"
	MQTTPublishFailedKind  Kind = "mqtt_publish_failed"
)

type Event struct {
	Kind               Kind
	DigitalInput       *DigitalInput
	PushButton         *PushButton
	Relay              *Relay
	LightState         *LightState
	Light              *Light
	Cover              *Cover
	OutputResult       *OutputResult
	InputFailure       *InputFailure
	IntegrationFailure *IntegrationFailure
	OutputCommand      *OutputCommand
	CoverObservation   *CoverObservation
	MQTT               *MQTT
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
	Name    string
	Action  entity.CoverAction
}

type OutputResult struct {
	CommandID   entity.OutputCommandID
	SysfsDevice entity.SysfsDeviceID
	Action      entity.OutputAction
	CompletedAt time.Time
	Error       error
}

type InputFailure struct {
	InputID     entity.DigitalInputID
	SysfsDevice entity.SysfsDeviceID
	Error       error
}

type IntegrationFailure struct {
	Integration string
	Error       string
}

type OutputCommand struct {
	CommandID entity.OutputCommandID
	RelayID   entity.RelayID
	Action    entity.OutputAction
}

type CoverObservation struct {
	CoverID           entity.CoverID
	State             entity.CoverState
	EstimatedPosition *float64
	Available         bool
	Direction         entity.CoverAction
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
