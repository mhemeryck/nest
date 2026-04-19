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
			fmt.Printf("  %s\n", d)
		}
	}

	ioPaths, err := sysfs.ListIOValueFiles(root)
	if err != nil {
		log.Printf("IO files listing failed: %v", err)
	} else {
		fmt.Printf("Found %d IO value files\n", len(ioPaths))
	}

	diDevices := sysfs.MatchDevices(ioPaths)
	fmt.Printf("Matched %d devices\n", len(diDevices))

	configs := sysfs.BuildWorkerConfigs(diDevices)

	stopChs, events := sysfs.StartWorkers(configs)

	go func() {
		for event := range events {
			fmt.Printf("%s (%s): %d -> %d (rising=%t)\n",
				event.Device.Identifier,
				event.Device.Path,
				event.OldValue.Int(),
				event.NewValue.Int(),
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
