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

func Run(parent context.Context, devices []*Device, commands <-chan Command, states chan<- PollEvent) func() {
	ctx, cancel := context.WithCancel(parent)
	configs := buildWorkerConfigs(devices)
	doneCh := startWorkers(ctx, configs, commands, states)

	return func() {
		cancel()
		<-doneCh
	}
}
