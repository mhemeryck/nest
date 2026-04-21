package controller

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunReturnsOnSignal(t *testing.T) {
	index := registry.Build(&entity.Root{})
	pollEvents := make(chan sysfs.PollEvent)
	sigCh := make(chan os.Signal, 1)
	done := make(chan struct{})

	go func() {
		Run(index, nil, pollEvents, sigCh)
		close(done)
	}()

	sigCh <- os.Interrupt

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after signal")
	}
}

func TestRunReturnsWhenPollEventsClose(t *testing.T) {
	index := registry.Build(&entity.Root{})
	pollEvents := make(chan sysfs.PollEvent)
	sigCh := make(chan os.Signal)
	done := make(chan struct{})

	go func() {
		Run(index, nil, pollEvents, sigCh)
		close(done)
	}()

	close(pollEvents)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after poll event channel closed")
	}
}

func TestHandlePollEventTogglesLightRelay(t *testing.T) {
	relayPath := filepath.Join(t.TempDir(), "ro_value")
	require.NoError(t, os.WriteFile(relayPath, []byte("0\n"), 0o644))

	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
		Lights:        []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Relays:        []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", Device: entity.DeviceID("ro_3_14")}},
		Bindings:      []entity.Binding{{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle}},
	})
	devicesByID := map[entity.DeviceID]*sysfs.Device{
		"ro_3_14": {Identifier: "ro_3_14", Path: relayPath, Value: sysfs.Off},
	}

	logs := captureLogs(t, func() {
		handlePollEvent(index, devicesByID, sysfs.PollEvent{
			Device:   sysfs.Device{Identifier: "di_3_16", Path: "/sys/di_3_16/di_value"},
			OldValue: sysfs.Off,
			NewValue: sysfs.On,
			IsRising: true,
		})
	})

	assert.Contains(t, logs, "digital input event")
	assert.Contains(t, logs, "push button event")
	assert.Contains(t, logs, "light toggled")

	data, err := os.ReadFile(relayPath)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(data))
}

func TestHandlePollEventLogsRawPollEventForUnknownDevice(t *testing.T) {
	index := registry.Build(&entity.Root{})

	logs := captureLogs(t, func() {
		handlePollEvent(index, nil, sysfs.PollEvent{
			Device:   sysfs.Device{Identifier: "ro_3_14", Path: "/sys/ro_3_14/ro_value"},
			OldValue: sysfs.Off,
			NewValue: sysfs.On,
			IsRising: true,
		})
	})

	assert.Contains(t, logs, "poll event")
	assert.Contains(t, logs, "ro_3_14")
	assert.NotContains(t, logs, "push button event")
}

func captureLogs(t *testing.T, fn func()) string {
	t.Helper()

	var buffer bytes.Buffer
	previous := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	fn()

	require.NotEmpty(t, buffer.String())
	return buffer.String()
}
