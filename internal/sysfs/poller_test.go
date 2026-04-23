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

func TestRunStartsAndStopsWorkers(t *testing.T) {
	commands := make(chan Command, 32)
	states := make(chan PollEvent, 32)
	shutdown := Run([]*Device{{Path: "/tmp/test1", Identifier: "test1"}}, commands, states)

	shutdown()
	close(states)
}

func TestPollWorkerDetectsChange(t *testing.T) {
	tmp := t.TempDir()
	path1 := filepath.Join(tmp, "gpio1")
	path2 := filepath.Join(tmp, "gpio2")

	err := writeValue(path1, Off)
	require.NoError(t, err)
	err = writeValue(path2, Off)
	require.NoError(t, err)

	var mu sync.Mutex
	var events []PollEvent

	commands := make(chan Command, 32)
	states := make(chan PollEvent, 32)
	shutdown := Run([]*Device{{Path: path1, Identifier: "di_1_01"}, {Path: path2, Identifier: "di_1_02"}}, commands, states)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for event := range states {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		}
	}()

	time.Sleep(50 * time.Millisecond)

	err = writeValue(path1, On)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	shutdown()
	close(states)
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

func TestWorkerCommandTogglesRelay(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ro_value")
	require.NoError(t, writeValue(path, Off))

	commands := make(chan Command, 32)
	states := make(chan PollEvent, 32)
	shutdown := Run([]*Device{{Path: path, Identifier: "ro_1_01", Type: RelayOutput, Value: Off}}, commands, states)
	defer func() {
		shutdown()
		close(states)
	}()

	commands <- Command{Kind: ToggleCommand, DeviceID: "ro_1_01"}

	select {
	case event := <-states:
		assert.Equal(t, "ro_1_01", event.Device.Identifier)
		assert.Equal(t, Off, event.OldValue)
		assert.Equal(t, On, event.NewValue)
	case <-time.After(time.Second):
		t.Fatal("expected relay toggle event")
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(data))
}

func TestNewDevice(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "di_1_01", "di_value")
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)
	err = writeValue(path, On)
	require.NoError(t, err)

	device, ok, err := newDevice(path)
	require.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, DigitalInput, device.Type)
	assert.Equal(t, "di_1_01", device.Identifier)
	assert.Equal(t, On, device.Value)

	path = filepath.Join(tmp, "do_1_01", "do_value")
	err = os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)
	err = writeValue(path, Off)
	require.NoError(t, err)

	device, ok, err = newDevice(path)
	require.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, DigitalOutput, device.Type)
	assert.Equal(t, "do_1_01", device.Identifier)
	assert.Equal(t, Off, device.Value)

	path = filepath.Join(tmp, "ro_1_01", "ro_value")
	err = os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)
	err = writeValue(path, On)
	require.NoError(t, err)

	device, ok, err = newDevice(path)
	require.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, RelayOutput, device.Type)
	assert.Equal(t, "ro_1_01", device.Identifier)
	assert.Equal(t, On, device.Value)

	device, ok, err = newDevice("/sys/devices/platform/unipi_plc/io_group1/ai_1_01/in_voltage0_raw")
	assert.False(t, ok)
	assert.Nil(t, device)
	require.NoError(t, err)
}

func TestBuildWorkerConfigs(t *testing.T) {
	devices := []*Device{
		{Path: "/di1", Type: DigitalInput, Identifier: "di1"},
		{Path: "/di2", Type: DigitalInput, Identifier: "di2"},
		{Path: "/do1", Type: DigitalOutput, Identifier: "do1"},
		{Path: "/ro1", Type: RelayOutput, Identifier: "ro1"},
	}

	configs := buildWorkerConfigs(devices)

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

	err := writeValue(path, Off)
	require.NoError(t, err)

	val, err := readValue(path)
	require.NoError(t, err)
	assert.Equal(t, Off, val)

	err = writeValue(path, On)
	require.NoError(t, err)

	val, err = readValue(path)
	require.NoError(t, err)
	assert.Equal(t, On, val)
}

func TestReadValueRejectsInvalidByte(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "value")

	err := os.WriteFile(path, []byte("x\n"), 0o644)
	require.NoError(t, err)

	_, err = readValue(path)
	assert.Error(t, err)
}

func TestReadDeviceUpdatesDeviceValue(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "value")

	err := writeValue(path, Off)
	require.NoError(t, err)

	device := &Device{Path: path, Identifier: "di_1_01"}

	value, err := readDevice(device)
	require.NoError(t, err)
	assert.Equal(t, Off, value)
	assert.Equal(t, Off, device.Value)

	err = writeValue(path, On)
	require.NoError(t, err)

	value, err = readDevice(device)
	require.NoError(t, err)
	assert.Equal(t, On, value)
	assert.Equal(t, On, device.Value)
}
