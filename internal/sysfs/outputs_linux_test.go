package sysfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExclusiveOutputsReadbackFailureBlocksActivation(t *testing.T) {
	first, second := outputPairFixture(t, On, On)
	// Writes succeed, but readback is empty rather than a confirmed OFF value.
	first.Path = "/dev/null"
	err := executeExclusiveOutputs(t.Context(), first, second, secondOn, writeAndConfirmOutput)
	require.ErrorContains(t, err, "readback")
	value, err := readValue(second.Path)
	require.NoError(t, err)
	assert.Equal(t, Off, value)
}
