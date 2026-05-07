package registry

import (
	"slices"

	"github.com/mhemeryck/nest/internal/entity"
)

type Index struct {
	DigitalInputsByDevice map[entity.SysfsDeviceID]entity.DigitalInput
	PushButtonsByID       map[entity.PushButtonID]entity.PushButton
	PushButtonsByInputID  map[entity.DigitalInputID][]entity.PushButton
	LightsByID            map[entity.LightID]entity.Light
	BindingsByButtonID    map[entity.PushButtonID][]entity.Binding
	RelaysByID            map[entity.RelayID]entity.Relay
	RelaysByDevice        map[entity.SysfsDeviceID]entity.Relay
}

func Build(root *entity.Root) *Index {
	index := &Index{
		DigitalInputsByDevice: make(map[entity.SysfsDeviceID]entity.DigitalInput, len(root.DigitalInputs)),
		PushButtonsByID:       make(map[entity.PushButtonID]entity.PushButton, len(root.PushButtons)),
		PushButtonsByInputID:  make(map[entity.DigitalInputID][]entity.PushButton),
		LightsByID:            make(map[entity.LightID]entity.Light, len(root.Lights)),
		BindingsByButtonID:    make(map[entity.PushButtonID][]entity.Binding),
		RelaysByID:            make(map[entity.RelayID]entity.Relay, len(root.Relays)),
		RelaysByDevice:        make(map[entity.SysfsDeviceID]entity.Relay, len(root.Relays)),
	}

	for _, input := range root.DigitalInputs {
		index.DigitalInputsByDevice[input.SysfsDevice] = input
	}

	for _, button := range root.PushButtons {
		index.PushButtonsByID[button.ID] = button
		index.PushButtonsByInputID[button.Input] = append(index.PushButtonsByInputID[button.Input], button)
	}

	for _, light := range root.Lights {
		index.LightsByID[light.ID] = light
	}

	for _, relay := range root.Relays {
		index.RelaysByID[relay.ID] = relay
		index.RelaysByDevice[relay.SysfsDevice] = relay
	}

	for _, binding := range root.Bindings {
		index.BindingsByButtonID[binding.Button] = append(index.BindingsByButtonID[binding.Button], binding)
	}

	return index
}

func SysfsDeviceIDs(index *Index) []entity.SysfsDeviceID {
	deviceIDs := make([]entity.SysfsDeviceID, 0, len(index.DigitalInputsByDevice)+len(index.RelaysByDevice))
	for deviceID := range index.DigitalInputsByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}
	for deviceID := range index.RelaysByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}

	slices.Sort(deviceIDs)

	return deviceIDs
}

func DigitalInputBySysfsDevice(index *Index, deviceID entity.SysfsDeviceID) (entity.DigitalInput, bool) {
	input, ok := index.DigitalInputsByDevice[deviceID]
	return input, ok
}

func PushButtonsByInput(index *Index, inputID entity.DigitalInputID) []entity.PushButton {
	return index.PushButtonsByInputID[inputID]
}

func BindingsByButton(index *Index, buttonID entity.PushButtonID) []entity.Binding {
	return index.BindingsByButtonID[buttonID]
}

func LightByID(index *Index, lightID entity.LightID) (entity.Light, bool) {
	light, ok := index.LightsByID[lightID]
	return light, ok
}

func RelayByID(index *Index, relayID entity.RelayID) (entity.Relay, bool) {
	relay, ok := index.RelaysByID[relayID]
	return relay, ok
}

func RelayBySysfsDevice(index *Index, deviceID entity.SysfsDeviceID) (entity.Relay, bool) {
	relay, ok := index.RelaysByDevice[deviceID]
	return relay, ok
}
