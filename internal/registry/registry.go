package registry

import (
	"slices"

	"github.com/mhemeryck/nest/internal/entity"
)

type Registry struct {
	sysfsRoot                         string
	sysfsPollIntervals                entity.PollIntervals
	mqtt                              entity.MQTT
	modbus                            entity.Modbus
	lights                            []entity.Light
	covers                            []entity.Cover
	coverControl                      entity.CoverControl
	persistencePath                   string
	coversByID                        map[entity.CoverID]entity.Cover
	coversByRelayID                   map[entity.RelayID]entity.Cover
	remoteTargetBindings              []entity.Binding
	digitalInputsByDevice             map[entity.SysfsDeviceID]entity.DigitalInput
	pushButtonsByID                   map[entity.PushButtonID]entity.PushButton
	pushButtonsByInputID              map[entity.DigitalInputID][]entity.PushButton
	lightsByID                        map[entity.LightID]entity.Light
	lightsByRelayID                   map[entity.RelayID][]entity.Light
	bindingsBySourceID                map[entity.ID][]entity.Binding
	remoteSourceBindingsBySourceID    map[entity.ID][]entity.Binding
	remoteTargetBindingsBySourceID    map[entity.ID][]entity.Binding
	modbusEventSignalsByCoil          map[int]entity.ModbusEventSignal
	modbusEventSignalWritesBySourceID map[entity.ID][]entity.ModbusEventSignalWrite
	modbusStatePointsByEntityID       map[entity.ID][]entity.ModbusStatePoint
	modbusStatePollsByCoil            map[modbusCoilKey]entity.ModbusStatePoll
	relaysByID                        map[entity.RelayID]entity.Relay
	relaysByDevice                    map[entity.SysfsDeviceID]entity.Relay
}

type modbusCoilKey struct {
	unitID uint8
	coil   uint16
}

func Build(root *entity.Root) *Registry {
	index := newRegistry(root)
	indexDigitalInputs(index, root.DigitalInputs)
	indexPushButtons(index, root.PushButtons)
	indexLights(index, root.Lights)
	indexCovers(index, root.Covers)
	indexRelays(index, root.Relays)
	indexBindings(index.bindingsBySourceID, root.Bindings)
	indexBindings(index.remoteSourceBindingsBySourceID, root.RemoteSourceBindings)
	indexBindings(index.remoteTargetBindingsBySourceID, root.RemoteTargetBindings)
	indexModbusRoutes(index, root.Modbus)
	return index
}

func newRegistry(root *entity.Root) *Registry {
	return &Registry{
		sysfsRoot:                         root.SysfsRoot,
		sysfsPollIntervals:                root.SysfsPollIntervals,
		mqtt:                              root.MQTT,
		modbus:                            copyModbus(root.Modbus),
		lights:                            slices.Clone(root.Lights),
		covers:                            slices.Clone(root.Covers),
		coverControl:                      root.CoverControl,
		persistencePath:                   root.PersistencePath,
		coversByID:                        make(map[entity.CoverID]entity.Cover, len(root.Covers)),
		coversByRelayID:                   make(map[entity.RelayID]entity.Cover, 2*len(root.Covers)),
		remoteTargetBindings:              slices.Clone(root.RemoteTargetBindings),
		digitalInputsByDevice:             make(map[entity.SysfsDeviceID]entity.DigitalInput, len(root.DigitalInputs)),
		pushButtonsByID:                   make(map[entity.PushButtonID]entity.PushButton, len(root.PushButtons)),
		pushButtonsByInputID:              make(map[entity.DigitalInputID][]entity.PushButton),
		lightsByID:                        make(map[entity.LightID]entity.Light, len(root.Lights)),
		lightsByRelayID:                   make(map[entity.RelayID][]entity.Light),
		bindingsBySourceID:                make(map[entity.ID][]entity.Binding),
		remoteSourceBindingsBySourceID:    make(map[entity.ID][]entity.Binding),
		remoteTargetBindingsBySourceID:    make(map[entity.ID][]entity.Binding),
		modbusEventSignalsByCoil:          make(map[int]entity.ModbusEventSignal, len(root.Modbus.EventSignals)),
		modbusEventSignalWritesBySourceID: make(map[entity.ID][]entity.ModbusEventSignalWrite),
		modbusStatePointsByEntityID:       make(map[entity.ID][]entity.ModbusStatePoint),
		modbusStatePollsByCoil:            make(map[modbusCoilKey]entity.ModbusStatePoll, len(root.Modbus.StatePolls)),
		relaysByID:                        make(map[entity.RelayID]entity.Relay, len(root.Relays)),
		relaysByDevice:                    make(map[entity.SysfsDeviceID]entity.Relay, len(root.Relays)),
	}
}

func indexDigitalInputs(index *Registry, inputs []entity.DigitalInput) {
	for _, input := range inputs {
		index.digitalInputsByDevice[input.SysfsDevice] = input
	}
}

func indexPushButtons(index *Registry, buttons []entity.PushButton) {
	for _, button := range buttons {
		index.pushButtonsByID[button.ID] = button
		index.pushButtonsByInputID[button.Input] = append(index.pushButtonsByInputID[button.Input], button)
	}
}

func indexLights(index *Registry, lights []entity.Light) {
	for _, light := range lights {
		index.lightsByID[light.ID] = light
		index.lightsByRelayID[light.Relay] = append(index.lightsByRelayID[light.Relay], light)
	}
}

func indexCovers(index *Registry, covers []entity.Cover) {
	for _, cover := range covers {
		index.coversByID[cover.ID] = cover
		index.coversByRelayID[cover.OpenRelay] = cover
		index.coversByRelayID[cover.CloseRelay] = cover
	}
}

func indexRelays(index *Registry, relays []entity.Relay) {
	for _, relay := range relays {
		index.relaysByID[relay.ID] = relay
		index.relaysByDevice[relay.SysfsDevice] = relay
	}
}

func indexBindings(bySource map[entity.ID][]entity.Binding, bindings []entity.Binding) {
	for _, binding := range bindings {
		bySource[binding.Source] = append(bySource[binding.Source], binding)
	}
}

func indexModbusRoutes(index *Registry, modbus entity.Modbus) {
	indexModbusEventSignals(index, modbus.EventSignals)
	indexModbusEventSignalWrites(index, modbus.EventSignalWrites)
	indexModbusStatePoints(index, modbus.StatePoints)
	indexModbusStatePolls(index, modbus.StatePolls)
}

func indexModbusEventSignals(index *Registry, signals []entity.ModbusEventSignal) {
	for _, signal := range signals {
		index.modbusEventSignalsByCoil[signal.Coil] = signal
	}
}

func indexModbusEventSignalWrites(index *Registry, writes []entity.ModbusEventSignalWrite) {
	for _, write := range writes {
		index.modbusEventSignalWritesBySourceID[write.Source] = append(index.modbusEventSignalWritesBySourceID[write.Source], write)
	}
}

func indexModbusStatePoints(index *Registry, points []entity.ModbusStatePoint) {
	for _, point := range points {
		index.modbusStatePointsByEntityID[point.Entity] = append(index.modbusStatePointsByEntityID[point.Entity], point)
	}
}

func indexModbusStatePolls(index *Registry, polls []entity.ModbusStatePoll) {
	for _, poll := range polls {
		index.modbusStatePollsByCoil[modbusCoilKey{unitID: poll.UnitID, coil: poll.Coil}] = poll
	}
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

func Covers(reg *Registry) []entity.Cover {
	return slices.Clone(reg.covers)
}

func CoverControl(reg *Registry) entity.CoverControl {
	return reg.coverControl
}

func PersistencePath(reg *Registry) string {
	return reg.persistencePath
}

func CoverByID(reg *Registry, coverID entity.CoverID) (entity.Cover, bool) {
	cover, ok := reg.coversByID[coverID]
	return cover, ok
}

func CoverByRelay(reg *Registry, relayID entity.RelayID) (entity.Cover, bool) {
	cover, ok := reg.coversByRelayID[relayID]
	return cover, ok
}

func RemoteTargetBindings(reg *Registry) []entity.Binding {
	return slices.Clone(reg.remoteTargetBindings)
}

func copyModbus(modbus entity.Modbus) entity.Modbus {
	return entity.Modbus{
		Mode:              modbus.Mode,
		Port:              modbus.Port,
		BaudRate:          modbus.BaudRate,
		Timeout:           modbus.Timeout,
		PollInterval:      modbus.PollInterval,
		UnitID:            modbus.UnitID,
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

func ModbusEventSignalByCoil(index *Registry, coil uint16) (entity.ModbusEventSignal, bool) {
	signal, ok := index.modbusEventSignalsByCoil[int(coil)]
	return signal, ok
}

func ModbusEventSignalWritesBySource(index *Registry, sourceID entity.ID) []entity.ModbusEventSignalWrite {
	return slices.Clone(index.modbusEventSignalWritesBySourceID[sourceID])
}

func ModbusStatePointsByEntity(index *Registry, entityID entity.ID) []entity.ModbusStatePoint {
	return slices.Clone(index.modbusStatePointsByEntityID[entityID])
}

func ModbusStatePollByCoil(index *Registry, unitID uint8, coil uint16) (entity.ModbusStatePoll, bool) {
	poll, ok := index.modbusStatePollsByCoil[modbusCoilKey{unitID: unitID, coil: coil}]
	return poll, ok
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
