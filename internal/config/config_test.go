package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	path := filepath.Join("..", "..", "test", "fixtures", "config.local.yaml")

	file, err := Load(path, "controller_1")
	require.NoError(t, err)

	assert.Equal(t, "test/fixtures", file.Sysfs.Root)
	assert.Equal(t, 100*time.Millisecond, file.Sysfs.PollIntervals.DigitalInput)
	assert.Equal(t, 250*time.Millisecond, file.Sysfs.PollIntervals.DigitalOutput)
	assert.Equal(t, time.Second, file.Sysfs.PollIntervals.RelayOutput)
	assert.Equal(t, "controller_1", file.MQTT.UnitID)
	assert.Len(t, file.DigitalInputs, 1)
	assert.Len(t, file.PushButtons, 1)
	assert.Len(t, file.Lights, 1)
	assert.Len(t, file.Relays, 1)
	assert.Len(t, file.Bindings, 1)
	assert.Equal(t, "controller_1.button.office_button", file.PushButtons[0].ID)
	assert.Equal(t, "controller_1.light.office_light", file.Lights[0].ID)
	assert.Equal(t, "controller_1.button.office_button", file.Bindings[0].Source)
	assert.Equal(t, "controller_1.light.office_light", file.Bindings[0].Target)
	assert.Equal(t, "office_button_input", file.PushButtons[0].Input)
	assert.Equal(t, "office_light_relay", file.Lights[0].Relay)
	assert.Equal(t, []string{"di_3_16", "ro_3_14"}, DeviceIDs(file))
}

func TestLoadLocalMQTTFixtureProjectsRemoteSourceBinding(t *testing.T) {
	path := filepath.Join("..", "..", "test", "fixtures", "config.local-mqtt.yaml")

	file, err := Load(path, "local")
	require.NoError(t, err)

	assert.Equal(t, "local", file.MQTT.UnitID)
	assert.True(t, file.MQTT.Enabled)
	require.Len(t, file.RemoteSourceBindings, 1)
	assert.Equal(t, BindingConfig{
		Source:             "local.button.office_button",
		Target:             "remote.light.remote_light",
		Action:             BindingActionToggle,
		ExecutionTransport: "modbus",
	}, file.RemoteSourceBindings[0])
}

func TestLoadLocalMQTTFixtureProjectsPhysicalModbusConfig(t *testing.T) {
	path := filepath.Join("..", "..", "test", "fixtures", "config.local-mqtt.yaml")

	local, err := Load(path, "local")
	require.NoError(t, err)
	assert.Equal(t, ModbusConfig{
		Mode:         ModbusModeMaster,
		Port:         "/dev/ttyNS0",
		BaudRate:     19200,
		Timeout:      500 * time.Millisecond,
		PollInterval: time.Second,
		EventSignalWrites: []ModbusEventSignalWriteConfig{{
			Unit:   "remote",
			Signal: "office_button_toggle",
			Source: "local.button.office_button",
			Target: "remote.light.remote_light",
			Action: BindingActionToggle,
			UnitID: 1,
			Coil:   1,
		}},
		StatePolls: []ModbusStatePollConfig{{
			Unit:   "remote",
			Point:  "remote_light_state",
			Entity: "remote.light.remote_light",
			UnitID: 1,
			Coil:   2,
		}},
	}, local.Modbus)

	remote, err := Load(path, "remote")
	require.NoError(t, err)
	assert.Equal(t, 1, remote.Modbus.UnitID)
	assert.Equal(t, 1, remote.Modbus.EventSignals[0].Coil)
	assert.Equal(t, 2, remote.Modbus.StatePoints[0].Coil)
}

func TestLoadLocalMQTTFixtureProjectsRemoteTargetBinding(t *testing.T) {
	path := filepath.Join("..", "..", "test", "fixtures", "config.local-mqtt.yaml")

	file, err := Load(path, "remote")
	require.NoError(t, err)

	assert.Equal(t, "remote", file.MQTT.UnitID)
	assert.True(t, file.MQTT.Enabled)
	require.Len(t, file.RemoteTargetBindings, 1)
	assert.Equal(t, BindingConfig{
		Source:             "local.button.office_button",
		Target:             "remote.light.remote_light",
		Action:             BindingActionToggle,
		ExecutionTransport: "modbus",
	}, file.RemoteTargetBindings[0])
}

func TestLoadRejectsUnknownUnit(t *testing.T) {
	path := filepath.Join("..", "..", "test", "fixtures", "config.local.yaml")

	_, err := Load(path, "missing_unit")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unit_id: unknown unit "missing_unit"`)
}

func TestProjectUnitAcceptsSysfsPollIntervals(t *testing.T) {
	file, err := ProjectUnit(&GlobalRoot{
		Units: map[string]UnitConfig{
			"controller_1": {
				Actors: UnitActorsConfig{
					Sysfs: UnitSysfsConfig{
						Root: "/tmp",
						PollIntervals: PollIntervalsConfig{
							DigitalInput:  100 * time.Millisecond,
							DigitalOutput: 250 * time.Millisecond,
							RelayOutput:   2 * time.Second,
						},
						DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
					},
				},
			},
		},
	}, "controller_1")
	require.NoError(t, err)

	assert.Equal(t, 100*time.Millisecond, file.Sysfs.PollIntervals.DigitalInput)
	assert.Equal(t, 250*time.Millisecond, file.Sysfs.PollIntervals.DigitalOutput)
	assert.Equal(t, 2*time.Second, file.Sysfs.PollIntervals.RelayOutput)
}

func TestProjectUnitProjectsTypedEntityEndpoints(t *testing.T) {
	file, err := ProjectUnit(&GlobalRoot{
		Units: map[string]UnitConfig{
			"controller_1": {
				Actors: UnitActorsConfig{
					Sysfs: UnitSysfsConfig{
						DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
						Relays:        []RelayConfig{{ID: "light_relay", Name: "Light relay", Device: "ro_3_14"}},
					},
				},
				Entities: UnitEntitiesConfig{
					Buttons: []UnitPushButtonConfig{{
						ID:   "button",
						Name: "Button",
						Input: EndpointRefConfig{
							Actor: "sysfs",
							Kind:  "digital_input",
							ID:    "button_input",
						},
					}},
					Lights: []UnitLightConfig{{
						ID:   "light",
						Name: "Light",
						Actuator: EndpointRefConfig{
							Actor: "sysfs",
							Kind:  "relay",
							ID:    "light_relay",
						},
					}},
				},
			},
		},
	}, "controller_1")
	require.NoError(t, err)

	require.Len(t, file.PushButtons, 1)
	require.Len(t, file.Lights, 1)
	assert.Equal(t, "controller_1.button.button", file.PushButtons[0].ID)
	assert.Equal(t, "controller_1.light.light", file.Lights[0].ID)
	assert.Equal(t, "button_input", file.PushButtons[0].Input)
	assert.Equal(t, "light_relay", file.Lights[0].Relay)
}

func TestProjectUnitDerivesUnitSpecificMQTTClientID(t *testing.T) {
	file, err := ProjectUnit(&GlobalRoot{
		Actors: GlobalActorsConfig{
			MQTT: GlobalMQTTConfig{
				Broker: MQTTConfig{
					Enabled:     true,
					Host:        "localhost",
					Port:        1883,
					ClientID:    "nest-local",
					TopicPrefix: "nest",
				},
			},
		},
		Units: map[string]UnitConfig{
			"remote": {
				Actors: UnitActorsConfig{
					MQTT: UnitMQTTConfig{Enabled: true},
				},
			},
		},
	}, "remote")
	require.NoError(t, err)

	assert.Equal(t, "remote", file.MQTT.UnitID)
	assert.Equal(t, "nest-local-remote", file.MQTT.ClientID)
}

func TestProjectUnitProjectsRemoteBindingsByPerspective(t *testing.T) {
	global := &GlobalRoot{
		Units: map[string]UnitConfig{
			"controller_1": {
				Actors: UnitActorsConfig{
					Sysfs: UnitSysfsConfig{
						DigitalInputs: []DigitalInputConfig{{ID: "office_button_input", Device: "di_3_16"}},
					},
				},
				Entities: UnitEntitiesConfig{
					Buttons: []UnitPushButtonConfig{{
						ID:   "office_button",
						Name: "Office button",
						Input: EndpointRefConfig{
							Actor: "sysfs",
							Kind:  "digital_input",
							ID:    "office_button_input",
						},
					}},
				},
			},
			"controller_2": {
				Actors: UnitActorsConfig{
					Sysfs: UnitSysfsConfig{
						Relays: []RelayConfig{{ID: "hall_light_relay", Name: "Hall light relay", Device: "ro_3_14"}},
					},
				},
				Entities: UnitEntitiesConfig{
					Lights: []UnitLightConfig{{
						ID:   "hall_light",
						Name: "Hall light",
						Actuator: EndpointRefConfig{
							Actor: "sysfs",
							Kind:  "relay",
							ID:    "hall_light_relay",
						},
					}},
				},
			},
		},
		Bindings: []GlobalBindingConfig{{
			Source:             "controller_1.button.office_button",
			Target:             "controller_2.light.hall_light",
			Action:             "toggle",
			ExecutionTransport: "mqtt",
		}},
	}

	sourceLocal, err := ProjectUnit(global, "controller_1")
	require.NoError(t, err)
	targetLocal, err := ProjectUnit(global, "controller_2")
	require.NoError(t, err)

	assert.Empty(t, sourceLocal.Bindings)
	assert.Empty(t, targetLocal.Bindings)
	assert.Equal(t, []BindingConfig{{
		Source:             "controller_1.button.office_button",
		Target:             "controller_2.light.hall_light",
		Action:             "toggle",
		ExecutionTransport: "mqtt",
	}}, sourceLocal.RemoteSourceBindings)
	assert.Equal(t, []BindingConfig{{
		Source:             "controller_1.button.office_button",
		Target:             "controller_2.light.hall_light",
		Action:             "toggle",
		ExecutionTransport: "mqtt",
	}}, targetLocal.RemoteTargetBindings)
}

func TestProjectUnitRejectsUnknownGlobalBindingEndpoints(t *testing.T) {
	_, err := ProjectUnit(&GlobalRoot{
		Units: map[string]UnitConfig{
			"controller_1": {
				Entities: UnitEntitiesConfig{
					Buttons: []UnitPushButtonConfig{{
						ID:   "office_button",
						Name: "Office button",
						Input: EndpointRefConfig{
							Actor: "sysfs",
							Kind:  "digital_input",
							ID:    "office_button_input",
						},
					}},
				},
			},
			"controller_2": {
				Entities: UnitEntitiesConfig{
					Lights: []UnitLightConfig{{
						ID:   "hall_light",
						Name: "Hall light",
						Actuator: EndpointRefConfig{
							Actor: "sysfs",
							Kind:  "relay",
							ID:    "hall_light_relay",
						},
					}},
				},
			},
		},
		Bindings: []GlobalBindingConfig{
			{Source: "controller_1.button.missing_button", Target: "controller_2.light.hall_light", Action: "toggle"},
			{Source: "controller_1.button.office_button", Target: "controller_2.light.missing_light", Action: "toggle"},
		},
	}, "controller_2")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `bindings[0].source: unknown button "controller_1.button.missing_button"`)
	assert.Contains(t, err.Error(), `bindings[1].target: unknown light "controller_2.light.missing_light"`)
}

func TestValidateGlobalBindingsRejectsModbusBindingWithoutWriteRoute(t *testing.T) {
	global := &GlobalRoot{
		Units: map[string]UnitConfig{
			"master": {
				Entities: UnitEntitiesConfig{
					Buttons: []UnitPushButtonConfig{{ID: "button"}},
				},
			},
			"slave": {
				Entities: UnitEntitiesConfig{
					Lights: []UnitLightConfig{{ID: "light"}},
				},
			},
		},
		Bindings: []GlobalBindingConfig{{
			Source:             "master.button.button",
			Target:             "slave.light.light",
			Action:             BindingActionToggle,
			ExecutionTransport: "modbus",
		}},
	}

	err := validateGlobalBindings(global)

	require.ErrorContains(t, err, `bindings[0]: missing matching Modbus event signal write`)
}

func TestProjectUnitProjectsUnitModbusConfig(t *testing.T) {
	global := &GlobalRoot{
		Units: map[string]UnitConfig{
			"local": {
				Actors: UnitActorsConfig{
					Modbus: UnitModbusConfig{
						Mode:         ModbusModeMaster,
						Port:         "/dev/ttyNS0",
						BaudRate:     19200,
						Timeout:      500 * time.Millisecond,
						PollInterval: time.Second,
						EventSignalWrites: []ModbusEventSignalWriteConfig{{
							Unit:   "remote",
							Signal: "office_button_toggle",
							Source: "local.button.office_button",
							Target: "remote.light.remote_light",
							Action: BindingActionToggle,
						}},
						StatePolls: []ModbusStatePollConfig{{
							Unit:   "remote",
							Point:  "remote_light_state",
							Entity: "remote.light.remote_light",
						}},
					},
				},
			},
			"remote": {
				Actors: UnitActorsConfig{
					Modbus: UnitModbusConfig{
						Mode:     ModbusModeSlave,
						Port:     "/dev/ttyNS0",
						BaudRate: 19200,
						Timeout:  500 * time.Millisecond,
						UnitID:   1,
						EventSignals: []ModbusEventSignalConfig{{
							ID:     "office_button_toggle",
							Coil:   1,
							Source: "local.button.office_button",
							Target: "remote.light.remote_light",
							Action: BindingActionToggle,
						}},
						StatePoints: []ModbusStatePointConfig{{
							ID:     "remote_light_state",
							Coil:   2,
							Entity: "remote.light.remote_light",
						}},
					},
				},
				Entities: UnitEntitiesConfig{
					Lights: []UnitLightConfig{{
						ID: "remote_light",
						Actuator: EndpointRefConfig{
							Actor: "sysfs",
							Kind:  "relay",
							ID:    "remote_light_relay",
						},
					}},
				},
			},
		},
	}

	local, err := ProjectUnit(global, "local")
	require.NoError(t, err)
	remote, err := ProjectUnit(global, "remote")
	require.NoError(t, err)

	assert.Equal(t, ModbusModeMaster, local.Modbus.Mode)
	assert.Equal(t, "/dev/ttyNS0", local.Modbus.Port)
	assert.Equal(t, 19200, local.Modbus.BaudRate)
	assert.Equal(t, 500*time.Millisecond, local.Modbus.Timeout)
	assert.Equal(t, time.Second, local.Modbus.PollInterval)
	assert.Equal(t, []ModbusEventSignalWriteConfig{{
		Unit:   "remote",
		Signal: "office_button_toggle",
		Source: "local.button.office_button",
		Target: "remote.light.remote_light",
		Action: BindingActionToggle,
		UnitID: 1,
		Coil:   1,
	}}, local.Modbus.EventSignalWrites)
	assert.Equal(t, []ModbusStatePollConfig{{
		Unit:   "remote",
		Point:  "remote_light_state",
		Entity: "remote.light.remote_light",
		UnitID: 1,
		Coil:   2,
	}}, local.Modbus.StatePolls)
	assert.Empty(t, local.Modbus.EventSignals)
	assert.Empty(t, local.Modbus.StatePoints)

	assert.Equal(t, ModbusModeSlave, remote.Modbus.Mode)
	assert.Equal(t, "/dev/ttyNS0", remote.Modbus.Port)
	assert.Equal(t, 19200, remote.Modbus.BaudRate)
	assert.Equal(t, 500*time.Millisecond, remote.Modbus.Timeout)
	assert.Equal(t, 1, remote.Modbus.UnitID)
	assert.Equal(t, []ModbusEventSignalConfig{{
		ID:     "office_button_toggle",
		Coil:   1,
		Source: "local.button.office_button",
		Target: "remote.light.remote_light",
		Action: BindingActionToggle,
	}}, remote.Modbus.EventSignals)
	assert.Equal(t, []ModbusStatePointConfig{{
		ID:     "remote_light_state",
		Coil:   2,
		Entity: "remote.light.remote_light",
	}}, remote.Modbus.StatePoints)
	assert.Empty(t, remote.Modbus.EventSignalWrites)
	assert.Empty(t, remote.Modbus.StatePolls)
}

func TestProjectUnitRejectsUnknownModbusRouteEndpoints(t *testing.T) {
	_, err := ProjectUnit(&GlobalRoot{
		Units: map[string]UnitConfig{
			"local": {
				Actors: UnitActorsConfig{
					Modbus: UnitModbusConfig{
						Mode: ModbusModeMaster,
						EventSignalWrites: []ModbusEventSignalWriteConfig{{
							Unit:   "remote",
							Signal: "missing_signal",
							Source: "local.button.office_button",
							Target: "remote.light.remote_light",
							Action: BindingActionToggle,
						}},
						StatePolls: []ModbusStatePollConfig{{
							Unit:   "remote",
							Point:  "missing_point",
							Entity: "remote.light.remote_light",
						}},
					},
				},
			},
			"remote": {
				Actors: UnitActorsConfig{
					Modbus: UnitModbusConfig{Mode: ModbusModeSlave},
				},
			},
		},
	}, "local")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `units.local.actors.modbus.event_signal_writes[0].signal: unknown event signal "missing_signal" on unit "remote"`)
	assert.Contains(t, err.Error(), `units.local.actors.modbus.state_polls[0].point: unknown state point "missing_point" on unit "remote"`)
}

func TestValidateGlobalModbusRejectsInvalidStatePointEntities(t *testing.T) {
	tests := []struct {
		name    string
		entity  string
		message string
	}{
		{
			name:    "missing light",
			entity:  "remote.light.missing",
			message: `units.remote.actors.modbus.state_points[0].entity: unknown light "remote.light.missing"`,
		},
		{
			name:    "non-local light",
			entity:  "local.light.local_light",
			message: `units.remote.actors.modbus.state_points[0].entity: must reference a light on unit "remote"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			global := &GlobalRoot{
				Units: map[string]UnitConfig{
					"local": {
						Actors: UnitActorsConfig{Modbus: UnitModbusConfig{
							Mode:     ModbusModeMaster,
							Port:     "/dev/ttyNS0",
							BaudRate: 19200,
						}},
						Entities: UnitEntitiesConfig{Lights: []UnitLightConfig{{ID: "local_light"}}},
					},
					"remote": {
						Actors: UnitActorsConfig{Modbus: UnitModbusConfig{
							Mode:     ModbusModeSlave,
							Port:     "/dev/ttyNS0",
							BaudRate: 19200,
							UnitID:   1,
							StatePoints: []ModbusStatePointConfig{{
								ID:     "light_state",
								Entity: tt.entity,
							}},
						}},
						Entities: UnitEntitiesConfig{Lights: []UnitLightConfig{{ID: "remote_light"}}},
					},
				},
			}

			err := validateGlobalModbus(global)

			require.ErrorContains(t, err, tt.message)
		})
	}
}

func TestValidateGlobalModbusRejectsMismatchedEventSignalWriteMetadata(t *testing.T) {
	tests := []struct {
		name         string
		write        ModbusEventSignalWriteConfig
		signalAction string
		message      string
	}{
		{
			name: "source",
			write: ModbusEventSignalWriteConfig{
				Source: "local.button.hallway_button",
				Target: "remote.light.remote_light",
				Action: BindingActionToggle,
			},
			message: `event_signal_writes[0].source: must match event signal "office_button_toggle" source "local.button.office_button"`,
		},
		{
			name: "target",
			write: ModbusEventSignalWriteConfig{
				Source: "local.button.office_button",
				Target: "remote.light.hallway_light",
				Action: BindingActionToggle,
			},
			message: `event_signal_writes[0].target: must match event signal "office_button_toggle" target "remote.light.remote_light"`,
		},
		{
			name: "action",
			write: ModbusEventSignalWriteConfig{
				Source: "local.button.office_button",
				Target: "remote.light.remote_light",
				Action: BindingActionToggle,
			},
			signalAction: "press",
			message:      `event_signal_writes[0].action: must match event signal "office_button_toggle" action "press"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.write.Unit = "remote"
			tt.write.Signal = "office_button_toggle"
			if tt.signalAction == "" {
				tt.signalAction = BindingActionToggle
			}
			global := &GlobalRoot{
				Units: map[string]UnitConfig{
					"local": {
						Actors: UnitActorsConfig{
							Modbus: UnitModbusConfig{
								Mode:              ModbusModeMaster,
								EventSignalWrites: []ModbusEventSignalWriteConfig{tt.write},
							},
						},
					},
					"remote": {
						Actors: UnitActorsConfig{
							Modbus: UnitModbusConfig{
								Mode:   ModbusModeSlave,
								UnitID: 1,
								EventSignals: []ModbusEventSignalConfig{{
									ID:     "office_button_toggle",
									Source: "local.button.office_button",
									Target: "remote.light.remote_light",
									Action: tt.signalAction,
								}},
							},
						},
					},
				},
			}

			err := validateGlobalModbus(global)

			require.ErrorContains(t, err, tt.message)
		})
	}
}

func TestProjectUnitRejectsMultipleModbusMasters(t *testing.T) {
	_, err := ProjectUnit(&GlobalRoot{
		Units: map[string]UnitConfig{
			"master_a": {Actors: UnitActorsConfig{Modbus: UnitModbusConfig{Mode: ModbusModeMaster}}},
			"master_b": {Actors: UnitActorsConfig{Modbus: UnitModbusConfig{Mode: ModbusModeMaster}}},
		},
	}, "master_a")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple master units")
}

func TestProjectUnitRejectsDuplicateModbusSlaveIDs(t *testing.T) {
	_, err := ProjectUnit(&GlobalRoot{
		Units: map[string]UnitConfig{
			"master":  {Actors: UnitActorsConfig{Modbus: UnitModbusConfig{Mode: ModbusModeMaster}}},
			"slave_a": {Actors: UnitActorsConfig{Modbus: UnitModbusConfig{Mode: ModbusModeSlave, UnitID: 1}}},
			"slave_b": {Actors: UnitActorsConfig{Modbus: UnitModbusConfig{Mode: ModbusModeSlave, UnitID: 1}}},
		},
	}, "master")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicates slave unit")
}

func TestProjectUnitRejectsModbusWithoutMaster(t *testing.T) {
	_, err := ProjectUnit(&GlobalRoot{
		Units: map[string]UnitConfig{
			"slave": {Actors: UnitActorsConfig{Modbus: UnitModbusConfig{Mode: ModbusModeSlave, UnitID: 1}}},
		},
	}, "slave")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one master unit is required")
}

func TestProjectUnitRejectsUnsupportedEntityEndpoints(t *testing.T) {
	_, err := ProjectUnit(&GlobalRoot{
		Units: map[string]UnitConfig{
			"controller_1": {
				Entities: UnitEntitiesConfig{
					Buttons: []UnitPushButtonConfig{{
						ID:   "button",
						Name: "Button",
						Input: EndpointRefConfig{
							Actor: "sysfs",
							Kind:  "relay",
							ID:    "button_input",
						},
					}},
					Lights: []UnitLightConfig{{
						ID:   "light",
						Name: "Light",
						Actuator: EndpointRefConfig{
							Actor: "mqtt",
							Kind:  "relay",
							ID:    "light_relay",
						},
					}},
				},
			},
		},
	}, "controller_1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `entities.buttons[0].input.kind: unsupported endpoint kind "relay", expected "digital_input"`)
	assert.Contains(t, err.Error(), `entities.lights[0].actuator.actor: unsupported endpoint actor "mqtt", expected "sysfs"`)
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	writeTestFile(t, path, "actors:\n  mqtt:\n    broker:\n      enabled: false\nunits: {}\nunknown: true\n")

	_, err := Load(path, "controller_1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field unknown not found")
}

func TestLoadRejectsTrailingYAMLDocuments(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	writeTestFile(t, path, "actors:\n  mqtt:\n    broker:\n      enabled: false\nunits: {}\n---\nextra: true\n")

	_, err := Load(path, "controller_1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple YAML documents are not supported")
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		file    Root
		message string
	}{
		{
			name:    "requires sysfs root",
			file:    Root{},
			message: "sysfs.root: required",
		},
		{
			name: "rejects configs with no devices",
			file: Root{
				Sysfs: SysfsConfig{Root: "/tmp"},
			},
			message: "at least one digital_input or relay is required",
		},
		{
			name: "rejects whitespace padded sysfs root",
			file: Root{
				Sysfs:  SysfsConfig{Root: " /tmp "},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "sysfs.root: must not have leading or trailing whitespace",
		},
		{
			name: "rejects negative sysfs poll intervals",
			file: Root{
				Sysfs: SysfsConfig{
					Root: "/tmp",
					PollIntervals: PollIntervalsConfig{
						DigitalInput: -time.Second,
					},
				},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
			},
			message: "sysfs.poll_intervals.digital_input: must not be negative",
		},
		{
			name: "requires enabled MQTT host",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithHost(""),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.host: required",
		},
		{
			name: "rejects enabled MQTT port below range",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithPort(0),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.port: must be between 1 and 65535",
		},
		{
			name: "rejects enabled MQTT port above range",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithPort(65536),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.port: must be between 1 and 65535",
		},
		{
			name: "rejects enabled MQTT topic prefix with empty segments",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithTopicPrefix("nest//controller"),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.topic_prefix: must be a relative MQTT topic prefix without empty segments",
		},
		{
			name: "rejects whitespace padded MQTT username",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithUsername(" user "),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: "mqtt.username: must not have leading or trailing whitespace",
		},
		{
			name: "rejects duplicate digital input ids",
			file: Root{
				Sysfs: SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{
					{ID: "button_input", Device: "di_3_16"},
					{ID: "button_input", Device: "di_3_15"},
				},
			},
			message: `digital_inputs[1].id: duplicate id "button_input"`,
		},
		{
			name: "rejects duplicate digital input devices",
			file: Root{
				Sysfs: SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{
					{ID: "button_input_1", Device: "di_3_16"},
					{ID: "button_input_2", Device: "di_3_16"},
				},
			},
			message: `digital_inputs[1].device: duplicate digital input device "di_3_16"`,
		},
		{
			name: "rejects invalid digital input devices",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "ro_3_14"}},
			},
			message: `digital_inputs[0].device: invalid digital input device "ro_3_14"`,
		},
		{
			name: "requires buttons to reference known inputs",
			file: Root{
				Sysfs:       SysfsConfig{Root: "/tmp"},
				PushButtons: []PushButtonConfig{{ID: "button", Name: "Button", Input: "missing"}},
			},
			message: `push_buttons[0].input: unknown digital input "missing"`,
		},
		{
			name: "rejects whitespace padded button ids",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: " button ", Name: "Button", Input: "button_input"}},
			},
			message: "push_buttons[0].id: must not have leading or trailing whitespace",
		},
		{
			name: "rejects MQTT unit ids with unsupported characters",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				MQTT:   validMQTTConfigWithUnitID("controller-1"),
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
			},
			message: `mqtt.unit_id: must contain only lowercase letters, numbers, and underscores "controller-1"`,
		},
		{
			name: "rejects light ids with unsupported characters",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Lights: []LightConfig{{ID: "office-light", Name: "Office light", Relay: "relay"}},
			},
			message: `lights[0].id: must be a local id or light semantic id "office-light"`,
		},
		{
			name: "rejects whitespace padded button input references",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: "button", Name: "Button", Input: " button_input "}},
			},
			message: "push_buttons[0].input: must not have leading or trailing whitespace",
		},
		{
			name: "requires lights to reference known relays",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Lights: []LightConfig{{ID: "light", Name: "Light", Relay: "missing"}},
			},
			message: `lights[0].relay: unknown relay "missing"`,
		},
		{
			name: "rejects invalid relay devices",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "di_3_16"}},
			},
			message: `relays[0].device: invalid relay device "di_3_16"`,
		},
		{
			name: "rejects duplicate relay devices",
			file: Root{
				Sysfs: SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{
					{ID: "relay_1", Name: "Relay 1", Device: "ro_3_14"},
					{ID: "relay_2", Name: "Relay 2", Device: "ro_3_14"},
				},
			},
			message: `relays[1].device: duplicate relay device "ro_3_14"`,
		},
		{
			name: "rejects whitespace padded relay devices",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: " ro_3_14 "}},
			},
			message: "relays[0].device: must not have leading or trailing whitespace",
		},
		{
			name: "requires bindings to reference known push buttons",
			file: Root{
				Sysfs:    SysfsConfig{Root: "/tmp"},
				Relays:   []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Lights:   []LightConfig{{ID: "light", Name: "Light", Relay: "relay"}},
				Bindings: []BindingConfig{{Source: "missing", Target: "light", Action: BindingActionToggle}},
			},
			message: `bindings[0].source: unknown push button "missing"`,
		},
		{
			name: "requires bindings to reference known lights",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: "button", Name: "Button", Input: "button_input"}},
				Bindings:      []BindingConfig{{Source: "button", Target: "missing", Action: BindingActionToggle}},
			},
			message: `bindings[0].target: unknown light "missing"`,
		},
		{
			name: "rejects unsupported binding actions",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: "button", Name: "Button", Input: "button_input"}},
				Relays:        []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Lights:        []LightConfig{{ID: "light", Name: "Light", Relay: "relay"}},
				Bindings:      []BindingConfig{{Source: "button", Target: "light", Action: "press"}},
			},
			message: `bindings[0].action: unsupported action "press"`,
		},
		{
			name: "rejects unsupported remote binding actions",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				RemoteSourceBindings: []BindingConfig{{
					Source: "controller_1.button.office_button",
					Target: "controller_2.light.hall_light",
					Action: "press",
				}},
			},
			message: `remote_source_bindings[0].action: unsupported action "press"`,
		},
		{
			name: "rejects duplicate bindings",
			file: Root{
				Sysfs:         SysfsConfig{Root: "/tmp"},
				DigitalInputs: []DigitalInputConfig{{ID: "button_input", Device: "di_3_16"}},
				PushButtons:   []PushButtonConfig{{ID: "button", Name: "Button", Input: "button_input"}},
				Relays:        []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Lights:        []LightConfig{{ID: "light", Name: "Light", Relay: "relay"}},
				Bindings: []BindingConfig{
					{Source: "button", Target: "light", Action: BindingActionToggle},
					{Source: "button", Target: "light", Action: BindingActionToggle},
				},
			},
			message: `bindings[1]: duplicate binding source "button" target "light" action "toggle"`,
		},
		{
			name: "rejects unsupported modbus mode",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode: "both",
				},
			},
			message: `modbus.mode: unsupported mode "both"`,
		},
		{
			name: "requires modbus mode when configured",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Port:     "/dev/ttyNS0",
					BaudRate: 19200,
				},
			},
			message: "modbus.mode: required when Modbus is configured",
		},
		{
			name: "requires modbus port in slave mode",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode:     ModbusModeSlave,
					BaudRate: 19200,
					UnitID:   1,
				},
			},
			message: "modbus.port: required",
		},
		{
			name: "requires positive modbus baud rate when configured",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode: ModbusModeMaster,
					Port: "/dev/ttyNS0",
				},
			},
			message: "modbus.baudrate: must be positive",
		},
		{
			name: "rejects slave modbus write routes",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode: ModbusModeSlave,
					EventSignalWrites: []ModbusEventSignalWriteConfig{{
						Unit:   "remote",
						Signal: "office_button_toggle",
						Source: "local.button.office_button",
						Target: "remote.light.remote_light",
						Action: BindingActionToggle,
					}},
				},
			},
			message: "modbus.event_signal_writes: requires master mode",
		},
		{
			name: "rejects master modbus exposed signals",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode: ModbusModeMaster,
					EventSignals: []ModbusEventSignalConfig{{
						ID:     "office_button_toggle",
						Source: "local.button.office_button",
						Target: "remote.light.remote_light",
						Action: BindingActionToggle,
					}},
				},
			},
			message: "modbus.event_signals: requires slave mode",
		},
		{
			name: "rejects invalid modbus event signal source",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode: ModbusModeSlave,
					EventSignals: []ModbusEventSignalConfig{{
						ID:     "office_button_toggle",
						Source: "local.light.office_light",
						Target: "remote.light.remote_light",
						Action: BindingActionToggle,
					}},
				},
			},
			message: `modbus.event_signals[0].source: must be a button semantic id "local.light.office_light"`,
		},
		{
			name: "rejects negative modbus timeout",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode:    ModbusModeMaster,
					Timeout: -time.Millisecond,
				},
			},
			message: "modbus.timeout: must not be negative",
		},
		{
			name: "rejects zero slave unit id",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode:   ModbusModeSlave,
					UnitID: 0,
				},
			},
			message: "modbus.unit_id: must be between 1 and 247",
		},
		{
			name: "rejects out of range modbus coil",
			file: Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode: ModbusModeSlave,
					EventSignals: []ModbusEventSignalConfig{{
						ID:     "signal",
						Coil:   65536,
						Source: "local.button.button",
						Target: "local.light.light",
						Action: BindingActionToggle,
					}},
				},
			},
			message: "modbus.event_signals[0].coil: must be between 0 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.file)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestValidateRejectsDuplicateModbusCoils(t *testing.T) {
	tests := []struct {
		name         string
		eventSignals []ModbusEventSignalConfig
		statePoints  []ModbusStatePointConfig
		message      string
	}{
		{
			name: "event signals",
			eventSignals: []ModbusEventSignalConfig{
				{ID: "button_a", Coil: 1, Source: "local.button.button_a", Target: "local.light.light", Action: BindingActionToggle},
				{ID: "button_b", Coil: 1, Source: "local.button.button_b", Target: "local.light.light", Action: BindingActionToggle},
			},
			message: "modbus.event_signals[1].coil: duplicates modbus.event_signals[0].coil at address 1",
		},
		{
			name: "state points",
			statePoints: []ModbusStatePointConfig{
				{ID: "light_a", Coil: 1, Entity: "local.light.light_a"},
				{ID: "light_b", Coil: 1, Entity: "local.light.light_b"},
			},
			message: "modbus.state_points[1].coil: duplicates modbus.state_points[0].coil at address 1",
		},
		{
			name: "event signal and state point",
			eventSignals: []ModbusEventSignalConfig{
				{ID: "button", Coil: 1, Source: "local.button.button", Target: "local.light.light", Action: BindingActionToggle},
			},
			statePoints: []ModbusStatePointConfig{
				{ID: "light", Coil: 1, Entity: "local.light.light"},
			},
			message: "modbus.state_points[0].coil: duplicates modbus.event_signals[0].coil at address 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&Root{
				Sysfs:  SysfsConfig{Root: "/tmp"},
				Relays: []RelayConfig{{ID: "relay", Name: "Relay", Device: "ro_3_14"}},
				Modbus: ModbusConfig{
					Mode:         ModbusModeSlave,
					Port:         "/dev/ttyNS0",
					BaudRate:     19200,
					UnitID:       1,
					EventSignals: tt.eventSignals,
					StatePoints:  tt.statePoints,
				},
			})

			require.ErrorContains(t, err, tt.message)
		})
	}
}

func TestValidateAcceptsValidFile(t *testing.T) {
	file := Root{
		Sysfs: SysfsConfig{Root: "/tmp"},
		MQTT:  validMQTTConfig(),
		DigitalInputs: []DigitalInputConfig{
			{ID: "office_button_input", Device: "di_3_16"},
		},
		PushButtons: []PushButtonConfig{
			{ID: "office_button", Name: "Office button", Input: "office_button_input"},
		},
		Relays: []RelayConfig{
			{ID: "office_light_relay", Name: "Office light relay", Device: "ro_3_14"},
		},
		Lights: []LightConfig{
			{ID: "office_light", Name: "Office light", Relay: "office_light_relay"},
		},
		Bindings: []BindingConfig{
			{Source: "office_button", Target: "office_light", Action: BindingActionToggle},
		},
	}

	require.NoError(t, Validate(&file))
}

func validMQTTConfig() MQTTConfig {
	return validMQTTConfigWithPort(1883)
}

func validMQTTConfigWithPort(port int) MQTTConfig {
	return MQTTConfig{
		Enabled:     true,
		Host:        "localhost",
		Port:        port,
		ClientID:    "nest-controller-1",
		TopicPrefix: "nest",
		UnitID:      "controller_1",
	}
}

func validMQTTConfigWithTopicPrefix(topicPrefix string) MQTTConfig {
	mqtt := validMQTTConfig()
	mqtt.TopicPrefix = topicPrefix
	return mqtt
}

func validMQTTConfigWithHost(host string) MQTTConfig {
	mqtt := validMQTTConfig()
	mqtt.Host = host
	return mqtt
}

func validMQTTConfigWithUsername(username string) MQTTConfig {
	mqtt := validMQTTConfig()
	mqtt.Username = username
	return mqtt
}

func validMQTTConfigWithUnitID(unitID string) MQTTConfig {
	mqtt := validMQTTConfig()
	mqtt.UnitID = unitID
	return mqtt
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
