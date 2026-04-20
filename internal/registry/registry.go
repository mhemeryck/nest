package registry

import (
	"sort"

	"github.com/mhemeryck/nest/internal/entity"
)

type Index struct {
	DigitalInputsByDevice map[entity.DeviceID]entity.DigitalInput
	PushButtonsByInputID  map[entity.DigitalInputID][]entity.PushButton
	RelaysByDevice        map[entity.DeviceID]entity.Relay
}

func Build(root *entity.Root) *Index {
	index := &Index{
		DigitalInputsByDevice: make(map[entity.DeviceID]entity.DigitalInput, len(root.DigitalInputs)),
		PushButtonsByInputID:  make(map[entity.DigitalInputID][]entity.PushButton),
		RelaysByDevice:        make(map[entity.DeviceID]entity.Relay, len(root.Relays)),
	}

	for _, input := range root.DigitalInputs {
		index.DigitalInputsByDevice[input.Device] = input
	}

	for _, button := range root.PushButtons {
		index.PushButtonsByInputID[button.Input] = append(index.PushButtonsByInputID[button.Input], button)
	}

	for _, relay := range root.Relays {
		index.RelaysByDevice[relay.Device] = relay
	}

	return index
}

func DeviceIDs(index *Index) []string {
	deviceIDs := make([]string, 0, len(index.DigitalInputsByDevice)+len(index.RelaysByDevice))
	for deviceID := range index.DigitalInputsByDevice {
		deviceIDs = append(deviceIDs, string(deviceID))
	}
	for deviceID := range index.RelaysByDevice {
		deviceIDs = append(deviceIDs, string(deviceID))
	}

	sort.Strings(deviceIDs)
	return deviceIDs
}
