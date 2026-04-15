package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mhemeryck/nest/internal/sysfs"
)

func main() {
	fmt.Println("Crawling sysfs device tree...")
	devices, err := sysfs.ListDevices("/sys/devices/platform")
	if err != nil {
		log.Printf("Crawl failed: %v", err)
	} else {
		fmt.Printf("Found %d devices\n", len(devices))
		for _, d := range devices[:5] {
			fmt.Printf("  %s\n", d)
		}
	}

	configs := []sysfs.WorkerConfig{
		{
			Interval: 100 * time.Millisecond,
			Paths:    []string{"/sys/devices/platform/unipi_plc/io_group1/di_1_01/di_value"},
		},
	}

	stopChs := sysfs.StartWorkers(configs, func(event sysfs.PollEvent) {
		fmt.Printf("%s: %d -> %d (rising=%t)\n",
			event.Path, event.OldValue, event.NewValue, event.IsRising)
	})

	fmt.Println("Polling devices... Press Ctrl+C to exit")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	sysfs.StopWorkers(stopChs)
}
