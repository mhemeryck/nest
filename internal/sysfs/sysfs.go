package sysfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var (
	ErrEmptyValue   = errors.New("empty value")
	ErrInvalidValue = errors.New("invalid value")
)

func ListDir(path string) ([]FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", path, err)
	}

	var result []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, FileInfo{
			Path:  filepath.Join(path, entry.Name()),
			IsDir: entry.IsDir(),
			Mode:  info.Mode(),
		})
	}
	return result, nil
}

type FileInfo struct {
	Path  string
	IsDir bool
	Mode  fs.FileMode
}

func ReadFileBytes(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	return data, nil
}

func WriteValue(path string, value Value) error {
	if value != Off && value != On {
		return fmt.Errorf("invalid value %q", value)
	}

	return os.WriteFile(path, []byte{byte(value), '\n'}, 0o644)
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
