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
	return joinTopic(topics, "units", topics.UnitID, "lights", lightTopicSegment(lightID), "state")
}

func LightCommandTopic(topics Topics, lightID entity.LightID) string {
	return joinTopic(topics, "units", topics.UnitID, "lights", lightTopicSegment(lightID), "command")
}

func LightCommandSubscriptionTopic(topics Topics) string {
	return joinTopic(topics, "units", topics.UnitID, "lights", "+", "command")
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

func ParseLightCommandTopic(topics Topics, topic string) (entity.LightID, bool) {
	prefix := joinTopic(topics, "units", topics.UnitID, "lights") + "/"
	if !strings.HasPrefix(topic, prefix) || !strings.HasSuffix(topic, "/command") {
		return "", false
	}

	localID := strings.TrimSuffix(strings.TrimPrefix(topic, prefix), "/command")
	if !entity.IsLocalID(localID) || strings.Contains(localID, "/") {
		return "", false
	}

	return entity.LightID(entity.NewID(topics.UnitID, entity.TypeLight, localID)), true
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

func lightTopicSegment(lightID entity.LightID) string {
	if localID, ok := entity.LocalID(string(lightID), entity.TypeLight); ok {
		return localID
	}

	return string(lightID)
}

func semanticSourceUnitID(sourceID entity.ID) (string, bool) {
	parts := strings.Split(string(sourceID), ".")
	if len(parts) != 3 || !entity.IsLocalID(parts[0]) {
		return "", false
	}

	return parts[0], true
}
