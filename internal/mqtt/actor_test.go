package mqtt

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublishCommand(t *testing.T) {
	message := PublishMessage{Topic: "nest/topic", Payload: []byte("payload"), Retain: true, QoS: 1}

	command := PublishCommand(message)

	assert.Equal(t, PublishCommandKind, command.Kind)
	assert.Equal(t, message, command.Publish)
}

func TestActorEvents(t *testing.T) {
	message := PublishMessage{Topic: "nest/topic", Payload: []byte("payload")}

	assert.Equal(t, Event{Kind: ConnectedEventKind}, ConnectedEvent())
	assert.Equal(t, Event{Kind: ConnectFailedKind, Error: "connect failed"}, ConnectFailedEvent(errors.New("connect failed")))
	assert.Equal(t, Event{Kind: ConnectFailedKind}, ConnectFailedEvent(nil))
	assert.Equal(t, Event{Kind: DisconnectedEventKind}, DisconnectedEvent())
	assert.Equal(t, Event{Kind: PublishedEventKind, Publish: message}, PublishedEvent(message))
	assert.Equal(t, Event{Kind: PublishFailedKind, Publish: message, Error: "publish failed"}, PublishFailedEvent(message, errors.New("publish failed")))
	assert.Equal(t, Event{Kind: PublishFailedKind, Publish: message}, PublishFailedEvent(message, nil))
	assert.Equal(t, Event{Kind: ReceivedEventKind, Message: ReceivedMessage{Topic: "nest/topic", Payload: []byte("ON")}}, ReceivedEvent(ReceivedMessage{Topic: "nest/topic", Payload: []byte("ON")}))
}
