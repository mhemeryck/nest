package mqtt

import (
	"strings"

	"github.com/mhemeryck/nest/internal/entity"
)

type Topics struct {
	Prefix string
	UnitID string
}

func NewTopics(prefix string, unitID string) Topics {
	return Topics{
		Prefix: strings.Trim(prefix, "/"),
		UnitID: unitID,
	}
}

func AvailabilityTopic(topics Topics) string {
	return joinTopic(topics, "units", topics.UnitID, "availability")
}

func DiscoveryTopic(topics Topics) string {
	return joinTopic(topics, "units", topics.UnitID, "discovery")
}

func DigitalInputStateTopic(topics Topics, inputID entity.DigitalInputID) string {
	return joinTopic(topics, "units", topics.UnitID, "digital_inputs", string(inputID), "state")
}

func PushButtonStateTopic(topics Topics, buttonID entity.PushButtonID) string {
	return joinTopic(topics, "units", topics.UnitID, "push_buttons", string(buttonID), "state")
}

func RelayStateTopic(topics Topics, relayID entity.RelayID) string {
	return joinTopic(topics, "units", topics.UnitID, "relays", string(relayID), "state")
}

func LightStateTopic(topics Topics, lightID entity.LightID) string {
	return joinTopic(topics, "units", topics.UnitID, "lights", string(lightID), "state")
}

func LightCommandTopic(topics Topics, lightID entity.LightID) string {
	return joinTopic(topics, "units", topics.UnitID, "lights", string(lightID), "command")
}

func joinTopic(topics Topics, parts ...string) string {
	segments := make([]string, 0, len(parts)+1)
	if topics.Prefix != "" {
		segments = append(segments, topics.Prefix)
	}
	segments = append(segments, parts...)

	return strings.Join(segments, "/")
}
