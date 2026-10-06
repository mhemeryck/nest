package nest

import (
	"context"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type sysfsActor struct {
	commands chan sysfs.Command
	states   chan sysfs.StateChange
	done     chan struct{}
	workers  []sysfs.WorkerConfig
}

func newSysfsActor(reg *registry.Registry) (sysfsActor, error) {
	devices, err := sysfsDevices(reg)
	if err != nil {
		return sysfsActor{}, err
	}
	logSysfsDevices(devices)

	return sysfsActor{
		commands: make(chan sysfs.Command, 32),
		states:   make(chan sysfs.StateChange, 32),
		done:     make(chan struct{}),
		workers:  sysfsWorkerConfigs(reg, devices),
	}, nil
}

func startSysfsActor(ctx context.Context, actor sysfsActor) {
	go sysfs.RunConfigured(ctx, actor.workers, actor.commands, actor.states, actor.done)
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

func sysfsWorkerConfigs(reg *registry.Registry, devices []*sysfs.Device) []sysfs.WorkerConfig {
	intervals := sysfsPollIntervals(reg)
	coverDevices := make(map[entity.CoverID][]*sysfs.Device)
	var otherDevices []*sysfs.Device
	for _, device := range devices {
		relay, found := registry.RelayBySysfsDevice(reg, entity.SysfsDeviceID(device.Identifier))
		if found {
			if cover, owned := registry.CoverByRelay(reg, relay.ID); owned {
				coverDevices[cover.ID] = append(coverDevices[cover.ID], device)
				continue
			}
		}
		otherDevices = append(otherDevices, device)
	}
	configs := sysfs.WorkerConfigs(otherDevices, intervals)
	for _, cover := range registry.Covers(reg) {
		configs = append(configs, sysfs.WorkerConfigs(coverDevices[cover.ID], intervals)...)
	}
	return configs
}
