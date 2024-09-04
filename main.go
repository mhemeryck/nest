package main

import (
	"log"
	"time"

	"github.com/mhemeryck/nest/pkg/device"
)

const (
	filename = "./foo"
)

func main() {
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
