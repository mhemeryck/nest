package cover

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMovementTransitions(t *testing.T) {
	now := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	maxRun := 30 * time.Second
	opening := State{Movement: Opening, Deadline: now.Add(10 * time.Second)}
	closing := State{Movement: Closing, Deadline: opening.Deadline}

	tests := []struct {
		name   string
		state  State
		input  Input
		result Result
	}{
		{"stopped open", State{}, Open, Result{State{Opening, now.Add(maxRun)}, StartOpening}},
		{"stopped close", State{}, Close, Result{State{Closing, now.Add(maxRun)}, StartClosing}},
		{"stopped stop requests stop movement", State{}, Stop, Result{Effect: StopMovement}},
		{"stopped deadline check", State{}, CheckDeadline, Result{}},
		{"opening open preserves deadline", opening, Open, Result{State: opening}},
		{"opening close stops", opening, Close, Result{Effect: StopMovement}},
		{"opening stop", opening, Stop, Result{Effect: StopMovement}},
		{"opening early deadline check", opening, CheckDeadline, Result{State: opening}},
		{"closing close preserves deadline", closing, Close, Result{State: closing}},
		{"closing open stops", closing, Open, Result{Effect: StopMovement}},
		{"closing stop", closing, Stop, Result{Effect: StopMovement}},
		{"closing early deadline check", closing, CheckDeadline, Result{State: closing}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Transition(tt.state, tt.input, now, maxRun)
			require.NoError(t, err)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestDeadlineAlwaysStopsMovement(t *testing.T) {
	deadline := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	for _, movement := range []Movement{Opening, Closing} {
		for _, input := range []Input{Open, Close, Stop, CheckDeadline} {
			for _, now := range []time.Time{deadline, deadline.Add(time.Second)} {
				result, err := Transition(State{movement, deadline}, input, now, 30*time.Second)
				require.NoError(t, err)
				assert.Equal(t, Result{Effect: StopMovement}, result)
			}
		}
	}
}

func TestOppositeRequestStopsWithoutQueuedReversal(t *testing.T) {
	now := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	maxRun := 30 * time.Second
	result, err := Transition(State{}, Open, now, maxRun)
	require.NoError(t, err)

	result, err = Transition(result.State, Close, now.Add(time.Second), maxRun)
	require.NoError(t, err)
	assert.Equal(t, Result{Effect: StopMovement}, result)

	result, err = Transition(result.State, CheckDeadline, now.Add(2*time.Second), maxRun)
	require.NoError(t, err)
	assert.Equal(t, Result{}, result)

	result, err = Transition(result.State, Close, now.Add(3*time.Second), maxRun)
	require.NoError(t, err)
	assert.Equal(t, Result{State{Closing, now.Add(3*time.Second + maxRun)}, StartClosing}, result)
}

func TestRepeatedRequestsCannotExtendMovement(t *testing.T) {
	now := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	maxRun := 30 * time.Second
	for _, input := range []Input{Open, Close} {
		result, err := Transition(State{}, input, now, maxRun)
		require.NoError(t, err)
		initialState := result.State

		for elapsed := time.Second; elapsed < maxRun; elapsed += time.Second {
			result, err = Transition(result.State, input, now.Add(elapsed), maxRun)
			require.NoError(t, err)
			assert.Equal(t, Result{State: initialState}, result)
		}

		result, err = Transition(result.State, input, now.Add(maxRun), maxRun)
		require.NoError(t, err)
		assert.Equal(t, Result{Effect: StopMovement}, result)
	}
}

func TestStaleDeadlineCheckDoesNotStopNewMovement(t *testing.T) {
	now := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	maxRun := 30 * time.Second
	result, err := Transition(State{}, Open, now, maxRun)
	require.NoError(t, err)
	oldDeadline := result.State.Deadline

	result, err = Transition(result.State, Stop, now.Add(time.Second), maxRun)
	require.NoError(t, err)
	result, err = Transition(result.State, Close, now.Add(2*time.Second), maxRun)
	require.NoError(t, err)
	newState := result.State

	result, err = Transition(newState, CheckDeadline, oldDeadline, maxRun)
	require.NoError(t, err)
	assert.Equal(t, Result{State: newState}, result)
}

func TestInvalidArgumentsPreserveStateWithoutEffects(t *testing.T) {
	now := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	maxRun := 30 * time.Second
	tests := []struct {
		name   string
		state  State
		input  Input
		now    time.Time
		maxRun time.Duration
	}{
		{"invalid movement", State{Movement: Movement(255)}, Open, now, maxRun},
		{"stopped with deadline", State{Deadline: now}, Open, now, maxRun},
		{"opening without deadline", State{Movement: Opening}, Open, now, maxRun},
		{"closing without deadline", State{Movement: Closing}, Close, now, maxRun},
		{"invalid input", State{}, Input(255), now, maxRun},
		{"zero input", State{}, Input(0), now, maxRun},
		{"zero time", State{}, Open, time.Time{}, maxRun},
		{"zero maximum duration", State{}, Open, now, 0},
		{"negative maximum duration", State{}, Open, now, -time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Transition(tt.state, tt.input, tt.now, tt.maxRun)
			require.Error(t, err)
			assert.Equal(t, Result{State: tt.state}, result)
		})
	}
}
