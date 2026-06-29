package mqtt

import (
	"errors"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrokerURL(t *testing.T) {
	url := brokerURL(entity.MQTT{Host: "mqtt.local", Port: 1883})

	assert.Equal(t, "tcp://mqtt.local:1883", url)
}

func TestClientOptions(t *testing.T) {
	events := make(chan Event, 1)
	options := clientOptions(t.Context(), entity.MQTT{
		Host:        "mqtt.local",
		Port:        1883,
		ClientID:    "nest-controller-1",
		Username:    "nest",
		Password:    "secret",
		TopicPrefix: "nest",
		UnitID:      "controller_1",
	}, NewTopics("nest", "controller_1"), nil, events)

	assert.Equal(t, "nest-controller-1", options.ClientID)
	assert.Equal(t, "nest", options.Username)
	assert.Equal(t, "secret", options.Password)
	assert.True(t, options.WillEnabled)
	assert.Equal(t, "nest/units/controller_1/availability", options.WillTopic)
	assert.Equal(t, []byte("offline"), options.WillPayload)
	assert.True(t, options.WillRetained)
	assert.Equal(t, byte(0), options.WillQos)
	if assert.Len(t, options.Servers, 1) {
		assert.Equal(t, "tcp://mqtt.local:1883", options.Servers[0].String())
	}

	options.OnConnect(nil)
	assert.Equal(t, ConnectedEvent(), <-events)
}

func TestSubscribeLightCommands(t *testing.T) {
	events := make(chan Event, 1)
	client := &fakeClient{token: fakeToken{}}

	err := subscribeLightCommands(t.Context(), client, NewTopics("nest", "controller_1"), events)
	require.NoError(t, err)
	assert.Equal(t, "nest/units/controller_1/lights/+/command", client.subscribedTopic)

	message := &fakeMessage{topic: "nest/units/controller_1/lights/office_light/command", payload: []byte("ON")}
	client.handler(nil, message)
	assert.Equal(t, ReceivedEvent(ReceivedMessage{Topic: message.topic, Payload: message.payload}), <-events)
}

func TestSubscribeLightCommandsReturnsSubscribeError(t *testing.T) {
	err := subscribeLightCommands(t.Context(), &fakeClient{token: fakeToken{err: errors.New("subscribe failed")}}, NewTopics("nest", "controller_1"), make(chan Event, 1))
	require.EqualError(t, err, "subscribe failed")
}

func TestSubscribeLightCommandsIgnoresRetainedMessages(t *testing.T) {
	events := make(chan Event, 1)
	client := &fakeClient{token: fakeToken{}}

	err := subscribeLightCommands(t.Context(), client, NewTopics("nest", "controller_1"), events)
	require.NoError(t, err)

	client.handler(nil, &fakeMessage{topic: "nest/units/controller_1/lights/office_light/command", payload: []byte("ON"), retained: true})

	assert.Empty(t, events)
}

func TestSubscribeSourceEvents(t *testing.T) {
	events := make(chan Event, 1)
	client := &fakeClient{token: fakeToken{}, handlers: make(map[string]paho.MessageHandler)}

	err := subscribeSourceEvents(t.Context(), client, []string{"nest/units/controller_2/sources/controller_2.button.hall/event"}, events)
	require.NoError(t, err)
	assert.Equal(t, []string{"nest/units/controller_2/sources/controller_2.button.hall/event"}, client.subscribedTopics)

	message := &fakeMessage{topic: "nest/units/controller_2/sources/controller_2.button.hall/event", payload: []byte(`{"source":"controller_2.button.hall","event":"pressed"}`)}
	client.handlers[message.topic](nil, message)
	assert.Equal(t, ReceivedEvent(ReceivedMessage{Topic: message.topic, Payload: message.payload}), <-events)
}

type fakeClient struct {
	token            fakeToken
	subscribedTopic  string
	subscribedTopics []string
	handler          paho.MessageHandler
	handlers         map[string]paho.MessageHandler
}

func (f *fakeClient) Publish(string, byte, bool, interface{}) paho.Token {
	return f.token
}

func (f *fakeClient) Subscribe(topic string, _ byte, callback paho.MessageHandler) paho.Token {
	f.subscribedTopic = topic
	f.subscribedTopics = append(f.subscribedTopics, topic)
	f.handler = callback
	if f.handlers != nil {
		f.handlers[topic] = callback
	}
	return f.token
}

type fakeToken struct {
	err error
}

func (f fakeToken) Wait() bool                       { return true }
func (f fakeToken) WaitTimeout(_ time.Duration) bool { return true }
func (f fakeToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (f fakeToken) Error() error { return f.err }

type fakeMessage struct {
	topic    string
	payload  []byte
	retained bool
}

func (f *fakeMessage) Duplicate() bool   { return false }
func (f *fakeMessage) Qos() byte         { return 0 }
func (f *fakeMessage) Retained() bool    { return f.retained }
func (f *fakeMessage) Topic() string     { return f.topic }
func (f *fakeMessage) MessageID() uint16 { return 0 }
func (f *fakeMessage) Payload() []byte   { return f.payload }
func (f *fakeMessage) Ack()              {}
