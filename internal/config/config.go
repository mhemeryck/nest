package config

import (
	"bytes"
	"errors"
	"fmt"
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

	if err := Validate(&file); err != nil {
		return nil, err
	}

	return &file, nil
}

func Validate(f *Root) error {
	var errs error

	if strings.TrimSpace(f.Sysfs.Root) == "" {
		errs = errors.Join(errs, fmt.Errorf("sysfs.root: required"))
	}

	inputIDs := make(map[string]struct{}, len(f.DigitalInputs))
	for i, input := range f.DigitalInputs {
		prefix := fmt.Sprintf("digital_inputs[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".id", input.ID, inputIDs),
			validateDevice(prefix+".device", input.Device, digitalInputPattern, "digital input"),
		)
	}

	buttonIDs := make(map[string]struct{}, len(f.PushButtons))
	for i, button := range f.PushButtons {
		prefix := fmt.Sprintf("push_buttons[%d]", i)
		errs = errors.Join(errs, validateID(prefix+".id", button.ID, buttonIDs))
		if strings.TrimSpace(button.Name) == "" {
			errs = errors.Join(errs, fmt.Errorf("%s.name: required", prefix))
		}
		if strings.TrimSpace(button.Input) == "" {
			errs = errors.Join(errs, fmt.Errorf("%s.input: required", prefix))
		} else if _, ok := inputIDs[button.Input]; !ok {
			errs = errors.Join(errs, fmt.Errorf("%s.input: unknown digital input %q", prefix, button.Input))
		}
	}

	relayIDs := make(map[string]struct{}, len(f.Relays))
	for i, relay := range f.Relays {
		prefix := fmt.Sprintf("relays[%d]", i)
		errs = errors.Join(
			errs,
			validateID(prefix+".id", relay.ID, relayIDs),
			validateDevice(prefix+".device", relay.Device, relayPattern, "relay"),
		)
		if strings.TrimSpace(relay.Name) == "" {
			errs = errors.Join(errs, fmt.Errorf("%s.name: required", prefix))
		}
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

func validateID(field string, value string, seen map[string]struct{}) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s: required", field)
	}

	if _, ok := seen[value]; ok {
		return fmt.Errorf("%s: duplicate id %q", field, value)
	}

	seen[value] = struct{}{}
	return nil
}

func validateDevice(field string, value string, pattern *regexp.Regexp, kind string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s: required", field)
	}

	if !pattern.MatchString(value) {
		return fmt.Errorf("%s: invalid %s device %q", field, kind, value)
	}

	return nil
}
