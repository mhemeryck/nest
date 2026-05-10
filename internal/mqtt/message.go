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

type DigitalInputObservation struct {
	InputID     entity.DigitalInputID `json:"input_id"`
	SysfsDevice entity.SysfsDeviceID  `json:"sysfs_device"`
	Value       int                   `json:"value"`
}

type PushButtonObservation struct {
	ButtonID entity.PushButtonID `json:"button_id"`
	Name     string              `json:"name"`
	State    string              `json:"state"`
}

type RelayObservation struct {
	RelayID     entity.RelayID       `json:"relay_id"`
	Name        string               `json:"name"`
	SysfsDevice entity.SysfsDeviceID `json:"sysfs_device"`
	Value       int                  `json:"value"`
}

type LightObservation struct {
	LightID entity.LightID `json:"light_id"`
	Name    string         `json:"name"`
	RelayID entity.RelayID `json:"relay_id"`
	Value   int            `json:"value"`
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

func DigitalInputStateMessage(topics Topics, observation DigitalInputObservation) (PublishMessage, error) {
	payload, err := json.Marshal(observation)
	if err != nil {
		return PublishMessage{}, err
	}

	return PublishMessage{
		Topic:   DigitalInputStateTopic(topics, observation.InputID),
		Payload: payload,
		Retain:  true,
		QoS:     0,
	}, nil
}

func PushButtonStateMessage(topics Topics, observation PushButtonObservation) (PublishMessage, error) {
	payload, err := json.Marshal(observation)
	if err != nil {
		return PublishMessage{}, err
	}

	return PublishMessage{
		Topic:   PushButtonStateTopic(topics, observation.ButtonID),
		Payload: payload,
		Retain:  false,
		QoS:     0,
	}, nil
}

func RelayStateMessage(topics Topics, observation RelayObservation) (PublishMessage, error) {
	payload, err := json.Marshal(observation)
	if err != nil {
		return PublishMessage{}, err
	}

	return PublishMessage{
		Topic:   RelayStateTopic(topics, observation.RelayID),
		Payload: payload,
		Retain:  true,
		QoS:     0,
	}, nil
}

func LightStateMessage(topics Topics, observation LightObservation) (PublishMessage, error) {
	payload, err := json.Marshal(observation)
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
