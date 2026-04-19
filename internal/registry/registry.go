package registry

import (
	"sort"

	"github.com/mhemeryck/nest/internal/config"
)

type Index struct {
	DigitalInputsByDevice map[string]config.DigitalInputConfig
	PushButtonsByInputID  map[string][]config.PushButtonConfig
	RelaysByDevice        map[string]config.RelayConfig
}

func Build(file *config.File) *Index {
	index := &Index{
		DigitalInputsByDevice: make(map[string]config.DigitalInputConfig, len(file.DigitalInputs)),
		PushButtonsByInputID:  make(map[string][]config.PushButtonConfig),
		RelaysByDevice:        make(map[string]config.RelayConfig, len(file.Relays)),
	}

	for _, input := range file.DigitalInputs {
		index.DigitalInputsByDevice[input.Device] = input
	}

	for _, button := range file.PushButtons {
		index.PushButtonsByInputID[button.Input] = append(index.PushButtonsByInputID[button.Input], button)
	}

	for _, relay := range file.Relays {
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
