package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionReportDoesNotProduceButtonEdges(t *testing.T) {
	reg := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: "input", SysfsDevice: "di_1_1"}},
		PushButtons:   []entity.PushButton{{ID: "button", Input: "input"}},
	})
	now := time.Now()
	report := sysfs.StateChange{Kind: sysfs.CompletionReportKind, Device: sysfs.Device{Identifier: "di_1_1"}, IsRising: true,
		Completion: &sysfs.CommandCompletion{Command: sysfs.Command{ID: 42, DeviceID: "di_1_1", Kind: sysfs.OnCommand}, CompletedAt: now},
	}
	events, ok := semanticEventsFromStateChange(reg, report)
	require.True(t, ok)
	require.Len(t, events, 1)
	assert.Equal(t, event.OutputResultKind, events[0].Kind)
	assert.Equal(t, &event.OutputResult{CommandID: 42, SysfsDevice: "di_1_1", Action: entity.OutputActionOn, CompletedAt: now}, events[0].OutputResult)
	edges, _ := pushButtonEventsFromStateChange(reg, report)
	assert.Empty(t, edges)
}

func TestInputFailureIdentifiesInputWithoutButtonEdges(t *testing.T) {
	reg := registry.Build(&entity.Root{DigitalInputs: []entity.DigitalInput{{ID: "input", SysfsDevice: "di_1_1"}}})
	failure := fmt.Errorf("read failed")
	events, ok := semanticEventsFromStateChange(reg, sysfs.StateChange{
		Kind: sysfs.InputFailureReportKind, Device: sysfs.Device{Identifier: "di_1_1"}, Error: failure,
	})
	require.True(t, ok)
	require.Len(t, events, 1)
	assert.Equal(t, event.InputFailureKind, events[0].Kind)
	assert.Equal(t, entity.DigitalInputID("input"), events[0].InputFailure.InputID)
	assert.ErrorIs(t, events[0].InputFailure.Error, failure)
}
