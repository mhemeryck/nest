package sysfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileBytes(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test")
	err := os.WriteFile(f, []byte("42\n"), 0o644)
	require.NoError(t, err)

	val, err := readFileBytes(f)
	require.NoError(t, err)
	assert.Equal(t, []byte("42\n"), val)
}

func TestReadFileBytesNotExist(t *testing.T) {
	_, err := readFileBytes("/nonexistent/file")
	assert.Error(t, err)
}

func TestWriteValue(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test")

	err := writeValue(f, On)
	require.NoError(t, err)

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(data))
}

func TestWriteValueRejectsInvalidValue(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test")

	err := writeValue(f, Value('2'))
	assert.Error(t, err)
}

func TestSetDeviceValue(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ro_value")
	device := &Device{Identifier: "ro_3_14", Path: path, Value: Off}

	err := SetDeviceValue(device, On)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(data))
	assert.Equal(t, Off, device.Value)
}

func TestToggleDevice(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ro_value")
	require.NoError(t, os.WriteFile(path, []byte("0\n"), 0o644))
	device := &Device{Identifier: "ro_3_14", Path: path, Value: Off}

	value, err := ToggleDevice(device)
	require.NoError(t, err)
	assert.Equal(t, On, value)
	assert.Equal(t, Off, device.Value)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(data))
}

func TestPrintableValue(t *testing.T) {
	assert.Equal(t, 0, PrintableValue(Off))
	assert.Equal(t, 1, PrintableValue(On))
	assert.Equal(t, int(Value('x')), PrintableValue(Value('x')))
}

func TestListDevices(t *testing.T) {
	fixtures := filepath.Join("..", "..", "test", "fixtures")

	devices, err := ListDevices(fixtures)
	require.NoError(t, err)
	assert.NotEmpty(t, devices)
	assert.Contains(t, devices[0].Path, "sys/devices/platform/unipi_plc/")
}

func TestListDevicesIncludesRelayOutputs(t *testing.T) {
	fixtures := filepath.Join("..", "..", "test", "fixtures")

	devices, err := ListDevices(fixtures)
	require.NoError(t, err)
	assert.NotEmpty(t, devices)

	found := false
	for _, device := range devices {
		if device.Path == filepath.Join(fixtures, "sys", "devices", "platform", "unipi_plc", "io_group2", "ro_2_01", "ro_value") {
			found = true
			assert.Equal(t, RelayOutput, device.Type)
			assert.Equal(t, "ro_2_01", device.Identifier)
			break
		}
	}

	assert.True(t, found)
}
