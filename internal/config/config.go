package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	errMissingConfigPath = errors.New("missing config path")
	digitalInputPattern  = regexp.MustCompile(`^di_\d+_\d+$`)
	relayPattern         = regexp.MustCompile(`^ro_\d+_\d+$`)
)

type File struct {
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

func Load(path string) (*File, error) {
	if path == "" {
		return nil, errMissingConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var file File
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

func Validate(f *File) error {
	var errs []string

	if strings.TrimSpace(f.Sysfs.Root) == "" {
		errs = append(errs, "sysfs.root: required")
	}

	inputIDs := make(map[string]struct{}, len(f.DigitalInputs))
	for i, input := range f.DigitalInputs {
		prefix := fmt.Sprintf("digital_inputs[%d]", i)
		validateID(prefix+".id", input.ID, inputIDs, &errs)
		validateDevice(prefix+".device", input.Device, digitalInputPattern, "digital input", &errs)
	}

	buttonIDs := make(map[string]struct{}, len(f.PushButtons))
	for i, button := range f.PushButtons {
		prefix := fmt.Sprintf("push_buttons[%d]", i)
		validateID(prefix+".id", button.ID, buttonIDs, &errs)
		if strings.TrimSpace(button.Name) == "" {
			errs = append(errs, prefix+".name: required")
		}
		if strings.TrimSpace(button.Input) == "" {
			errs = append(errs, prefix+".input: required")
		} else if _, ok := inputIDs[button.Input]; !ok {
			errs = append(errs, fmt.Sprintf("%s.input: unknown digital input %q", prefix, button.Input))
		}
	}

	relayIDs := make(map[string]struct{}, len(f.Relays))
	for i, relay := range f.Relays {
		prefix := fmt.Sprintf("relays[%d]", i)
		validateID(prefix+".id", relay.ID, relayIDs, &errs)
		if strings.TrimSpace(relay.Name) == "" {
			errs = append(errs, prefix+".name: required")
		}
		validateDevice(prefix+".device", relay.Device, relayPattern, "relay", &errs)
	}

	if len(errs) == 0 {
		return nil
	}

	sort.Strings(errs)
	return errors.New(strings.Join(errs, "; "))
}

func DeviceIDs(f *File) []string {
	deviceIDs := make([]string, 0, len(f.DigitalInputs)+len(f.Relays))
	for _, input := range f.DigitalInputs {
		deviceIDs = append(deviceIDs, input.Device)
	}
	for _, relay := range f.Relays {
		deviceIDs = append(deviceIDs, relay.Device)
	}

	return deviceIDs
}

func validateID(field string, value string, seen map[string]struct{}, errs *[]string) {
	value = strings.TrimSpace(value)
	if value == "" {
		*errs = append(*errs, field+": required")
		return
	}

	if _, ok := seen[value]; ok {
		*errs = append(*errs, fmt.Sprintf("%s: duplicate id %q", field, value))
		return
	}

	seen[value] = struct{}{}
}

func validateDevice(field string, value string, pattern *regexp.Regexp, kind string, errs *[]string) {
	value = strings.TrimSpace(value)
	if value == "" {
		*errs = append(*errs, field+": required")
		return
	}

	if !pattern.MatchString(value) {
		*errs = append(*errs, fmt.Sprintf("%s: invalid %s device %q", field, kind, value))
	}
}
