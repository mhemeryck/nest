package mqtt

type CommandKind string

const PublishCommandKind CommandKind = "publish"

type Command struct {
	Kind    CommandKind
	Publish PublishMessage
}

type ReceivedMessage struct {
	Topic   string
	Payload []byte
}

type EventKind string

const (
	ConnectedEventKind    EventKind = "connected"
	ConnectFailedKind     EventKind = "connect_failed"
	DisconnectedEventKind EventKind = "disconnected"
	PublishedEventKind    EventKind = "published"
	PublishFailedKind     EventKind = "publish_failed"
	ReceivedEventKind     EventKind = "received"
)

type Event struct {
	Kind    EventKind
	Publish PublishMessage
	Message ReceivedMessage
	Error   string
}

func PublishCommand(message PublishMessage) Command {
	return Command{
		Kind:    PublishCommandKind,
		Publish: message,
	}
}

func ConnectedEvent() Event {
	return Event{Kind: ConnectedEventKind}
}

func ConnectFailedEvent(err error) Event {
	event := Event{Kind: ConnectFailedKind}
	if err != nil {
		event.Error = err.Error()
	}

	return event
}

func DisconnectedEvent() Event {
	return Event{Kind: DisconnectedEventKind}
}

func PublishedEvent(message PublishMessage) Event {
	return Event{
		Kind:    PublishedEventKind,
		Publish: message,
	}
}

func PublishFailedEvent(message PublishMessage, err error) Event {
	event := Event{
		Kind:    PublishFailedKind,
		Publish: message,
	}
	if err != nil {
		event.Error = err.Error()
	}

	return event
}

func ReceivedEvent(message ReceivedMessage) Event {
	return Event{
		Kind:    ReceivedEventKind,
		Message: message,
	}
}
