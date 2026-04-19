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

	val, err := ReadFileBytes(f)
	require.NoError(t, err)
	assert.Equal(t, []byte("42\n"), val)
}

func TestReadFileBytesNotExist(t *testing.T) {
	_, err := ReadFileBytes("/nonexistent/file")
	assert.Error(t, err)
}

func TestWriteValue(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test")

	err := WriteValue(f, On)
	require.NoError(t, err)

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(data))
}

func TestListDir(t *testing.T) {
	tmp := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmp, "file"), []byte(""), 0o644)
	require.NoError(t, err)

	entries, err := ListDir(tmp)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestWriteValueRejectsInvalidValue(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test")

	err := WriteValue(f, Value('2'))
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
