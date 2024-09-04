package device

import (
	"io"
	"log"
	"os"
	"time"
)

const (
	pollIntervalMillis = 250
)

type DevicePayload bool

type Device struct {
	Filename string

	WriteEvents <-chan DevicePayload
	ReadEvents  chan<- DevicePayload

	filehandle *os.File
	prev       DevicePayload
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

func (d *Device) Write(payload DevicePayload) error {
	var err error

	f, err := os.Create(d.Filename)
	defer f.Close()
	if err != nil {
		return err
	}

	b := make([]byte, 1)
	switch payload {
	case true:
		b = []byte("1")
	case false:
		b = []byte("0")
	}

	_, err = f.Write(b)
	return err
}

func (d *Device) Loop() {
	log.Printf("Start device loop")

	ticker := time.NewTicker(pollIntervalMillis * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			value, err := d.Read()
			if err != nil {
				log.Fatalf("Error reading file: %v", err)
			}
			log.Printf("Current value: %v", value)
			if d.prev != value {
				d.ReadEvents <- value
				d.prev = value
			}
		case msg := <-d.WriteEvents:
			log.Printf("Got message to write %v", msg)
			d.Write(msg)
		}
	}
}
