package nest

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func sysfsDevices(root *entity.Root, index *registry.Index) ([]*sysfs.Device, error) {
	slog.Info("crawling sysfs device tree", "root", root.SysfsRoot)
	devices, err := sysfs.ListDevices(root.SysfsRoot)
	if err != nil {
		return nil, fmt.Errorf("crawl sysfs: %w", err)
	}

	configured, missing := configuredDevices(devices, registry.SysfsDeviceIDs(index))
	if len(missing) > 0 {
		return nil, missingDevicesError(missing)
	}

	return configured, nil
}

func missingDevicesError(missing []entity.SysfsDeviceID) error {
	var errs error
	for _, deviceID := range missing {
		slog.Error("configured device not found in sysfs", "device_id", deviceID)
		errs = errors.Join(errs, fmt.Errorf("configured device %q not found in sysfs", deviceID))
	}

	return errs
}

func logSysfsDevices(devices []*sysfs.Device) {
	slog.Info("configured devices", "count", len(devices))
	for _, device := range devices {
		slog.Info("configured device", "identifier", device.Identifier, "path", device.Path)
	}
}

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
