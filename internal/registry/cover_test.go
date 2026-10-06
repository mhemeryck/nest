package registry

import (
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverIndexes(t *testing.T) {
	cover := entity.Cover{ID: "unit.cover.office", Name: "Office", OpenRelay: "open", CloseRelay: "close"}
	settings := entity.CoverControl{FullTravelDuration: 30 * time.Second}
	root := &entity.Root{Covers: []entity.Cover{cover}, CoverControl: settings}
	reg := Build(root)
	root.Covers[0].Name = "Changed"
	found, ok := CoverByID(reg, cover.ID)
	require.True(t, ok)
	assert.Equal(t, cover, found)
	for _, relayID := range []entity.RelayID{cover.OpenRelay, cover.CloseRelay} {
		found, ok := CoverByRelay(reg, relayID)
		require.True(t, ok)
		assert.Equal(t, cover, found)
	}
	_, ok = CoverByRelay(reg, "missing")
	assert.False(t, ok)
	_, ok = CoverByID(reg, "missing")
	assert.False(t, ok)
	covers := Covers(reg)
	covers[0].Name = "Also changed"
	assert.Equal(t, []entity.Cover{cover}, Covers(reg))
	assert.Equal(t, settings, CoverControl(reg))
}
