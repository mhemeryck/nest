package nest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunValidateOnlyAcceptsValidConfig(t *testing.T) {
	err := Run(t.Context(), Options{
		ConfigPath:   filepath.Join("..", "..", "test", "fixtures", "config.local.yaml"),
		UnitID:       "controller_1",
		ValidateOnly: true,
	})

	require.NoError(t, err)
}

func TestRunValidateOnlyReturnsLoadError(t *testing.T) {
	err := Run(t.Context(), Options{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"), UnitID: "controller_1", ValidateOnly: true})

	require.Error(t, err)
	require.ErrorContains(t, err, "load config")
}

func TestRunValidateOnlyRejectsUnknownUnit(t *testing.T) {
	err := Run(t.Context(), Options{
		ConfigPath:   filepath.Join("..", "..", "test", "fixtures", "config.local.yaml"),
		UnitID:       "missing_unit",
		ValidateOnly: true,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, `unit_id: unknown unit "missing_unit"`)
}
