package sysfs

import (
	"io/fs"
	"path/filepath"
	"regexp"
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
	Ready      bool
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

		device, ok := NewDevice(path)
		if ok {
			devices = append(devices, device)
		}

		return nil
	})

	return devices, err
}

func NewDevice(path string) (*Device, bool) {
	for _, pattern := range devicePatterns {
		if pattern.Regex.MatchString(path) {
			return &Device{
				Path:       path,
				Type:       pattern.Type,
				Identifier: filepath.Base(filepath.Dir(path)),
			}, true
		}
	}

	return nil, false
}

func ValueInt(value Value) int {
	if value == On {
		return 1
	}

	return 0
}

func ReadDevice(device *Device) (bool, Value, error) {
	value, err := ReadValue(device.Path)
	if err != nil {
		return false, Off, err
	}
	if !device.Ready {
		device.Value = value
		device.Ready = true

		return false, value, nil
	}

	oldValue := device.Value
	changed := oldValue != value
	device.Value = value

	return changed, oldValue, nil
}

func ReadValue(path string) (Value, error) {
	data, err := ReadFileBytes(path)
	if err != nil {
		return Off, err
	}
	if len(data) == 0 {
		return Off, ErrEmptyValue
	}

	switch data[0] {
	case '0':
		return Off, nil
	case '1':
		return On, nil
	default:
		return Off, ErrInvalidValue
	}
}
