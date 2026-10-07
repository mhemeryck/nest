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
	observation.HAState = string(observation.State)
	if observation.State == entity.CoverStateUnknown {
		observation.HAState = "None"
	}
	if observation.EstimatedPosition != nil {
		position := math.Round(*observation.EstimatedPosition)
		observation.EstimatedPosition = &position
	}
	payload, err := json.Marshal(struct {
		State             entity.CoverState `json:"state"`
		HAState           string            `json:"ha_state"`
		EstimatedPosition *float64          `json:"estimated_position"`
		Available         bool              `json:"available"`
	}{observation.State, observation.HAState, observation.EstimatedPosition, observation.Available})
	if err != nil {
		return PublishMessage{}, err
	}
	return PublishMessage{Topic: CoverStateTopic(topics, observation.CoverID), Payload: payload, Retain: true}, nil
}

func CoverCommandMessage(topics Topics, coverID entity.CoverID, action entity.CoverAction) (PublishMessage, error) {
	if !entity.IsID(string(coverID), entity.TypeCover) {
		return PublishMessage{}, fmt.Errorf("invalid remote cover id %q", coverID)
	}
	var payload string
	switch action {
	case entity.CoverActionOpen:
		payload = "OPEN"
	case entity.CoverActionClose:
		payload = "CLOSE"
	case entity.CoverActionStop:
		payload = "STOP"
	default:
		return PublishMessage{}, fmt.Errorf("invalid cover action %q", action)
	}
	unitID := strings.Split(string(coverID), ".")[0]
	targetTopics := NewTopics(topics.Prefix, unitID)
	return PublishMessage{Topic: CoverCommandTopic(targetTopics, coverID), Payload: []byte(payload)}, nil
}
