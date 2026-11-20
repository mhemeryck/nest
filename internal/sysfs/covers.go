package sysfs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mhemeryck/nest/internal/controller/cover"
	"github.com/mhemeryck/nest/internal/entity"
)

type Options struct {
	PollIntervals PollIntervals
	Covers        []CoverConfig
	MaxRun        time.Duration
}

type CoverConfig struct {
	ID            entity.CoverID
	OpenDeviceID  string
	CloseDeviceID string
}

type CoverRequest struct {
	ID     entity.CoverID
	Action entity.CoverAction
}

// CoverState is a snapshot, not a relay command acknowledgement.
// On failure Movement remains the previous state, not a claim of confirmed stopping.
type CoverState struct {
	ID       entity.CoverID
	Movement cover.Movement
	Error    error
}

type localCover struct {
	id    entity.CoverID
	open  *Device
	close *Device
	state cover.State
}

// Separate device ownership before starting any worker.
func prepareCovers(devices []*Device, opts Options) ([]localCover, []*Device, error) {
	if len(opts.Covers) > 0 && opts.MaxRun <= 0 {
		return nil, nil, fmt.Errorf("cover maximum run duration must be positive")
	}
	byID := make(map[string]*Device, len(devices))
	for _, device := range devices {
		byID[device.Identifier] = device
	}
	owned := make(map[string]bool)
	ids := make(map[entity.CoverID]bool)
	var covers []localCover
	for _, cfg := range opts.Covers {
		if cfg.ID == "" || ids[cfg.ID] {
			return nil, nil, fmt.Errorf("missing or duplicate cover ID %q", cfg.ID)
		}
		ids[cfg.ID] = true
		for _, id := range []string{cfg.OpenDeviceID, cfg.CloseDeviceID} {
			device := byID[id]
			if device == nil || device.Type != RelayOutput {
				return nil, nil, fmt.Errorf("cover %q: missing relay output %q", cfg.ID, id)
			}
			if owned[device.Path] {
				return nil, nil, fmt.Errorf("cover %q: shared relay output %q", cfg.ID, id)
			}
			owned[device.Path] = true
		}
		covers = append(covers, localCover{id: cfg.ID, open: byID[cfg.OpenDeviceID], close: byID[cfg.CloseDeviceID]})
	}
	var remaining []*Device
	for _, device := range devices {
		if !owned[device.Path] {
			remaining = append(remaining, device)
		}
	}
	return covers, remaining, nil
}

func coverOwnsDevice(covers []localCover, id string) bool {
	for _, c := range covers {
		if c.open.Identifier == id || c.close.Identifier == id {
			return true
		}
	}
	return false
}

// One owner for requests, deadlines, state, and I/O across local covers.
// Pending snapshots coalesce by cover; publishing never blocks safety work.
func runCovers(ctx context.Context, covers []localCover, maxRun time.Duration, commands <-chan CoverRequest, states chan<- StateChange) (err error) {
	// Safe startup and cleanup on exit.
	if err := stopCovers(covers); err != nil {
		return fmt.Errorf("initialize covers: %w", err)
	}
	defer func() { err = errors.Join(err, stopCovers(covers)) }()

	// Initial snapshots and shared deadline timer.
	pending := make(map[entity.CoverID]CoverState)
	for _, c := range covers {
		pending[c.id] = CoverState{ID: c.id, Movement: cover.Stopped}
	}
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	// Expired movements before snapshot delivery or new requests.
	for {
		if ctx.Err() != nil {
			return nil
		}
		if c, err := stopExpiredCovers(ctx, covers, time.Now(), maxRun, pending); err != nil {
			return reportCoverFailure(ctx, states, c, err)
		}

		// Earliest deadline and next pending snapshot.
		var deadline <-chan time.Time
		if next := nextCoverDeadline(covers); !next.IsZero() {
			timer.Reset(time.Until(next))
			deadline = timer.C
		} else {
			timer.Stop()
		}
		var publish chan<- StateChange
		snapshot, ok := nextCoverSnapshot(covers, pending)
		if ok {
			publish = states
		}
		// Cancellation, deadline wakeup, snapshot delivery, or request execution.
		select {
		case <-ctx.Done():
			return nil
		case <-deadline:
			// Evaluate current deadlines at the top of the loop.
		case publish <- StateChange{Cover: &snapshot}:
			delete(pending, snapshot.ID)
		case request := <-commands:
			c := findLocalCover(covers, request.ID)
			if c == nil {
				return fmt.Errorf("unknown local cover %q", request.ID)
			}
			input, err := coverInput(request.Action)
			if err != nil {
				return reportCoverFailure(ctx, states, c, err)
			}
			if err := advanceCover(ctx, c, input, time.Now(), maxRun, pending); err != nil {
				return reportCoverFailure(ctx, states, c, err)
			}
		}
	}
}

func stopExpiredCovers(ctx context.Context, covers []localCover, now time.Time, maxRun time.Duration, pending map[entity.CoverID]CoverState) (*localCover, error) {
	for i := range covers {
		c := &covers[i]
		if c.state.Movement != cover.Stopped && !now.Before(c.state.Deadline) {
			if err := advanceCover(ctx, c, cover.CheckDeadline, now, maxRun, pending); err != nil {
				return c, err
			}
		}
	}
	return nil, nil
}

func nextCoverSnapshot(covers []localCover, pending map[entity.CoverID]CoverState) (CoverState, bool) {
	for _, c := range covers {
		if state, ok := pending[c.id]; ok {
			return state, true
		}
	}
	return CoverState{}, false
}

func advanceCover(ctx context.Context, c *localCover, input cover.Input, now time.Time, maxRun time.Duration, pending map[entity.CoverID]CoverState) error {
	result, err := cover.Transition(c.state, input, now, maxRun)
	if err != nil {
		return err
	}
	if result.Effect == cover.NoEffect {
		return nil
	}
	active := bothOff
	switch result.Effect {
	case cover.StartOpening:
		active = firstOn
	case cover.StartClosing:
		active = secondOn
	case cover.StopMovement:
	default:
		return fmt.Errorf("unsupported cover effect %d", result.Effect)
	}
	if err := executeExclusiveOutputs(ctx, c.open, c.close, active, writeAndConfirmOutput); err != nil {
		return err
	}
	c.state = result.State
	pending[c.id] = CoverState{ID: c.id, Movement: c.state.Movement}
	return nil
}

func stopCovers(covers []localCover) error {
	var errs error
	for _, c := range covers {
		if err := executeExclusiveOutputs(context.Background(), c.open, c.close, bothOff, writeAndConfirmOutput); err != nil {
			slog.Error("cover output cleanup failed", "cover_id", c.id, "error", err)
			errs = errors.Join(errs, fmt.Errorf("stop cover %q: %w", c.id, err))
		}
	}
	return errs
}

func reportCoverFailure(ctx context.Context, states chan<- StateChange, c *localCover, err error) error {
	if ctx.Err() != nil && err == ctx.Err() {
		return nil
	}
	slog.Error("cover execution failed", "cover_id", c.id, "error", err)
	select {
	case states <- StateChange{Cover: &CoverState{ID: c.id, Movement: c.state.Movement, Error: err}}:
	default:
		// Runtime exit also carries the failure when observation delivery is blocked.
	}
	return fmt.Errorf("cover %q: %w", c.id, err)
}

func nextCoverDeadline(covers []localCover) time.Time {
	var next time.Time
	for _, c := range covers {
		if !c.state.Deadline.IsZero() && (next.IsZero() || c.state.Deadline.Before(next)) {
			next = c.state.Deadline
		}
	}
	return next
}

func findLocalCover(covers []localCover, id entity.CoverID) *localCover {
	for i := range covers {
		if covers[i].id == id {
			return &covers[i]
		}
	}
	return nil
}

func coverInput(action entity.CoverAction) (cover.Input, error) {
	switch action {
	case entity.CoverActionOpen:
		return cover.Open, nil
	case entity.CoverActionClose:
		return cover.Close, nil
	case entity.CoverActionStop:
		return cover.Stop, nil
	default:
		return 0, fmt.Errorf("invalid cover action %q", action)
	}
}
