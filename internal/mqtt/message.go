package mqtt

import (
	"encoding/json"

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
