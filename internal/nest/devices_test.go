package nest

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
)

func TestConfiguredDevices(t *testing.T) {
	devices := []*sysfs.Device{
		{Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
		{Identifier: "ro_3_14", Path: "/sys/ro_3_14/ro_value"},
		{Identifier: "di_3_15", Path: "/sys/di_3_15/di_value"},
	}
	wanted := []entity.SysfsDeviceID{
		entity.SysfsDeviceID("ro_3_14"),
		entity.SysfsDeviceID("missing_2"),
		entity.SysfsDeviceID("di_3_16"),
		entity.SysfsDeviceID("missing_1"),
	}

	configured, missing := configuredDevices(devices, wanted)

	assert.Equal(t, []*sysfs.Device{devices[1], devices[0]}, configured)
	assert.Equal(t, []entity.SysfsDeviceID{entity.SysfsDeviceID("missing_1"), entity.SysfsDeviceID("missing_2")}, missing)
}
