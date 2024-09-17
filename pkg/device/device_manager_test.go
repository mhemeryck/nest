package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_NewDeviceManagerFromPath does some integration test to see whether we can crawl a folder structure to find all required devices with their meta
func Test_NewDeviceManagerFromPath(t *testing.T) {
	mgr, err := NewDeviceManagerFromPath("../../test/fixtures")

	require.NoError(t, err, "Could not crawl fixtures path")
	assert.Equal(t, len(mgr.Devices), 68)
}
