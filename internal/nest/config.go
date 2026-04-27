package nest

import (
	"github.com/mhemeryck/nest/internal/entity"
	nestmqtt "github.com/mhemeryck/nest/internal/mqtt"
)

func mqttConfigFromEntity(cfg entity.MQTT) nestmqtt.Config {
	return nestmqtt.Config{
		Enabled:         cfg.Enabled,
		Broker:          cfg.Broker,
		UnitID:          cfg.UnitID,
		ClientID:        cfg.ClientID,
		Username:        cfg.Username,
		Password:        cfg.Password,
		DiscoveryPrefix: cfg.DiscoveryPrefix,
	}
}
