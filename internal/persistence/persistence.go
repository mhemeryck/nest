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
	return &Store{path: path, commands: make(chan Command, capacity), invalidate: make(chan struct{}, 1),
		snapshot: initialSnapshot(unitID, assignments),
	}
}

func initialSnapshot(unitID string, assignments []Assignment) Snapshot {
	snapshot := Snapshot{Version: 1, UnitID: unitID, Covers: make(map[entity.CoverID]Record)}
	for _, assignment := range assignments {
		snapshot.Covers[assignment.CoverID] = Record{OpenRelay: assignment.OpenRelay, CloseRelay: assignment.CloseRelay,
			OpenDevice: assignment.OpenDevice, CloseDevice: assignment.CloseDevice, Unfinished: true}
	}
	return snapshot
}

func Restore(store *Store) map[entity.CoverID]*float64 {
	positions := make(map[entity.CoverID]*float64)
	saved, err := readSnapshot(store.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("persistence restore failed", "error", err)
		}
		return positions
	}
	if !trustedSnapshotForUnit(saved, store.snapshot.UnitID) {
		return positions
	}
	return restoreCoverPositions(store, saved)
}

func trustedSnapshotForUnit(snapshot Snapshot, unitID string) bool {
	return snapshot.Version == 1 && snapshot.UnitID == unitID && snapshot.Clean
}

func readSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var saved Snapshot
	err = json.Unmarshal(data, &saved)
	return saved, err
}

func restoreCoverPositions(store *Store, saved Snapshot) map[entity.CoverID]*float64 {
	positions := make(map[entity.CoverID]*float64)
	for id, expected := range store.snapshot.Covers {
		record, exists := saved.Covers[id]
		if !exists || !trustedCoverRecord(record, expected) {
			continue
		}
		position := *record.Position
		positions[id] = &position
		expected.Position = &position
		store.snapshot.Covers[id] = expected
	}
	return positions
}

func trustedCoverRecord(record, expected Record) bool {
	if record.Unfinished || record.OpenRelay != expected.OpenRelay || record.CloseRelay != expected.CloseRelay || record.OpenDevice != expected.OpenDevice || record.CloseDevice != expected.CloseDevice {
		return false
	}
	return record.Position != nil && !math.IsNaN(*record.Position) && !math.IsInf(*record.Position, 0) && *record.Position >= 0 && *record.Position <= 100
}

func QueueCommand(store *Store, command Command) error {
	command = ownedStorageCommand(command)
	select {
	case store.commands <- command:
		return nil
	default:
		invalidateStoredSession(store)
		return fmt.Errorf("persistence handoff capacity exhausted")
	}
}

func ownedStorageCommand(command Command) Command {
	if command.Position != nil {
		position := *command.Position
		command.Position = &position
	}
	return command
}

func invalidateStoredSession(store *Store) {
	store.untrusted.Store(true)
	select {
	case store.invalidate <- struct{}{}:
	default:
	}
}

func Run(ctx context.Context, store *Store, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-store.invalidate:
			persistUncleanSnapshot(store)
		case command := <-store.commands:
			handleStorageCommand(store, command)
		}
	}
}

func handleStorageCommand(store *Store, command Command) {
	switch command.Kind {
	case Flush:
		flushStoredCommands(store, command.Done)
	default:
		applyCommand(store, command)
		persistSnapshot(store)
	}
}

func persistUncleanSnapshot(store *Store) {
	store.snapshot.Clean = false
	persistSnapshot(store)
}

func flushStoredCommands(store *Store, done chan struct{}) {
	if store.untrusted.Load() {
		persistSnapshot(store)
	}
	close(done)
}

func applyCommand(store *Store, command Command) {
	switch command.Kind {
	case StartSession:
		store.snapshot.Clean = false
	case StopSession:
		completeStoredSession(store)
	case StartMovement, StopMovement:
		updateStoredMovement(store, command)
	}
}

func completeStoredSession(store *Store) {
	store.snapshot.Clean = !store.untrusted.Load()
	for _, record := range store.snapshot.Covers {
		if record.Unfinished {
			store.snapshot.Clean = false
			break
		}
	}
}

func updateStoredMovement(store *Store, command Command) {
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

func persistSnapshot(store *Store) {
	if store.untrusted.Load() {
		store.snapshot.Clean = false
	}
	if err := writeSnapshot(store.path, store.snapshot); err != nil {
		recordSnapshotWriteFailure(store, err)
	}
}

func recordSnapshotWriteFailure(store *Store, failure error) {
	store.untrusted.Store(true)
	store.snapshot.Clean = false
	slog.Error("persistence write failed", "path", store.path, "error", failure)
}

func writeSnapshot(path string, snapshot Snapshot) error {
	data, err := encodeSnapshot(snapshot)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	file, err := createSnapshotFile(directory)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if err := writeAndSyncSnapshot(file, data); err != nil {
		return err
	}
	if err := replaceSnapshotFile(file.Name(), path); err != nil {
		return err
	}
	return syncSnapshotDirectory(directory)
}

func encodeSnapshot(snapshot Snapshot) ([]byte, error) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot: %w", err)
	}
	return data, nil
}

func replaceSnapshotFile(temporaryPath, path string) error {
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return nil
}

func createSnapshotFile(directory string) (*os.File, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create snapshot directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".nest-snapshot-*")
	if err != nil {
		return nil, fmt.Errorf("create snapshot temporary file: %w", err)
	}
	return file, nil
}

func writeAndSyncSnapshot(file *os.File, data []byte) error {
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
	return nil
}

func syncSnapshotDirectory(directory string) error {
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
