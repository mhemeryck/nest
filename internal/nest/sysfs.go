package nest

import "github.com/mhemeryck/nest/internal/sysfs"

func sysfsChannels() (chan sysfs.Command, chan sysfs.StateChange, chan struct{}) {
	commands := make(chan sysfs.Command, 32)
	states := make(chan sysfs.StateChange, 32)
	done := make(chan struct{})

	return commands, states, done
}
