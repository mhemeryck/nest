package sysfs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

func WriteValue(path string, value int) error {
	if value != 0 && value != 1 {
		return fmt.Errorf("invalid value %d", value)
	}

	return os.WriteFile(path, []byte{byte('0' + value), '\n'}, 0o644)
}

func ListDevices(sysfsPath string) ([]string, error) {
	var devices []string
	err := filepath.WalkDir(sysfsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Name() == "uevent" {
			devices = append(devices, filepath.Dir(path))
		}
		return nil
	})
	return devices, err
}

func ListIOValueFiles(root string) ([]string, error) {
	var paths []string
	patterns := []string{"di_*/di_value", "do_*/do_value", "ro_*/ro_value"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, "sys/devices/platform/unipi_plc/*/", pattern))
		if err != nil {
			return nil, err
		}
		paths = append(paths, matches...)
	}
	return paths, nil
}
