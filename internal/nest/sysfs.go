package nest

import (
	"context"

	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type sysfsActor struct {
	commands      chan sysfs.Command
	states        chan sysfs.StateChange
	done          chan struct{}
	devices       []*sysfs.Device
	pollIntervals sysfs.PollIntervals
}

func newSysfsActor(reg *registry.Registry) (sysfsActor, error) {
	devices, err := sysfsDevices(reg)
	if err != nil {
		return sysfsActor{}, err
	}
	logSysfsDevices(devices)

	return sysfsActor{
		commands:      make(chan sysfs.Command, 32),
		states:        make(chan sysfs.StateChange, 32),
		done:          make(chan struct{}),
		devices:       devices,
		pollIntervals: sysfsPollIntervals(reg),
	}, nil
}

func startSysfsActor(ctx context.Context, actor sysfsActor) {
	go sysfs.Run(ctx, actor.devices, actor.commands, actor.states, actor.done, actor.pollIntervals)
}

func waitForSysfsActor(actor sysfsActor) {
	<-actor.done
	close(actor.states)
}

func sysfsPollIntervals(reg *registry.Registry) sysfs.PollIntervals {
	pollIntervals := registry.SysfsPollIntervals(reg)

	return sysfs.PollIntervals{
		DigitalInput:  pollIntervals.DigitalInput,
		DigitalOutput: pollIntervals.DigitalOutput,
		RelayOutput:   pollIntervals.RelayOutput,
	}
}
