package mqtt

import (
	"fmt"
	"strings"

	"github.com/mhemeryck/nest/internal/entity"
)

func AvailabilityTopic(cfg Config) string {
	return fmt.Sprintf("%s/availability", cfg.UnitID)
}

func UnitDiscoveryTopic(cfg Config) string {
	return fmt.Sprintf("%s/discovery", cfg.UnitID)
}

func SysfsStateTopic(cfg Config, deviceID entity.DeviceID) string {
	return fmt.Sprintf("%s/sysfs/%s/state", cfg.UnitID, deviceID)
}

func InputStateTopic(cfg Config, deviceID entity.DeviceID) string {
	return fmt.Sprintf("%s/input/%s/state", cfg.UnitID, DeviceTopicSlot(deviceID))
}

func InputEventTopic(cfg Config, deviceID entity.DeviceID) string {
	return fmt.Sprintf("%s/input/%s/event", cfg.UnitID, DeviceTopicSlot(deviceID))
}

func InputCommandTopic(cfg Config, deviceID entity.DeviceID) string {
	return fmt.Sprintf("%s/input/%s/set", cfg.UnitID, DeviceTopicSlot(deviceID))
}

func PushButtonEventTopic(cfg Config, buttonID entity.PushButtonID) string {
	return fmt.Sprintf("%s/push_button/%s/event", cfg.UnitID, buttonID)
}

func RelayStateTopic(cfg Config, deviceID entity.DeviceID) string {
	return fmt.Sprintf("%s/relay/%s/state", cfg.UnitID, DeviceTopicSlot(deviceID))
}

func RelayCommandTopic(cfg Config, deviceID entity.DeviceID) string {
	return fmt.Sprintf("%s/relay/%s/set", cfg.UnitID, DeviceTopicSlot(deviceID))
}

func HASwitchDiscoveryTopic(cfg Config, buttonID entity.PushButtonID) string {
	prefix := DefaultDiscoveryPrefix(cfg.DiscoveryPrefix)
	return fmt.Sprintf("%s/switch/%s_%s/config", prefix, cfg.UnitID, buttonID)
}

func HALightDiscoveryTopic(cfg Config, lightID entity.LightID) string {
	prefix := DefaultDiscoveryPrefix(cfg.DiscoveryPrefix)
	return fmt.Sprintf("%s/light/%s_%s/config", prefix, cfg.UnitID, lightID)
}

func DeviceTopicSlot(deviceID entity.DeviceID) string {
	value := string(deviceID)
	if slot, ok := strings.CutPrefix(value, "di_"); ok {
		return slot
	}
	if slot, ok := strings.CutPrefix(value, "ro_"); ok {
		return slot
	}

	return value
}
