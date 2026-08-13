package nest

import (
	"context"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/registry"
)

type modbusActor struct {
	enabled  bool
	commands chan modbus.Command
	events   chan modbus.Event
	done     chan struct{}
	cfg      entity.Modbus
}

func newModbusActor(reg *registry.Registry) modbusActor {
	cfg := registry.Modbus(reg)
	if cfg.Mode == "" {
		return modbusActor{}
	}

	return modbusActor{
		enabled:  true,
		commands: make(chan modbus.Command, 32),
		events:   make(chan modbus.Event, 32),
		done:     make(chan struct{}),
		cfg:      cfg,
	}
}

func startModbusActor(ctx context.Context, actor modbusActor) {
	if !actor.enabled {
		return
	}

	go modbus.Run(ctx, actor.cfg, actor.commands, actor.events, actor.done)
}

func waitForModbusActor(actor modbusActor) {
	if !actor.enabled {
		return
	}

	<-actor.done
	close(actor.events)
}
