package entity

import (
	"fmt"
	"regexp"
	"strings"
)

type Type string

const (
	TypeButton Type = "button"
	TypeLight  Type = "light"
)

type ID string

var localIDPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

func NewID(unitID string, entityType Type, localID string) ID {
	return ID(fmt.Sprintf("%s.%s.%s", unitID, entityType, localID))
}

func IsLocalID(value string) bool {
	return localIDPattern.MatchString(value)
}

func IsID(value string, entityType Type) bool {
	parts := strings.Split(value, ".")
	return len(parts) == 3 &&
		IsLocalID(parts[0]) &&
		parts[1] == string(entityType) &&
		IsLocalID(parts[2])
}

func IsIDForUnit(value string, unitID string, entityType Type) bool {
	parts := strings.Split(value, ".")
	return len(parts) == 3 &&
		parts[0] == unitID &&
		parts[1] == string(entityType) &&
		IsLocalID(parts[2])
}

func LocalID(value string, entityType Type) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || !IsLocalID(parts[0]) || parts[1] != string(entityType) || !IsLocalID(parts[2]) {
		return "", false
	}

	return parts[2], true
}
