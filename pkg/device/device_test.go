package device

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DeviceReadWrite(t *testing.T) {
	temp_dir := t.TempDir()

	reader := make(chan DevicePayload)

	device := &Device{
		Path:       path.Join(temp_dir, "bar"),
		ReadEvents: reader,
		DeviceIdentifier: DeviceIdentifier{
			Format: DeviceFormat_DigitalOutput,
		},
	}

	go device.Loop()

	device.Write(true)
	msg := <-reader
	assert.Equal(t, msg.Message, MessageType_TurnOn)

	device.Write(false)
	msg = <-reader
	assert.Equal(t, msg.Message, MessageType_TurnOff)
}

func Test_DeviceReadWriteDigitalInput(t *testing.T) {
	temp_dir := t.TempDir()

	reader := make(chan DevicePayload)

	device := &Device{
		Path:       path.Join(temp_dir, "bar"),
		ReadEvents: reader,
		DeviceIdentifier: DeviceIdentifier{
			Format: DeviceFormat_DigitalInput,
		},
	}

	err := device.Write(true)
	assert.Error(t, err)
}

func Test_NewDeviceFromPath(t *testing.T) {
	tests := map[string]struct {
		path            string
		expectedFormat  DeviceFormat
		expectedIOGroup IOGroup
		expectedNumber  DeviceNumber
		expectError     bool
	}{
		"No match": {
			path:        "",
			expectError: true,
		},
		"Malformed number": {
			path:        "sys/devices/platform/unipi_plc/io_group3/ro_3_aa/ro_value",
			expectError: true,
		},
		"Regular match": {
			path:            "sys/devices/platform/unipi_plc/io_group3/ro_3_13/ro_value",
			expectedFormat:  DeviceFormat_RelayOutput,
			expectedIOGroup: 3,
			expectedNumber:  13,
			expectError:     false,
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			device, err := NewDeviceFromPath(testCase.path)

			if testCase.expectError {
				assert.Error(t, err, "Expected an error, got none")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedFormat, device.Format)
				assert.Equal(t, testCase.expectedIOGroup, device.Group)
				assert.Equal(t, testCase.expectedNumber, device.Number)
				assert.Equal(t, testCase.path, device.Path)
			}

		})
	}
}

func Test_DeviceIdentifierSlug(t *testing.T) {
	id := DeviceIdentifier{
		Format: DeviceFormat_DigitalOutput,
		Group:  3,
		Number: 7,
	}
	assert.Equal(t, "do-3-07", id.Slug())
}
