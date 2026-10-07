package controller

import (
	"context"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeriodicCoverReportingPreservesTravelDeadline(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	completeCoverCommands(t, controller, coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now)), now, hardware)
	deadline := controller.covers["a"].travelDeadline
	for i := 1; i < 10; i++ {
		events := processCoverDeadlines(controller, now.Add(time.Duration(i)*time.Second))
		require.Len(t, events, 1)
		assert.Equal(t, event.CoverObservationKind, events[0].Kind)
		assert.Equal(t, deadline, controller.covers["a"].travelDeadline)
	}
	assert.Len(t, coverOutputCommands(processCoverDeadlines(controller, deadline)), 2)
}

func TestDelayedActivationUsesHardwareCompletionTime(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	position := 20.0
	controller.covers["a"].position = &position
	prepare := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	var activation []event.OutputCommand
	for _, command := range prepare {
		activation = append(activation, coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, command, now, nil), now))...)
	}
	require.Len(t, activation, 1)
	reportedAt := now.Add(time.Second)
	events := handleCoverOutputResult(controller, coverResult(controller, activation[0], now, nil), reportedAt)
	require.Len(t, events, 1)
	require.NotNil(t, events[0].CoverObservation.EstimatedPosition)
	assert.InDelta(t, 30, *events[0].CoverObservation.EstimatedPosition, 0.001)
	assert.Equal(t, now.Add(10*time.Second), controller.covers["a"].travelDeadline)
}

func TestCoverActivationTimeoutRequestsOff(t *testing.T) {
	controller, now := testCoverController()
	hardware := make(map[entity.RelayID]bool)
	readyTestCovers(t, controller, now, hardware)
	prepare := coverOutputCommands(handleCoverRequest(controller, event.Cover{CoverID: "a", Action: entity.CoverActionOpen}, now))
	var activation []event.OutputCommand
	for _, command := range prepare {
		activation = append(activation, coverOutputCommands(handleCoverOutputResult(controller, coverResult(controller, command, now, nil), now))...)
	}
	require.Len(t, activation, 1)
	expired := processCoverDeadlines(controller, now.Add(controller.settings.OperationTimeout))
	assert.Len(t, coverOutputCommands(expired), 2)
	assert.Error(t, controller.covers["a"].fault)
	assert.Empty(t, handleCoverOutputResult(controller, coverResult(controller, activation[0], now, nil), now.Add(3*time.Second)))
	assert.True(t, controller.covers["a"].travelDeadline.IsZero())
}

func TestCoverDeadlineProgressesUnderEventTrafficAndModbusBackpressure(t *testing.T) {
	settings := entity.CoverControl{FullTravelDuration: 30 * time.Millisecond, OperationTimeout: time.Second, OffRetryInterval: time.Second, ShutdownPeriod: time.Second, ReportingInterval: 10 * time.Millisecond}
	reg := registry.Build(&entity.Root{
		CoverControl: settings, Covers: []entity.Cover{{ID: "a", OpenRelay: "open", CloseRelay: "close"}},
		Relays:               []entity.Relay{{ID: "open", SysfsDevice: "ro_1_1"}, {ID: "close", SysfsDevice: "ro_1_2"}},
		Modbus:               entity.Modbus{EventSignalWrites: []entity.ModbusEventSignalWrite{{Source: "remote", UnitID: 1, Coil: 1}}},
		RemoteSourceBindings: []entity.Binding{{Source: "remote", Target: "other.light.office", Action: "toggle", ExecutionTransport: entity.ExecutionTransportMQTT}},
	})
	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan sysfs.Command, 32)
	semantic := make(chan event.Event, 32)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = dispatchEvents(ctx, reg, commands, make(chan mqtt.Command), make(chan modbus.Command), mqtt.Topics{}, semantic, RuntimeOptions{})
	}()
	actorDone := make(chan struct{})
	stopped := make(chan struct{}, 1)
	go func() {
		defer close(actorDone)
		started := false
		for {
			select {
			case <-ctx.Done():
				return
			case command := <-commands:
				if command.Kind == sysfs.OnCommand {
					started = true
				}
				if started && command.Kind == sysfs.OffCommand {
					select {
					case stopped <- struct{}{}:
					default:
					}
				}
				result := event.Event{Kind: event.OutputResultKind, OutputResult: &event.OutputResult{CommandID: command.ID, SysfsDevice: entity.SysfsDeviceID(command.DeviceID), Action: entity.OutputAction(command.Kind), CompletedAt: time.Now()}}
				select {
				case semantic <- result:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	floodDone := make(chan struct{})
	go func() {
		defer close(floodDone)
		for {
			for _, incoming := range []event.Event{
				{Kind: event.CoverKind, Cover: &event.Cover{CoverID: "a", Action: entity.CoverActionOpen}},
				{Kind: event.PushButtonPressedKind, PushButton: &event.PushButton{ButtonID: "remote"}},
			} {
				select {
				case semantic <- incoming:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done; <-actorDone; <-floodDone }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "movement deadline starved by event traffic or Modbus backpressure")
	}
}
