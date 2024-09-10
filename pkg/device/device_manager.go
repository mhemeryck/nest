package device

import (
	"io/fs"
	"path/filepath"
)

type DeviceManager struct {
	Devices []*Device
}

// NewDeviceManagerFromPath crawls given `path` for devices and accumulates them
func NewDeviceManagerFromPath(path string) (DeviceManager, error) {
	devices := make([]*Device, 0)

	err := filepath.WalkDir(path,
		func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			device, err := NewDeviceFromPath(p)
			// We're only interested if there's a match
			if err == nil {
				devices = append(devices, &device)
			}
			return nil
		})

	return DeviceManager{Devices: devices}, err
}
