package sysfs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

func ReadFileValue(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func WriteFileValue(path string, value string) error {
	return os.WriteFile(path, []byte(value+"\n"), 0o644)
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

type DeviceEntry struct {
	Path     string
	Name     string
	Children []DeviceEntry
}

func CrawlDevice(rootPath string) (DeviceEntry, error) {
	name, _ := ReadFileValue(filepath.Join(rootPath, "name"))

	var children []DeviceEntry
	entries, err := os.ReadDir(rootPath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "power" {
				continue
			}
			child, err := CrawlDevice(filepath.Join(rootPath, entry.Name()))
			if err == nil {
				children = append(children, child)
			}
		}
	}

	return DeviceEntry{Path: rootPath, Name: name, Children: children}, nil
}
