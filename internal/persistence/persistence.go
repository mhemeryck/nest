package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/mhemeryck/nest/internal/entity"
)

type Assignment struct {
	CoverID     entity.CoverID
	OpenRelay   entity.RelayID
	CloseRelay  entity.RelayID
	OpenDevice  entity.SysfsDeviceID
	CloseDevice entity.SysfsDeviceID
}

type Record struct {
	OpenRelay   entity.RelayID       `json:"open_relay"`
	CloseRelay  entity.RelayID       `json:"close_relay"`
	OpenDevice  entity.SysfsDeviceID `json:"open_device"`
	CloseDevice entity.SysfsDeviceID `json:"close_device"`
	Position    *float64             `json:"position"`
	Unfinished  bool                 `json:"unfinished"`
}

type Snapshot struct {
	Version int                       `json:"version"`
	UnitID  string                    `json:"unit_id"`
	Clean   bool                      `json:"clean"`
	Covers  map[entity.CoverID]Record `json:"covers"`
}

type CommandKind string

const (
	StartSession  CommandKind = "start_session"
	StartMovement CommandKind = "start_movement"
	StopMovement  CommandKind = "stop_movement"
	StopSession   CommandKind = "stop_session"
	Flush         CommandKind = "flush"
)

type Command struct {
	Kind     CommandKind
	CoverID  entity.CoverID
	Position *float64
	Done     chan struct{}
}

type Store struct {
	path       string
	snapshot   Snapshot
	commands   chan Command
	invalidate chan struct{}
	untrusted  atomic.Bool
}

func NewStore(path string, unitID string, assignments []Assignment, capacity int) *Store {
	store := &Store{path: path, commands: make(chan Command, capacity), invalidate: make(chan struct{}, 1),
		snapshot: Snapshot{Version: 1, UnitID: unitID, Covers: make(map[entity.CoverID]Record)},
	}
	for _, assignment := range assignments {
		store.snapshot.Covers[assignment.CoverID] = Record{OpenRelay: assignment.OpenRelay, CloseRelay: assignment.CloseRelay,
			OpenDevice: assignment.OpenDevice, CloseDevice: assignment.CloseDevice, Unfinished: true}
	}
	return store
}

func Restore(store *Store) map[entity.CoverID]*float64 {
	positions := make(map[entity.CoverID]*float64)
	data, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return positions
	}
	if err != nil {
		slog.Error("persistence restore failed", "error", err)
		return positions
	}
	var saved Snapshot
	if err := json.Unmarshal(data, &saved); err != nil {
		slog.Error("persistence restore failed", "error", err)
		return positions
	}
	if saved.Version != 1 || saved.UnitID != store.snapshot.UnitID || !saved.Clean {
		return positions
	}
	for id, expected := range store.snapshot.Covers {
		record, exists := saved.Covers[id]
		if !exists || record.Unfinished || record.OpenRelay != expected.OpenRelay || record.CloseRelay != expected.CloseRelay || record.OpenDevice != expected.OpenDevice || record.CloseDevice != expected.CloseDevice {
			continue
		}
		if record.Position == nil || math.IsNaN(*record.Position) || math.IsInf(*record.Position, 0) || *record.Position < 0 || *record.Position > 100 {
			continue
		}
		position := *record.Position
		positions[id] = &position
		expected.Position = &position
		store.snapshot.Covers[id] = expected
	}
	return positions
}

func QueueCommand(store *Store, command Command) error {
	if command.Position != nil {
		position := *command.Position
		command.Position = &position
	}
	select {
	case store.commands <- command:
		return nil
	default:
		store.untrusted.Store(true)
		select {
		case store.invalidate <- struct{}{}:
		default:
		}
		return fmt.Errorf("persistence handoff capacity exhausted")
	}
}

func Run(ctx context.Context, store *Store, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-store.invalidate:
			store.snapshot.Clean = false
			persistSnapshot(store)
		case command := <-store.commands:
			if command.Kind == Flush {
				if store.untrusted.Load() {
					persistSnapshot(store)
				}
				close(command.Done)
				continue
			}
			applyCommand(store, command)
			persistSnapshot(store)
		}
	}
}

func applyCommand(store *Store, command Command) {
	switch command.Kind {
	case StartSession:
		store.snapshot.Clean = false
	case StopSession:
		store.snapshot.Clean = !store.untrusted.Load()
		for _, record := range store.snapshot.Covers {
			if record.Unfinished {
				store.snapshot.Clean = false
				break
			}
		}
	case StartMovement, StopMovement:
		record, exists := store.snapshot.Covers[command.CoverID]
		if !exists {
			store.untrusted.Store(true)
			return
		}
		record.Unfinished = command.Kind == StartMovement
		if command.Kind == StopMovement {
			record.Position = command.Position
		}
		store.snapshot.Covers[command.CoverID] = record
	}
}

func persistSnapshot(store *Store) {
	if store.untrusted.Load() {
		store.snapshot.Clean = false
	}
	if err := writeSnapshot(store.path, store.snapshot); err != nil {
		store.untrusted.Store(true)
		store.snapshot.Clean = false
		slog.Error("persistence write failed", "path", store.path, "error", err)
	}
}

func writeSnapshot(path string, snapshot Snapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".nest-snapshot-*")
	if err != nil {
		return fmt.Errorf("create snapshot temporary file: %w", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync snapshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close snapshot: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	dir, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open snapshot directory: %w", err)
	}
	defer func() { _ = dir.Close() }()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync snapshot directory: %w", err)
	}
	return nil
}

func FlushWithin(ctx context.Context, store *Store) bool {
	done := make(chan struct{})
	if err := QueueCommand(store, Command{Kind: Flush, Done: done}); err != nil {
		return false
	}
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}
