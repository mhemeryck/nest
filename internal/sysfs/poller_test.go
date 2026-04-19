package sysfs

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartStopWorkers(t *testing.T) {
	configs := []WorkerConfig{
		{Interval: 10 * time.Millisecond, Devices: []*Device{{Path: "/tmp/test1", Identifier: "test1"}}},
	}

	stopChs, events := StartWorkers(configs)
	require.Len(t, stopChs, 1)
	require.NotNil(t, events)

	StopWorkers(stopChs)
}

func TestPollWorkerDetectsChange(t *testing.T) {
	tmp := t.TempDir()
	path1 := filepath.Join(tmp, "gpio1")
	path2 := filepath.Join(tmp, "gpio2")

	err := WriteValue(path1, Off)
	require.NoError(t, err)
	err = WriteValue(path2, Off)
	require.NoError(t, err)

	configs := []WorkerConfig{
		{Interval: 20 * time.Millisecond, Devices: []*Device{{Path: path1, Identifier: "di_1_01"}, {Path: path2, Identifier: "di_1_02"}}},
	}

	var mu sync.Mutex
	var events []PollEvent

	stopChs, eventCh := StartWorkers(configs)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for event := range eventCh {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		}
	}()

	time.Sleep(50 * time.Millisecond)

	err = WriteValue(path1, On)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	StopWorkers(stopChs)
	<-done

	foundRising := false
	mu.Lock()
	for _, e := range events {
		if e.Device.Path == path1 && e.Device.Identifier == "di_1_01" && e.IsRising && e.OldValue == Off && e.NewValue == On {
			foundRising = true
			break
		}
	}
	mu.Unlock()
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
	devices := []*Device{
		{Path: "/di1", Type: DigitalInput, Identifier: "di1"},
		{Path: "/di2", Type: DigitalInput, Identifier: "di2"},
		{Path: "/do1", Type: DigitalOutput, Identifier: "do1"},
		{Path: "/ro1", Type: RelayOutput, Identifier: "ro1"},
	}

	configs := BuildWorkerConfigs(devices)

	assert.Len(t, configs, 3)

	configByType := make(map[DeviceType]WorkerConfig)
	for _, cfg := range configs {
		for _, device := range cfg.Devices {
			switch device.Path {
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
	assert.Len(t, diCfg.Devices, 2)
	assert.Equal(t, 20*time.Millisecond, diCfg.Interval)

	roCfg, ok := configByType[RelayOutput]
	require.True(t, ok)
	assert.Len(t, roCfg.Devices, 1)
	assert.Equal(t, 1*time.Second, roCfg.Interval)
}

func TestReadValue(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "di_value")

	err := WriteValue(path, Off)
	require.NoError(t, err)

	val, err := ReadValue(path)
	require.NoError(t, err)
	assert.Equal(t, Off, val)

	err = WriteValue(path, On)
	require.NoError(t, err)

	val, err = ReadValue(path)
	require.NoError(t, err)
	assert.Equal(t, On, val)
}

func TestReadValueRejectsInvalidByte(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "value")

	err := os.WriteFile(path, []byte("x\n"), 0o644)
	require.NoError(t, err)

	_, err = ReadValue(path)
	assert.Error(t, err)
}

func TestDeviceTypeString(t *testing.T) {
	assert.Equal(t, "di", DigitalInput.String())
	assert.Equal(t, "do", DigitalOutput.String())
	assert.Equal(t, "ro", RelayOutput.String())
}

func TestReadDeviceUpdatesDeviceValue(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "value")

	err := WriteValue(path, Off)
	require.NoError(t, err)

	device := &Device{Path: path, Identifier: "di_1_01"}

	changed, oldValue, err := ReadDevice(device)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, Off, oldValue)
	assert.Equal(t, Off, device.Value)
	assert.True(t, device.Ready)

	err = WriteValue(path, On)
	require.NoError(t, err)

	changed, oldValue, err = ReadDevice(device)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, Off, oldValue)
	assert.Equal(t, On, device.Value)
}
