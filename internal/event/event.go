package event

import (
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type Kind string

const (
	DigitalInputKind Kind = "digital_input"
	PushButtonKind   Kind = "push_button"

	PushButtonPressed = "pressed"
)

type Event struct {
	Kind         Kind
	DigitalInput *DigitalInputEvent
	PushButton   *PushButtonEvent
}

type DigitalInputEvent struct {
	InputID   entity.DigitalInputID
	DeviceID  entity.DeviceID
	IsRising  bool
	IsFalling bool
}

type PushButtonEvent struct {
	ButtonID entity.PushButtonID
	Name     string
	Kind     string
}

func StateChangeToDigitalInputEvent(index *registry.Index, stateChange sysfs.StateChange) (Event, bool) {
	input, ok := index.DigitalInputsByDevice[entity.DeviceID(stateChange.Device.Identifier)]
	if !ok {
		return Event{}, false
	}

	return Event{
		Kind: DigitalInputKind,
		DigitalInput: &DigitalInputEvent{
			InputID:   input.ID,
			DeviceID:  entity.DeviceID(stateChange.Device.Identifier),
			IsRising:  stateChange.IsRising,
			IsFalling: stateChange.OldValue == sysfs.On && stateChange.NewValue == sysfs.Off,
		},
	}, true
}

func DigitalInputEventToPushButtonEvents(index *registry.Index, inputEvent DigitalInputEvent) []Event {
	if !inputEvent.IsRising {
		return nil
	}

	buttons := index.PushButtonsByInputID[inputEvent.InputID]
	buttonEvents := make([]Event, 0, len(buttons))
	for _, button := range buttons {
		buttonEvents = append(buttonEvents, Event{
			Kind: PushButtonKind,
			PushButton: &PushButtonEvent{
				ButtonID: button.ID,
				Name:     button.Name,
				Kind:     PushButtonPressed,
			},
		})
	}

	return buttonEvents
}
