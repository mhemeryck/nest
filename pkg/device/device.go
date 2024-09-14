package device

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"
)

const (
	pollIntervalMillis = 250
)

var (
	filenameRegex = regexp.MustCompile(`/io_group(1|2|3)/(?P<device_fmt>di|do|ro)_(?P<io_group>1|2|3)_(?P<number>[0-9]{2})/(di|do|ro)_value$`)

	DeviceFormat_DigitalInput  = DeviceFormat("DigitalInput")
	DeviceFormat_DigitalOutput = DeviceFormat("DigitalOutput")
	DeviceFormat_RelayOutput   = DeviceFormat("RelayOutput")
)

type DevicePayload bool
type DeviceFormat string
type IOGroup int
type DeviceNumber int

type Device struct {
	Path   string
	Format DeviceFormat
	Group  IOGroup
	Number DeviceNumber

	ReadEvents chan<- DevicePayload

	filehandle *os.File
	prev       DevicePayload
	lock       sync.RWMutex
}

func NewDeviceFromPath(path string) (*Device, error) {
	match := filenameRegex.FindStringSubmatch(path)
	if len(match) == 0 {
		return &Device{}, fmt.Errorf("No device matched path")
	}

	d := &Device{Path: path}

	for k, name := range filenameRegex.SubexpNames() {
		if k != 0 && name != "" {
			switch name {
			case "device_fmt":
				switch match[k] {
				case "di":
					d.Format = DeviceFormat_DigitalInput

				case "do":
					d.Format = DeviceFormat_DigitalOutput

				case "ro":
					d.Format = DeviceFormat_RelayOutput
				}
			case "io_group":
				i, err := strconv.Atoi(match[k])
				if err != nil {
					return d, err
				}
				d.Group = IOGroup(i)
			case "number":
				i, err := strconv.Atoi(match[k])
				if err != nil {
					return d, err
				}
				d.Number = DeviceNumber(i)
			}
		}
	}

	return d, nil
}

func (d *Device) Read() (DevicePayload, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	var err error

	if d.filehandle == nil {
		d.filehandle, err = os.Open(d.Path)
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
	if !slices.Contains([]DeviceFormat{DeviceFormat_DigitalOutput, DeviceFormat_RelayOutput}, d.Format) {
		return fmt.Errorf("Cannot write to device %v: wrong format %v", d, d.Format)
	}

	log.Printf("Writing payload %v for device %v\n", payload, d)
	d.lock.Lock()
	defer d.lock.Unlock()

	var err error

	f, err := os.Create(d.Path)
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
	log.Printf("Start device loop for %v", d)

	for {
		value, err := d.Read()
		if err != nil {
			log.Fatalf("Error reading file: %v", err)
		}
		// log.Printf("Current value: %v for %v", value, d.Number)
		if d.prev != value {
			d.ReadEvents <- value
			d.prev = value
		}
		time.Sleep(pollIntervalMillis * time.Millisecond)
	}
}
