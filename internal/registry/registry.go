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
	digitalInputsByDevice          map[entity.SysfsDeviceID]entity.DigitalInput
	pushButtonsByID                map[entity.PushButtonID]entity.PushButton
	pushButtonsByInputID           map[entity.DigitalInputID][]entity.PushButton
	lightsByID                     map[entity.LightID]entity.Light
	lightsByRelayID                map[entity.RelayID][]entity.Light
	bindingsBySourceID             map[entity.ID][]entity.Binding
	remoteSourceBindingsBySourceID map[entity.ID][]entity.Binding
	remoteTargetBindingsBySourceID map[entity.ID][]entity.Binding
	relaysByID                     map[entity.RelayID]entity.Relay
	relaysByDevice                 map[entity.SysfsDeviceID]entity.Relay
}

func Build(root *entity.Root) *Registry {
	index := &Registry{
		sysfsRoot:                      root.SysfsRoot,
		sysfsPollIntervals:             root.SysfsPollIntervals,
		mqtt:                           root.MQTT,
		modbus:                         copyModbus(root.Modbus),
		lights:                         slices.Clone(root.Lights),
		remoteTargetBindings:           slices.Clone(root.RemoteTargetBindings),
		digitalInputsByDevice:          make(map[entity.SysfsDeviceID]entity.DigitalInput, len(root.DigitalInputs)),
		pushButtonsByID:                make(map[entity.PushButtonID]entity.PushButton, len(root.PushButtons)),
		pushButtonsByInputID:           make(map[entity.DigitalInputID][]entity.PushButton),
		lightsByID:                     make(map[entity.LightID]entity.Light, len(root.Lights)),
		lightsByRelayID:                make(map[entity.RelayID][]entity.Light),
		bindingsBySourceID:             make(map[entity.ID][]entity.Binding),
		remoteSourceBindingsBySourceID: make(map[entity.ID][]entity.Binding),
		remoteTargetBindingsBySourceID: make(map[entity.ID][]entity.Binding),
		relaysByID:                     make(map[entity.RelayID]entity.Relay, len(root.Relays)),
		relaysByDevice:                 make(map[entity.SysfsDeviceID]entity.Relay, len(root.Relays)),
	}

	for _, input := range root.DigitalInputs {
		index.digitalInputsByDevice[input.SysfsDevice] = input
	}

	for _, button := range root.PushButtons {
		index.pushButtonsByID[button.ID] = button
		index.pushButtonsByInputID[button.Input] = append(index.pushButtonsByInputID[button.Input], button)
	}

	for _, light := range root.Lights {
		index.lightsByID[light.ID] = light
		index.lightsByRelayID[light.Relay] = append(index.lightsByRelayID[light.Relay], light)
	}

	for _, relay := range root.Relays {
		index.relaysByID[relay.ID] = relay
		index.relaysByDevice[relay.SysfsDevice] = relay
	}

	for _, binding := range root.Bindings {
		index.bindingsBySourceID[binding.Source] = append(index.bindingsBySourceID[binding.Source], binding)
	}

	for _, binding := range root.RemoteSourceBindings {
		index.remoteSourceBindingsBySourceID[binding.Source] = append(index.remoteSourceBindingsBySourceID[binding.Source], binding)
	}

	for _, binding := range root.RemoteTargetBindings {
		index.remoteTargetBindingsBySourceID[binding.Source] = append(index.remoteTargetBindingsBySourceID[binding.Source], binding)
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
	return slices.Clone(reg.lights)
}

func RemoteTargetBindings(reg *Registry) []entity.Binding {
	return slices.Clone(reg.remoteTargetBindings)
}

func copyModbus(modbus entity.Modbus) entity.Modbus {
	return entity.Modbus{
		Mode:              modbus.Mode,
		EventSignals:      slices.Clone(modbus.EventSignals),
		StatePoints:       slices.Clone(modbus.StatePoints),
		EventSignalWrites: slices.Clone(modbus.EventSignalWrites),
		StatePolls:        slices.Clone(modbus.StatePolls),
	}
}

func SysfsDeviceIDs(index *Registry) []entity.SysfsDeviceID {
	deviceIDs := make([]entity.SysfsDeviceID, 0, len(index.digitalInputsByDevice)+len(index.relaysByDevice))
	for deviceID := range index.digitalInputsByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}
	for deviceID := range index.relaysByDevice {
		deviceIDs = append(deviceIDs, deviceID)
	}

	slices.Sort(deviceIDs)

	return deviceIDs
}

func DigitalInputBySysfsDevice(index *Registry, deviceID entity.SysfsDeviceID) (entity.DigitalInput, bool) {
	input, ok := index.digitalInputsByDevice[deviceID]
	return input, ok
}

func PushButtonsByInput(index *Registry, inputID entity.DigitalInputID) []entity.PushButton {
	return slices.Clone(index.pushButtonsByInputID[inputID])
}

func BindingsByButton(index *Registry, buttonID entity.PushButtonID) []entity.Binding {
	return BindingsBySource(index, entity.ID(buttonID))
}

func BindingsBySource(index *Registry, sourceID entity.ID) []entity.Binding {
	return slices.Clone(index.bindingsBySourceID[sourceID])
}

func RemoteSourceBindingsByButton(index *Registry, buttonID entity.PushButtonID) []entity.Binding {
	return RemoteSourceBindingsBySource(index, entity.ID(buttonID))
}

func RemoteSourceBindingsBySource(index *Registry, sourceID entity.ID) []entity.Binding {
	return slices.Clone(index.remoteSourceBindingsBySourceID[sourceID])
}

func RemoteTargetBindingsByButton(index *Registry, buttonID entity.PushButtonID) []entity.Binding {
	return RemoteTargetBindingsBySource(index, entity.ID(buttonID))
}

func RemoteTargetBindingsBySource(index *Registry, sourceID entity.ID) []entity.Binding {
	return slices.Clone(index.remoteTargetBindingsBySourceID[sourceID])
}

func LightByID(index *Registry, lightID entity.LightID) (entity.Light, bool) {
	light, ok := index.lightsByID[lightID]
	return light, ok
}

func LightsByRelay(index *Registry, relayID entity.RelayID) []entity.Light {
	return slices.Clone(index.lightsByRelayID[relayID])
}

func RelayByID(index *Registry, relayID entity.RelayID) (entity.Relay, bool) {
	relay, ok := index.relaysByID[relayID]
	return relay, ok
}

func RelayBySysfsDevice(index *Registry, deviceID entity.SysfsDeviceID) (entity.Relay, bool) {
	relay, ok := index.relaysByDevice[deviceID]
	return relay, ok
}
