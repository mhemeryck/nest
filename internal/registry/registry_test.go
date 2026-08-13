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

	reg := Build(root)
	require.NotNil(t, reg)

	input, ok := DigitalInputBySysfsDevice(reg, entity.SysfsDeviceID("di_3_16"))
	require.True(t, ok)
	assert.Equal(t, root.DigitalInputs[0], input)
	assert.Equal(t, root.PushButtons, PushButtonsByInput(reg, entity.DigitalInputID("office_button_input")))
	assert.Equal(t, root.Lights, LightsByRelay(reg, entity.RelayID("office_light_relay")))
	assert.Equal(t, root.Bindings, BindingsBySource(reg, entity.ID("office_button")))
	assert.Equal(t, root.RemoteSourceBindings, RemoteSourceBindingsBySource(reg, entity.ID("controller_1.button.office_button")))
	assert.Equal(t, root.RemoteTargetBindings, RemoteTargetBindingsBySource(reg, entity.ID("controller_2.button.hall_button")))
	assert.Equal(t, []entity.SysfsDeviceID{"di_3_16", "ro_3_14"}, SysfsDeviceIDs(reg))

	light, ok := LightByID(reg, entity.LightID("office_light"))
	require.True(t, ok)
	assert.Equal(t, root.Lights[0], light)

	relay, ok := RelayByID(reg, entity.RelayID("office_light_relay"))
	require.True(t, ok)
	assert.Equal(t, root.Relays[0], relay)

	relay, ok = RelayBySysfsDevice(reg, entity.SysfsDeviceID("ro_3_14"))
	require.True(t, ok)
	assert.Equal(t, root.Relays[0], relay)
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

func TestLookupHelpersReturnCopies(t *testing.T) {
	root := &entity.Root{
		PushButtons: []entity.PushButton{
			{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")},
		},
		Lights: []entity.Light{
			{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")},
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
	reg := Build(root)

	pushButtons := PushButtonsByInput(reg, entity.DigitalInputID("office_button_input"))
	pushButtons[0].Name = "Changed button"
	lights := LightsByRelay(reg, entity.RelayID("office_light_relay"))
	lights[0].Name = "Changed light"
	bindings := BindingsBySource(reg, entity.ID("office_button"))
	bindings[0].Action = entity.Action("changed")
	remoteSourceBindings := RemoteSourceBindingsBySource(reg, entity.ID("controller_1.button.office_button"))
	remoteSourceBindings[0].Action = entity.Action("changed")
	remoteTargetBindings := RemoteTargetBindingsBySource(reg, entity.ID("controller_2.button.hall_button"))
	remoteTargetBindings[0].Action = entity.Action("changed")

	assert.Equal(t, root.PushButtons, PushButtonsByInput(reg, entity.DigitalInputID("office_button_input")))
	assert.Equal(t, root.Lights, LightsByRelay(reg, entity.RelayID("office_light_relay")))
	assert.Equal(t, root.Bindings, BindingsBySource(reg, entity.ID("office_button")))
	assert.Equal(t, root.RemoteSourceBindings, RemoteSourceBindingsBySource(reg, entity.ID("controller_1.button.office_button")))
	assert.Equal(t, root.RemoteTargetBindings, RemoteTargetBindingsBySource(reg, entity.ID("controller_2.button.hall_button")))
}

func TestModbusEventSignalByCoil(t *testing.T) {
	root := &entity.Root{
		Modbus: entity.Modbus{
			EventSignals: []entity.ModbusEventSignal{{
				ID:     entity.ModbusEventSignalID("office_button_signal"),
				Coil:   12,
				Source: entity.ID("controller_1.button.office_button"),
				Target: entity.ID("controller_2.light.office_light"),
				Action: entity.ActionToggle,
			}},
		},
	}
	reg := Build(root)

	signal, ok := ModbusEventSignalByCoil(reg, 12)
	require.True(t, ok)
	assert.Equal(t, root.Modbus.EventSignals[0], signal)

	_, ok = ModbusEventSignalByCoil(reg, 13)
	assert.False(t, ok)
}

func TestModbusStatePointAndPollLookups(t *testing.T) {
	root := &entity.Root{
		Modbus: entity.Modbus{
			StatePoints: []entity.ModbusStatePoint{{
				ID:     entity.ModbusStatePointID("office_light_state"),
				Coil:   12,
				Entity: entity.ID("remote.light.office_light"),
			}},
			StatePolls: []entity.ModbusStatePoll{{
				Unit:   "remote",
				Point:  entity.ModbusStatePointID("office_light_state"),
				Entity: entity.ID("remote.light.office_light"),
				UnitID: 1,
				Coil:   12,
			}},
		},
	}
	reg := Build(root)

	assert.Equal(t, root.Modbus.StatePoints, ModbusStatePointsByEntity(reg, entity.ID("remote.light.office_light")))
	poll, ok := ModbusStatePollByCoil(reg, 1, 12)
	require.True(t, ok)
	assert.Equal(t, root.Modbus.StatePolls[0], poll)

	_, ok = ModbusStatePollByCoil(reg, 1, 13)
	assert.False(t, ok)
}
