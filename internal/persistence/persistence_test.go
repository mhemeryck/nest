package persistence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAssignment() Assignment {
	return Assignment{CoverID: "unit.cover.office", OpenRelay: "open", CloseRelay: "close", OpenDevice: "ro_1_1", CloseDevice: "ro_1_2"}
}

func TestOrderedPersistenceAndCleanRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "positions.json")
	assignment := testAssignment()
	store := NewStore(path, "unit", []Assignment{assignment}, 8)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go Run(ctx, store, done)
	defer func() { cancel(); <-done }()
	position := 60.0
	for _, command := range []Command{
		{Kind: StartSession}, {Kind: StartMovement, CoverID: assignment.CoverID},
		{Kind: StopMovement, CoverID: assignment.CoverID, Position: &position}, {Kind: StopSession},
	} {
		require.NoError(t, QueueCommand(store, command))
	}
	flushContext, flushCancel := context.WithTimeout(t.Context(), time.Second)
	defer flushCancel()
	require.True(t, FlushWithin(flushContext, store))
	restarted := NewStore(path, "unit", []Assignment{assignment}, 8)
	positions := Restore(restarted)
	require.Contains(t, positions, assignment.CoverID)
	assert.Equal(t, 60.0, *positions[assignment.CoverID])
	assert.True(t, restarted.snapshot.Covers[assignment.CoverID].Unfinished)
	assert.False(t, restarted.snapshot.Clean)
}

func TestUntrustedRecordsRestoreUnknown(t *testing.T) {
	assignment := testAssignment()
	for _, tc := range []string{"unclean", "unfinished", "version", "unit", "relay", "device", "missing", "out_of_range", "malformed"} {
		t.Run(tc, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "positions.json")
			store := NewStore(path, "unit", []Assignment{assignment}, 8)
			position := 60.0
			record := store.snapshot.Covers[assignment.CoverID]
			record.Position = &position
			record.Unfinished = false
			store.snapshot.Clean = true
			switch tc {
			case "unclean":
				store.snapshot.Clean = false
			case "unfinished":
				record.Unfinished = true
			case "version":
				store.snapshot.Version = 2
			case "unit":
				store.snapshot.UnitID = "other"
			case "relay":
				record.OpenRelay = "other"
			case "device":
				record.OpenDevice = "ro_2_1"
			case "out_of_range":
				position = 101
			}
			store.snapshot.Covers[assignment.CoverID] = record
			if tc == "missing" {
				delete(store.snapshot.Covers, assignment.CoverID)
			}
			if tc == "malformed" {
				require.NoError(t, os.WriteFile(path, []byte("bad json"), 0o600))
			} else {
				require.NoError(t, writeSnapshot(path, store.snapshot))
			}
			assert.Empty(t, Restore(NewStore(path, "unit", []Assignment{assignment}, 8)))
		})
	}
}

func TestPersistenceOverloadInvalidatesCleanShutdown(t *testing.T) {
	assignment := testAssignment()
	path := filepath.Join(t.TempDir(), "positions.json")
	store := NewStore(path, "unit", []Assignment{assignment}, 1)
	require.NoError(t, QueueCommand(store, Command{Kind: StopMovement, CoverID: assignment.CoverID}))
	require.Error(t, QueueCommand(store, Command{Kind: StartMovement, CoverID: assignment.CoverID}))
	assert.True(t, store.untrusted.Load())
	applyCommand(store, <-store.commands)
	applyCommand(store, Command{Kind: StopSession})
	persistSnapshot(store)
	assert.Empty(t, Restore(NewStore(path, "unit", []Assignment{assignment}, 8)))
}

func TestWriteFailureDoesNotAllowCleanMarker(t *testing.T) {
	store := NewStore(t.TempDir(), "unit", []Assignment{testAssignment()}, 8)
	applyCommand(store, Command{Kind: StopMovement, CoverID: testAssignment().CoverID})
	persistSnapshot(store)
	assert.True(t, store.untrusted.Load())
	applyCommand(store, Command{Kind: StopSession})
	assert.False(t, store.snapshot.Clean)
}

func TestInterruptedPersistenceFlushIsBounded(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "snapshot"), "unit", []Assignment{testAssignment()}, 8)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	assert.False(t, FlushWithin(ctx, store), "a stopped writer cannot confirm final persistence")
}

func TestSnapshotReplacementLeavesNoTemporaryFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "positions.json")
	store := NewStore(path, "unit", []Assignment{testAssignment()}, 8)
	require.NoError(t, writeSnapshot(path, store.snapshot))
	require.NoError(t, writeSnapshot(path, store.snapshot))
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "positions.json", entries[0].Name())
}

func TestMissingFileHasUnknownPosition(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing"), "unit", []Assignment{testAssignment()}, 8)
	assert.Empty(t, Restore(store))
}
