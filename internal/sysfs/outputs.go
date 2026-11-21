package sysfs

import (
	"context"
	"errors"
	"fmt"
	"os"
)

type activeOutput uint8

const (
	bothOff activeOutput = iota
	firstOn
	secondOn
)

// Local ordered I/O; no channel acknowledgements between output writes.
func executeExclusiveOutputs(
	ctx context.Context,
	first, second *Device,
	active activeOutput,
	setOutput func(*Device, Value) error,
) error {
	firstErr := setOutput(first, Off)
	secondErr := setOutput(second, Off)
	if err := errors.Join(firstErr, secondErr); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var selected *Device
	switch active {
	case bothOff:
		return nil
	case firstOn:
		selected = first
	case secondOn:
		selected = second
	default:
		return fmt.Errorf("invalid active output %d", active)
	}
	activateErr := setOutput(selected, On)
	if activateErr != nil || ctx.Err() != nil {
		// Activation may have energized an output; remove power without retrying ON.
		firstErr = setOutput(first, Off)
		secondErr = setOutput(second, Off)
		if err := errors.Join(activateErr, firstErr, secondErr); err != nil {
			return err
		}
		return ctx.Err()
	}
	return nil
}

func writeAndConfirmOutput(device *Device, value Value) error {
	if err := writeExistingOutput(device.Path, value); err != nil {
		return fmt.Errorf("output %q: write %d: %w", device.Identifier, PrintableValue(value), err)
	}
	actual, err := readValue(device.Path)
	if err != nil {
		return fmt.Errorf("output %q: readback: %w", device.Identifier, err)
	}
	device.Value = actual
	if actual != value {
		return fmt.Errorf("output %q: expected %d, read %d", device.Identifier, PrintableValue(value), PrintableValue(actual))
	}
	return nil
}

func writeExistingOutput(path string, value Value) error {
	// Missing hardware must fail, not create a replacement value file.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	_, writeErr := file.Write([]byte{byte(value), '\n'})
	return errors.Join(writeErr, file.Close())
}
