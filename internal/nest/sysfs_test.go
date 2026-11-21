package nest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/controller/cover"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSysfsActorConfiguresLocalCoverFromRegistry(t *testing.T) {
	local, err := config.Load(filepath.Join("..", "..", "test", "fixtures", "config.covers.yaml"), "controller_1")
	require.NoError(t, err)
	root := config.ToEntityRoot(local)
	root.SysfsRoot = t.TempDir()
	for _, relay := range root.Relays {
		dir := filepath.Join(root.SysfsRoot, "sys", "devices", "platform", "unipi_plc", "io_group3", string(relay.SysfsDevice))
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ro_value"), []byte("0\n"), 0o600))
	}
	for _, input := range root.DigitalInputs {
		dir := filepath.Join(root.SysfsRoot, "sys", "devices", "platform", "unipi_plc", "io_group3", string(input.SysfsDevice))
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "di_value"), []byte("0\n"), 0o600))
	}
	actor, err := newSysfsActor(registry.Build(root))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, actor.maxRun)
	assert.Equal(t, []sysfs.CoverConfig{{ID: "controller_1.cover.office", OpenDeviceID: "ro_3_14", CloseDeviceID: "ro_3_13"}}, actor.covers)
	ctx, cancel := context.WithCancel(t.Context())
	startSysfsActor(ctx, actor)
	t.Cleanup(func() { cancel(); assert.NoError(t, waitForSysfsActor(actor)) })
	for _, movement := range []cover.Movement{cover.Stopped, cover.Opening} {
		if movement == cover.Opening {
			actor.commands <- sysfs.Command{Kind: sysfs.CoverCommand, Cover: &sysfs.CoverRequest{ID: entity.CoverID("controller_1.cover.office"), Action: entity.CoverActionOpen}}
		}
		for {
			select {
			case state := <-actor.states:
				if state.Cover == nil {
					continue
				}
				require.NoError(t, state.Cover.Error)
				assert.Equal(t, movement, state.Cover.Movement)
			case <-time.After(2 * time.Second):
				t.Fatal("missing configured cover state")
			}
			break
		}
	}
}
