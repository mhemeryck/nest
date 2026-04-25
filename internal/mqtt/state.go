package mqtt

type StateKind string

const (
	AvailabilityState StateKind = "availability"
	DiscoveryState    StateKind = "discovery"
	SysfsState        StateKind = "sysfs"
	InputState        StateKind = "input"
	PushButtonState   StateKind = "push_button"
	RelayState        StateKind = "relay"
)

type State struct {
	Kind    StateKind
	Topic   string
	Payload []byte
	Retain  bool
}

type Command struct{}

type Config struct {
	Enabled         bool
	Broker          string
	UnitID          string
	ClientID        string
	Username        string
	Password        string
	DiscoveryPrefix string
}

func DefaultDiscoveryPrefix(prefix string) string {
	if prefix == "" {
		return "homeassistant"
	}

	return prefix
}
