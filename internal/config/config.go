package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

var errMissingConfigPath = errors.New("missing config path")

func Load(path string, unitID string) (*Root, error) {
	if path == "" {
		return nil, errMissingConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	global, err := decodeGlobalRoot(path, data)
	if err != nil {
		return nil, err
	}

	local, err := ProjectUnit(global, unitID)
	if err != nil {
		return nil, err
	}

	if err := Validate(local); err != nil {
		return nil, err
	}

	return local, nil
}

func decodeGlobalRoot(path string, data []byte) (*GlobalRoot, error) {
	var file GlobalRoot
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

	return &file, nil
}
