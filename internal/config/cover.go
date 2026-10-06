package config

import (
	"errors"
	"fmt"

	"github.com/mhemeryck/nest/internal/entity"
)

func validateCovers(covers []CoverConfig, lights []LightConfig, knownRelayIDs map[string]struct{}) (map[string]struct{}, error) {
	knownCoverIDs := make(map[string]struct{}, len(covers))
	owners := make(map[string]string)
	for _, light := range lights {
		owners[light.Relay] = "light " + light.ID
	}
	var errs error
	for i, cover := range covers {
		prefix := fmt.Sprintf("covers[%d]", i)
		errs = errors.Join(errs, validateEntityID(prefix+".id", cover.ID, entity.TypeCover), validateRequiredField(prefix+".name", cover.Name))
		if _, exists := knownCoverIDs[cover.ID]; exists {
			errs = errors.Join(errs, fmt.Errorf("%s.id: duplicate cover id %q", prefix, cover.ID))
		}
		knownCoverIDs[cover.ID] = struct{}{}
		for _, relay := range []struct{ field, id string }{{"open_relay", cover.OpenRelay}, {"close_relay", cover.CloseRelay}} {
			field := prefix + "." + relay.field
			if err := validateRequiredField(field, relay.id); err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			if _, exists := knownRelayIDs[relay.id]; !exists {
				errs = errors.Join(errs, fmt.Errorf("%s: unknown relay %q", field, relay.id))
			}
			if owner, exists := owners[relay.id]; exists {
				errs = errors.Join(errs, fmt.Errorf("%s: relay %q already owned by %s", field, relay.id, owner))
			} else {
				owners[relay.id] = "cover " + cover.ID
			}
		}
	}
	return knownCoverIDs, errs
}

func bindingTargetType(target string) entity.Type {
	if entity.IsID(target, entity.TypeCover) {
		return entity.TypeCover
	}
	return entity.TypeLight
}

func validateBindingAction(field string, action string, targetType entity.Type) error {
	if targetType != entity.TypeCover {
		return validateModbusBindingAction(field, action)
	}
	if err := validateRequiredField(field, action); err != nil {
		return err
	}
	switch action {
	case BindingActionOpen, BindingActionClose, BindingActionStop:
		return nil
	default:
		return fmt.Errorf("%s: unsupported cover action %q", field, action)
	}
}
