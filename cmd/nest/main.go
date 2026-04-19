package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mhemeryck/nest/internal/sysfs"
)

func main() {
	root := "/home/mhemeryck/Projects/nest/test/fixtures"

	fmt.Println("Crawling sysfs device tree...")
	devices, err := sysfs.ListDevices(root)
	if err != nil {
		log.Printf("Crawl failed: %v", err)
	} else {
		fmt.Printf("Found %d devices\n", len(devices))
		for _, d := range devices[:5] {
			fmt.Printf("  %s (%s)\n", d.Identifier, d.Path)
		}
	}

	configs := sysfs.BuildWorkerConfigs(devices)

	stopChs, events := sysfs.StartWorkers(configs)

	go func() {
		for event := range events {
			fmt.Printf("%s (%s): %d -> %d (rising=%t)\n",
				event.Device.Identifier,
				event.Device.Path,
				sysfs.ValueInt(event.OldValue),
				sysfs.ValueInt(event.NewValue),
				event.IsRising,
			)
		}
	}()

	fmt.Println("Polling devices... Press Ctrl+C to exit")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	sysfs.StopWorkers(stopChs)
}
