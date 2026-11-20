package sysfs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/cover"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverRequestsStartupAndShutdown(t *testing.T) {
	first, second := outputPairFixture(t, On, On)
	light, _ := outputPairFixture(t, On, Off)
	light.Identifier = "light"
	states := make(chan StateChange, 16)
	commands, result, cancel := startCoverTest(t, []*Device{first, second, light}, states, time.Second)
	receiveCover(t, states, cover.Stopped)
	assertOutput(t, first, Off)
	assertOutput(t, second, Off)
	assertOutput(t, light, On)

	commands <- coverTestCommand(entity.CoverActionOpen)
	receiveCover(t, states, cover.Opening)
	assertOutput(t, first, On)
	assertOutput(t, second, Off)
	commands <- coverTestCommand(entity.CoverActionClose)
	receiveCover(t, states, cover.Stopped)
	assertOutput(t, first, Off)
	assertOutput(t, second, Off)
	commands <- coverTestCommand(entity.CoverActionClose)
	receiveCover(t, states, cover.Closing)
	assertOutput(t, second, On)
	commands <- coverTestCommand(entity.CoverActionStop)
	receiveCover(t, states, cover.Stopped)
	commands <- coverTestCommand(entity.CoverActionOpen)
	receiveCover(t, states, cover.Opening)
	cancel()
	require.NoError(t, <-result)
	assertOutput(t, first, Off)
	assertOutput(t, second, Off)
	assertOutput(t, light, On)
}

func TestCoverDeadlineWorksWithBlockedStates(t *testing.T) {
	first, second := outputPairFixture(t, Off, Off)
	states := make(chan StateChange)
	commands, result, cancel := startCoverTest(t, []*Device{first, second}, states, 100*time.Millisecond)
	commands <- coverTestCommand(entity.CoverActionOpen)
	require.Eventually(t, func() bool { value, err := readValue(first.Path); return err == nil && value == On }, time.Second, time.Millisecond)
	require.Eventually(t, func() bool { value, err := readValue(first.Path); return err == nil && value == Off }, time.Second, time.Millisecond)
	cancel()
	require.NoError(t, <-result)
	assertOutput(t, second, Off)
}

func TestCoverShutdownWorksWithBlockedStates(t *testing.T) {
	first, second := outputPairFixture(t, Off, Off)
	commands, result, cancel := startCoverTest(t, []*Device{first, second}, make(chan StateChange), time.Hour)
	commands <- coverTestCommand(entity.CoverActionClose)
	require.Eventually(t, func() bool { value, err := readValue(second.Path); return err == nil && value == On }, time.Second, time.Millisecond)
	cancel()
	require.NoError(t, <-result)
	assertOutput(t, first, Off)
	assertOutput(t, second, Off)
}

func TestCoverFailureStopsActorAndDoesNotClaimStopped(t *testing.T) {
	first, second := outputPairFixture(t, Off, Off)
	states := make(chan StateChange, 8)
	commands, result, _ := startCoverTest(t, []*Device{first, second}, states, time.Hour)
	receiveCover(t, states, cover.Stopped)
	commands <- coverTestCommand(entity.CoverActionOpen)
	receiveCover(t, states, cover.Opening)
	require.NoError(t, os.Remove(first.Path))
	commands <- coverTestCommand(entity.CoverActionStop)
	require.ErrorContains(t, <-result, "first")
	failure := <-states
	require.NotNil(t, failure.Cover)
	require.Error(t, failure.Cover.Error)
	assert.Equal(t, cover.Opening, failure.Cover.Movement)
	assertOutput(t, second, Off)
}

func TestCoverStartupFailureStillAttemptsOtherOutput(t *testing.T) {
	first, second := outputPairFixture(t, On, On)
	require.NoError(t, os.Remove(first.Path))
	states := make(chan StateChange, 8)
	_, result, _ := startCoverTest(t, []*Device{first, second}, states, time.Hour)
	require.ErrorContains(t, <-result, "initialize covers")
	assertOutput(t, second, Off)
	assert.Empty(t, states)
}

func TestCoverShutdownReportsCleanupFailure(t *testing.T) {
	first, second := outputPairFixture(t, Off, Off)
	states := make(chan StateChange, 8)
	commands, result, cancel := startCoverTest(t, []*Device{first, second}, states, time.Hour)
	receiveCover(t, states, cover.Stopped)
	commands <- coverTestCommand(entity.CoverActionClose)
	receiveCover(t, states, cover.Closing)
	require.NoError(t, os.Remove(first.Path))
	cancel()
	require.ErrorContains(t, <-result, "stop cover")
	assertOutput(t, second, Off)
}

func TestMultipleCoversHaveIndependentState(t *testing.T) {
	first, second := outputPairFixture(t, Off, Off)
	third, fourth := outputPairFixture(t, Off, Off)
	third.Identifier, fourth.Identifier = "third", "fourth"
	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan Command, 8)
	states := make(chan StateChange, 8)
	result := make(chan error, 1)
	go func() {
		result <- Run(ctx, []*Device{first, second, third, fourth}, commands, states, make(chan struct{}), Options{
			Covers: []CoverConfig{
				{ID: "unit.cover.office", OpenDeviceID: "first", CloseDeviceID: "second"},
				{ID: "unit.cover.bedroom", OpenDeviceID: "third", CloseDeviceID: "fourth"},
			}, MaxRun: time.Hour,
		})
	}()
	t.Cleanup(func() { cancel(); assert.NoError(t, <-result) })
	for range 2 {
		<-states
	}
	commands <- coverTestCommand(entity.CoverActionOpen)
	receiveCover(t, states, cover.Opening)
	commands <- Command{Kind: CoverCommand, Cover: &CoverRequest{ID: "unit.cover.bedroom", Action: entity.CoverActionClose}}
	bedroom := <-states
	require.NotNil(t, bedroom.Cover)
	assert.Equal(t, cover.Closing, bedroom.Cover.Movement)
	commands <- coverTestCommand(entity.CoverActionStop)
	receiveCover(t, states, cover.Stopped)
	assertOutput(t, first, Off)
	assertOutput(t, fourth, On)
}

func TestDirectCoverRelayCommandIsRejected(t *testing.T) {
	first, second := outputPairFixture(t, Off, Off)
	states := make(chan StateChange, 8)
	commands, result, _ := startCoverTest(t, []*Device{first, second}, states, time.Hour)
	receiveCover(t, states, cover.Stopped)
	commands <- Command{Kind: OnCommand, DeviceID: first.Identifier}
	require.ErrorContains(t, <-result, "cover-owned output")
	assertOutput(t, first, Off)
	assertOutput(t, second, Off)
}

func TestCoverDeviceOwnershipValidation(t *testing.T) {
	first, second := outputPairFixture(t, Off, Off)
	valid := CoverConfig{ID: "unit.cover.office", OpenDeviceID: "first", CloseDeviceID: "second"}
	tests := []Options{
		{Covers: []CoverConfig{valid}},
		{Covers: []CoverConfig{{ID: valid.ID, OpenDeviceID: "first", CloseDeviceID: "first"}}, MaxRun: time.Second},
		{Covers: []CoverConfig{{ID: valid.ID, OpenDeviceID: "missing", CloseDeviceID: "second"}}, MaxRun: time.Second},
		{Covers: []CoverConfig{valid, valid}, MaxRun: time.Second},
	}
	for _, opts := range tests {
		_, _, err := prepareCovers([]*Device{first, second}, opts)
		require.Error(t, err)
	}
}

func startCoverTest(t *testing.T, devices []*Device, states chan StateChange, maxRun time.Duration) (chan Command, <-chan error, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan Command, 32)
	result := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		result <- Run(ctx, devices, commands, states, make(chan struct{}), Options{
			Covers: []CoverConfig{{ID: "unit.cover.office", OpenDeviceID: "first", CloseDeviceID: "second"}}, MaxRun: maxRun,
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
			t.Error("cover actor did not stop")
		}
	})
	return commands, result, cancel
}

func coverTestCommand(action entity.CoverAction) Command {
	return Command{Kind: CoverCommand, Cover: &CoverRequest{ID: "unit.cover.office", Action: action}}
}

func receiveCover(t *testing.T, states <-chan StateChange, movement cover.Movement) {
	t.Helper()
	for {
		select {
		case state := <-states:
			if state.Cover == nil {
				continue
			}
			require.NoError(t, state.Cover.Error)
			require.Equal(t, entity.CoverID("unit.cover.office"), state.Cover.ID)
			require.Equal(t, movement, state.Cover.Movement)
			return
		case <-time.After(2 * time.Second):
			t.Fatal("missing cover snapshot")
		}
	}
}

func assertOutput(t *testing.T, device *Device, expected Value) {
	t.Helper()
	value, err := readValue(device.Path)
	require.NoError(t, err)
	assert.Equal(t, expected, value)
}
