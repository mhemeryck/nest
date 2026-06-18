package mqtt

import (
	"encoding/json"
	"strings"

	"github.com/mhemeryck/nest/internal/entity"
)

type PublishMessage struct {
	Topic   string
	Payload []byte
	Retain  bool
	QoS     byte
}

type AvailabilityStatus string

const (
	AvailabilityOnline  AvailabilityStatus = "online"
	AvailabilityOffline AvailabilityStatus = "offline"
)

type LightObservation struct {
	LightID entity.LightID
	State   string `json:"state"`
}

type SemanticSourceEventObservation struct {
	SourceID entity.ID
	Event    string
}

func AvailabilityMessage(topics Topics, status AvailabilityStatus) PublishMessage {
	return PublishMessage{
		Topic:   AvailabilityTopic(topics),
		Payload: []byte(status),
		Retain:  true,
		QoS:     0,
	}
}

func LightStateMessage(topics Topics, observation LightObservation) (PublishMessage, error) {
	payload, err := json.Marshal(struct {
		State string `json:"state"`
	}{State: observation.State})
	if err != nil {
		return PublishMessage{}, err
	}

	return PublishMessage{
		Topic:   LightStateTopic(topics, observation.LightID),
		Payload: payload,
		Retain:  true,
		QoS:     0,
	}, nil
}

func SemanticSourceEventMessage(topics Topics, observation SemanticSourceEventObservation) (PublishMessage, error) {
	payload, err := json.Marshal(struct {
		Source string `json:"source"`
		Event  string `json:"event"`
	}{
		Source: string(observation.SourceID),
		Event:  observation.Event,
	})
	if err != nil {
		return PublishMessage{}, err
	}

	return PublishMessage{
		Topic:   SemanticSourceEventTopic(topics, observation.SourceID),
		Payload: payload,
		Retain:  false,
		QoS:     0,
	}, nil
}

func ParseSemanticSourceEventMessage(message ReceivedMessage, topicPrefix string) (SemanticSourceEventObservation, bool) {
	sourceID, ok := parseSemanticSourceEventTopic(topicPrefix, message.Topic)
	if !ok {
		return SemanticSourceEventObservation{}, false
	}

	var payload struct {
		Source string `json:"source"`
		Event  string `json:"event"`
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return SemanticSourceEventObservation{}, false
	}
	if payload.Source != string(sourceID) {
		return SemanticSourceEventObservation{}, false
	}
	if payload.Event != "pressed" && payload.Event != "released" {
		return SemanticSourceEventObservation{}, false
	}

	return SemanticSourceEventObservation{
		SourceID: sourceID,
		Event:    payload.Event,
	}, true
}

func parseSemanticSourceEventTopic(topicPrefix string, topic string) (entity.ID, bool) {
	segments := strings.Split(strings.Trim(topic, "/"), "/")
	if len(segments) == 5 {
		if segments[0] != "units" || segments[2] != "sources" || segments[4] != "event" {
			return "", false
		}
	} else if len(segments) == 6 {
		if segments[0] != strings.Trim(topicPrefix, "/") || segments[1] != "units" || segments[3] != "sources" || segments[5] != "event" {
			return "", false
		}
		segments = segments[1:]
	} else {
		return "", false
	}

	unitID := segments[1]
	sourceID := segments[3]
	if !entity.IsID(sourceID, entity.TypeButton) || !strings.HasPrefix(sourceID, unitID+".") {
		return "", false
	}

	return entity.ID(sourceID), true
}
