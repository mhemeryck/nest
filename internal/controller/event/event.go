package event

import "github.com/mhemeryck/nest/internal/entity"

type Kind string

const (
	PushButtonKind Kind = "push_button"
	LightKind      Kind = "light"

	PushButtonPressed  = "pressed"
	PushButtonReleased = "released"
)

type Event struct {
	Kind       Kind
	PushButton *PushButton
	Light      *Light
}

type PushButton struct {
	ButtonID entity.PushButtonID
	Name     string
	Kind     string
}

type Light struct {
	LightID entity.LightID
	Name    string
	Action  entity.LightAction
}
