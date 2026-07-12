package nest

import (
	"context"

	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/registry"
)

func startController(
	ctx context.Context,
	reg *registry.Registry,
	sysfsActor sysfsActor,
	mqttActor mqttActor,
) <-chan struct{} {
	done := make(chan struct{})
	go controller.Run(
		ctx,
		reg,
		sysfsActor.commands,
		mqttActor.commands,
		mqttActor.topics,
		sysfsActor.states,
		mqttActor.events,
		done,
	)

	return done
}

func waitForShutdown(
	cancel context.CancelFunc,
	controllerDone <-chan struct{},
	sysfsActor sysfsActor,
	mqttActor mqttActor,
) {
	<-controllerDone
	cancel()
	waitForSysfsActor(sysfsActor)
	waitForMQTTActor(mqttActor)
}
