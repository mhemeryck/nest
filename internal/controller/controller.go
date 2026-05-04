package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func Run(
	ctx context.Context,
	index *registry.Index,
	sysfsCommands chan<- sysfs.Command,
	stateChanges <-chan sysfs.StateChange,
	done chan<- struct{},
) {
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			return
		case stateChange, ok := <-stateChanges:
			if !ok {
				return
			}

			handleSysfsStateChange(ctx, index, sysfsCommands, stateChange)
		}
	}
}

func handleSysfsStateChange(ctx context.Context, index *registry.Index, sysfsCommands chan<- sysfs.Command, stateChange sysfs.StateChange) {
	pushButtonEvents, handled := pushButtonEventsFromStateChange(index, stateChange)
	if handled {
		for _, pushButtonEvent := range pushButtonEvents {
			dispatchEvent(ctx, index, sysfsCommands, pushButtonEvent)
		}
		return
	}

	slog.Info(
		"state change",
		"identifier",
		stateChange.Device.Identifier,
		"path",
		stateChange.Device.Path,
		"old_value",
		sysfs.PrintableValue(stateChange.OldValue),
		"new_value",
		sysfs.PrintableValue(stateChange.NewValue),
		"rising",
		stateChange.IsRising,
	)
}
