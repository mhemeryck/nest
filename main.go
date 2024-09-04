package main

import (
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mhemeryck/nest/pkg/device"
)

const (
	filename = "./foo"
)

var (
	filenameRegex = regexp.MustCompile("/io_group(1|2|3)/(?P<device_fmt>di|do|ro)_(?P<io_group>1|2|3)_(?P<number>[0-9]{2})/(di|do|ro)_value$")
)

func main2() {
	reader := make(chan device.DevicePayload)
	writer := make(chan device.DevicePayload)

	d := &device.Device{
		Filename:    filename,
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

func main() {
	err := filepath.WalkDir("test/fixtures",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Fatalf("Error walking %v", err)
			}
			log.Printf("%v - %v\n", path, d)
			return nil
		})
	if err != nil {
		panic(err)
	}

}
