package sysfs

import (
	"math"
	"time"

	"github.com/mhemeryck/nest/internal/controller/cover"
	"github.com/mhemeryck/nest/internal/entity"
)

func estimateCoverPosition(c localCover, now time.Time, maxRun time.Duration) *float64 {
	if c.position == nil || c.state.Movement == cover.Stopped {
		return c.position
	}

	elapsed := max(0, now.Sub(c.state.Deadline.Add(-maxRun)))
	change := 100 * float64(elapsed) / float64(maxRun)
	var position float64
	switch c.state.Movement {
	case cover.Opening:
		position = min(100, *c.position+change)
	case cover.Closing:
		position = max(0, *c.position-change)
	default:
		panic("cover position requires a valid movement state")
	}
	return &position
}

// A confirmed stop at the deadline establishes an estimated endpoint.
func positionAfterStop(c localCover, now time.Time, maxRun time.Duration) *float64 {
	if c.state.Movement == cover.Stopped || now.Before(c.state.Deadline) {
		return estimateCoverPosition(c, now, maxRun)
	}
	var endpoint float64
	switch c.state.Movement {
	case cover.Opening:
		endpoint = 100
	case cover.Closing:
		endpoint = 0
	default:
		panic("cover position requires a valid movement state")
	}
	return &endpoint
}

func coverSnapshot(c localCover, now time.Time, maxRun time.Duration) CoverState {
	var position *int
	if estimate := estimateCoverPosition(c, now, maxRun); estimate != nil {
		percent := int(math.Round(*estimate))
		position = &percent
	}
	return CoverState{ID: c.id, Movement: c.state.Movement, Position: position}
}

func queueMovingPositions(covers []localCover, now time.Time, maxRun time.Duration, pending map[entity.CoverID]CoverState) {
	for _, c := range covers {
		if c.state.Movement != cover.Stopped && c.position != nil {
			pending[c.id] = coverSnapshot(c, now, maxRun)
		}
	}
}
