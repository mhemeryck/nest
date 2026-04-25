package sysfs

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollWorkerDetectsChange(t *testing.T) {
	tmp := t.TempDir()
	path1 := filepath.Join(tmp, "gpio1")
	path2 := filepath.Join(tmp, "gpio2")

	err := writeValue(path1, Off)
	require.NoError(t, err)
	err = writeValue(path2, Off)
	require.NoError(t, err)

	var mu sync.Mutex
	var events []StateChange

	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan Command, 32)
	states := make(chan StateChange, 32)
	doneSysfs := make(chan struct{})
	go Run(ctx, []*Device{{Path: path1, Identifier: "di_1_01"}, {Path: path2, Identifier: "di_1_02"}}, commands, states, doneSysfs)

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

	cancel()
	<-doneSysfs
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

	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan Command, 32)
	states := make(chan StateChange, 32)
	doneSysfs := make(chan struct{})
	go Run(ctx, []*Device{{Path: path, Identifier: "ro_1_01", Type: RelayOutput, Value: Off}}, commands, states, doneSysfs)
	defer func() {
		cancel()
		<-doneSysfs
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

func TestHandleCommandIgnoresUnknownDevice(t *testing.T) {
	ctx := t.Context()
	states := make(chan StateChange, 1)
	handleCommand(map[string]*Device{}, Command{Kind: ToggleCommand, DeviceID: "missing"}, ctx, states)

	select {
	case event := <-states:
		t.Fatalf("unexpected state event: %+v", event)
	default:
	}
}

func TestHandleCommandIgnoresUnsupportedCommand(t *testing.T) {
	ctx := t.Context()
	states := make(chan StateChange, 1)
	device := &Device{Identifier: "ro_1_01", Path: filepath.Join(t.TempDir(), "ro_value"), Value: Off}
	require.NoError(t, writeValue(device.Path, Off))

	handleCommand(map[string]*Device{"ro_1_01": device}, Command{Kind: CommandKind("invalid"), DeviceID: "ro_1_01"}, ctx, states)

	select {
	case event := <-states:
		t.Fatalf("unexpected state event: %+v", event)
	default:
	}
	assert.Equal(t, Off, device.Value)
}

func TestPollDevicesIgnoresReadErrors(t *testing.T) {
	states := make(chan StateChange, 1)
	ctx := t.Context()
	pollDevices([]*Device{{Identifier: "missing", Path: filepath.Join(t.TempDir(), "missing")}}, ctx, states)

	select {
	case event := <-states:
		t.Fatalf("unexpected state event: %+v", event)
	default:
	}
}

func TestPublishStateReturnsFalseWhenStopping(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	states := make(chan StateChange)
	cancel()

	published := publishState(ctx, states, StateChange{})
	assert.False(t, published)
}

func TestHandleCommandReturnsDuringShutdownWhenStateChannelBlocks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	states := make(chan StateChange)
	device := &Device{Identifier: "ro_1_01", Path: filepath.Join(t.TempDir(), "ro_value"), Value: Off}
	require.NoError(t, writeValue(device.Path, Off))

	done := make(chan struct{})
	go func() {
		defer close(done)
		handleCommand(map[string]*Device{"ro_1_01": device}, Command{Kind: ToggleCommand, DeviceID: "ro_1_01"}, ctx, states)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleCommand did not return after shutdown")
	}
}

func TestHandleCommandLogsWriteFailure(t *testing.T) {
	var buffer bytes.Buffer
	previous := slog.Default()
	logger := slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelError}))
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	ctx := t.Context()
	states := make(chan StateChange, 1)
	device := &Device{Identifier: "ro_1_01", Path: filepath.Join(t.TempDir(), "missing", "ro_value"), Value: Off}

	handleCommand(map[string]*Device{"ro_1_01": device}, Command{Kind: ToggleCommand, DeviceID: "ro_1_01"}, ctx, states)

	assert.Contains(t, buffer.String(), "sysfs write failed")
	select {
	case event := <-states:
		t.Fatalf("unexpected state event: %+v", event)
	default:
	}
}
