package controller

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/persistence"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownKeepsFeedbackAliveAndPersistsConfirmedPosition(t *testing.T) {
	controller, _ := testCoverController()
	assignments := []persistence.Assignment{
		{CoverID: "a", OpenRelay: "a_open", CloseRelay: "a_close", OpenDevice: "ro_1_1", CloseDevice: "ro_1_2"},
		{CoverID: "b", OpenRelay: "b_open", CloseRelay: "b_close", OpenDevice: "ro_1_3", CloseDevice: "ro_1_4"},
	}
	path := filepath.Join(t.TempDir(), "snapshot.json")
	store := persistence.NewStore(path, "unit", assignments, 32)
	actorContext, cancelActors := context.WithCancel(t.Context())
	shutdownContext, cancelController := context.WithCancel(t.Context())
	storeDone := make(chan struct{})
	go persistence.Run(actorContext, store, storeDone)
	commands := make(chan sysfs.Command, 32)
	states := make(chan sysfs.StateChange, 32)
	controllerDone := make(chan struct{})
	result := make(chan error, 1)
	position := 60.0
	go func() {
		defer close(controllerDone)
		result <- Run(shutdownContext, controller.reg, commands, nil, nil, mqtt.Topics{}, states, nil, nil,
			RuntimeOptions{Persistence: store, RestoredPositions: map[entity.CoverID]*float64{"a": &position, "b": &position}, FeedbackContext: actorContext})
	}()
	startup := make(chan struct{})
	moving := make(chan struct{})
	actorDone := make(chan struct{})
	go func() {
		defer close(actorDone)
		count := 0
		movementReported := false
		for {
			select {
			case <-actorContext.Done():
				return
			case command := <-commands:
				states <- sysfs.StateChange{Kind: sysfs.CompletionReportKind, Completion: &sysfs.CommandCompletion{Command: command, CompletedAt: time.Now()}}
				count++
				if count == 4 {
					close(startup)
				}
				if command.Kind == sysfs.OnCommand && !movementReported {
					close(moving)
					movementReported = true
				}
			}
		}
	}()
	defer func() { cancelController(); cancelActors(); <-actorDone; <-storeDone; <-controllerDone }()
	select {
	case <-startup:
	case <-time.After(time.Second):
		require.FailNow(t, "startup did not complete")
	}
	states <- sysfs.StateChange{Device: sysfs.Device{Identifier: "di_1_1"}, IsRising: true, NewValue: sysfs.On}
	select {
	case <-moving:
	case <-time.After(time.Second):
		require.FailNow(t, "movement did not activate")
	}
	cancelController()
	assert.NoError(t, actorContext.Err(), "actor lifetime must survive shutdown request")
	select {
	case <-controllerDone:
		require.NoError(t, <-result)
	case <-time.After(time.Second):
		require.FailNow(t, "confirmed shutdown did not finish")
	}
	restored := persistence.Restore(persistence.NewStore(path, "unit", assignments, 32))
	require.Contains(t, restored, entity.CoverID("a"))
	assert.GreaterOrEqual(t, *restored["a"], 60.0)
	assert.Less(t, *restored["a"], 100.0)
}

func TestShutdownMissingResultsRemainsUncleanAndBounded(t *testing.T) {
	controller, _ := testCoverController()
	controller.settings.ShutdownPeriod = 40 * time.Millisecond
	// Registry settings determine the controller's bound
	root := &entity.Root{CoverControl: controller.settings, Covers: []entity.Cover{controller.covers["a"].cover},
		Relays: []entity.Relay{{ID: "a_open", SysfsDevice: "ro_1_1"}, {ID: "a_close", SysfsDevice: "ro_1_2"}}}
	reg := registry.Build(root)
	assignment := persistence.Assignment{CoverID: "a", OpenRelay: "a_open", CloseRelay: "a_close", OpenDevice: "ro_1_1", CloseDevice: "ro_1_2"}
	path := filepath.Join(t.TempDir(), "snapshot.json")
	store := persistence.NewStore(path, "unit", []persistence.Assignment{assignment}, 32)
	actorContext, cancelActors := context.WithCancel(t.Context())
	storeDone := make(chan struct{})
	go persistence.Run(actorContext, store, storeDone)
	shutdownContext, cancelController := context.WithCancel(t.Context())
	commands := make(chan sysfs.Command, 32)
	states := make(chan sysfs.StateChange, 32)
	done := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		defer close(done)
		result <- Run(shutdownContext, reg, commands, nil, nil, mqtt.Topics{}, states, nil, nil,
			RuntimeOptions{Persistence: store, FeedbackContext: actorContext})
	}()
	defer func() { cancelController(); cancelActors(); <-storeDone; <-done }()
	select {
	case <-commands:
	case <-time.After(time.Second):
		require.FailNow(t, "startup command missing")
	}
	started := time.Now()
	cancelController()
	select {
	case <-done:
		require.ErrorContains(t, <-result, `cover "a" shutdown: outputs unconfirmed OFF at deadline`)
	case <-time.After(500 * time.Millisecond):
		require.FailNow(t, "shutdown exceeded bounded period")
	}
	assert.Less(t, time.Since(started), 500*time.Millisecond)
	flushContext, cancelFlush := context.WithTimeout(t.Context(), time.Second)
	defer cancelFlush()
	require.True(t, persistence.FlushWithin(flushContext, store))
	assert.Empty(t, persistence.Restore(persistence.NewStore(path, "unit", []persistence.Assignment{assignment}, 32)))
}

func TestShutdownCancelsNotYetSubmittedActivation(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	prepare := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	var activation []event.OutputCommand
	for _, command := range prepare {
		activation = append(activation, coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, command, now, nil), now))...)
	}
	require.Len(t, activation, 1)
	assert.True(t, currentCoverCommand(controller, activation[0]))
	shutdown := coverOutputCommands(beginCoverShutdown(controller, now))
	assert.False(t, currentCoverCommand(controller, activation[0]), "queued stale activation must not run after shutdown OFF")
	assert.Empty(t, handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	completeCoverCommands(t, controller, shutdown, now, hardware)
	assert.True(t, coverOutputsStopped(controller))
}

func TestCoverLoopShutdownPreemptsQueuedActivationAndRequests(t *testing.T) {
	controller, _ := testCoverController()
	controller.settings.ShutdownPeriod = time.Second
	now := time.Now()
	readyTestCovers(t, controller, now, make(map[entity.RelayID]bool))
	prepare := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	var activation []event.OutputCommand
	for _, command := range prepare {
		activation = append(activation, coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, command, now, nil), now))...)
	}
	require.Len(t, activation, 1)
	queue := []event.Event{{Kind: event.OutputCommandKind, OutputCommand: &activation[0]}}
	for range 1024 {
		queue = append(queue, event.Event{Kind: event.CoverKind, Cover: &event.Cover{CoverID: "a", Action: entity.CoverActionOpen}})
	}
	shutdownContext, cancel := context.WithCancel(t.Context())
	cancel()
	loop := &coverLoop{covers: controller, queue: queue, shutdownSignal: shutdownContext.Done()}
	commands := make(chan sysfs.Command, 32)
	semanticEvents := make(chan event.Event, 32)
	targets := dispatchTargets{reg: controller.reg, sysfsCommands: commands}
	done := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		defer close(done)
		result <- processCoverLoop(t.Context(), loop, targets, semanticEvents)
	}()
	defer func() { <-done }()
	var executed []sysfs.Command
	timeout := time.After(2 * time.Second)
	for {
		select {
		case command := <-commands:
			executed = append(executed, command)
			semanticEvents <- event.Event{Kind: event.OutputResultKind, OutputResult: &event.OutputResult{
				CommandID: command.ID, SysfsDevice: entity.SysfsDeviceID(command.DeviceID),
				Action: entity.OutputAction(command.Kind), CompletedAt: time.Now(),
			}}
		case err := <-result:
			require.NoError(t, err)
			require.Len(t, executed, 4, "shutdown must switch off both outputs of both covers")
			for _, command := range executed {
				assert.Equal(t, sysfs.OffCommand, command.Kind, "queued activation and requests must not energize an output")
			}
			assert.True(t, coverOutputsStopped(controller))
			assert.Empty(t, loop.queue, "shutdown must drain derived observations before returning")
			return
		case <-timeout:
			require.FailNow(t, "shutdown did not drain queued work and confirm switch-off")
		}
	}
}

func TestClosedSemanticInputStartsBoundedShutdown(t *testing.T) {
	controller, _ := testCoverController()
	settings := controller.settings
	settings.ShutdownPeriod = 40 * time.Millisecond
	reg := registry.Build(&entity.Root{
		CoverControl: settings,
		Covers:       []entity.Cover{controller.covers["a"].cover},
		Relays: []entity.Relay{
			{ID: "a_open", SysfsDevice: "ro_1_1"},
			{ID: "a_close", SysfsDevice: "ro_1_2"},
		},
	})
	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan sysfs.Command, 32)
	semanticEvents := make(chan event.Event, 32)
	actorDone := make(chan struct{})
	go func() {
		defer close(actorDone)
		for range 2 {
			select {
			case command := <-commands:
				semanticEvents <- event.Event{Kind: event.OutputResultKind, OutputResult: &event.OutputResult{
					CommandID: command.ID, SysfsDevice: entity.SysfsDeviceID(command.DeviceID),
					Action: entity.OutputAction(command.Kind), CompletedAt: time.Now(),
				}}
			case <-ctx.Done():
				return
			}
		}
		close(semanticEvents)
	}()
	loopDone := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		defer close(loopDone)
		result <- runCoverLoop(ctx, reg, commands, nil, nil, mqtt.Topics{}, semanticEvents, RuntimeOptions{})
	}()
	defer func() { cancel(); <-actorDone; <-loopDone }()
	select {
	case err := <-result:
		require.ErrorContains(t, err, `cover "a" shutdown: outputs unconfirmed OFF at deadline`)
	case <-time.After(time.Second):
		require.FailNow(t, "closed semantic input did not produce bounded shutdown")
	}
	require.NoError(t, ctx.Err(), "input closure must initiate shutdown without application cancellation")
	require.Len(t, commands, 2, "input closure must start one switch-off attempt")
	for range 2 {
		assert.Equal(t, sysfs.OffCommand, (<-commands).Kind)
	}
}
