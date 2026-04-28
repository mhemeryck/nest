package nest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunValidateOnlyAcceptsValidConfig(t *testing.T) {
	err := Run(t.Context(), Options{
		ConfigPath:   filepath.Join("..", "..", "test", "fixtures", "config.local.yaml"),
		ValidateOnly: true,
	})

	require.NoError(t, err)
}

func TestRunValidateOnlyReturnsLoadError(t *testing.T) {
	err := Run(t.Context(), Options{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"), ValidateOnly: true})

	require.Error(t, err)
	require.ErrorContains(t, err, "load config")
}
