package nest

import (
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func sysfsChannels() (chan sysfs.Command, chan sysfs.StateChange, chan struct{}) {
	commands := make(chan sysfs.Command, 32)
	states := make(chan sysfs.StateChange, 32)
	done := make(chan struct{})

	return commands, states, done
}

func sysfsPollIntervals(root *entity.Root) sysfs.PollIntervals {
	return sysfs.PollIntervals{
		DigitalInput:  root.SysfsPollIntervals.DigitalInput,
		DigitalOutput: root.SysfsPollIntervals.DigitalOutput,
		RelayOutput:   root.SysfsPollIntervals.RelayOutput,
	}
}
