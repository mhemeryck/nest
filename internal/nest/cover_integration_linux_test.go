//go:build linux

package nest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockedCoverWorkerDoesNotBlockOtherCoverLifecycle(t *testing.T) {
	reg := registry.Build(&entity.Root{
		MQTT:               entity.MQTT{Enabled: true, TopicPrefix: "nest", UnitID: "unit"},
		SysfsPollIntervals: entity.PollIntervals{RelayOutput: time.Hour},
		CoverControl:       entity.CoverControl{FullTravelDuration: 200 * time.Millisecond, OperationTimeout: 25 * time.Millisecond, OffRetryInterval: 5 * time.Millisecond, ShutdownPeriod: 500 * time.Millisecond, ReportingInterval: 20 * time.Millisecond},
		Covers:             []entity.Cover{{ID: "unit.cover.a", OpenRelay: "a_open", CloseRelay: "a_close"}, {ID: "unit.cover.b", OpenRelay: "b_open", CloseRelay: "b_close"}},
		Relays:             []entity.Relay{{ID: "a_open", SysfsDevice: "ro_1_1"}, {ID: "a_close", SysfsDevice: "ro_1_2"}, {ID: "b_open", SysfsDevice: "ro_1_3"}, {ID: "b_close", SysfsDevice: "ro_1_4"}},
	})
	directory := t.TempDir()
	var devices []*sysfs.Device
	for _, id := range []string{"ro_1_1", "ro_1_2", "ro_1_3", "ro_1_4"} {
		path := filepath.Join(directory, id)
		require.NoError(t, os.WriteFile(path, []byte("0\n"), 0o600))
		devices = append(devices, &sysfs.Device{Identifier: id, Path: path, Type: sysfs.RelayOutput, Value: sysfs.Off})
	}
	actorContext, cancelActors := context.WithCancel(t.Context())
	shutdownContext, cancelController := context.WithCancel(t.Context())
	rawStates := make(chan sysfs.StateChange, 32)
	sysfsActor := sysfsActor{commands: make(chan sysfs.Command, 32), states: make(chan sysfs.StateChange, 32), done: make(chan struct{}), workers: sysfsWorkerConfigs(reg, devices)}
	go sysfs.RunConfigured(actorContext, sysfsActor.workers, sysfsActor.commands, rawStates, sysfsActor.done)
	queueRejected := make(chan struct{}, 1)
	forwardDone := make(chan struct{})
	go func() {
		defer close(forwardDone)
		for {
			select {
			case <-actorContext.Done():
				return
			case report := <-rawStates:
				if report.Completion != nil && report.Completion.Error != nil && strings.Contains(report.Completion.Error.Error(), "queue exhausted") {
					select {
					case queueRejected <- struct{}{}:
					default:
					}
				}
				select {
				case sysfsActor.states <- report:
				case <-actorContext.Done():
					return
				}
			}
		}
	}()
	topics := mqtt.NewTopics("nest", "unit")
	mqttActor := mqttActor{enabled: true, commands: make(chan mqtt.Command), events: make(chan mqtt.Event, 32), done: make(chan struct{}), topics: topics}
	reports := make(chan mqtt.PublishMessage, 128)
	go func() {
		defer close(mqttActor.done)
		for {
			select {
			case <-actorContext.Done():
				return
			case command := <-mqttActor.commands:
				if command.Publish.Topic == mqtt.CoverStateTopic(topics, "unit.cover.a") || command.Publish.Topic == mqtt.CoverStateTopic(topics, "unit.cover.b") {
					select {
					case reports <- command.Publish:
					case <-actorContext.Done():
						return
					}
				}
			}
		}
	}()
	controllerDone := startController(shutdownContext, reg, sysfsActor, mqttActor, modbusActor{}, controller.RuntimeOptions{FeedbackContext: actorContext})
	fifoCreated := false
	t.Cleanup(func() {
		var reader *os.File
		if fifoCreated {
			var err error
			reader, err = os.OpenFile(devices[0].Path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			require.NoError(t, err)
			defer func() { assert.NoError(t, reader.Close()) }()
		}
		cancelController()
		select {
		case <-controllerDone:
		case <-time.After(2 * time.Second):
			t.Error("controller failed to stop")
		}
		cancelActors()
		select {
		case <-sysfsActor.done:
		case <-time.After(time.Second):
			t.Error("sysfs workers failed to stop after blocked I/O release")
		}
		<-mqttActor.done
		<-forwardDone
	})
	waitForCoverReadiness(t, reports, topics, []entity.CoverID{"unit.cover.a", "unit.cover.b"})
	require.NoError(t, os.Remove(devices[0].Path))
	require.NoError(t, syscall.Mkfifo(devices[0].Path, 0o600))
	fifoCreated = true
	mqttActor.events <- mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: mqtt.CoverCommandTopic(topics, "unit.cover.a"), Payload: []byte("OPEN")})
	select {
	case <-queueRejected:
	case <-time.After(3 * time.Second):
		require.FailNow(t, "blocked cover retries did not saturate its worker queue")
	}
	mqttActor.events <- mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: mqtt.CoverCommandTopic(topics, "unit.cover.b"), Payload: []byte("OPEN")})
	waitForCoverReport(t, reports, mqtt.CoverStateTopic(topics, "unit.cover.b"), entity.CoverStateOpen)
	assertCoverRelayValues(t, devices[2].Path, devices[3].Path, "0\n", "0\n")
	mqttActor.events <- mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: mqtt.CoverCommandTopic(topics, "unit.cover.b"), Payload: []byte("CLOSE")})
	waitForCoverReport(t, reports, mqtt.CoverStateTopic(topics, "unit.cover.b"), entity.CoverStateClosing)
	assertCoverRelayValues(t, devices[2].Path, devices[3].Path, "0\n", "1\n")
	mqttActor.events <- mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: mqtt.CoverCommandTopic(topics, "unit.cover.b"), Payload: []byte("STOP"), Retained: true})
	waitForCoverReport(t, reports, mqtt.CoverStateTopic(topics, "unit.cover.b"), entity.CoverStateStopped)
	assertCoverRelayValues(t, devices[2].Path, devices[3].Path, "0\n", "0\n")
}

func waitForCoverReport(t *testing.T, reports <-chan mqtt.PublishMessage, topic string, state entity.CoverState) {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case report := <-reports:
			if report.Topic != topic {
				continue
			}
			var observation mqtt.CoverObservation
			require.NoError(t, json.Unmarshal(report.Payload, &observation))
			if observation.State == state && observation.Available {
				return
			}
		case <-timer.C:
			require.FailNow(t, "cover did not report expected state", "%s: %s", topic, state)
		}
	}
}

func assertCoverRelayValues(t *testing.T, openPath string, closePath string, openValue string, closeValue string) {
	t.Helper()
	open, err := os.ReadFile(openPath)
	require.NoError(t, err)
	close, err := os.ReadFile(closePath)
	require.NoError(t, err)
	assert.Equal(t, openValue, string(open))
	assert.Equal(t, closeValue, string(close))
}

func TestSimultaneousCoverRunsThroughSysfsAndMQTTActors(t *testing.T) {
	reg := registry.Build(&entity.Root{
		MQTT:         entity.MQTT{Enabled: true, TopicPrefix: "nest", UnitID: "unit"},
		CoverControl: entity.CoverControl{FullTravelDuration: 5 * time.Second, OperationTimeout: time.Second, OffRetryInterval: time.Second, ShutdownPeriod: 500 * time.Millisecond, ReportingInterval: 20 * time.Millisecond},
		Covers:       []entity.Cover{{ID: "unit.cover.a", OpenRelay: "a_open", CloseRelay: "a_close"}, {ID: "unit.cover.b", OpenRelay: "b_open", CloseRelay: "b_close"}},
		Relays:       []entity.Relay{{ID: "a_open", SysfsDevice: "ro_1_1"}, {ID: "a_close", SysfsDevice: "ro_1_2"}, {ID: "b_open", SysfsDevice: "ro_1_3"}, {ID: "b_close", SysfsDevice: "ro_1_4"}},
	})
	actorContext, cancelActors := context.WithCancel(t.Context())
	shutdownContext, cancelController := context.WithCancel(t.Context())
	sysfsActor := sysfsActor{commands: make(chan sysfs.Command, 32), states: make(chan sysfs.StateChange, 32), done: make(chan struct{})}
	topics := mqtt.NewTopics("nest", "unit")
	mqttActor := mqttActor{enabled: true, commands: make(chan mqtt.Command), events: make(chan mqtt.Event, 32), done: make(chan struct{}), topics: topics}
	physicalStates := make(chan [4]bool, 32)
	go func() {
		defer close(sysfsActor.done)
		var physical [4]bool
		indexes := map[string]int{"ro_1_1": 0, "ro_1_2": 1, "ro_1_3": 2, "ro_1_4": 3}
		for {
			select {
			case <-actorContext.Done():
				return
			case command := <-sysfsActor.commands:
				physical[indexes[command.DeviceID]] = command.Kind == sysfs.OnCommand
				assert.False(t, physical[0] && physical[1], "cover A interlock after each write")
				assert.False(t, physical[2] && physical[3], "cover B interlock after each write")
				physicalStates <- physical
				select {
				case sysfsActor.states <- sysfs.StateChange{Kind: sysfs.CompletionReportKind, Completion: &sysfs.CommandCompletion{Command: command, CompletedAt: time.Now()}}:
				case <-actorContext.Done():
					return
				}
			}
		}
	}()
	reports := make(chan mqtt.PublishMessage, 128)
	go func() {
		defer close(mqttActor.done)
		for {
			select {
			case <-actorContext.Done():
				return
			case command := <-mqttActor.commands:
				if strings.Contains(command.Publish.Topic, "/covers/") {
					select {
					case reports <- command.Publish:
					case <-actorContext.Done():
						return
					}
				}
			}
		}
	}()
	controllerDone := startController(shutdownContext, reg, sysfsActor, mqttActor, modbusActor{}, controller.RuntimeOptions{FeedbackContext: actorContext})
	t.Cleanup(func() {
		cancelController()
		select {
		case <-controllerDone:
		case <-time.After(time.Second):
			t.Error("controller failed to shut down")
		}
		cancelActors()
		<-sysfsActor.done
		<-mqttActor.done
	})
	mqttActor.events <- mqtt.ConnectedEvent()
	waitForCoverReadiness(t, reports, topics, []entity.CoverID{"unit.cover.a", "unit.cover.b"})
	for _, id := range []entity.CoverID{"unit.cover.a", "unit.cover.b"} {
		mqttActor.events <- mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: mqtt.CoverCommandTopic(topics, id), Payload: []byte("OPEN")})
	}
	bothMoving := false
	moveTimer := time.NewTimer(2 * time.Second)
	defer moveTimer.Stop()
	for !bothMoving {
		select {
		case physical := <-physicalStates:
			bothMoving = physical[0] && physical[2]
		case <-moveTimer.C:
			require.FailNow(t, "both covers did not move together")
		}
	}
	mqttActor.events <- mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: mqtt.CoverCommandTopic(topics, "unit.cover.a"), Payload: []byte("CLOSE")})
	waitForCoverReport(t, reports, mqtt.CoverStateTopic(topics, "unit.cover.a"), entity.CoverStateStopped)
	waitForCoverReport(t, reports, mqtt.CoverStateTopic(topics, "unit.cover.b"), entity.CoverStateOpening)
	mqttActor.events <- mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: mqtt.CoverCommandTopic(topics, "unit.cover.b"), Payload: []byte("STOP")})
	waitForCoverReport(t, reports, mqtt.CoverStateTopic(topics, "unit.cover.b"), entity.CoverStateStopped)
}

func waitForCoverReadiness(t *testing.T, reports <-chan mqtt.PublishMessage, topics mqtt.Topics, coverIDs []entity.CoverID) {
	t.Helper()
	pending := make(map[string]bool)
	for _, id := range coverIDs {
		pending[mqtt.CoverStateTopic(topics, id)] = true
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for len(pending) > 0 {
		select {
		case report := <-reports:
			if !pending[report.Topic] {
				continue
			}
			var observation mqtt.CoverObservation
			require.NoError(t, json.Unmarshal(report.Payload, &observation))
			if observation.Available && observation.State == entity.CoverStateStopped {
				delete(pending, report.Topic)
			}
		case <-timer.C:
			require.FailNow(t, "covers did not become ready")
		}
	}
}
