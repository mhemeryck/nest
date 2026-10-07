package sysfs

import (
	"context"
	"fmt"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
)

type ReportKind string

const (
	ObservationReportKind  ReportKind = ""
	CompletionReportKind   ReportKind = "completion"
	InputFailureReportKind ReportKind = "input_failure"
)

type StateChange struct {
	Kind       ReportKind
	Completion *CommandCompletion
	Error      error
	Device     Device
	OldValue   Value
	NewValue   Value
	IsRising   bool
	Initial    bool
}

type CommandCompletion struct {
	Command     Command
	CompletedAt time.Time
	Error       error
}

type CommandKind string

const (
	ToggleCommand CommandKind = "toggle"
	OnCommand     CommandKind = "on"
	OffCommand    CommandKind = "off"
)

type Command struct {
	ID       entity.OutputCommandID
	Kind     CommandKind
	DeviceID string
}

// Nil result: admitted, awaiting execution confirmation
func AdmitCommand(commands chan<- Command, cmd Command) *CommandCompletion {
	select {
	case commands <- cmd:
		return nil
	default:
		return &CommandCompletion{Command: cmd, CompletedAt: time.Now(), Error: fmt.Errorf("sysfs command admission exhausted for device %q", cmd.DeviceID)}
	}
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
	runWorkers(ctx, configs, commands, states)
}

func RunConfigured(ctx context.Context, configs []WorkerConfig, commands <-chan Command, states chan<- StateChange, done chan<- struct{}) {
	defer close(done)
	runWorkers(ctx, configs, commands, states)
}

func runWorkers(ctx context.Context, configs []WorkerConfig, commands <-chan Command, states chan<- StateChange) {
	doneCh := startWorkers(ctx, configs, commands, states)
	<-doneCh
}

func pollIntervals(intervals []PollIntervals) PollIntervals {
	if len(intervals) == 0 {
		return PollIntervals{}
	}

	return intervals[0]
}
