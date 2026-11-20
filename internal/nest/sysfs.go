package nest

import (
	"context"
	"fmt"
	"time"

	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type sysfsActor struct {
	commands      chan sysfs.Command
	states        chan sysfs.StateChange
	done          chan struct{}
	devices       []*sysfs.Device
	pollIntervals sysfs.PollIntervals
	covers        []sysfs.CoverConfig
	maxRun        time.Duration
	result        chan error
}

func newSysfsActor(reg *registry.Registry) (sysfsActor, error) {
	devices, err := sysfsDevices(reg)
	if err != nil {
		return sysfsActor{}, err
	}
	logSysfsDevices(devices)
	var covers []sysfs.CoverConfig
	for _, cover := range registry.Covers(reg) {
		open, openOK := registry.RelayByID(reg, cover.OpenRelay)
		close, closeOK := registry.RelayByID(reg, cover.CloseRelay)
		if !openOK || !closeOK {
			return sysfsActor{}, fmt.Errorf("cover %q references missing relays", cover.ID)
		}
		covers = append(covers, sysfs.CoverConfig{ID: cover.ID, OpenDeviceID: string(open.SysfsDevice), CloseDeviceID: string(close.SysfsDevice)})
	}

	return sysfsActor{
		commands:      make(chan sysfs.Command, 32),
		states:        make(chan sysfs.StateChange, 32),
		done:          make(chan struct{}),
		devices:       devices,
		pollIntervals: sysfsPollIntervals(reg),
		covers:        covers,
		maxRun:        registry.CoverControl(reg).MaxRun,
		result:        make(chan error, 1),
	}, nil
}

func startSysfsActor(ctx context.Context, actor sysfsActor) {
	go func() {
		actor.result <- sysfs.Run(ctx, actor.devices, actor.commands, actor.states, actor.done, sysfs.Options{
			PollIntervals: actor.pollIntervals, Covers: actor.covers, MaxRun: actor.maxRun,
		})
	}()
}

func waitForSysfsActor(actor sysfsActor) error {
	<-actor.done
	close(actor.states)
	if actor.result != nil {
		return <-actor.result
	}
	return nil
}

func sysfsPollIntervals(reg *registry.Registry) sysfs.PollIntervals {
	pollIntervals := registry.SysfsPollIntervals(reg)

	return sysfs.PollIntervals{
		DigitalInput:  pollIntervals.DigitalInput,
		DigitalOutput: pollIntervals.DigitalOutput,
		RelayOutput:   pollIntervals.RelayOutput,
	}
}
