package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func main() {
	configPath := flag.String("config", "", "Path to config file")
	validateOnly := flag.Bool("validate", false, "Validate config and exit")
	flag.Parse()

	file, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Load config failed: %v", err)
	}

	if *validateOnly {
		fmt.Printf("Config is valid: %s\n", *configPath)
		return
	}

	fmt.Println("Crawling sysfs device tree...")
	devices, err := sysfs.ListDevices(file.Sysfs.Root)
	if err != nil {
		log.Printf("Crawl failed: %v", err)
		return
	}

	configuredDevices, missing := configuredDevices(devices, config.DeviceIDs(file))
	if len(missing) > 0 {
		for _, deviceID := range missing {
			log.Printf("Configured device not found in sysfs: %s", deviceID)
		}
		os.Exit(1)
	}

	fmt.Printf("Configured %d devices\n", len(configuredDevices))
	for _, device := range configuredDevices {
		fmt.Printf("  %s (%s)\n", device.Identifier, device.Path)
	}

	configs := sysfs.BuildWorkerConfigs(configuredDevices)

	stopChs, events := sysfs.StartWorkers(configs)

	go func() {
		for event := range events {
			fmt.Printf("%s (%s): %d -> %d (rising=%t)\n",
				event.Device.Identifier,
				event.Device.Path,
				int(event.OldValue-'0'),
				int(event.NewValue-'0'),
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

func configuredDevices(devices []*sysfs.Device, wanted []string) ([]*sysfs.Device, []string) {
	byIdentifier := make(map[string]*sysfs.Device, len(devices))
	for _, device := range devices {
		byIdentifier[device.Identifier] = device
	}

	configured := make([]*sysfs.Device, 0, len(wanted))
	missing := make([]string, 0)
	for _, identifier := range wanted {
		device, ok := byIdentifier[identifier]
		if !ok {
			missing = append(missing, identifier)
			continue
		}
		configured = append(configured, device)
	}

	sort.Strings(missing)
	return configured, missing
}
