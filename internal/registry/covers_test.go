package registry

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverRegistryFromGlobalConfig(t *testing.T) {
	local, err := config.Load(filepath.Join("..", "..", "test", "fixtures", "config.covers.yaml"), "controller_1")
	require.NoError(t, err)
	root := config.ToEntityRoot(local)
	reg := Build(root)
	expected := root.Covers[0]

	assert.Equal(t, entity.CoverControl{MaxRun: 30 * time.Second}, CoverControl(reg))
	assert.Equal(t, root.Covers, Covers(reg))
	cover, ok := CoverByID(reg, entity.CoverID("controller_1.cover.office"))
	require.True(t, ok)
	assert.Equal(t, expected, cover)
	for _, relayID := range []entity.RelayID{expected.OpenRelay, expected.CloseRelay} {
		cover, ok = CoverByRelay(reg, relayID)
		require.True(t, ok)
		assert.Equal(t, expected, cover)
		_, ok := RelayByID(reg, relayID)
		assert.True(t, ok)
	}

	_, ok = CoverByID(reg, entity.CoverID("controller_2.cover.bedroom"))
	assert.False(t, ok)
	_, ok = CoverByRelay(reg, entity.RelayID("office_light"))
	assert.False(t, ok)
	_, ok = LightByID(reg, entity.LightID("controller_1.light.office"))
	assert.True(t, ok)

	root.Covers[0].Name = "Changed source"
	assert.Equal(t, []entity.Cover{expected}, Covers(reg))
	covers := Covers(reg)
	covers[0].Name = "Changed result"
	assert.Equal(t, []entity.Cover{expected}, Covers(reg))
	cover, ok = CoverByID(reg, expected.ID)
	require.True(t, ok)
	assert.Equal(t, expected, cover)
}
