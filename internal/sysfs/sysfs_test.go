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

	err := WriteValue(f, 1)
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

	err := WriteValue(f, 2)
	assert.Error(t, err)
}

func TestListDevices(t *testing.T) {
	fixtures := filepath.Join("..", "..", "test", "fixtures", "sys")

	devices, err := ListDevices(fixtures)
	require.NoError(t, err)
	assert.NotEmpty(t, devices)
}

func TestListIOValueFilesIncludesRelayOutputs(t *testing.T) {
	fixtures := filepath.Join("..", "..", "test", "fixtures")

	paths, err := ListIOValueFiles(fixtures)
	require.NoError(t, err)
	assert.NotEmpty(t, paths)
	assert.Contains(t, paths, filepath.Join(fixtures, "sys", "devices", "platform", "unipi_plc", "io_group2", "ro_2_01", "ro_value"))
}
