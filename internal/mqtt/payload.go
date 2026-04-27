package mqtt

import (
	"encoding/json"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/sysfs"
)

const (
	payloadOn  = "ON"
	payloadOff = "OFF"
)

type sysfsPayload struct {
	DeviceID string `json:"device_id"`
	Type     string `json:"type"`
	Old      string `json:"old"`
	New      string `json:"new"`
	Rising   bool   `json:"rising"`
	Falling  bool   `json:"falling"`
}

type inputEventPayload struct {
	InputID  string `json:"input_id"`
	DeviceID string `json:"device_id"`
	State    string `json:"state"`
	Rising   bool   `json:"rising"`
	Falling  bool   `json:"falling"`
}

type pushButtonPayload struct {
	ButtonID string `json:"button_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

func AvailabilityOnline(cfg Config) State {
	return State{Kind: AvailabilityState, Topic: AvailabilityTopic(cfg), Payload: []byte(payloadOn), Retain: true}
}

func AvailabilityOffline(cfg Config) State {
	return State{Kind: AvailabilityState, Topic: AvailabilityTopic(cfg), Payload: []byte(payloadOff), Retain: true}
}

func SysfsStateChange(cfg Config, stateChange sysfs.StateChange) State {
	return State{
		Kind:   SysfsState,
		Topic:  SysfsStateTopic(cfg, entity.DeviceID(stateChange.Device.Identifier)),
		Retain: true,
		Payload: marshal(sysfsPayload{
			DeviceID: stateChange.Device.Identifier,
			Type:     deviceType(stateChange.Device.Type),
			Old:      valuePayload(stateChange.OldValue),
			New:      valuePayload(stateChange.NewValue),
			Rising:   stateChange.IsRising,
			Falling:  stateChange.OldValue == sysfs.On && stateChange.NewValue == sysfs.Off,
		}),
	}
}

func DigitalInputStates(cfg Config, inputEvent event.DigitalInputEvent, value sysfs.Value) []State {
	return []State{
		{
			Kind:    InputState,
			Topic:   InputStateTopic(cfg, inputEvent.DeviceID),
			Payload: []byte(valuePayload(value)),
			Retain:  true,
		},
		{
			Kind:  InputState,
			Topic: InputEventTopic(cfg, inputEvent.DeviceID),
			Payload: marshal(inputEventPayload{
				InputID:  string(inputEvent.InputID),
				DeviceID: string(inputEvent.DeviceID),
				State:    valuePayload(value),
				Rising:   inputEvent.IsRising,
				Falling:  inputEvent.IsFalling,
			}),
		},
	}
}

func PushButtonEvent(cfg Config, buttonEvent event.PushButtonEvent) State {
	return State{
		Kind:  PushButtonState,
		Topic: PushButtonEventTopic(cfg, buttonEvent.ButtonID),
		Payload: marshal(pushButtonPayload{
			ButtonID: string(buttonEvent.ButtonID),
			Name:     buttonEvent.Name,
			Kind:     buttonEvent.Kind,
		}),
	}
}

func RelayStateChange(cfg Config, relay entity.Relay, value sysfs.Value) State {
	return State{
		Kind:    RelayState,
		Topic:   RelayStateTopic(cfg, relay.Device),
		Payload: []byte(valuePayload(value)),
		Retain:  true,
	}
}

func valuePayload(value sysfs.Value) string {
	if value == sysfs.On {
		return payloadOn
	}

	return payloadOff
}

func deviceType(deviceType sysfs.DeviceType) string {
	switch deviceType {
	case sysfs.DigitalInput:
		return "digital_input"
	case sysfs.DigitalOutput:
		return "digital_output"
	case sysfs.RelayOutput:
		return "relay"
	default:
		return "unknown"
	}
}

func marshal(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}

	return data
}
