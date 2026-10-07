package controller

import (
	"log/slog"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/persistence"
)

func dispatchPersistenceEvent(store *persistence.Store, incoming event.Event) []event.Event {
	if store == nil {
		return nil
	}
	command, handled := persistenceCommand(incoming)
	if !handled {
		return nil
	}
	if err := persistence.QueueCommand(store, command); err != nil {
		slog.Error("persistence integration failure", "error", err)
		return []event.Event{{Kind: event.IntegrationFailureKind, IntegrationFailure: &event.IntegrationFailure{Integration: "persistence", Error: err.Error()}}}
	}
	return nil
}

func persistenceCommand(incoming event.Event) (persistence.Command, bool) {
	switch incoming.Kind {
	case event.SessionStartedKind:
		return persistence.Command{Kind: persistence.StartSession}, true
	case event.SessionStoppedKind:
		return persistence.Command{Kind: persistence.StopSession}, true
	case event.CoverStartIntentKind:
		return persistence.Command{Kind: persistence.StartMovement, CoverID: incoming.CoverObservation.CoverID}, true
	case event.CoverStoppedKind:
		return persistence.Command{Kind: persistence.StopMovement, CoverID: incoming.CoverObservation.CoverID, Position: incoming.CoverObservation.EstimatedPosition}, true
	default:
		return persistence.Command{}, false
	}
}
