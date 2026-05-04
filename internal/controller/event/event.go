package event

import "github.com/mhemeryck/nest/internal/entity"

type Kind string

const (
	PushButtonPressedKind  Kind = "push_button_pressed"
	PushButtonReleasedKind Kind = "push_button_released"
	LightKind              Kind = "light"
)

type Event struct {
	Kind       Kind
	PushButton *PushButton
	Light      *Light
}

type PushButton struct {
	ButtonID entity.PushButtonID
	Name     string
}

type Light struct {
	LightID entity.LightID
	Name    string
	Action  entity.LightAction
}
