package config

import (
	"errors"
	"fmt"

	"github.com/mhemeryck/nest/internal/entity"
)

func projectCovers(unitID string, covers []UnitCoverConfig) ([]CoverConfig, error) {
	projected := make([]CoverConfig, 0, len(covers))
	var errs error
	for i, cover := range covers {
		prefix := fmt.Sprintf("entities.covers[%d]", i)
		openRelay, openErr := projectEndpointRef(prefix+".open_actuator", cover.OpenActuator, endpointActorSysfs, endpointKindSysfsRelay)
		closeRelay, closeErr := projectEndpointRef(prefix+".close_actuator", cover.CloseActuator, endpointActorSysfs, endpointKindSysfsRelay)
		errs = errors.Join(errs, openErr, closeErr)
		projected = append(projected, CoverConfig{
			ID:         string(entity.NewID(unitID, entity.TypeCover, cover.ID)),
			Name:       cover.Name,
			OpenRelay:  openRelay,
			CloseRelay: closeRelay,
		})
	}
	return projected, errs
}

func validateCovers(covers []CoverConfig, knownRelayIDs map[string]struct{}) (map[string]struct{}, error) {
	var errs error
	ids := make([]string, 0, len(covers))
	knownCoverIDs := make(map[string]struct{}, len(covers))
	for i, cover := range covers {
		prefix := fmt.Sprintf("covers[%d]", i)
		errs = errors.Join(
			errs,
			validateEntityID(prefix+".id", cover.ID, entity.TypeCover),
			validateRequiredField(prefix+".name", cover.Name),
			validateCoverRelay(prefix+".open_relay", cover.OpenRelay, knownRelayIDs),
			validateCoverRelay(prefix+".close_relay", cover.CloseRelay, knownRelayIDs),
		)
		ids = append(ids, cover.ID)
		knownCoverIDs[cover.ID] = struct{}{}
	}
	return knownCoverIDs, errors.Join(errs, validateUniqueValues("covers", "id", "id", ids))
}

func validateCoverRelay(field, relayID string, knownRelayIDs map[string]struct{}) error {
	if err := validateRequiredField(field, relayID); err != nil {
		return err
	}
	if _, ok := knownRelayIDs[relayID]; !ok {
		return fmt.Errorf("%s: unknown relay %q", field, relayID)
	}
	return nil
}

// Cover ownership uses physical devices, including any differently named aliases.
// Existing light-to-light relay sharing remains unchanged.
func validateCoverRelayOwnership(covers []CoverConfig, lights []LightConfig, relays []RelayConfig) error {
	devicesByRelay := make(map[string]string, len(relays))
	for _, relay := range relays {
		devicesByRelay[relay.ID] = relay.Device
	}
	owners := make(map[string]string)
	for i, light := range lights {
		if device := devicesByRelay[light.Relay]; device != "" {
			owners[device] = fmt.Sprintf("lights[%d].relay", i)
		}
	}

	var errs error
	for i, cover := range covers {
		for _, ref := range []struct{ field, relay string }{
			{fmt.Sprintf("covers[%d].open_relay", i), cover.OpenRelay},
			{fmt.Sprintf("covers[%d].close_relay", i), cover.CloseRelay},
		} {
			device := devicesByRelay[ref.relay]
			if device == "" {
				continue // Missing references are reported by entity validation.
			}
			if owner, exists := owners[device]; exists {
				errs = errors.Join(errs, fmt.Errorf("%s: relay device %q already used by %s", ref.field, device, owner))
				continue
			}
			owners[device] = ref.field
		}
	}
	return errs
}

func validateCoverControl(settings CoverControlConfig, hasCovers bool) error {
	if settings.MaxRun < 0 || (hasCovers && settings.MaxRun == 0) {
		return fmt.Errorf("cover_control.max_run: must be positive")
	}
	return nil
}
