package controller

import (
	"fmt"
	"testing"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMQTTCapacityIncludesRemoteLightsAfterReportingDrains(t *testing.T) {
	topics := mqtt.NewTopics("nest", "local")
	root := &entity.Root{
		MQTT:   entity.MQTT{TopicPrefix: topics.Prefix, UnitID: topics.UnitID},
		Lights: []entity.Light{{ID: "local.light.office"}},
		Covers: []entity.Cover{{ID: "local.cover.office"}},
	}
	for coil := range 40 {
		root.Modbus.StatePolls = append(root.Modbus.StatePolls, entity.ModbusStatePoll{
			Entity: entity.ID(fmt.Sprintf("remote.light.light_%d", coil)), UnitID: 1, Coil: uint16(coil),
		})
	}
	// Duplicate entity polls consume one reporting topic
	root.Modbus.StatePolls = append(root.Modbus.StatePolls, entity.ModbusStatePoll{Entity: "local.light.office", UnitID: 2, Coil: 0})
	reg := registry.Build(root)
	handoff := mqtt.NewHandoff(mqttHandoffCapacity(reg))
	commands := make(chan mqtt.Command, 64)
	for coil := range 40 {
		incoming, handled := semanticEventFromModbusEvent(reg, modbus.Event{Kind: modbus.CoilReadEventKind, UnitID: 1, Coil: uint16(coil), Value: true})
		require.True(t, handled)
		dispatchMQTTCommand(t.Context(), reg, commands, topics, incoming, handoff)
	}
	require.False(t, mqtt.FlushAvailable(handoff, commands))
	require.Len(t, commands, 40)
	for range 40 {
		<-commands
	}
	// Discovery and availability must still fit after all remote topics are cached
	require.NoError(t, publishMQTTStartup(t.Context(), reg, commands, handoff))
	dispatchMQTTCommand(t.Context(), reg, commands, topics, event.Event{Kind: event.LightStateKind,
		LightState: &event.LightState{LightID: "local.light.office", Value: 1}}, handoff)
	dispatchMQTTCommand(t.Context(), reg, commands, topics, event.Event{Kind: event.CoverObservationKind,
		CoverObservation: &event.CoverObservation{CoverID: "local.cover.office", State: entity.CoverStateStopped, Available: true}}, handoff)
	require.False(t, mqtt.FlushAvailable(handoff, commands))
	require.Len(t, commands, 4)
	seen := make(map[string]bool)
	for range 4 {
		seen[(<-commands).Publish.Topic] = true
	}
	assert.True(t, seen[mqtt.HomeAssistantDeviceDiscoveryTopic(topics)])
	assert.True(t, seen[mqtt.AvailabilityTopic(topics)])
	assert.True(t, seen[mqtt.LightStateTopic(topics, "local.light.office")])
	assert.True(t, seen[mqtt.CoverStateTopic(topics, "local.cover.office")])
}
