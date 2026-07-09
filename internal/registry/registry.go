package registry

import (
	"slices"

	"github.com/mhemeryck/nest/internal/entity"
)

type Registry struct {
	sysfsRoot                      string
	sysfsPollIntervals             entity.PollIntervals
	mqtt                           entity.MQTT
	modbus                         entity.Modbus
	lights                         []entity.Light
	remoteTargetBindings           []entity.Binding
	DigitalInputsByDevice          map[entity.SysfsDeviceID]entity.DigitalInput
	PushButtonsByID                map[entity.PushButtonID]entity.PushButton
	PushButtonsByInputID           map[entity.DigitalInputID][]entity.PushButton
	LightsByID                     map[entity.LightID]entity.Light
	LightsByRelayID                map[entity.RelayID][]entity.Light
	BindingsBySourceID             map[entity.ID][]entity.Binding
	RemoteSourceBindingsBySourceID map[entity.ID][]entity.Binding
	RemoteTargetBindingsBySourceID map[entity.ID][]entity.Binding
	RelaysByID                     map[entity.RelayID]entity.Relay
	RelaysByDevice                 map[entity.SysfsDeviceID]entity.Relay
}

func Build(root *entity.Root) *Registry {
	index := &Registry{
		sysfsRoot:                      root.SysfsRoot,
		sysfsPollIntervals:             root.SysfsPollIntervals,
		mqtt:                           root.MQTT,
		modbus:                         copyModbus(root.Modbus),
		lights:                         append([]entity.Light(nil), root.Lights...),
		remoteTargetBindings:           append([]entity.Binding(nil), root.RemoteTargetBindings...),
		DigitalInputsByDevice:          make(map[entity.SysfsDeviceID]entity.DigitalInput, len(root.DigitalInputs)),
		PushButtonsByID:                make(map[entity.PushButtonID]entity.PushButton, len(root.PushButtons)),
		PushButtonsByInputID:           make(map[entity.DigitalInputID][]entity.PushButton),
		LightsByID:                     make(map[entity.LightID]entity.Light, len(root.Lights)),
		LightsByRelayID:                make(map[entity.RelayID][]entity.Light),
		BindingsBySourceID:             make(map[entity.ID][]entity.Binding),
		RemoteSourceBindingsBySourceID: make(map[entity.ID][]entity.Binding),
		RemoteTargetBindingsBySourceID: make(map[entity.ID][]entity.Binding),
		RelaysByID:                     make(map[entity.RelayID]entity.Relay, len(root.Relays)),
		RelaysByDevice:                 make(map[entity.SysfsDeviceID]entity.Relay, len(root.Relays)),
	}

	for _, input := range root.DigitalInputs {
		index.DigitalInputsByDevice[input.SysfsDevice] = input
	}

	for _, button := range root.PushButtons {
		index.PushButtonsByID[button.ID] = button
		index.PushButtonsByInputID[button.Input] = append(index.PushButtonsByInputID[button.Input], button)
	}

	for _, light := range root.Lights {
		index.LightsByID[light.ID] = light
		index.LightsByRelayID[light.Relay] = append(index.LightsByRelayID[light.Relay], light)
	}

	for _, relay := range root.Relays {
		index.RelaysByID[relay.ID] = relay
		index.RelaysByDevice[relay.SysfsDevice] = relay
	}

	for _, binding := range root.Bindings {
		index.BindingsBySourceID[binding.Source] = append(index.BindingsBySourceID[binding.Source], binding)
	}

	for _, binding := range root.RemoteSourceBindings {
		index.RemoteSourceBindingsBySourceID[binding.Source] = append(index.RemoteSourceBindingsBySourceID[binding.Source], binding)
	}

	for _, binding := range root.RemoteTargetBindings {
		index.RemoteTargetBindingsBySourceID[binding.Source] = append(index.RemoteTargetBindingsBySourceID[binding.Source], binding)
	}

	return index
}

func SysfsRoot(reg *Registry) string {
	return reg.sysfsRoot
}

func SysfsPollIntervals(reg *Registry) entity.PollIntervals {
	return reg.sysfsPollIntervals
}

func MQTT(reg *Registry) entity.MQTT {
	return reg.mqtt
}

func Modbus(reg *Registry) entity.Modbus {
	return copyModbus(reg.modbus)
}

func Lights(reg *Registry) []entity.Light {
	return append([]entity.Light(nil), reg.lights...)
}

func RemoteTargetBindings(reg *Registry) []entity.Binding {
	return append([]entity.Binding(nil), reg.remoteTargetBindings...)
}

func copyModbus(modbus entity.Modbus) entity.Modbus {
	return entity.Modbus{
		Mode:              modbus.Mode,
		EventSignals:      append([]entity.ModbusEventSignal(nil), modbus.EventSignals...),
		StatePoints:       append([]entity.ModbusStatePoint(nil), modbus.StatePoints...),
		EventSignalWrites: append([]entity.ModbusEventSignalWrite(nil), modbus.EventSignalWrites...),
		StatePolls:        append([]entity.ModbusStatePoll(nil), modbus.StatePolls...),
	}
}

func SysfsDeviceIDs(index *Registry) []entity.SysfsDeviceID {
	deviceIDs := make([]entity.SysfsDeviceID, 0, len(index.DigitalInputsByDevice)+len(index.RelaysByDevice))
	for deviceID := range index.DigitalInputsByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}
	for deviceID := range index.RelaysByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}

	slices.Sort(deviceIDs)

	return deviceIDs
}

func DigitalInputBySysfsDevice(index *Registry, deviceID entity.SysfsDeviceID) (entity.DigitalInput, bool) {
	input, ok := index.DigitalInputsByDevice[deviceID]
	return input, ok
}

func PushButtonsByInput(index *Registry, inputID entity.DigitalInputID) []entity.PushButton {
	return index.PushButtonsByInputID[inputID]
}

func BindingsByButton(index *Registry, buttonID entity.PushButtonID) []entity.Binding {
	return BindingsBySource(index, entity.ID(buttonID))
}

func BindingsBySource(index *Registry, sourceID entity.ID) []entity.Binding {
	return index.BindingsBySourceID[sourceID]
}

func RemoteSourceBindingsByButton(index *Registry, buttonID entity.PushButtonID) []entity.Binding {
	return RemoteSourceBindingsBySource(index, entity.ID(buttonID))
}

func RemoteSourceBindingsBySource(index *Registry, sourceID entity.ID) []entity.Binding {
	return index.RemoteSourceBindingsBySourceID[sourceID]
}

func RemoteTargetBindingsByButton(index *Registry, buttonID entity.PushButtonID) []entity.Binding {
	return RemoteTargetBindingsBySource(index, entity.ID(buttonID))
}

func RemoteTargetBindingsBySource(index *Registry, sourceID entity.ID) []entity.Binding {
	return index.RemoteTargetBindingsBySourceID[sourceID]
}

func LightByID(index *Registry, lightID entity.LightID) (entity.Light, bool) {
	light, ok := index.LightsByID[lightID]
	return light, ok
}

func LightsByRelay(index *Registry, relayID entity.RelayID) []entity.Light {
	return index.LightsByRelayID[relayID]
}

func RelayByID(index *Registry, relayID entity.RelayID) (entity.Relay, bool) {
	relay, ok := index.RelaysByID[relayID]
	return relay, ok
}

func RelayBySysfsDevice(index *Registry, deviceID entity.SysfsDeviceID) (entity.Relay, bool) {
	relay, ok := index.RelaysByDevice[deviceID]
	return relay, ok
}
