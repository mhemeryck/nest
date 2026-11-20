package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/cover"
	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemanticCoverRequestThroughSysfs(t *testing.T) {
	id := entity.CoverID("unit.cover.office")
	reg := registry.Build(&entity.Root{Covers: []entity.Cover{{ID: id, OpenRelay: "up", CloseRelay: "down"}}})
	var devices []*sysfs.Device
	for _, name := range []string{"up", "down"} {
		path := filepath.Join(t.TempDir(), "ro_value")
		require.NoError(t, os.WriteFile(path, []byte("0\n"), 0o600))
		devices = append(devices, &sysfs.Device{Identifier: name, Path: path, Type: sysfs.RelayOutput, Value: sysfs.Off})
	}
	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan sysfs.Command, 8)
	states := make(chan sysfs.StateChange, 8)
	result := make(chan error, 1)
	go func() {
		result <- sysfs.Run(ctx, devices, commands, states, make(chan struct{}), sysfs.Options{
			Covers: []sysfs.CoverConfig{{ID: id, OpenDeviceID: "up", CloseDeviceID: "down"}}, MaxRun: time.Second,
		})
	}()
	t.Cleanup(func() { cancel(); assert.NoError(t, <-result) })
	startup := <-states
	require.NotNil(t, startup.Cover)
	assert.Equal(t, cover.Stopped, startup.Cover.Movement)
	derived := dispatchEvent(ctx, reg, commands, nil, nil, mqtt.Topics{}, event.Event{
		Kind: event.CoverKind, Cover: &event.Cover{CoverID: id, Action: entity.CoverActionOpen},
	})
	assert.Empty(t, derived)
	select {
	case observation := <-states:
		events, handled := semanticEventsFromStateChange(reg, observation)
		require.True(t, handled)
		require.Len(t, events, 1)
		assert.Equal(t, event.CoverStateKind, events[0].Kind)
		assert.Equal(t, &event.CoverState{CoverID: id, Movement: cover.Opening}, events[0].CoverState)
	case <-time.After(2 * time.Second):
		t.Fatal("missing semantic cover observation")
	}
}
