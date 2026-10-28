package shade

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTime = time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)

const (
	testTravel = 30 * time.Second
	testRetry  = time.Second
)

func TestLifecycleRequiresPositiveDurations(t *testing.T) {
	for _, durations := range [][2]time.Duration{
		{0, testRetry}, {-time.Second, testRetry},
		{testTravel, 0}, {testTravel, -time.Second},
	} {
		_, err := newLifecycle(durations[0], durations[1])
		require.Error(t, err)
	}
}

func TestStartupConfirmsBothOutputsOff(t *testing.T) {
	state := newTestLifecycle(t)
	assert.Equal(t, recovering, state.phase)
	assert.False(t, bothOutputsOff(&state))
	assert.Equal(t, noDirection, state.direction)

	// Startup requests must not become movement after cleanup.
	require.NoError(t, handleRequest(&state, openRequest))
	completeOffCleanup(t, &state, testTime)
	assert.Equal(t, stopped, state.phase)
	assert.Equal(t, noDirection, state.direction)
	_, started := claimNextWrite(&state, testTime)
	assert.False(t, started)
}

func TestNormalMovement(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		request   request
		direction direction
	}{
		{"opening", openRequest, opening},
		{"closing", closeRequest, closing},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := readyLifecycle(t)
			require.NoError(t, handleRequest(&state, scenario.request))
			assert.Equal(t, preparing, state.phase)
			assert.False(t, bothOutputsOff(&state), "each start requires fresh OFF confirmation")

			completePreparation(t, &state)
			assert.Equal(t, activating, state.phase)
			assert.True(t, state.deadline.IsZero())
			activation, started := claimNextWrite(&state, testTime)
			require.True(t, started)
			assert.Equal(t, write{direction: scenario.direction, on: true}, activation)
			assert.True(t, state.deadline.IsZero(), "starting ON does not start the movement deadline")

			onSucceeded := testTime.Add(2 * time.Second)
			require.NoError(t, completeWrite(&state, nil, onSucceeded))
			assert.Equal(t, moving, state.phase)
			assert.Equal(t, scenario.direction, state.direction)
			assert.Equal(t, onSucceeded.Add(testTravel), state.deadline)

			expireMovement(&state, state.deadline.Add(-time.Nanosecond))
			assert.Equal(t, moving, state.phase)
			expireMovement(&state, state.deadline)
			assert.Equal(t, stopping, state.phase)
			assert.True(t, state.deadline.IsZero())
			completeOffCleanup(t, &state, onSucceeded.Add(testTravel))
			assert.Equal(t, stopped, state.phase)
			assert.Equal(t, noDirection, state.direction)
		})
	}
}

func TestRepeatedDirectionPreservesIntentAndDeadline(t *testing.T) {
	state := readyLifecycle(t)
	require.NoError(t, handleRequest(&state, openRequest))
	require.NoError(t, handleRequest(&state, openRequest))
	assert.Equal(t, preparing, state.phase)
	completePreparation(t, &state)
	require.NoError(t, handleRequest(&state, openRequest))
	assert.Equal(t, activating, state.phase)
	_, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	require.NoError(t, handleRequest(&state, openRequest))
	require.NoError(t, completeWrite(&state, nil, testTime))
	deadline := state.deadline
	require.NoError(t, handleRequest(&state, openRequest))
	assert.Equal(t, deadline, state.deadline)
	assert.Equal(t, moving, state.phase)
	_, started = claimNextWrite(&state, testTime.Add(time.Second))
	assert.False(t, started, "repeated requests never issue duplicate ON")
}

func TestStopAndOppositeDirectionCancelWithoutReversal(t *testing.T) {
	for _, command := range []request{stopRequest, closeRequest} {
		for _, stage := range []struct {
			name  string
			setup func(*testing.T) lifecycle
		}{
			{"preparing", preparingLifecycle},
			{"off_in_progress", startingOffLifecycle},
			{"before_on", activatingLifecycle},
			{"on_in_progress", startingOnLifecycle},
			{"moving", movingLifecycle},
		} {
			t.Run(testRequestName(command)+"/"+stage.name, func(t *testing.T) {
				state := stage.setup(t)

				require.NoError(t, handleRequest(&state, command))
				assert.Equal(t, noDirection, state.direction)
				assert.True(t, state.deadline.IsZero())
				if state.writing {
					_, started := claimNextWrite(&state, testTime)
					assert.False(t, started, "cleanup cannot bypass an in-progress write")
					require.NoError(t, completeWrite(&state, nil, testTime))
					assert.NotEqual(t, moving, state.phase, "late ON cannot restore movement")
				}
				completeOffCleanup(t, &state, testTime)
				assert.Equal(t, stopped, state.phase)
				_, started := claimNextWrite(&state, testTime)
				assert.False(t, started, "neither command starts the opposite direction")

				require.NoError(t, handleRequest(&state, closeRequest))
				assert.Equal(t, preparing, state.phase)
				assert.Equal(t, closing, state.direction, "fresh requests are allowed after confirmed OFF")
			})
		}
	}
}

func TestRequestsDuringStoppingAreNotRetained(t *testing.T) {
	state := movingLifecycle(t)
	require.NoError(t, handleRequest(&state, stopRequest))
	for _, command := range []request{openRequest, closeRequest, stopRequest} {
		require.NoError(t, handleRequest(&state, command))
		assert.Equal(t, stopping, state.phase)
		assert.Equal(t, noDirection, state.direction)
	}
	completeOffCleanup(t, &state, testTime)
	_, started := claimNextWrite(&state, testTime)
	assert.False(t, started)
	require.NoError(t, handleRequest(&state, stopRequest))
	assert.Equal(t, stopped, state.phase)
}

func TestOnErrorCancelsMovementAndRemainsVisible(t *testing.T) {
	state := readyLifecycle(t)
	require.NoError(t, handleRequest(&state, openRequest))
	completePreparation(t, &state)
	_, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	writeErr := errors.New("relay unavailable")
	err := completeWrite(&state, writeErr, testTime)
	require.ErrorIs(t, err, writeErr)
	assert.Equal(t, recovering, state.phase)
	assert.Equal(t, noDirection, state.direction)
	assert.True(t, state.deadline.IsZero())
	assert.False(t, bothOutputsOff(&state))
	require.NoError(t, handleRequest(&state, closeRequest))
	completeOffCleanup(t, &state, testTime)
	assert.Equal(t, stopped, state.phase)
	_, started = claimNextWrite(&state, testTime)
	assert.False(t, started, "recovery never retries ON or resumes movement")
}

func TestOffErrorsUseFixedRetriesWithoutBlockingOtherOutput(t *testing.T) {
	state := newTestLifecycle(t)
	first, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	assert.Equal(t, write{direction: opening}, first)
	writeErr := errors.New("OFF failed")
	require.ErrorIs(t, completeWrite(&state, writeErr, testTime), writeErr)
	assert.Equal(t, recovering, state.phase)
	assert.False(t, bothOutputsOff(&state))

	// A failed first relay must not prevent attempting OFF on the second.
	second, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	assert.Equal(t, write{direction: closing}, second)
	require.NoError(t, completeWrite(&state, nil, testTime))
	_, started = claimNextWrite(&state, testTime.Add(testRetry-time.Nanosecond))
	assert.False(t, started)

	for attempt := 1; attempt <= 3; attempt++ {
		now := testTime.Add(time.Duration(attempt) * testRetry)
		retry, started := claimNextWrite(&state, now)
		require.True(t, started)
		assert.Equal(t, first, retry)
		_, duplicate := claimNextWrite(&state, now.Add(testRetry))
		assert.False(t, duplicate, "pending retries do not accumulate")
		require.ErrorIs(t, completeWrite(&state, writeErr, now), writeErr)
		for _, command := range []request{stopRequest, openRequest, closeRequest} {
			require.NoError(t, handleRequest(&state, command))
		}
		_, early := claimNextWrite(&state, now.Add(testRetry-time.Nanosecond))
		assert.False(t, early, "requests cannot shorten the retry interval")
	}
	completeOffCleanup(t, &state, testTime.Add(4*testRetry))
	assert.Equal(t, stopped, state.phase)
	require.NoError(t, handleRequest(&state, openRequest))
	assert.Equal(t, preparing, state.phase)
}

func TestOffErrorDuringNormalStopRequiresRecovery(t *testing.T) {
	state := movingLifecycle(t)
	require.NoError(t, handleRequest(&state, stopRequest))
	_, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	writeErr := errors.New("OFF failed")
	require.ErrorIs(t, completeWrite(&state, writeErr, testTime), writeErr)
	assert.Equal(t, recovering, state.phase)
	assert.Equal(t, noDirection, state.direction)
	assert.False(t, bothOutputsOff(&state))
	completeOffCleanup(t, &state, testTime.Add(testRetry))
	assert.Equal(t, stopped, state.phase)
}

func TestPreparationErrorCancelsStart(t *testing.T) {
	state := startingOffLifecycle(t)
	require.NoError(t, completeWrite(&state, nil, testTime))
	operation, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	assert.Equal(t, write{direction: closing}, operation)
	writeErr := errors.New("preparation OFF failed")
	require.ErrorIs(t, completeWrite(&state, writeErr, testTime), writeErr)
	assert.Equal(t, recovering, state.phase)
	assert.Equal(t, noDirection, state.direction)
	completeOffCleanup(t, &state, testTime.Add(testRetry))
	_, started = claimNextWrite(&state, testTime.Add(testRetry))
	assert.False(t, started, "successful recovery must not activate the canceled start")
}

func TestShutdownNeverRestoresMovement(t *testing.T) {
	for _, stage := range []struct {
		name  string
		setup func(*testing.T) lifecycle
	}{
		{"startup", newTestLifecycle},
		{"stopped", readyLifecycle},
		{"preparing", preparingLifecycle},
		{"off_in_progress", startingOffLifecycle},
		{"activating", activatingLifecycle},
		{"on_in_progress", startingOnLifecycle},
		{"moving", movingLifecycle},
		{"stopping", stoppingLifecycle},
		{"recovering", recoveringLifecycle},
	} {
		t.Run(stage.name, func(t *testing.T) {
			state := stage.setup(t)

			beginShutdown(&state)
			beginShutdown(&state)
			assert.True(t, state.shuttingDown)
			assert.Equal(t, noDirection, state.direction)
			assert.True(t, state.deadline.IsZero())
			if state.writing {
				_, started := claimNextWrite(&state, testTime)
				assert.False(t, started)
				require.NoError(t, completeWrite(&state, nil, testTime))
			}
			completeOffCleanup(t, &state, testTime.Add(testRetry))
			assert.Equal(t, stopped, state.phase)
			for _, command := range []request{openRequest, closeRequest, stopRequest} {
				require.NoError(t, handleRequest(&state, command))
				assert.Equal(t, stopped, state.phase)
			}
			_, started := claimNextWrite(&state, testTime)
			assert.False(t, started)
			assert.True(t, state.shuttingDown)
		})
	}
}

func TestShutdownContinuesOffRetries(t *testing.T) {
	state := movingLifecycle(t)
	beginShutdown(&state)
	_, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	require.Error(t, completeWrite(&state, errors.New("OFF failed"), testTime))
	assert.False(t, bothOutputsOff(&state))
	assert.Equal(t, recovering, state.phase)
	completeOffCleanup(t, &state, testTime.Add(testRetry))
	assert.Equal(t, stopped, state.phase)
	assert.True(t, state.shuttingDown)
}

func TestSelectNextWriteDoesNotChangeLifecycle(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		setup     func(*testing.T) lifecycle
		want      write
		available bool
	}{
		{"startup", newTestLifecycle, write{direction: opening}, true},
		{"preparing", preparingLifecycle, write{direction: opening}, true},
		{"activating", activatingLifecycle, write{direction: opening, on: true}, true},
		{"write_in_progress", startingOnLifecycle, write{}, false},
		{"stopped", readyLifecycle, write{}, false},
		{"moving", movingLifecycle, write{}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := scenario.setup(t)
			before := state
			next, available := selectNextWrite(&state, testTime)
			assert.Equal(t, scenario.want, next)
			assert.Equal(t, scenario.available, available)
			assert.Equal(t, before, state)
		})
	}
}

func TestInvalidLifecycleInputsRemainVisible(t *testing.T) {
	state := readyLifecycle(t)
	before := state
	require.Error(t, handleRequest(&state, request(255)))
	assert.Equal(t, before, state)
	require.Error(t, completeWrite(&state, nil, testTime))
	assert.Equal(t, before, state)
}

func newTestLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state, err := newLifecycle(testTravel, testRetry)
	require.NoError(t, err)
	return state
}

func readyLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state := newTestLifecycle(t)
	completeOffCleanup(t, &state, testTime)
	return state
}

func movingLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state := startingOnLifecycle(t)
	require.NoError(t, completeWrite(&state, nil, testTime))
	require.Equal(t, moving, state.phase)
	return state
}

func preparingLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state := readyLifecycle(t)
	require.NoError(t, handleRequest(&state, openRequest))
	return state
}

func startingOffLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state := preparingLifecycle(t)
	_, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	return state
}

func activatingLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state := preparingLifecycle(t)
	completePreparation(t, &state)
	return state
}

func startingOnLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state := activatingLifecycle(t)
	_, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	return state
}

func stoppingLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state := movingLifecycle(t)
	require.NoError(t, handleRequest(&state, stopRequest))
	return state
}

func recoveringLifecycle(t *testing.T) lifecycle {
	t.Helper()
	state := stoppingLifecycle(t)
	_, started := claimNextWrite(&state, testTime)
	require.True(t, started)
	require.Error(t, completeWrite(&state, errors.New("OFF failed"), testTime))
	return state
}

func completePreparation(t *testing.T, state *lifecycle) {
	t.Helper()
	for _, expected := range []direction{opening, closing} {
		operation, started := claimNextWrite(state, testTime)
		require.True(t, started)
		require.Equal(t, write{direction: expected}, operation)
		_, duplicate := claimNextWrite(state, testTime)
		require.False(t, duplicate)
		require.NoError(t, completeWrite(state, nil, testTime))
	}
	require.Equal(t, activating, state.phase)
}

func completeOffCleanup(t *testing.T, state *lifecycle, now time.Time) {
	t.Helper()
	for range 2 {
		operation, started := claimNextWrite(state, now)
		if !started {
			break
		}
		require.False(t, operation.on, "cleanup must never issue ON")
		require.NoError(t, completeWrite(state, nil, now))
	}
	require.Equal(t, stopped, state.phase)
	require.True(t, bothOutputsOff(state))
}

func testRequestName(command request) string {
	if command == stopRequest {
		return "stop"
	}
	return "opposite"
}
