package nest

import (
	"context"
	"fmt"

	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/registry"
)

func startController(
	ctx context.Context,
	reg *registry.Registry,
	sysfsActor sysfsActor,
	mqttActor mqttActor,
	modbusActor modbusActor,
) <-chan struct{} {
	done := make(chan struct{})
	go controller.Run(
		ctx,
		reg,
		sysfsActor.commands,
		mqttActor.commands,
		modbusActor.commands,
		mqttActor.topics,
		sysfsActor.states,
		mqttActor.events,
		modbusActor.events,
		done,
	)

	return done
}

func waitForShutdown(
	ctx context.Context,
	cancel context.CancelFunc,
	controllerDone <-chan struct{},
	sysfsActor sysfsActor,
	mqttActor mqttActor,
	modbusActor modbusActor,
) error {
	var shutdownErr error
	select {
	case <-controllerDone:
		cancel()
	case <-modbusActor.done:
		if ctx.Err() == nil {
			shutdownErr = fmt.Errorf("modbus actor stopped")
		}
		cancel()
		<-controllerDone
	}

	waitForSysfsActor(sysfsActor)
	waitForMQTTActor(mqttActor)
	waitForModbusActor(modbusActor)

	return shutdownErr
}
