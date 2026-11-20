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

func logCoverState(state event.CoverState) {
	if state.Error != nil {
		slog.Error("cover failure", "cover_id", state.CoverID, "error", state.Error)
		return
	}
	attrs := []slog.Attr{slog.String("cover_id", string(state.CoverID)), slog.Any("movement", state.Movement)}
	switch state.Position {
	case nil:
		attrs = append(attrs, slog.String("position", "unknown"))
	default:
		attrs = append(attrs, slog.Int("position", *state.Position))
	}
	slog.LogAttrs(context.Background(), slog.LevelInfo, "cover state", attrs...)
}
