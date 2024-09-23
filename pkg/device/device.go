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

	MessageType_Unknown = MessageType("Unknown")
	MessageType_TurnOn  = MessageType("TurnOn")
	MessageType_TurnOff = MessageType("TurnOff")
)

type MessageType string

type DevicePayload struct {
	// Message is just on or off for now
	Message MessageType
	// Id is just a string / slug to identify the device for now
	Id string
}

type DeviceFormat string
type IOGroup int
type DeviceNumber int

// String implements the stringer interface, such that we can easily get an abbreviation for a given device format
func (f DeviceFormat) String() string {
	switch f {
	case DeviceFormat_DigitalInput:
		return "di"
	case DeviceFormat_DigitalOutput:
		return "do"
	case DeviceFormat_RelayOutput:
		return "ro"
	}
	return ""
}

type DeviceIdentifier struct {
	Format DeviceFormat
	Group  IOGroup
	Number DeviceNumber
}

// Slug generates a unique identifier for
func (id DeviceIdentifier) Slug() string {
	return fmt.Sprintf("%s-%d-%02d", id.Format, id.Group, id.Number)
}

type Device struct {
	Path       string
	ReadEvents chan<- DevicePayload

	filehandle *os.File
	prev       DevicePayload
	lock       sync.RWMutex
	loop       bool

	DeviceIdentifier
}

func NewDeviceFromPath(path string) (*Device, error) {
	match := filenameRegex.FindStringSubmatch(path)
	if len(match) == 0 {
		return &Device{}, fmt.Errorf("No device matched path")
	}

	d := &Device{Path: path}
	id := DeviceIdentifier{}

	for k, name := range filenameRegex.SubexpNames() {
		if k != 0 && name != "" {
			switch name {
			case "device_fmt":
				switch match[k] {
				case "di":
					id.Format = DeviceFormat_DigitalInput

				case "do":
					id.Format = DeviceFormat_DigitalOutput

				case "ro":
					id.Format = DeviceFormat_RelayOutput
				}
			case "io_group":
				i, err := strconv.Atoi(match[k])
				if err != nil {
					return d, err
				}
				id.Group = IOGroup(i)
			case "number":
				i, err := strconv.Atoi(match[k])
				if err != nil {
					return d, err
				}
				id.Number = DeviceNumber(i)
			}
		}
	}
	d.DeviceIdentifier = id

	return d, nil
}

func (d *Device) Read() (DevicePayload, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	var err error

	if d.filehandle == nil {
		d.filehandle, err = os.Open(d.Path)
		if err != nil {
			return DevicePayload{
				Message: MessageType_Unknown,
				Id:      d.Slug(),
			}, err
		}
	}

	_, err = d.filehandle.Seek(0, io.SeekStart)
	if err != nil {
		return DevicePayload{
			Message: MessageType_Unknown,
			Id:      d.Slug(),
		}, err
	}

	b := make([]byte, 1)
	_, err = d.filehandle.Read(b)
	if err != nil {
		return DevicePayload{Message: MessageType_Unknown, Id: d.Slug()}, err
	}

	switch b[0] {
	case '0':
		return DevicePayload{Message: MessageType_TurnOff, Id: d.Slug()}, err
	case '1':
		return DevicePayload{Message: MessageType_TurnOn, Id: d.Slug()}, err
	}
	return DevicePayload{Message: MessageType_Unknown, Id: d.Slug()}, err
}

func (d *Device) Write(payload bool) error {
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
	d.loop = true

	for d.loop {
		value, err := d.Read()
		if err != nil {
			log.Fatalf("Error reading file: %v", err)
		}
		if d.prev != value {
			d.ReadEvents <- value
			d.prev = value
		}
		// }else {
		// 	log.Printf("Got the same value twice in a row")
		// }
		time.Sleep(pollIntervalMillis * time.Millisecond)
	}
}

// Close will just do some simple cleanup
func (d *Device) Close() error {
	d.loop = false
	close(d.ReadEvents)
	return nil
}
