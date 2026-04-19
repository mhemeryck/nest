package sysfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

var (
	errEmptyValue   = errors.New("empty value")
	errInvalidValue = errors.New("invalid value")
)

type Value byte

const (
	Off Value = '0'
	On  Value = '1'
)

type DeviceType int

const (
	DigitalInput DeviceType = iota
	DigitalOutput
	RelayOutput
)

type Device struct {
	Type       DeviceType
	Path       string
	Identifier string
	Value      Value
}

type DevicePattern struct {
	Type  DeviceType
	Regex *regexp.Regexp
}

var devicePatterns = []DevicePattern{
	{Type: DigitalInput, Regex: regexp.MustCompile(`/di_\d+_\d+/di_value$`)},
	{Type: DigitalOutput, Regex: regexp.MustCompile(`/do_\d+_\d+/do_value$`)},
	{Type: RelayOutput, Regex: regexp.MustCompile(`/ro_\d+_\d+/ro_value$`)},
}

func ListDevices(root string) ([]*Device, error) {
	var devices []*Device
	err := filepath.WalkDir(filepath.Join(root, "sys", "devices", "platform", "unipi_plc"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		device, ok, err := newDevice(path)
		if err != nil {
			return err
		}
		if ok {
			devices = append(devices, device)
		}

		return nil
	})

	return devices, err
}

func newDevice(path string) (*Device, bool, error) {
	for _, pattern := range devicePatterns {
		if pattern.Regex.MatchString(path) {
			value, err := readValue(path)
			if err != nil {
				return nil, false, err
			}

			return &Device{
				Path:       path,
				Type:       pattern.Type,
				Identifier: filepath.Base(filepath.Dir(path)),
				Value:      value,
			}, true, nil
		}
	}

	return nil, false, nil
}

func readFileBytes(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	return data, nil
}

func writeValue(path string, value Value) error {
	if value != Off && value != On {
		return fmt.Errorf("invalid value %q", value)
	}

	return os.WriteFile(path, []byte{byte(value), '\n'}, 0o644)
}

func readDevice(device *Device) (Value, error) {
	value, err := readValue(device.Path)
	if err != nil {
		return Off, err
	}
	device.Value = value

	return value, nil
}

func readValue(path string) (Value, error) {
	data, err := readFileBytes(path)
	if err != nil {
		return Off, err
	}
	if len(data) == 0 {
		return Off, errEmptyValue
	}

	switch data[0] {
	case '0':
		return Off, nil
	case '1':
		return On, nil
	default:
		return Off, errInvalidValue
	}
}
