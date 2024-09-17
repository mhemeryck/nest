package main

import (
	"log"
	"time"

	"github.com/mhemeryck/nest/pkg/device"
)

var (
	// direct mapping between input and output
	automations = map[string]string{
		"do-1-01": "ro-2-13",
	}
)

func main() {
	mgr, err := device.NewDeviceManagerFromPath("test/fixtures")
	if err != nil {
		log.Fatalf("Can't start a device manager: %v", err)
	}

	reader := make(chan device.DevicePayload)

	for _, d := range mgr.Devices {
		log.Printf("Found device %v\n", d)
		d.ReadEvents = reader
		go d.Loop()
	}

	go func() {
		for msg := range reader {
			log.Printf("Reader got value %v", msg)
			if output, ok := automations[msg.Id]; ok {
				log.Printf("Got a match on the input, can now trigger the output: %v", output)
				err = mgr.Devices[output].Write(msg.Message)
				if err != nil {
					panic(err)
				}
			}
		}

	}()

	d := mgr.Devices["do-1-01"]
	log.Printf("device %v\n", d)
	for i := 0; i < 5; i++ {
		d.Write(i%2 == 0)
		time.Sleep(3 * time.Second)
	}
}
