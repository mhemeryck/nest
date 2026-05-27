package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewID(t *testing.T) {
	assert.Equal(t, ID("controller_1.light.office"), NewID("controller_1", TypeLight, "office"))
}

func TestIsLocalID(t *testing.T) {
	assert.True(t, IsLocalID("office_light"))
	assert.False(t, IsLocalID("office-light"))
	assert.False(t, IsLocalID("controller_1.light.office"))
}

func TestIsID(t *testing.T) {
	assert.True(t, IsID("controller_1.button.office", TypeButton))
	assert.False(t, IsID("controller_1.light.office", TypeButton))
	assert.False(t, IsID("office", TypeButton))
}

func TestIsIDForUnit(t *testing.T) {
	assert.True(t, IsIDForUnit("controller_1.light.office", "controller_1", TypeLight))
	assert.False(t, IsIDForUnit("controller_2.light.office", "controller_1", TypeLight))
	assert.False(t, IsIDForUnit("controller_1.button.office", "controller_1", TypeLight))
}
