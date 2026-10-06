package nest

import (
	"context"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/persistence"
	"github.com/mhemeryck/nest/internal/registry"
)

type persistenceActor struct {
	store     *persistence.Store
	positions map[entity.CoverID]*float64
	done      chan struct{}
}

func newPersistenceActor(reg *registry.Registry) persistenceActor {
	covers := registry.Covers(reg)
	if len(covers) == 0 {
		return persistenceActor{}
	}
	assignments := make([]persistence.Assignment, 0, len(covers))
	for _, cover := range covers {
		openRelay, _ := registry.RelayByID(reg, cover.OpenRelay)
		closeRelay, _ := registry.RelayByID(reg, cover.CloseRelay)
		assignments = append(assignments, persistence.Assignment{CoverID: cover.ID, OpenRelay: cover.OpenRelay, CloseRelay: cover.CloseRelay,
			OpenDevice: openRelay.SysfsDevice, CloseDevice: closeRelay.SysfsDevice})
	}
	store := persistence.NewStore(registry.PersistencePath(reg), registry.MQTT(reg).UnitID, assignments, max(32, 2*len(covers)+4))
	return persistenceActor{store: store, positions: persistence.Restore(store), done: make(chan struct{})}
}

func startPersistenceActor(ctx context.Context, actor persistenceActor) {
	if actor.store != nil {
		go persistence.Run(ctx, actor.store, actor.done)
	}
}
