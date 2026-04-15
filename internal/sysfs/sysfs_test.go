package sysfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileValue(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test")
	err := os.WriteFile(f, []byte("42\n"), 0o644)
	require.NoError(t, err)

	val, err := ReadFileValue(f)
	require.NoError(t, err)
	assert.Equal(t, "42", val)
}

func TestReadFileValueNotExist(t *testing.T) {
	_, err := ReadFileValue("/nonexistent/file")
	assert.Error(t, err)
}

func TestWriteFileValue(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test")

	err := WriteFileValue(f, "hello")
	require.NoError(t, err)

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(data))
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

func TestCrawlDeviceFixtures(t *testing.T) {
	fixtures := filepath.Join("..", "..", "test", "fixtures", "sys", "devices", "platform", "unipi_plc")

	entry, err := CrawlDevice(fixtures)
	require.NoError(t, err)
	assert.NotEmpty(t, entry.Path)
	assert.NotEmpty(t, entry.Children)
}

func TestListDevices(t *testing.T) {
	fixtures := filepath.Join("..", "..", "test", "fixtures", "sys")

	devices, err := ListDevices(fixtures)
	require.NoError(t, err)
	assert.NotEmpty(t, devices)
}
