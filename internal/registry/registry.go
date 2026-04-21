package registry

import (
	"slices"

	"github.com/mhemeryck/nest/internal/entity"
)

type Index struct {
	DigitalInputsByDevice map[entity.DeviceID]entity.DigitalInput
	PushButtonsByID       map[entity.PushButtonID]entity.PushButton
	PushButtonsByInputID  map[entity.DigitalInputID][]entity.PushButton
	LightsByID            map[entity.LightID]entity.Light
	BindingsByButtonID    map[entity.PushButtonID][]entity.Binding
	RelaysByID            map[entity.RelayID]entity.Relay
	RelaysByDevice        map[entity.DeviceID]entity.Relay
}

func Build(root *entity.Root) *Index {
	index := &Index{
		DigitalInputsByDevice: make(map[entity.DeviceID]entity.DigitalInput, len(root.DigitalInputs)),
		PushButtonsByID:       make(map[entity.PushButtonID]entity.PushButton, len(root.PushButtons)),
		PushButtonsByInputID:  make(map[entity.DigitalInputID][]entity.PushButton),
		LightsByID:            make(map[entity.LightID]entity.Light, len(root.Lights)),
		BindingsByButtonID:    make(map[entity.PushButtonID][]entity.Binding),
		RelaysByID:            make(map[entity.RelayID]entity.Relay, len(root.Relays)),
		RelaysByDevice:        make(map[entity.DeviceID]entity.Relay, len(root.Relays)),
	}

	for _, input := range root.DigitalInputs {
		index.DigitalInputsByDevice[input.Device] = input
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
		index.RelaysByDevice[relay.Device] = relay
	}

	for _, binding := range root.Bindings {
		index.BindingsByButtonID[binding.Button] = append(index.BindingsByButtonID[binding.Button], binding)
	}

	return index
}

func DeviceIDs(index *Index) []entity.DeviceID {
	deviceIDs := make([]entity.DeviceID, 0, len(index.DigitalInputsByDevice)+len(index.RelaysByDevice))
	for deviceID := range index.DigitalInputsByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}
	for deviceID := range index.RelaysByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}

	slices.Sort(deviceIDs)

	return deviceIDs
}
