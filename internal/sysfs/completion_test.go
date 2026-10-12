package sysfs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandCompletionForUnchangedWrite(t *testing.T) {
	device := &Device{Identifier: "relay", Path: filepath.Join(t.TempDir(), "value"), Value: Off}
	require.NoError(t, writeValue(device.Path, Off))
	states := make(chan StateChange, 2)
	cmd := Command{ID: 123, DeviceID: "relay", Kind: OffCommand}
	before := time.Now()
	handleCommand(map[string]*Device{"relay": device}, cmd, t.Context(), states)
	report := <-states
	require.Equal(t, CompletionReportKind, report.Kind)
	require.NotNil(t, report.Completion)
	assert.Equal(t, cmd, report.Completion.Command)
	assert.NoError(t, report.Completion.Error)
	assert.False(t, report.Completion.CompletedAt.Before(before))
	assert.False(t, report.Completion.CompletedAt.After(time.Now()))
	assert.Empty(t, states)
}

func TestFailedWriteReturnsMatchingCompletion(t *testing.T) {
	device := &Device{Identifier: "relay", Path: t.TempDir(), Value: Off}
	states := make(chan StateChange, 2)
	cmd := Command{ID: 456, DeviceID: "relay", Kind: OffCommand}
	handleCommand(map[string]*Device{"relay": device}, cmd, t.Context(), states)
	report := <-states
	require.Equal(t, CompletionReportKind, report.Kind)
	assert.Equal(t, cmd, report.Completion.Command)
	assert.Error(t, report.Completion.Error)
	assert.ErrorContains(t, report.Completion.Error, "write device relay")
	assert.False(t, report.Completion.CompletedAt.IsZero())
}

func TestExplicitWritesDoNotRequireValidRead(t *testing.T) {
	for _, kind := range []CommandKind{OnCommand, OffCommand} {
		t.Run(string(kind), func(t *testing.T) {
			device := &Device{Identifier: "relay", Path: filepath.Join(t.TempDir(), "missing_value")}
			_, err := readValue(device.Path)
			require.Error(t, err)
			states := make(chan StateChange, 2)
			handleCommand(map[string]*Device{"relay": device}, Command{ID: 7, DeviceID: "relay", Kind: kind}, t.Context(), states)
			<-states // Changed-value observation
			report := <-states
			require.Equal(t, CompletionReportKind, report.Kind)
			assert.NoError(t, report.Completion.Error)
			value, err := readValue(device.Path)
			require.NoError(t, err)
			want := On
			if kind == OffCommand {
				want = Off
			}
			assert.Equal(t, want, value)
		})
	}
}

func TestWorkerExecutesOnBeforeFollowingOff(t *testing.T) {
	device := &Device{Identifier: "relay", Type: RelayOutput, Path: filepath.Join(t.TempDir(), "value"), Value: Off}
	require.NoError(t, writeValue(device.Path, Off))
	commands := make(chan Command, 2)
	states := make(chan StateChange, 4)
	commands <- Command{ID: 1, DeviceID: "relay", Kind: OnCommand}
	commands <- Command{ID: 2, DeviceID: "relay", Kind: OffCommand}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		pollWorker(ctx, WorkerConfig{Interval: time.Hour, Devices: []*Device{device}}, commands, states)
	}()
	defer func() { cancel(); <-done }()
	for _, want := range []Value{On, Off} {
		select {
		case report := <-states:
			require.Equal(t, ObservationReportKind, report.Kind)
			assert.Equal(t, want, report.NewValue)
		case <-time.After(time.Second):
			require.FailNow(t, "missing write observation")
		}
		select {
		case report := <-states:
			require.Equal(t, CompletionReportKind, report.Kind)
			assert.NoError(t, report.Completion.Error)
			id := uint64(1)
			if want == Off {
				id = 2
			}
			assert.EqualValues(t, id, report.Completion.Command.ID)
		case <-time.After(time.Second):
			require.FailNow(t, "missing command completion")
		}
	}
	value, err := readValue(device.Path)
	require.NoError(t, err)
	assert.Equal(t, Off, value)
}

func TestOneCoverWorkerPreservesOrderAcrossBothRelays(t *testing.T) {
	devices := []*Device{
		{Identifier: "open", Type: RelayOutput, Path: filepath.Join(t.TempDir(), "open"), Value: Off},
		{Identifier: "close", Type: RelayOutput, Path: filepath.Join(t.TempDir(), "close"), Value: Off},
	}
	for _, device := range devices {
		require.NoError(t, writeValue(device.Path, Off))
	}
	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan Command, 3)
	states := make(chan StateChange, 8)
	done := make(chan struct{})
	go RunConfigured(ctx, []WorkerConfig{{Interval: time.Hour, Devices: devices}}, commands, states, done)
	defer func() { cancel(); <-done }()
	for range devices {
		select {
		case report := <-states:
			require.True(t, report.Initial)
		case <-time.After(time.Second):
			require.FailNow(t, "missing initial observation")
		}
	}
	commands <- Command{ID: 1, DeviceID: "open", Kind: OffCommand}
	commands <- Command{ID: 2, DeviceID: "close", Kind: OffCommand}
	commands <- Command{ID: 3, DeviceID: "open", Kind: OnCommand}
	var ids []uint64
	for len(ids) < 3 {
		select {
		case report := <-states:
			if report.Kind == CompletionReportKind {
				require.NoError(t, report.Completion.Error)
				ids = append(ids, uint64(report.Completion.Command.ID))
			}
		case <-time.After(time.Second):
			require.FailNow(t, "missing ordered completion")
		}
	}
	assert.Equal(t, []uint64{1, 2, 3}, ids)
}
