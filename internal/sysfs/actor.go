package sysfs

import "context"

type StateChange struct {
	Device   Device
	OldValue Value
	NewValue Value
	IsRising bool
	Initial  bool
}

type CommandKind string

const (
	ToggleCommand CommandKind = "toggle"
	OnCommand     CommandKind = "on"
	OffCommand    CommandKind = "off"
)

type Command struct {
	Kind     CommandKind
	DeviceID string
}

func Run(
	ctx context.Context,
	devices []*Device,
	commands <-chan Command,
	states chan<- StateChange,
	done chan<- struct{},
	intervals ...PollIntervals,
) {
	defer close(done)

	configs := buildWorkerConfigs(devices, pollIntervals(intervals))
	doneCh := startWorkers(ctx, configs, commands, states)
	<-doneCh
}

func pollIntervals(intervals []PollIntervals) PollIntervals {
	if len(intervals) == 0 {
		return PollIntervals{}
	}

	return intervals[0]
}
