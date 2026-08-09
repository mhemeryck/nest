package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
)

var (
	digitalInputPattern = regexp.MustCompile(`^di_\d+_\d+$`)
	relayPattern        = regexp.MustCompile(`^ro_\d+_\d+$`)
	idPattern           = regexp.MustCompile(`^[a-z0-9_]+$`)
)

func Validate(f *Root) error {
	var errs error

	knownInputIDs, inputErr := validateDigitalInputs(f.DigitalInputs)
	knownButtonIDs, buttonErr := validatePushButtons(f.PushButtons, knownInputIDs)
	knownRelayIDs, relayErr := validateRelays(f.Relays)
	knownLightIDs, lightErr := validateLights(f.Lights, knownRelayIDs)

	if len(f.DigitalInputs) == 0 && len(f.Relays) == 0 {
		errs = errors.Join(errs, fmt.Errorf("at least one digital_input or relay is required"))
	}

	errs = errors.Join(
		errs,
		validateSysfs(f.Sysfs),
		validateMQTT(f.MQTT),
		validateModbus(f.Modbus),
		inputErr,
		buttonErr,
		relayErr,
		lightErr,
		validateBindings(f.Bindings, knownButtonIDs, knownLightIDs),
		validateRemoteBindings("remote_source_bindings", f.RemoteSourceBindings),
		validateRemoteBindings("remote_target_bindings", f.RemoteTargetBindings),
	)

	return errs
}

func validateModbus(modbus ModbusConfig) error {
	switch modbus.Mode {
	case "":
		if modbusConfigured(modbus) {
			return fmt.Errorf("modbus.mode: required when Modbus is configured")
		}
		return nil
	case ModbusModeMaster:
		return validateModbusMaster(modbus)
	case ModbusModeSlave:
		return validateModbusSlave(modbus)
	default:
		return fmt.Errorf("modbus.mode: unsupported mode %q", modbus.Mode)
	}
}

func validateModbusMaster(modbus ModbusConfig) error {
	return errors.Join(
		validateModbusSerialConfig(modbus),
		validateModbusDurations(modbus),
		validateModbusSlaveEndpoints(modbus),
		validateMasterUnitID(modbus),
		validateModbusEventSignalWrites(modbus.EventSignalWrites),
		validateModbusStatePolls(modbus.StatePolls),
	)
}

func validateModbusSlave(modbus ModbusConfig) error {
	return errors.Join(
		validateModbusSerialConfig(modbus),
		validateModbusDurations(modbus),
		validateModbusMasterRoutes(modbus),
		validateSlaveUnitID(modbus),
		validateModbusEventSignals(modbus.EventSignals),
		validateModbusStatePoints(modbus.StatePoints),
		validateUniqueModbusCoils(modbus.EventSignals, modbus.StatePoints),
	)
}

func modbusConfigured(modbus ModbusConfig) bool {
	return modbus.Port != "" ||
		modbus.BaudRate != 0 ||
		modbus.Timeout != 0 ||
		modbus.PollInterval != 0 ||
		modbus.UnitID != 0 ||
		len(modbus.EventSignals) != 0 ||
		len(modbus.StatePoints) != 0 ||
		len(modbus.EventSignalWrites) != 0 ||
		len(modbus.StatePolls) != 0
}

func validateModbusSerialConfig(modbus ModbusConfig) error {
	return errors.Join(
		validateRequiredField("modbus.port", modbus.Port),
		validateModbusBaudRate(modbus.BaudRate),
	)
}

func validateModbusBaudRate(baudRate int) error {
	if baudRate <= 0 {
		return fmt.Errorf("modbus.baudrate: must be positive")
	}

	return nil
}

func validateModbusDurations(modbus ModbusConfig) error {
	var errs error

	if modbus.Timeout < 0 {
		errs = errors.Join(errs, fmt.Errorf("modbus.timeout: must not be negative"))
	}
	if modbus.PollInterval < 0 {
		errs = errors.Join(errs, fmt.Errorf("modbus.poll_interval: must not be negative"))
	}
	return errs
}

func validateModbusSlaveEndpoints(modbus ModbusConfig) error {
	var errs error
	if len(modbus.EventSignals) > 0 {
		errs = errors.Join(errs, fmt.Errorf("modbus.event_signals: requires slave mode"))
	}
	if len(modbus.StatePoints) > 0 {
		errs = errors.Join(errs, fmt.Errorf("modbus.state_points: requires slave mode"))
	}
	return errs
}

func validateModbusMasterRoutes(modbus ModbusConfig) error {
	var errs error
	if len(modbus.EventSignalWrites) > 0 {
		errs = errors.Join(errs, fmt.Errorf("modbus.event_signal_writes: requires master mode"))
	}
	if len(modbus.StatePolls) > 0 {
		errs = errors.Join(errs, fmt.Errorf("modbus.state_polls: requires master mode"))
	}
	return errs
}

func validateSlaveUnitID(modbus ModbusConfig) error {
	if modbus.UnitID < 1 || modbus.UnitID > 247 {
		return fmt.Errorf("modbus.unit_id: must be between 1 and 247")
	}

	return nil
}

func validateMasterUnitID(modbus ModbusConfig) error {
	if modbus.UnitID != 0 {
		return fmt.Errorf("modbus.unit_id: requires slave mode")
	}

	return nil
}

func validateModbusEventSignals(signals []ModbusEventSignalConfig) error {
	var errs error
	ids := make([]string, 0, len(signals))
	for i, signal := range signals {
		prefix := fmt.Sprintf("modbus.event_signals[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".id", signal.ID),
			validateModbusCoil(prefix+".coil", signal.Coil),
			validateButtonSemanticID(prefix+".source", signal.Source),
			validateLightSemanticID(prefix+".target", signal.Target),
			validateModbusBindingAction(prefix+".action", signal.Action),
		)
		ids = append(ids, signal.ID)
	}

	return errors.Join(errs, validateUniqueValues("modbus.event_signals", "id", "id", ids))
}

func validateModbusStatePoints(points []ModbusStatePointConfig) error {
	var errs error
	ids := make([]string, 0, len(points))
	for i, point := range points {
		prefix := fmt.Sprintf("modbus.state_points[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".id", point.ID),
			validateModbusCoil(prefix+".coil", point.Coil),
			validateLightSemanticID(prefix+".entity", point.Entity),
		)
		ids = append(ids, point.ID)
	}

	return errors.Join(errs, validateUniqueValues("modbus.state_points", "id", "id", ids))
}

func validateUniqueModbusCoils(signals []ModbusEventSignalConfig, points []ModbusStatePointConfig) error {
	seen := make(map[int]string, len(signals)+len(points))
	var errs error
	register := func(field string, coil int) {
		if coil < 0 || coil > 65535 {
			return
		}
		if previous, exists := seen[coil]; exists {
			errs = errors.Join(errs, fmt.Errorf("%s: duplicates %s at address %d", field, previous, coil))
			return
		}
		seen[coil] = field
	}

	for i, signal := range signals {
		register(fmt.Sprintf("modbus.event_signals[%d].coil", i), signal.Coil)
	}
	for i, point := range points {
		register(fmt.Sprintf("modbus.state_points[%d].coil", i), point.Coil)
	}

	return errs
}

func validateModbusCoil(field string, coil int) error {
	if coil < 0 || coil > 65535 {
		return fmt.Errorf("%s: must be between 0 and 65535", field)
	}

	return nil
}

func validateModbusEventSignalWrites(writes []ModbusEventSignalWriteConfig) error {
	var errs error
	seen := make(map[string]int, len(writes))
	for i, write := range writes {
		prefix := fmt.Sprintf("modbus.event_signal_writes[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".unit", write.Unit),
			validateID(prefix+".signal", write.Signal),
			validateButtonSemanticID(prefix+".source", write.Source),
			validateLightSemanticID(prefix+".target", write.Target),
			validateModbusBindingAction(prefix+".action", write.Action),
		)

		key := write.Unit + "\x00" + write.Signal + "\x00" + write.Source + "\x00" + write.Target + "\x00" + write.Action
		if _, ok := seen[key]; ok {
			errs = errors.Join(errs, fmt.Errorf("%s: duplicate event signal write", prefix))
		} else {
			seen[key] = i
		}
	}

	return errs
}

func validateModbusStatePolls(polls []ModbusStatePollConfig) error {
	var errs error
	seen := make(map[string]int, len(polls))
	for i, poll := range polls {
		prefix := fmt.Sprintf("modbus.state_polls[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".unit", poll.Unit),
			validateID(prefix+".point", poll.Point),
			validateLightSemanticID(prefix+".entity", poll.Entity),
		)

		key := poll.Unit + "\x00" + poll.Point + "\x00" + poll.Entity
		if _, ok := seen[key]; ok {
			errs = errors.Join(errs, fmt.Errorf("%s: duplicate state poll", prefix))
		} else {
			seen[key] = i
		}
	}

	return errs
}

func validateButtonSemanticID(field string, value string) error {
	if err := validateRequiredField(field, value); err != nil {
		return err
	}
	if !entity.IsID(value, entity.TypeButton) {
		return fmt.Errorf("%s: must be a button semantic id %q", field, value)
	}

	return nil
}

func validateLightSemanticID(field string, value string) error {
	if err := validateRequiredField(field, value); err != nil {
		return err
	}
	if !entity.IsID(value, entity.TypeLight) {
		return fmt.Errorf("%s: must be a light semantic id %q", field, value)
	}

	return nil
}

func validateModbusBindingAction(field string, action string) error {
	if err := validateRequiredField(field, action); err != nil {
		return err
	}
	if action != BindingActionToggle {
		return fmt.Errorf("%s: unsupported action %q", field, action)
	}

	return nil
}

func validateGlobalBindings(global *GlobalRoot) error {
	var errs error
	seen := make(map[string]int, len(global.Bindings))
	for i, binding := range global.Bindings {
		prefix := fmt.Sprintf("bindings[%d]", i)
		sourceErr := validateGlobalBindingEndpoint(global, prefix+".source", binding.Source, entity.TypeButton)
		targetErr := validateGlobalBindingEndpoint(global, prefix+".target", binding.Target, entity.TypeLight)
		actionErr := validateModbusBindingAction(prefix+".action", binding.Action)
		transportErr := error(nil)

		if sourceErr == nil && targetErr == nil && actionErr == nil {
			sourceUnit := strings.Split(binding.Source, ".")[0]
			targetUnit := strings.Split(binding.Target, ".")[0]
			if sourceUnit == targetUnit {
				if binding.ExecutionTransport != "" {
					transportErr = fmt.Errorf("%s.execution_transport: only valid for cross-unit bindings", prefix)
				}
			} else {
				transportErr = validateExecutionTransport(prefix+".execution_transport", binding.ExecutionTransport)
			}

			key := binding.Source + "\x00" + binding.Target + "\x00" + binding.Action
			if _, ok := seen[key]; ok {
				errs = errors.Join(
					errs,
					fmt.Errorf(
						"%s: duplicate binding source %q target %q action %q",
						prefix,
						binding.Source,
						binding.Target,
						binding.Action,
					),
				)
			} else {
				seen[key] = i
			}
		}

		errs = errors.Join(errs, sourceErr, targetErr, actionErr, transportErr)
	}

	return errs
}

func validateExecutionTransport(field string, value string) error {
	if value != string(entity.ExecutionTransportMQTT) && value != string(entity.ExecutionTransportModbus) {
		return fmt.Errorf("%s: must be %q or %q", field, entity.ExecutionTransportMQTT, entity.ExecutionTransportModbus)
	}

	return nil
}

func validateGlobalBindingEndpoint(global *GlobalRoot, field string, value string, entityType entity.Type) error {
	if err := validateRequiredField(field, value); err != nil {
		return err
	}
	if !entity.IsID(value, entityType) {
		return fmt.Errorf("%s: must be a %s semantic id %q", field, entityType, value)
	}

	parts := strings.Split(value, ".")
	unit, ok := global.Units[parts[0]]
	if !ok {
		return fmt.Errorf("%s: unknown unit %q", field, parts[0])
	}

	localID := parts[2]
	switch entityType {
	case entity.TypeButton:
		for _, button := range unit.Entities.Buttons {
			if button.ID == localID {
				return nil
			}
		}
	case entity.TypeLight:
		for _, light := range unit.Entities.Lights {
			if light.ID == localID {
				return nil
			}
		}
	}

	return fmt.Errorf("%s: unknown %s %q", field, entityType, value)
}

func validateGlobalModbus(global *GlobalRoot) error {
	var errs error
	var masterUnit string
	modbusConfigured := false
	seenSlaveIDs := make(map[int]string)

	for unitID, unit := range global.Units {
		prefix := fmt.Sprintf("units.%s.actors.modbus", unitID)
		errs = errors.Join(errs, validateModbus(unit.Actors.Modbus))
		if unit.Actors.Modbus.Mode == ModbusModeMaster {
			modbusConfigured = true
			if masterUnit != "" {
				errs = errors.Join(errs, fmt.Errorf("%s.mode: multiple master units, already configured on %q", prefix, masterUnit))
			} else {
				masterUnit = unitID
			}
		}
		if unit.Actors.Modbus.Mode == ModbusModeSlave {
			modbusConfigured = true
			if unit.Actors.Modbus.UnitID != 0 {
				if previous, ok := seenSlaveIDs[unit.Actors.Modbus.UnitID]; ok {
					errs = errors.Join(errs, fmt.Errorf("%s.unit_id: duplicates slave unit %q", prefix, previous))
				} else {
					seenSlaveIDs[unit.Actors.Modbus.UnitID] = unitID
				}
			}
		}

		for i, write := range unit.Actors.Modbus.EventSignalWrites {
			routePrefix := fmt.Sprintf("%s.event_signal_writes[%d]", prefix, i)
			target, ok := global.Units[write.Unit]
			if !ok {
				errs = errors.Join(errs, fmt.Errorf("%s.unit: unknown unit %q", routePrefix, write.Unit))
				continue
			}
			signal, ok := modbusEventSignalByID(target.Actors.Modbus.EventSignals, write.Signal)
			if !ok {
				errs = errors.Join(errs, fmt.Errorf("%s.signal: unknown event signal %q on unit %q", routePrefix, write.Signal, write.Unit))
				continue
			}
			if write.Source != signal.Source {
				errs = errors.Join(errs, fmt.Errorf("%s.source: must match event signal %q source %q", routePrefix, write.Signal, signal.Source))
			}
			if write.Target != signal.Target {
				errs = errors.Join(errs, fmt.Errorf("%s.target: must match event signal %q target %q", routePrefix, write.Signal, signal.Target))
			}
			if write.Action != signal.Action {
				errs = errors.Join(errs, fmt.Errorf("%s.action: must match event signal %q action %q", routePrefix, write.Signal, signal.Action))
			}
		}

		for i, poll := range unit.Actors.Modbus.StatePolls {
			routePrefix := fmt.Sprintf("%s.state_polls[%d]", prefix, i)
			target, ok := global.Units[poll.Unit]
			if !ok {
				errs = errors.Join(errs, fmt.Errorf("%s.unit: unknown unit %q", routePrefix, poll.Unit))
				continue
			}
			point, ok := modbusStatePointByID(target.Actors.Modbus.StatePoints, poll.Point)
			if !ok {
				errs = errors.Join(errs, fmt.Errorf("%s.point: unknown state point %q on unit %q", routePrefix, poll.Point, poll.Unit))
			} else if point.Entity != poll.Entity {
				errs = errors.Join(errs, fmt.Errorf("%s.entity: must match state point %q entity %q", routePrefix, poll.Point, point.Entity))
			}
		}
	}
	if modbusConfigured && masterUnit == "" {
		errs = errors.Join(errs, fmt.Errorf("modbus: exactly one master unit is required when Modbus is configured"))
	}

	return errs
}

func modbusEventSignalByID(signals []ModbusEventSignalConfig, id string) (ModbusEventSignalConfig, bool) {
	for _, signal := range signals {
		if signal.ID == id {
			return signal, true
		}
	}

	return ModbusEventSignalConfig{}, false
}

func modbusStatePointExists(points []ModbusStatePointConfig, id string) bool {
	_, ok := modbusStatePointByID(points, id)
	return ok
}

func modbusStatePointByID(points []ModbusStatePointConfig, id string) (ModbusStatePointConfig, bool) {
	for _, point := range points {
		if point.ID == id {
			return point, true
		}
	}

	return ModbusStatePointConfig{}, false
}

func validateSysfs(sysfs SysfsConfig) error {
	return errors.Join(
		validateRequiredField("sysfs.root", sysfs.Root),
		validatePollInterval("sysfs.poll_intervals.digital_input", sysfs.PollIntervals.DigitalInput),
		validatePollInterval("sysfs.poll_intervals.digital_output", sysfs.PollIntervals.DigitalOutput),
		validatePollInterval("sysfs.poll_intervals.relay_output", sysfs.PollIntervals.RelayOutput),
	)
}

func validatePollInterval(field string, value time.Duration) error {
	if value < 0 {
		return fmt.Errorf("%s: must not be negative", field)
	}

	return nil
}

func validateMQTT(mqtt MQTTConfig) error {
	if !mqtt.Enabled {
		return nil
	}

	var portErr error
	if mqtt.Port < 1 || mqtt.Port > 65535 {
		portErr = fmt.Errorf("mqtt.port: must be between 1 and 65535")
	}

	return errors.Join(
		validateRequiredField("mqtt.host", mqtt.Host),
		portErr,
		validateRequiredField("mqtt.client_id", mqtt.ClientID),
		validateOptionalField("mqtt.username", mqtt.Username),
		validateTopicPrefix("mqtt.topic_prefix", mqtt.TopicPrefix),
		validateID("mqtt.unit_id", mqtt.UnitID),
	)
}

func validateDigitalInputs(inputs []DigitalInputConfig) (map[string]struct{}, error) {
	var errs error

	inputIDs := make([]string, 0, len(inputs))
	inputDevices := make([]string, 0, len(inputs))
	for i, input := range inputs {
		prefix := fmt.Sprintf("digital_inputs[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".id", input.ID),
			validateDevice(prefix+".device", input.Device, digitalInputPattern, "digital input"),
		)
		inputIDs = append(inputIDs, input.ID)
		inputDevices = append(inputDevices, input.Device)
	}

	errs = errors.Join(
		errs,
		validateUniqueValues("digital_inputs", "id", "id", inputIDs),
		validateUniqueValues("digital_inputs", "device", "digital input device", inputDevices),
	)

	knownInputIDs := make(map[string]struct{}, len(inputIDs))
	for _, inputID := range inputIDs {
		knownInputIDs[inputID] = struct{}{}
	}

	return knownInputIDs, errs
}

func validatePushButtons(buttons []PushButtonConfig, knownInputIDs map[string]struct{}) (map[string]struct{}, error) {
	var errs error

	buttonIDs := make([]string, 0, len(buttons))
	for i, button := range buttons {
		prefix := fmt.Sprintf("push_buttons[%d]", i)
		var inputErr error
		if err := validateRequiredField(prefix+".input", button.Input); err != nil {
			inputErr = err
		} else if _, ok := knownInputIDs[button.Input]; !ok {
			inputErr = fmt.Errorf("%s.input: unknown digital input %q", prefix, button.Input)
		}

		errs = errors.Join(
			errs,
			validateEntityID(prefix+".id", button.ID, entity.TypeButton),
			validateRequiredField(prefix+".name", button.Name),
			inputErr,
		)
		buttonIDs = append(buttonIDs, button.ID)
	}

	err := errors.Join(errs, validateUniqueValues("push_buttons", "id", "id", buttonIDs))
	knownButtonIDs := make(map[string]struct{}, len(buttonIDs))
	for _, buttonID := range buttonIDs {
		knownButtonIDs[buttonID] = struct{}{}
	}

	return knownButtonIDs, err
}

func validateRelays(relays []RelayConfig) (map[string]struct{}, error) {
	var errs error

	relayIDs := make([]string, 0, len(relays))
	relayDevices := make([]string, 0, len(relays))
	for i, relay := range relays {
		prefix := fmt.Sprintf("relays[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".id", relay.ID),
			validateDevice(prefix+".device", relay.Device, relayPattern, "relay"),
			validateRequiredField(prefix+".name", relay.Name),
		)
		relayIDs = append(relayIDs, relay.ID)
		relayDevices = append(relayDevices, relay.Device)
	}

	err := errors.Join(
		errs,
		validateUniqueValues("relays", "id", "id", relayIDs),
		validateUniqueValues("relays", "device", "relay device", relayDevices),
	)
	knownRelayIDs := make(map[string]struct{}, len(relayIDs))
	for _, relayID := range relayIDs {
		knownRelayIDs[relayID] = struct{}{}
	}

	return knownRelayIDs, err
}

func validateLights(lights []LightConfig, knownRelayIDs map[string]struct{}) (map[string]struct{}, error) {
	var errs error

	lightIDs := make([]string, 0, len(lights))
	for i, light := range lights {
		prefix := fmt.Sprintf("lights[%d]", i)
		var relayErr error
		if err := validateRequiredField(prefix+".relay", light.Relay); err != nil {
			relayErr = err
		} else if _, ok := knownRelayIDs[light.Relay]; !ok {
			relayErr = fmt.Errorf("%s.relay: unknown relay %q", prefix, light.Relay)
		}

		errs = errors.Join(
			errs,
			validateEntityID(prefix+".id", light.ID, entity.TypeLight),
			validateRequiredField(prefix+".name", light.Name),
			relayErr,
		)
		lightIDs = append(lightIDs, light.ID)
	}

	err := errors.Join(errs, validateUniqueValues("lights", "id", "id", lightIDs))
	knownLightIDs := make(map[string]struct{}, len(lightIDs))
	for _, lightID := range lightIDs {
		knownLightIDs[lightID] = struct{}{}
	}

	return knownLightIDs, err
}

func validateBindings(bindings []BindingConfig, knownButtonIDs map[string]struct{}, knownLightIDs map[string]struct{}) error {
	var errs error
	seen := make(map[string]int, len(bindings))

	for i, binding := range bindings {
		prefix := fmt.Sprintf("bindings[%d]", i)
		var sourceErr error
		if err := validateRequiredField(prefix+".source", binding.Source); err != nil {
			sourceErr = err
		} else if _, ok := knownButtonIDs[binding.Source]; !ok {
			sourceErr = fmt.Errorf("%s.source: unknown push button %q", prefix, binding.Source)
		}

		var targetErr error
		if err := validateRequiredField(prefix+".target", binding.Target); err != nil {
			targetErr = err
		} else if _, ok := knownLightIDs[binding.Target]; !ok {
			targetErr = fmt.Errorf("%s.target: unknown light %q", prefix, binding.Target)
		}

		actionErr := validateRequiredField(prefix+".action", binding.Action)
		if actionErr == nil && binding.Action != BindingActionToggle {
			actionErr = fmt.Errorf("%s.action: unsupported action %q", prefix, binding.Action)
		}

		if sourceErr == nil && targetErr == nil && actionErr == nil {
			key := binding.Source + "\x00" + binding.Target + "\x00" + binding.Action
			if _, ok := seen[key]; ok {
				errs = errors.Join(
					errs,
					fmt.Errorf(
						"%s: duplicate binding source %q target %q action %q",
						prefix,
						binding.Source,
						binding.Target,
						binding.Action,
					),
				)
			} else {
				seen[key] = i
			}
		}

		errs = errors.Join(errs, sourceErr, targetErr, actionErr)
	}

	return errs
}

func validateRemoteBindings(field string, bindings []BindingConfig) error {
	var errs error
	seen := make(map[string]int, len(bindings))

	for i, binding := range bindings {
		prefix := fmt.Sprintf("%s[%d]", field, i)
		var sourceErr error
		if err := validateRequiredField(prefix+".source", binding.Source); err != nil {
			sourceErr = err
		} else if !entity.IsID(binding.Source, entity.TypeButton) {
			sourceErr = fmt.Errorf("%s.source: must be a button semantic id %q", prefix, binding.Source)
		}

		var targetErr error
		if err := validateRequiredField(prefix+".target", binding.Target); err != nil {
			targetErr = err
		} else if !entity.IsID(binding.Target, entity.TypeLight) {
			targetErr = fmt.Errorf("%s.target: must be a light semantic id %q", prefix, binding.Target)
		}

		actionErr := validateRequiredField(prefix+".action", binding.Action)
		if actionErr == nil && binding.Action != BindingActionToggle {
			actionErr = fmt.Errorf("%s.action: unsupported action %q", prefix, binding.Action)
		}

		if sourceErr == nil && targetErr == nil && actionErr == nil {
			key := binding.Source + "\x00" + binding.Target + "\x00" + binding.Action
			if _, ok := seen[key]; ok {
				errs = errors.Join(
					errs,
					fmt.Errorf(
						"%s: duplicate remote binding source %q target %q action %q",
						prefix,
						binding.Source,
						binding.Target,
						binding.Action,
					),
				)
			} else {
				seen[key] = i
			}
		}

		errs = errors.Join(errs, sourceErr, targetErr, actionErr)
	}

	return errs
}

func DeviceIDs(f *Root) []string {
	deviceIDs := make([]string, 0, len(f.DigitalInputs)+len(f.Relays))
	for _, input := range f.DigitalInputs {
		deviceIDs = append(deviceIDs, input.Device)
	}
	for _, relay := range f.Relays {
		deviceIDs = append(deviceIDs, relay.Device)
	}

	return deviceIDs
}

func validateID(field string, value string) error {
	if err := validateRequiredField(field, value); err != nil {
		return err
	}

	return validatePattern(field, value, idPattern, "must contain only lowercase letters, numbers, and underscores")
}

func validateEntityID(field string, value string, entityType entity.Type) error {
	if err := validateRequiredField(field, value); err != nil {
		return err
	}

	if entity.IsLocalID(value) || entity.IsID(value, entityType) {
		return nil
	}

	return fmt.Errorf("%s: must be a local id or %s semantic id %q", field, entityType, value)
}

func validateDevice(field string, value string, pattern *regexp.Regexp, kind string) error {
	if err := validateRequiredField(field, value); err != nil {
		return err
	}

	return validatePattern(field, value, pattern, fmt.Sprintf("invalid %s device", kind))
}

func validateNoOuterWhitespace(field string, value string) error {
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("%s: must not have leading or trailing whitespace", field)
	}

	return nil
}

func validateRequired(field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s: required", field)
	}

	return nil
}

func validateRequiredField(field string, value string) error {
	if err := validateRequired(field, value); err != nil {
		return err
	}

	return validateNoOuterWhitespace(field, value)
}

func validateOptionalField(field string, value string) error {
	if value == "" {
		return nil
	}

	return validateNoOuterWhitespace(field, value)
}

func validateTopicPrefix(field string, value string) error {
	if err := validateRequiredField(field, value); err != nil {
		return err
	}

	if strings.Contains(value, "//") || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return fmt.Errorf("%s: must be a relative MQTT topic prefix without empty segments", field)
	}

	return nil
}

func validatePattern(field string, value string, pattern *regexp.Regexp, label string) error {
	if !pattern.MatchString(value) {
		return fmt.Errorf("%s: %s %q", field, label, value)
	}

	return nil
}

func validateUniqueValues(section string, field string, label string, values []string) error {
	seen := make(map[string]int, len(values))
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}

		if _, ok := seen[value]; ok {
			return fmt.Errorf("%s[%d].%s: duplicate %s %q", section, i, field, label, value)
		}

		seen[value] = i
	}

	return nil
}
