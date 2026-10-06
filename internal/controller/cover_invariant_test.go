package controller

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectionExclusivityAcrossCompetingRequestsAndLateWrites(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	queued := make(map[entity.CoverID][]event.OutputCommand)
	for _, id := range controller.order {
		completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: id, Action: entity.CoverActionOpen}, now)), now, hardware)
	}
	assert.True(t, hardware["a_open"])
	assert.True(t, hardware["b_open"])
	queueEffects := func(events []event.Event) {
		pending := coverOutputCommands(events)
		for len(pending) > 0 {
			command := pending[0]
			pending = pending[1:]
			cover, ok := registry.CoverByRelay(controller.reg, command.RelayID)
			require.True(t, ok)
			if len(queued[cover.ID]) >= 32 {
				failure := coverResult(controller, command, now, fmt.Errorf("worker admission exhausted"))
				pending = append(pending, coverOutputCommands(handleCoverOutputResult(controller, failure, now))...)
			} else {
				queued[cover.ID] = append(queued[cover.ID], command)
			}
		}
	}
	random := rand.New(rand.NewPCG(42, 73))
	actions := []entity.CoverAction{entity.CoverActionOpen, entity.CoverActionClose, entity.CoverActionStop}
	for range 1000 {
		now = now.Add(100 * time.Millisecond)
		queueEffects(processCoverDeadlines(controller, now))
		id := controller.order[random.IntN(len(controller.order))]
		queueEffects(handleCoverRequest(controller, event.Cover{CoverID: id, Action: actions[random.IntN(len(actions))]}, now))
		// Independent device progress; queued operations can outlive their controller generation
		id = controller.order[random.IntN(len(controller.order))]
		if len(queued[id]) == 0 || random.IntN(4) == 0 {
			continue
		}
		command := queued[id][0]
		queued[id] = queued[id][1:]
		var failure error
		if random.IntN(8) == 0 {
			failure = fmt.Errorf("write failed")
		}
		if failure == nil || (command.Action == entity.OutputActionOn && random.IntN(2) == 0) {
			hardware[command.RelayID] = command.Action == entity.OutputActionOn
		}
		for _, runtime := range controller.covers {
			require.False(t, hardware[runtime.cover.OpenRelay] && hardware[runtime.cover.CloseRelay], "direction exclusivity after every attempted hardware write")
		}
		queueEffects(handleCoverOutputResult(controller, coverResult(controller, command, now, failure), now))
	}
}

func TestEstimatedClosedEndpointDoesNotCompleteAnEarlyStop(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	position := 10.25
	controller.covers["a"].position = &position
	completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionClose}, now)), now, hardware)
	observation := coverObservation(controller.covers["a"], now.Add(2*time.Second), controller.settings.FullTravelDuration, event.CoverObservationKind)
	require.NotNil(t, observation.CoverObservation.EstimatedPosition)
	assert.Equal(t, 0.0, *observation.CoverObservation.EstimatedPosition)
	assert.Equal(t, entity.CoverStateClosing, observation.CoverObservation.State)
	commands := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionStop}, now.Add(2*time.Second)))
	completeCoverCommands(t, controller, commands, now.Add(2*time.Second), hardware)
	assert.Equal(t, entity.CoverStateStopped, controller.covers["a"].state)
	require.NotNil(t, controller.covers["a"].position)
	assert.Equal(t, 0.0, *controller.covers["a"].position)
}
