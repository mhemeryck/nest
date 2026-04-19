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
