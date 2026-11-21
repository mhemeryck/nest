package sysfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExclusiveOutputs(t *testing.T) {
	for _, active := range []activeOutput{bothOff, firstOn, secondOn} {
		first, second := outputPairFixture(t, On, On)
		require.NoError(t, executeExclusiveOutputs(t.Context(), first, second, active, writeAndConfirmOutput))
		expectedFirst, expectedSecond := Off, Off
		if active == firstOn {
			expectedFirst = On
		}
		if active == secondOn {
			expectedSecond = On
		}
		value, err := readValue(first.Path)
		require.NoError(t, err)
		assert.Equal(t, expectedFirst, value)
		value, err = readValue(second.Path)
		require.NoError(t, err)
		assert.Equal(t, expectedSecond, value)
	}
}

func TestOffFailureBlocksActivationAndStillStopsOtherOutput(t *testing.T) {
	for _, failFirst := range []bool{true, false} {
		first, second := outputPairFixture(t, On, On)
		failed, other := first, second
		if !failFirst {
			failed, other = second, first
		}
		require.NoError(t, os.Remove(failed.Path))
		err := executeExclusiveOutputs(t.Context(), first, second, firstOn, writeAndConfirmOutput)
		require.ErrorContains(t, err, failed.Identifier)
		value, err := readValue(other.Path)
		require.NoError(t, err)
		assert.Equal(t, Off, value)
		_, err = os.Stat(failed.Path)
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestFailedActivationAttemptsBothOffWithoutRetryingOn(t *testing.T) {
	var writes []string
	err := executeExclusiveOutputs(t.Context(), &Device{Identifier: "first"}, &Device{Identifier: "second"}, firstOn, func(device *Device, value Value) error {
		writes = append(writes, fmt.Sprintf("%s=%d", device.Identifier, PrintableValue(value)))
		if value == On {
			return fmt.Errorf("activation readback failed")
		}
		if len(writes) == 4 {
			return fmt.Errorf("first cleanup failed")
		}
		return nil
	})
	require.ErrorContains(t, err, "activation readback failed")
	assert.ErrorContains(t, err, "first cleanup failed")
	assert.Equal(t, []string{"first=0", "second=0", "first=1", "first=0", "second=0"}, writes)
}

func TestCancellationDuringActivationRemovesPower(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var writes []Value
	err := executeExclusiveOutputs(ctx, &Device{}, &Device{}, secondOn, func(_ *Device, value Value) error {
		writes = append(writes, value)
		if value == On {
			cancel()
		}
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []Value{Off, Off, On, Off, Off}, writes)
}

func outputPairFixture(t *testing.T, firstValue, secondValue Value) (*Device, *Device) {
	t.Helper()
	makeOutput := func(id string, value Value) *Device {
		path := filepath.Join(t.TempDir(), "ro_value")
		require.NoError(t, writeValue(path, value))
		return &Device{Identifier: id, Path: path, Type: RelayOutput, Value: value}
	}
	return makeOutput("first", firstValue), makeOutput("second", secondValue)
}
