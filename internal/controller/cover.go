package controller

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func dispatchCoverEvent(ctx context.Context, reg *registry.Registry, commands chan<- sysfs.Command, request event.Cover) {
	if _, ok := registry.CoverByID(reg, request.CoverID); !ok {
		slog.Error("unknown cover", "cover_id", request.CoverID)
		return
	}
	select {
	case <-ctx.Done():
	case commands <- sysfs.Command{Kind: sysfs.CoverCommand, Cover: &sysfs.CoverRequest{ID: request.CoverID, Action: request.Action}}:
	}
}
