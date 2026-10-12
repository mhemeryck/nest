package mqtt

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mhemeryck/nest/internal/entity"
)

type CoverObservation struct {
	CoverID           entity.CoverID
	State             entity.CoverState `json:"state"`
	HAState           string            `json:"ha_state"`
	EstimatedPosition *float64          `json:"estimated_position"`
	Available         bool              `json:"available"`
}

func CoverStateMessage(topics Topics, observation CoverObservation) (PublishMessage, error) {
	payload, err := json.Marshal(coverStatePayload(observation))
	if err != nil {
		return PublishMessage{}, err
	}
	return PublishMessage{Topic: CoverStateTopic(topics, observation.CoverID), Payload: payload, Retain: true}, nil
}

type coverStateReport struct {
	State             entity.CoverState `json:"state"`
	HAState           string            `json:"ha_state"`
	EstimatedPosition *float64          `json:"estimated_position"`
	Available         bool              `json:"available"`
}

func coverStatePayload(observation CoverObservation) coverStateReport {
	haState := string(observation.State)
	if observation.State == entity.CoverStateUnknown {
		haState = "None"
	}
	if observation.EstimatedPosition != nil {
		position := math.Round(*observation.EstimatedPosition)
		observation.EstimatedPosition = &position
	}
	return coverStateReport{
		State: observation.State, HAState: haState,
		EstimatedPosition: observation.EstimatedPosition, Available: observation.Available,
	}
}

func CoverCommandMessage(topics Topics, coverID entity.CoverID, action entity.CoverAction) (PublishMessage, error) {
	if !entity.IsID(string(coverID), entity.TypeCover) {
		return PublishMessage{}, fmt.Errorf("invalid remote cover id %q", coverID)
	}
	payload, err := coverCommandPayload(action)
	if err != nil {
		return PublishMessage{}, err
	}
	unitID := strings.Split(string(coverID), ".")[0]
	targetTopics := NewTopics(topics.Prefix, unitID)
	return PublishMessage{Topic: CoverCommandTopic(targetTopics, coverID), Payload: []byte(payload)}, nil
}

func coverCommandPayload(action entity.CoverAction) (string, error) {
	switch action {
	case entity.CoverActionOpen:
		return "OPEN", nil
	case entity.CoverActionClose:
		return "CLOSE", nil
	case entity.CoverActionStop:
		return "STOP", nil
	default:
		return "", fmt.Errorf("invalid cover action %q", action)
	}
}
