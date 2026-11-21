package mqtt

import (
	"slices"
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

func digitalInputTopic(topics Topics, inputID entity.DigitalInputID, suffix topicSuffix) string {
	return joinTopic(topics, "units", topics.UnitID, "digital_inputs", string(inputID), string(suffix))
}

func pushButtonTopic(topics Topics, buttonID entity.PushButtonID, suffix topicSuffix) string {
	return joinTopic(topics, "units", topics.UnitID, "push_buttons", string(buttonID), string(suffix))
}

func relayTopic(topics Topics, relayID entity.RelayID, suffix topicSuffix) string {
	return joinTopic(topics, "units", topics.UnitID, "relays", string(relayID), string(suffix))
}

func lightTopic(topics Topics, lightID entity.LightID, suffix topicSuffix) string {
	localID, ok := entity.LocalID(string(lightID), entity.TypeLight)
	if !ok {
		localID = string(lightID)
	}
	return joinTopic(topics, "units", topics.UnitID, "lights", localID, string(suffix))
}

func CoverCommandSubscriptionTopic(topics Topics) string {
	return joinTopic(topics, "units", topics.UnitID, "covers", "+", string(commandSuffix))
}

func ParseEntityCommandTopic(topics Topics, topic string) (entity.Type, entity.ID) {
	prefix := joinTopic(topics, "units", topics.UnitID) + "/"
	path, ok := strings.CutPrefix(topic, prefix)
	if !ok {
		return "", ""
	}
	path, ok = strings.CutSuffix(path, "/"+string(commandSuffix))
	if !ok {
		return "", ""
	}
	group, localID, ok := strings.Cut(path, "/")
	if !ok || !entity.IsLocalID(localID) {
		return "", ""
	}

	var kind entity.Type
	switch group {
	case "lights":
		kind = entity.TypeLight
	case "covers":
		kind = entity.TypeCover
	default:
		return "", ""
	}
	return kind, entity.NewID(topics.UnitID, kind, localID)
}

func LightCommandSubscriptionTopic(topics Topics) string {
	return joinTopic(topics, "units", topics.UnitID, "lights", "+", string(commandSuffix))
}

func SemanticSourceEventTopic(topics Topics, sourceID entity.ID) string {
	unitID := topics.UnitID
	if sourceUnitID, ok := semanticSourceUnitID(sourceID); ok {
		unitID = sourceUnitID
	}

	return joinTopic(topics, "units", unitID, "sources", string(sourceID), "event")
}

func SemanticSourceEventSubscriptionTopics(topics Topics, bindings []entity.Binding) []string {
	seen := make(map[string]struct{}, len(bindings))
	subscriptions := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		topic := SemanticSourceEventTopic(topics, binding.Source)
		if _, ok := seen[topic]; ok {
			continue
		}

		seen[topic] = struct{}{}
		subscriptions = append(subscriptions, topic)
	}
	slices.Sort(subscriptions)

	return subscriptions
}

func HomeAssistantDeviceDiscoveryTopic(topics Topics) string {
	return strings.Join([]string{"homeassistant", "device", homeAssistantUniqueID(topics, "unit"), "config"}, "/")
}

func joinTopic(topics Topics, parts ...string) string {
	segments := make([]string, 0, len(parts)+1)
	if topics.Prefix != "" {
		segments = append(segments, topics.Prefix)
	}
	segments = append(segments, parts...)

	return strings.Join(segments, "/")
}

func homeAssistantUniqueID(topics Topics, entityID string) string {
	return strings.Join([]string{"nest", topics.UnitID, entityID}, "_")
}

func semanticSourceUnitID(sourceID entity.ID) (string, bool) {
	parts := strings.Split(string(sourceID), ".")
	if len(parts) != 3 || !entity.IsLocalID(parts[0]) {
		return "", false
	}

	return parts[0], true
}

type topicSuffix string

const (
	commandSuffix  topicSuffix = "command"
	stateSuffix    topicSuffix = "state"
	positionSuffix topicSuffix = "position"
)

func coverTopic(topics Topics, id entity.CoverID, suffix topicSuffix) string {
	localID, ok := entity.LocalID(string(id), entity.TypeCover)
	if !ok {
		localID = string(id)
	}
	return joinTopic(topics, "units", topics.UnitID, "covers", localID, string(suffix))
}
