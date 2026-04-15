package sysfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartStopWorkers(t *testing.T) {
	configs := []WorkerConfig{
		{Interval: 10 * time.Millisecond, Paths: []string{"/tmp/test1"}},
	}

	events := make([]PollEvent, 0)
	onEvent := func(e PollEvent) {
		events = append(events, e)
	}

	stopChs := StartWorkers(configs, onEvent)
	require.Len(t, stopChs, 1)

	StopWorkers(stopChs)
}

func TestPollWorkerDetectsChange(t *testing.T) {
	tmp := t.TempDir()
	path1 := filepath.Join(tmp, "gpio1")
	path2 := filepath.Join(tmp, "gpio2")

	err := WriteFileValue(path1, "0")
	require.NoError(t, err)
	err = WriteFileValue(path2, "0")
	require.NoError(t, err)

	configs := []WorkerConfig{
		{Interval: 20 * time.Millisecond, Paths: []string{path1, path2}},
	}

	var events []PollEvent
	onEvent := func(e PollEvent) {
		events = append(events, e)
	}

	stopChs := StartWorkers(configs, onEvent)

	time.Sleep(50 * time.Millisecond)

	err = WriteFileValue(path1, "1")
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	StopWorkers(stopChs)

	foundRising := false
	for _, e := range events {
		if e.Path == path1 && e.IsRising && e.OldValue == 0 && e.NewValue == 1 {
			foundRising = true
			break
		}
	}
	assert.True(t, foundRising, "Should detect rising edge on path1")
}

func TestMatchDevices(t *testing.T) {
	paths := []string{
		"/sys/devices/platform/unipi_plc/io_group1/di_1_01/di_value",
		"/sys/devices/platform/unipi_plc/io_group1/do_1_01/do_value",
		"/sys/devices/platform/unipi_plc/io_group1/ro_1_01/ro_value",
		"/sys/devices/platform/unipi_plc/io_group1/ai_1_01/in_voltage0_raw",
		"/sys/devices/platform/unipi_plc/io_group1/leds/test/brightness",
	}

	matched := MatchDevices(paths)

	assert.Len(t, matched, 3)
	for _, m := range matched {
		switch m.Type {
		case DigitalInput:
			assert.Contains(t, m.Path, "di_1_01/di_value")
		case DigitalOutput:
			assert.Contains(t, m.Path, "do_1_01/do_value")
		case RelayOutput:
			assert.Contains(t, m.Path, "ro_1_01/ro_value")
		}
	}
}

func TestBuildWorkerConfigs(t *testing.T) {
	devices := []MatchedDevice{
		{Path: "/di1", Type: DigitalInput},
		{Path: "/di2", Type: DigitalInput},
		{Path: "/do1", Type: DigitalOutput},
		{Path: "/ro1", Type: RelayOutput},
	}

	configs := BuildWorkerConfigs(devices)

	assert.Len(t, configs, 3)

	configByType := make(map[DeviceType]WorkerConfig)
	for _, cfg := range configs {
		for _, path := range cfg.Paths {
			switch path {
			case "/di1", "/di2":
				configByType[DigitalInput] = cfg
			case "/do1":
				configByType[DigitalOutput] = cfg
			case "/ro1":
				configByType[RelayOutput] = cfg
			}
		}
	}

	diCfg, ok := configByType[DigitalInput]
	require.True(t, ok)
	assert.Len(t, diCfg.Paths, 2)
	assert.Equal(t, 20*time.Millisecond, diCfg.Interval)

	roCfg, ok := configByType[RelayOutput]
	require.True(t, ok)
	assert.Len(t, roCfg.Paths, 1)
	assert.Equal(t, 1*time.Second, roCfg.Interval)
}

func TestReadWriteDIValue(t *testing.T) {
	tmp := t.TempDir()
	devicePath := filepath.Join(tmp, "di_1_01")

	err := os.MkdirAll(devicePath, 0o755)
	require.NoError(t, err)

	err = WriteFileValue(filepath.Join(devicePath, "di_value"), "0")
	require.NoError(t, err)

	val, err := ReadDIValue(devicePath)
	require.NoError(t, err)
	assert.Equal(t, 0, val)

	err = WriteFileValue(filepath.Join(devicePath, "di_value"), "1")
	require.NoError(t, err)

	val, err = ReadDIValue(devicePath)
	require.NoError(t, err)
	assert.Equal(t, 1, val)
}

func TestDeviceTypeString(t *testing.T) {
	assert.Equal(t, "di", DigitalInput.String())
	assert.Equal(t, "do", DigitalOutput.String())
	assert.Equal(t, "ro", RelayOutput.String())
}
