package nest

import (
	"context"

	"github.com/mhemeryck/nest/internal/entity"
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

func newSysfsActor(root *entity.Root, index *registry.Registry) (sysfsActor, error) {
	devices, err := sysfsDevices(root, index)
	if err != nil {
		return sysfsActor{}, err
	}
	logSysfsDevices(devices)

	return sysfsActor{
		commands:      make(chan sysfs.Command, 32),
		states:        make(chan sysfs.StateChange, 32),
		done:          make(chan struct{}),
		devices:       devices,
		pollIntervals: sysfsPollIntervals(root),
	}, nil
}

func startSysfsActor(ctx context.Context, actor sysfsActor) {
	go sysfs.Run(ctx, actor.devices, actor.commands, actor.states, actor.done, actor.pollIntervals)
}

func waitForSysfsActor(actor sysfsActor) {
	<-actor.done
	close(actor.states)
}

func sysfsPollIntervals(root *entity.Root) sysfs.PollIntervals {
	return sysfs.PollIntervals{
		DigitalInput:  root.SysfsPollIntervals.DigitalInput,
		DigitalOutput: root.SysfsPollIntervals.DigitalOutput,
		RelayOutput:   root.SysfsPollIntervals.RelayOutput,
	}
}
