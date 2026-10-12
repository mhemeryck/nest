package config

import (
	"errors"
	"fmt"

	"github.com/mhemeryck/nest/internal/entity"
)

func validateCovers(covers []CoverConfig, lights []LightConfig, knownRelayIDs map[string]struct{}) (map[string]struct{}, error) {
	knownCoverIDs := make(map[string]struct{}, len(covers))
	owners := lightRelayOwners(lights)
	var errs error
	for i, cover := range covers {
		prefix := fmt.Sprintf("covers[%d]", i)
		errs = errors.Join(errs,
			validateCoverIdentity(prefix, cover, knownCoverIDs),
			validateCoverRelay(prefix+".open_relay", cover.OpenRelay, cover.ID, knownRelayIDs, owners),
			validateCoverRelay(prefix+".close_relay", cover.CloseRelay, cover.ID, knownRelayIDs, owners),
		)
	}
	return knownCoverIDs, errs
}

func lightRelayOwners(lights []LightConfig) map[string]string {
	owners := make(map[string]string)
	for _, light := range lights {
		owners[light.Relay] = "light " + light.ID
	}
	return owners
}

func validateCoverIdentity(prefix string, cover CoverConfig, knownCoverIDs map[string]struct{}) error {
	errs := errors.Join(validateEntityID(prefix+".id", cover.ID, entity.TypeCover), validateRequiredField(prefix+".name", cover.Name))
	if _, exists := knownCoverIDs[cover.ID]; exists {
		errs = errors.Join(errs, fmt.Errorf("%s.id: duplicate cover id %q", prefix, cover.ID))
	}
	knownCoverIDs[cover.ID] = struct{}{}
	return errs
}

func validateCoverRelay(field, relayID, coverID string, knownRelayIDs map[string]struct{}, owners map[string]string) error {
	if err := validateRequiredField(field, relayID); err != nil {
		return err
	}
	return errors.Join(validateKnownCoverRelay(field, relayID, knownRelayIDs), claimCoverRelay(field, relayID, coverID, owners))
}

func validateKnownCoverRelay(field, relayID string, knownRelayIDs map[string]struct{}) error {
	if _, exists := knownRelayIDs[relayID]; !exists {
		return fmt.Errorf("%s: unknown relay %q", field, relayID)
	}
	return nil
}

func claimCoverRelay(field, relayID, coverID string, owners map[string]string) error {
	if owner, exists := owners[relayID]; exists {
		return fmt.Errorf("%s: relay %q already owned by %s", field, relayID, owner)
	}
	owners[relayID] = "cover " + coverID
	return nil
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
