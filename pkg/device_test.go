package device

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DeviceReadWrite(t *testing.T) {
	temp_dir := t.TempDir()

	reader := make(chan DevicePayload)
	writer := make(chan DevicePayload)

	device := &Device{
		Filename:    path.Join(temp_dir, "bar"),
		ReadEvents:  reader,
		WriteEvents: writer,
	}

	go device.Loop()

	writer <- DevicePayload(true)
	msg := <-reader
	assert.Equal(t, msg, DevicePayload(true))

	writer <- DevicePayload(false)
	msg = <-reader
	assert.Equal(t, msg, DevicePayload(false))
}
