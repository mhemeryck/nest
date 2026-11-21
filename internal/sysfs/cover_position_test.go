package sysfs

import (
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/cover"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverPositionFollowsConfirmedMovement(t *testing.T) {
	open, close := outputPairFixture(t, Off, Off)
	c := localCover{id: "unit.cover.office", open: open, close: close}
	pending := make(map[entity.CoverID]CoverState)
	now := time.Now()
	maxRun := 30 * time.Second
	advance := func(input cover.Input) {
		t.Helper()
		require.NoError(t, advanceCover(t.Context(), &c, input, now, maxRun, pending))
	}
	assertPosition := func(expected int) {
		t.Helper()
		position := pending[c.id].Position
		require.NotNil(t, position)
		assert.Equal(t, expected, *position)
	}

	assert.Nil(t, coverSnapshot(c, now, maxRun).Position)
	advance(cover.Open)
	now = now.Add(10 * time.Second)
	advance(cover.Stop)
	assert.Nil(t, pending[c.id].Position)

	advance(cover.Close)
	now = now.Add(maxRun)
	advance(cover.CheckDeadline)
	assertPosition(0)
	assertOutput(t, close, Off)

	advance(cover.Open)
	now = now.Add(15 * time.Second)
	queueMovingPositions([]localCover{c}, now, maxRun, pending)
	assertPosition(50)
	assertOutput(t, open, On)
	advance(cover.Stop)
	assertPosition(50)
	assertOutput(t, open, Off)
	now = now.Add(time.Minute)
	frozen := coverSnapshot(c, now, maxRun)
	require.NotNil(t, frozen.Position)
	assert.Equal(t, 50, *frozen.Position)

	advance(cover.Open)
	now = now.Add(20 * time.Second)
	advance(cover.CheckDeadline)
	assertOutput(t, open, On)
	now = now.Add(10 * time.Second)
	advance(cover.CheckDeadline)
	assertPosition(100)
	assertOutput(t, open, Off)
}
