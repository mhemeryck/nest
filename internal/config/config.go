package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	errMissingConfigPath = errors.New("missing config path")
	digitalInputPattern  = regexp.MustCompile(`^di_\d+_\d+$`)
	relayPattern         = regexp.MustCompile(`^ro_\d+_\d+$`)
)

type Root struct {
	Sysfs         SysfsConfig          `yaml:"sysfs"`
	DigitalInputs []DigitalInputConfig `yaml:"digital_inputs"`
	PushButtons   []PushButtonConfig   `yaml:"push_buttons"`
	Relays        []RelayConfig        `yaml:"relays"`
}

type SysfsConfig struct {
	Root string `yaml:"root"`
}

type DigitalInputConfig struct {
	ID     string `yaml:"id"`
	Device string `yaml:"device"`
}

type PushButtonConfig struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Input string `yaml:"input"`
}

type RelayConfig struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Device string `yaml:"device"`
}

func Load(path string) (*Root, error) {
	if path == "" {
		return nil, errMissingConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var file Root
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("decode trailing config %s: %w", path, err)
		}

		return nil, fmt.Errorf("decode config %s: multiple YAML documents are not supported", path)
	}

	if err := Validate(&file); err != nil {
		return nil, err
	}

	return &file, nil
}

func Validate(f *Root) error {
	var errs error

	errs = errors.Join(errs, validateRequiredField("sysfs.root", f.Sysfs.Root))

	if len(f.DigitalInputs) == 0 && len(f.Relays) == 0 {
		errs = errors.Join(errs, fmt.Errorf("at least one digital_input or relay is required"))
	}

	inputIDs := make([]string, 0, len(f.DigitalInputs))
	inputDevices := make([]string, 0, len(f.DigitalInputs))
	for i, input := range f.DigitalInputs {
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

	buttonIDs := make([]string, 0, len(f.PushButtons))
	for i, button := range f.PushButtons {
		prefix := fmt.Sprintf("push_buttons[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".id", button.ID),
			validateRequiredField(prefix+".name", button.Name),
		)
		if err := validateRequiredField(prefix+".input", button.Input); err != nil {
			errs = errors.Join(errs, err)
		} else if _, ok := knownInputIDs[button.Input]; !ok {
			errs = errors.Join(errs, fmt.Errorf("%s.input: unknown digital input %q", prefix, button.Input))
		}
		buttonIDs = append(buttonIDs, button.ID)
	}
	errs = errors.Join(errs, validateUniqueValues("push_buttons", "id", "id", buttonIDs))

	relayIDs := make([]string, 0, len(f.Relays))
	relayDevices := make([]string, 0, len(f.Relays))
	for i, relay := range f.Relays {
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
	errs = errors.Join(
		errs,
		validateUniqueValues("relays", "id", "id", relayIDs),
		validateUniqueValues("relays", "device", "relay device", relayDevices),
	)

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
	return nil
}

func validateDevice(field string, value string, pattern *regexp.Regexp, kind string) error {
	if err := validateRequiredField(field, value); err != nil {
		return err
	}

	if !pattern.MatchString(value) {
		return fmt.Errorf("%s: invalid %s device %q", field, kind, value)
	}

	return nil
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
