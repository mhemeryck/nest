package sysfs

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

func Run(devices []*Device, commands <-chan Command, states chan<- PollEvent) func() {
	configs := buildWorkerConfigs(devices)
	stopCh, doneCh := startWorkers(configs, commands, states)

	return func() {
		stopWorkers(stopCh, doneCh)
	}
}

func stopWorkers(stopCh chan<- struct{}, doneCh <-chan struct{}) {
	close(stopCh)
	<-doneCh
}
