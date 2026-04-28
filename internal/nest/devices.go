package nest

import (
	"slices"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func configuredDevices(devices []*sysfs.Device, wanted []entity.SysfsDeviceID) ([]*sysfs.Device, []entity.SysfsDeviceID) {
	byIdentifier := make(map[string]*sysfs.Device, len(devices))
	for _, device := range devices {
		byIdentifier[device.Identifier] = device
	}

	configured := make([]*sysfs.Device, 0, len(wanted))
	missing := make([]entity.SysfsDeviceID, 0)
	for _, identifier := range wanted {
		device, ok := byIdentifier[string(identifier)]
		if !ok {
			missing = append(missing, identifier)
			continue
		}
		configured = append(configured, device)
	}

	slices.Sort(missing)

	return configured, missing
}
