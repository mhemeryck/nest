package sysfs

import "context"

type StateChange struct {
	Device   Device
	OldValue Value
	NewValue Value
	IsRising bool
}

type CommandKind string

const ToggleCommand CommandKind = "toggle"

type Command struct {
	Kind     CommandKind
	DeviceID string
}

func Run(ctx context.Context, devices []*Device, commands <-chan Command, states chan<- StateChange, done chan<- struct{}) {
	defer close(done)

	configs := buildWorkerConfigs(devices)
	doneCh := startWorkers(ctx, configs, commands, states)
	<-doneCh
}
