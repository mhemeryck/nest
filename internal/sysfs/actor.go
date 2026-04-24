package sysfs

import "context"

type PollEvent struct {
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

func Run(ctx context.Context, devices []*Device, commands <-chan Command, states chan<- PollEvent, done chan<- struct{}) {
	defer close(done)

	configs := buildWorkerConfigs(devices)
	doneCh := startWorkers(ctx, configs, commands, states)
	<-doneCh
}
