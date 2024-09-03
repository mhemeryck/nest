package main

import (
	"io"
	"log"
	"os"
	"time"
)

const (
	filename           = "./foo"
	pollIntervalMillis = 250
)

var (
	logger *log.Logger
)

type DevicePayload bool

type Device struct {
	Filename string

	WriteEvents <-chan DevicePayload
	ReadEvents  chan<- DevicePayload

	filehandle *os.File
	prev       []byte
}

func (d *Device) Read() (DevicePayload, error) {
	var err error

	if d.filehandle == nil {
		d.filehandle, err = os.Open(d.Filename)
		if err != nil {
			return DevicePayload(false), err
		}
	}

	_, err = d.filehandle.Seek(0, io.SeekStart)
	if err != nil {
		return DevicePayload(false), err
	}

	b := make([]byte, 1)
	_, err = d.filehandle.Read(b)
	if err != nil {
		return DevicePayload(false), err
	}

	switch b[0] {
	case '0':
		return DevicePayload(false), err
	case '1':
		return DevicePayload(true), err
	}
	return DevicePayload(false), err
}

func (d *Device) Loop() {
	logger.Printf("Start device loop")

	ticker := time.NewTicker(pollIntervalMillis * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			value, err := d.Read()
			if err != nil {
				logger.Fatalf("Error reading file: %w", err)
			}
			logger.Printf("Current value: %v", value)
			d.ReadEvents <- value
		}
	}
}

func main() {
	logger = log.New(os.Stdout, "nest: ", log.LstdFlags)

	reader := make(chan DevicePayload)

	device := &Device{
		Filename:   filename,
		ReadEvents: reader,
	}
	go device.Loop()

	for msg := range reader {
		logger.Printf("Reader got value %v", msg)
	}
}
