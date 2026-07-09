package registry

import (
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	root := &entity.Root{
		DigitalInputs: []entity.DigitalInput{
			{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")},
		},
		PushButtons: []entity.PushButton{
			{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")},
		},
		Lights: []entity.Light{
			{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")},
		},
		Relays: []entity.Relay{
			{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")},
		},
		Bindings: []entity.Binding{
			{Source: entity.ID("office_button"), Target: entity.ID("office_light"), Action: entity.ActionToggle},
		},
		RemoteSourceBindings: []entity.Binding{
			{Source: entity.ID("controller_1.button.office_button"), Target: entity.ID("controller_2.light.hall_light"), Action: entity.ActionToggle},
		},
		RemoteTargetBindings: []entity.Binding{
			{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle},
		},
	}

	index := Build(root)
	require.NotNil(t, index)

	assert.Equal(t, root.DigitalInputs[0], index.DigitalInputsByDevice[entity.SysfsDeviceID("di_3_16")])
	assert.Equal(t, root.PushButtons[0], index.PushButtonsByID[entity.PushButtonID("office_button")])
	assert.Equal(t, root.PushButtons, index.PushButtonsByInputID[entity.DigitalInputID("office_button_input")])
	assert.Equal(t, root.Lights[0], index.LightsByID[entity.LightID("office_light")])
	assert.Equal(t, root.Lights, index.LightsByRelayID[entity.RelayID("office_light_relay")])
	assert.Equal(t, root.Bindings, index.BindingsBySourceID[entity.ID("office_button")])
	assert.Equal(t, root.RemoteSourceBindings, index.RemoteSourceBindingsBySourceID[entity.ID("controller_1.button.office_button")])
	assert.Equal(t, root.RemoteTargetBindings, index.RemoteTargetBindingsBySourceID[entity.ID("controller_2.button.hall_button")])
	assert.Equal(t, root.Relays[0], index.RelaysByID[entity.RelayID("office_light_relay")])
	assert.Equal(t, root.Relays[0], index.RelaysByDevice[entity.SysfsDeviceID("ro_3_14")])
	assert.Equal(t, []entity.SysfsDeviceID{"di_3_16", "ro_3_14"}, SysfsDeviceIDs(index))
}

func TestLookupHelpers(t *testing.T) {
	root := &entity.Root{
		DigitalInputs: []entity.DigitalInput{
			{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")},
		},
		PushButtons: []entity.PushButton{
			{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")},
		},
		Lights: []entity.Light{
			{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")},
		},
		Relays: []entity.Relay{
			{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")},
		},
		Bindings: []entity.Binding{
			{Source: entity.ID("office_button"), Target: entity.ID("office_light"), Action: entity.ActionToggle},
		},
		RemoteSourceBindings: []entity.Binding{
			{Source: entity.ID("controller_1.button.office_button"), Target: entity.ID("controller_2.light.hall_light"), Action: entity.ActionToggle},
		},
		RemoteTargetBindings: []entity.Binding{
			{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle},
		},
	}
	index := Build(root)

	input, ok := DigitalInputBySysfsDevice(index, entity.SysfsDeviceID("di_3_16"))
	require.True(t, ok)
	assert.Equal(t, root.DigitalInputs[0], input)

	assert.Equal(t, root.PushButtons, PushButtonsByInput(index, entity.DigitalInputID("office_button_input")))
	assert.Equal(t, root.Bindings, BindingsByButton(index, entity.PushButtonID("office_button")))
	assert.Equal(t, root.RemoteSourceBindings, RemoteSourceBindingsByButton(index, entity.PushButtonID("controller_1.button.office_button")))
	assert.Equal(t, root.RemoteTargetBindings, RemoteTargetBindingsByButton(index, entity.PushButtonID("controller_2.button.hall_button")))

	light, ok := LightByID(index, entity.LightID("office_light"))
	require.True(t, ok)
	assert.Equal(t, root.Lights[0], light)
	assert.Equal(t, root.Lights, LightsByRelay(index, entity.RelayID("office_light_relay")))

	relay, ok := RelayByID(index, entity.RelayID("office_light_relay"))
	require.True(t, ok)
	assert.Equal(t, root.Relays[0], relay)

	relay, ok = RelayBySysfsDevice(index, entity.SysfsDeviceID("ro_3_14"))
	require.True(t, ok)
	assert.Equal(t, root.Relays[0], relay)
}

func TestRuntimeDataHelpers(t *testing.T) {
	root := &entity.Root{
		SysfsRoot: "/sys/test",
		SysfsPollIntervals: entity.PollIntervals{
			DigitalInput:  100 * time.Millisecond,
			DigitalOutput: 200 * time.Millisecond,
			RelayOutput:   300 * time.Millisecond,
		},
		MQTT: entity.MQTT{
			Enabled:     true,
			Host:        "mqtt.local",
			Port:        1883,
			ClientID:    "nest-test",
			Username:    "nest",
			Password:    "secret",
			TopicPrefix: "nest",
			UnitID:      "controller_1",
		},
		Modbus: entity.Modbus{
			Mode: entity.ModbusModeMaster,
			EventSignals: []entity.ModbusEventSignal{
				{
					ID:     entity.ModbusEventSignalID("office_button_signal"),
					Source: entity.ID("controller_1.button.office_button"),
					Target: entity.ID("controller_2.light.office_light"),
					Action: entity.ActionToggle,
				},
			},
			StatePoints: []entity.ModbusStatePoint{
				{
					ID:     entity.ModbusStatePointID("office_light_state"),
					Entity: entity.ID("controller_2.light.office_light"),
				},
			},
			EventSignalWrites: []entity.ModbusEventSignalWrite{
				{
					Unit:   "controller_2",
					Signal: entity.ModbusEventSignalID("office_button_signal"),
					Source: entity.ID("controller_1.button.office_button"),
					Target: entity.ID("controller_2.light.office_light"),
					Action: entity.ActionToggle,
				},
			},
			StatePolls: []entity.ModbusStatePoll{
				{
					Unit:   "controller_2",
					Point:  entity.ModbusStatePointID("office_light_state"),
					Entity: entity.ID("controller_2.light.office_light"),
				},
			},
		},
		Lights: []entity.Light{
			{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")},
		},
		RemoteTargetBindings: []entity.Binding{
			{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle},
		},
	}
	reg := Build(root)

	assert.Equal(t, root.SysfsRoot, SysfsRoot(reg))
	assert.Equal(t, root.SysfsPollIntervals, SysfsPollIntervals(reg))
	assert.Equal(t, root.MQTT, MQTT(reg))
	assert.Equal(t, root.Modbus, Modbus(reg))
	assert.Equal(t, root.Lights, Lights(reg))
	assert.Equal(t, root.RemoteTargetBindings, RemoteTargetBindings(reg))
}

func TestRuntimeDataHelpersReturnCopies(t *testing.T) {
	root := &entity.Root{
		Modbus: entity.Modbus{
			EventSignals: []entity.ModbusEventSignal{
				{ID: entity.ModbusEventSignalID("office_button_signal")},
			},
		},
		Lights: []entity.Light{
			{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")},
		},
		RemoteTargetBindings: []entity.Binding{
			{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle},
		},
	}
	reg := Build(root)

	modbus := Modbus(reg)
	modbus.EventSignals[0].ID = entity.ModbusEventSignalID("changed_signal")
	lights := Lights(reg)
	lights[0].Name = "Changed light"
	bindings := RemoteTargetBindings(reg)
	bindings[0].Action = entity.Action("changed")

	assert.Equal(t, root.Modbus, Modbus(reg))
	assert.Equal(t, root.Lights, Lights(reg))
	assert.Equal(t, root.RemoteTargetBindings, RemoteTargetBindings(reg))
}
