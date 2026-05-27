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
		inputErr,
		buttonErr,
		relayErr,
		lightErr,
		validateBindings(f.Bindings, knownButtonIDs, knownLightIDs),
	)

	return errs
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
		var buttonErr error
		if err := validateRequiredField(prefix+".button", binding.Button); err != nil {
			buttonErr = err
		} else if _, ok := knownButtonIDs[binding.Button]; !ok {
			buttonErr = fmt.Errorf("%s.button: unknown push button %q", prefix, binding.Button)
		}

		var lightErr error
		if err := validateRequiredField(prefix+".light", binding.Light); err != nil {
			lightErr = err
		} else if _, ok := knownLightIDs[binding.Light]; !ok {
			lightErr = fmt.Errorf("%s.light: unknown light %q", prefix, binding.Light)
		}

		actionErr := validateRequiredField(prefix+".action", binding.Action)
		if actionErr == nil && binding.Action != BindingActionToggle {
			actionErr = fmt.Errorf("%s.action: unsupported action %q", prefix, binding.Action)
		}

		if buttonErr == nil && lightErr == nil && actionErr == nil {
			key := binding.Button + "\x00" + binding.Light + "\x00" + binding.Action
			if _, ok := seen[key]; ok {
				errs = errors.Join(
					errs,
					fmt.Errorf(
						"%s: duplicate binding button %q light %q action %q",
						prefix,
						binding.Button,
						binding.Light,
						binding.Action,
					),
				)
			} else {
				seen[key] = i
			}
		}

		errs = errors.Join(errs, buttonErr, lightErr, actionErr)
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
