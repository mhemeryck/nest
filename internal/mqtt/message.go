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

func AvailabilityMessage(topics Topics, status AvailabilityStatus) PublishMessage {
	return PublishMessage{
		Topic:   AvailabilityTopic(topics),
		Payload: []byte(status),
		Retain:  true,
		QoS:     0,
	}
}

func DiscoveryMessage(root *entity.Root, topics Topics) (PublishMessage, error) {
	payload, err := DiscoveryPayload(root, topics)
	if err != nil {
		return PublishMessage{}, err
	}

	return PublishMessage{
		Topic:   DiscoveryTopic(topics),
		Payload: payload,
		Retain:  true,
		QoS:     0,
	}, nil
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
