package sysfs

import (
	"context"
	"errors"
	"fmt"
)

type StateChange struct {
	Device   Device
	OldValue Value
	NewValue Value
	IsRising bool
	Initial  bool
	// Cover replaces the device observation fields for a cover snapshot.
	Cover *CoverState
}

type CommandKind string

const (
	ToggleCommand CommandKind = "toggle"
	OnCommand     CommandKind = "on"
	OffCommand    CommandKind = "off"
	CoverCommand  CommandKind = "cover"
)

type Command struct {
	Kind     CommandKind
	DeviceID string
	Cover    *CoverRequest
}

func Run(
	ctx context.Context,
	devices []*Device,
	commands <-chan Command,
	states chan<- StateChange,
	done chan<- struct{},
	options ...Options,
) (err error) {
	defer close(done)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	localCovers, remaining, err := prepareCovers(devices, opts)
	if err != nil {
		return err
	}
	workerCommands := make(chan Command, 32)
	workersDone := startWorkers(ctx, buildWorkerConfigs(remaining, opts.PollIntervals), workerCommands, states)
	coverCommands := make(chan CoverRequest)
	coverDone := make(chan error, 1)
	go func() { coverDone <- runCovers(ctx, localCovers, opts.MaxRun, coverCommands, states) }()
	defer func() {
		cancel()
		<-workersDone
		if coverDone != nil {
			err = errors.Join(err, <-coverDone)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-coverDone:
			coverDone = nil
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("sysfs cover owner stopped unexpectedly")
		case command, ok := <-commands:
			if !ok {
				return nil
			}
			if command.Kind == CoverCommand {
				if command.Cover == nil {
					return fmt.Errorf("sysfs cover command has no request")
				}
				select {
				case <-ctx.Done():
					return nil
				case err := <-coverDone:
					coverDone = nil
					return err
				case coverCommands <- *command.Cover:
				}
				continue
			}
			if coverOwnsDevice(localCovers, command.DeviceID) {
				return fmt.Errorf("direct command to cover-owned output %q", command.DeviceID)
			}
			select {
			case <-ctx.Done():
				return nil
			case err := <-coverDone:
				coverDone = nil
				return err
			case workerCommands <- command:
			}
		}
	}
}
