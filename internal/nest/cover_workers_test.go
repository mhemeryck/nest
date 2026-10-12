package nest

import (
	"context"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverWorkerPartitionAndPolling(t *testing.T) {
	reg := registry.Build(&entity.Root{
		SysfsPollIntervals: entity.PollIntervals{RelayOutput: 2 * time.Second, DigitalInput: 100 * time.Millisecond},
		Covers: []entity.Cover{
			{ID: "a", OpenRelay: "a_open", CloseRelay: "a_close"},
			{ID: "b", OpenRelay: "b_open", CloseRelay: "b_close"},
		},
		Relays: []entity.Relay{
			{ID: "a_open", SysfsDevice: "ro_1_1"}, {ID: "a_close", SysfsDevice: "ro_1_2"},
			{ID: "b_open", SysfsDevice: "ro_1_3"}, {ID: "b_close", SysfsDevice: "ro_1_4"},
			{ID: "light", SysfsDevice: "ro_1_5"},
		},
	})
	devices := []*sysfs.Device{
		{Identifier: "ro_1_1", Type: sysfs.RelayOutput}, {Identifier: "ro_1_2", Type: sysfs.RelayOutput},
		{Identifier: "ro_1_3", Type: sysfs.RelayOutput}, {Identifier: "ro_1_4", Type: sysfs.RelayOutput},
		{Identifier: "ro_1_5", Type: sysfs.RelayOutput}, {Identifier: "di_1_1", Type: sysfs.DigitalInput},
	}
	workers := sysfsWorkerConfigs(reg, devices)
	require.Len(t, workers, 4)
	owners := make(map[string]int)
	for i, worker := range workers {
		for _, device := range worker.Devices {
			_, duplicate := owners[device.Identifier]
			assert.False(t, duplicate, "device must have one execution and polling owner")
			owners[device.Identifier] = i
			wantInterval := 2 * time.Second
			if device.Type == sysfs.DigitalInput {
				wantInterval = 100 * time.Millisecond
			}
			assert.Equal(t, wantInterval, worker.Interval)
		}
	}
	assert.Len(t, owners, len(devices))
	assert.Equal(t, owners["ro_1_1"], owners["ro_1_2"])
	assert.Equal(t, owners["ro_1_3"], owners["ro_1_4"])
	assert.NotEqual(t, owners["ro_1_1"], owners["ro_1_3"])
	assert.NotEqual(t, owners["ro_1_1"], owners["ro_1_5"])
}

func TestConfiguredSysfsActorTerminatesOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	actor := sysfsActor{commands: make(chan sysfs.Command, 1), states: make(chan sysfs.StateChange, 1), done: make(chan struct{})}
	startSysfsActor(ctx, actor)
	cancel()
	select {
	case <-actor.done:
	case <-time.After(time.Second):
		require.FailNow(t, "configured sysfs actor failed to stop")
	}
}
