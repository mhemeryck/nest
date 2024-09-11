package main

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/mhemeryck/nest/pkg/device"
)

const (
	filename = "./foo"
)

var (
	filenameRegex = regexp.MustCompile(`/io_group(1|2|3)/(?P<device_fmt>di|do|ro)_(?P<io_group>1|2|3)_(?P<number>[0-9]{2})/(di|do|ro)_value$`)

	DeviceFormat_DigitalInput  = DeviceFormat("DigitalInput")
	DeviceFormat_DigitalOutput = DeviceFormat("DigitalOutput")
	DeviceFormat_RelayOutput   = DeviceFormat("RelayOutput")
)

func main2() {
	reader := make(chan device.DevicePayload)
	writer := make(chan device.DevicePayload)

	d := &device.Device{
		Path:        filename,
		ReadEvents:  reader,
		WriteEvents: writer,
	}
	go d.Loop()

	go func() {
		for msg := range reader {
			log.Printf("Reader got value %v", msg)
		}
	}()

	// test setup to send occasional tick to write something to a file
	ticker := time.NewTicker(3 * time.Second)
	for {
		select {
		case t := <-ticker.C:
			writer <- device.DevicePayload(t.Second()%2 == 0)
		}
	}
}

type DeviceFormat string
type IOGroup int
type DeviceNumber int

type DeviceMeta struct {
	Format DeviceFormat
	Group  IOGroup
	Number DeviceNumber
	Path   string
}

func DeviceMetaFromPath(path string) (DeviceMeta, error) {
	match := filenameRegex.FindStringSubmatch(path)
	if len(match) == 0 {
		return DeviceMeta{}, fmt.Errorf("No device matched path")
	}

	d := DeviceMeta{Path: path}

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

func main3() {
	// paths := make([]string, 0)
	metas := make([]DeviceMeta, 0)
	err := filepath.WalkDir("test/fixtures",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Fatalf("Error walking %v", err)
			}
			// if filenameRegex.MatchString(path) {
			// 	log.Printf("%v - %v\n", path, d.Name())
			// 	paths = append(paths, path)
			// }
			meta, err := DeviceMetaFromPath(path)
			if err == nil {
				log.Printf("%v - %v - %v\n", path, d.Name(), meta)
				metas = append(metas, meta)
			}

			return nil
		})
	if err != nil {
		panic(err)
	}
	log.Printf("Found %d matching paths", len(metas))
}

func main() {
	mgr, err := device.NewDeviceManagerFromPath("test/fixtures")
	if err != nil {
		log.Fatalf("Can't start a device manager: %v", err)
	}

	reader := make(chan device.DevicePayload)

	for k, d := range mgr.Devices {
		log.Printf("Found device %v\n", d)
		d.ReadEvents = reader
		go d.Loop()
		if k > 1 {
			break
		}
	}

	go func() {
		for msg := range reader {
			log.Printf("Reader got value %v", msg)
		}
	}()

	d := mgr.Devices[0]
	log.Printf("device %v\n", d)
	for i := 0; i < 5; i++ {
		d.Write(device.DevicePayload(i%2 == 0))
		time.Sleep(3 * time.Second)
	}

}
