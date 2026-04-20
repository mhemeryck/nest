package registry

import (
	"sort"

	"github.com/mhemeryck/nest/internal/entity"
)

type Index struct {
	DigitalInputsByDevice map[string]entity.DigitalInput
	PushButtonsByInputID  map[string][]entity.PushButton
	RelaysByDevice        map[string]entity.Relay
}

func Build(root *entity.Root) *Index {
	index := &Index{
		DigitalInputsByDevice: make(map[string]entity.DigitalInput, len(root.DigitalInputs)),
		PushButtonsByInputID:  make(map[string][]entity.PushButton),
		RelaysByDevice:        make(map[string]entity.Relay, len(root.Relays)),
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
		deviceIDs = append(deviceIDs, deviceID)
	}
	for deviceID := range index.RelaysByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}

	sort.Strings(deviceIDs)
	return deviceIDs
}
