package sysfs

import (
	"errors"
	"fmt"
	"os"
)

var (
	ErrEmptyValue   = errors.New("empty value")
	ErrInvalidValue = errors.New("invalid value")
)

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
