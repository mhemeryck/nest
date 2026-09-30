package config

import (
	"errors"
	"fmt"
	"strings"

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
			ID: string(entity.NewID(unitID, entity.TypeCover, cover.ID)), Name: cover.Name,
			OpenRelay: openRelay, CloseRelay: closeRelay,
			MovementTimeout: cover.MovementTimeout, ReversalDelay: cover.ReversalDelay,
		})
	}
	return projected, errs
}

func validateCoverConfiguration(root *Root, relays, buttons map[string]struct{}) error {
	var errs error
	ids := make([]string, 0, len(root.Covers))
	known := make(map[string]struct{}, len(root.Covers))
	owners := make(map[string]string)
	for _, light := range root.Lights {
		owners[light.Relay] = light.ID
	}
	for i, cover := range root.Covers {
		prefix := fmt.Sprintf("covers[%d]", i)
		ids = append(ids, cover.ID)
		known[cover.ID] = struct{}{}
		errs = errors.Join(errs,
			validateEntityID(prefix+".id", cover.ID, entity.TypeCover),
			validateRequiredField(prefix+".name", cover.Name),
		)
		if cover.MovementTimeout <= 0 {
			errs = errors.Join(errs, fmt.Errorf("%s.movement_timeout: must be positive", prefix))
		}
		if cover.ReversalDelay <= 0 {
			errs = errors.Join(errs, fmt.Errorf("%s.reversal_delay: must be positive", prefix))
		}
		for _, actuator := range []struct{ field, relay string }{
			{"open_relay", cover.OpenRelay}, {"close_relay", cover.CloseRelay},
		} {
			field := prefix + "." + actuator.field
			if _, ok := relays[actuator.relay]; !ok {
				errs = errors.Join(errs, fmt.Errorf("%s: unknown relay %q", field, actuator.relay))
			}
			if owner, ok := owners[actuator.relay]; ok {
				errs = errors.Join(errs, fmt.Errorf("%s: relay %q already owned by %q", field, actuator.relay, owner))
			} else {
				owners[actuator.relay] = cover.ID
			}
		}
	}
	return errors.Join(errs, validateUniqueValues("covers", "id", "id", ids), validateCoverBindings(root.Bindings, buttons, known))
}

func validateCoverBindings(bindings []BindingConfig, buttons, covers map[string]struct{}) error {
	var errs error
	seen := make(map[string]struct{})
	for i, binding := range bindings {
		if !entity.IsID(binding.Target, entity.TypeCover) {
			continue
		}
		prefix := fmt.Sprintf("bindings[%d]", i)
		errs = errors.Join(errs, validateCoverBinding(prefix, binding))
		if _, ok := buttons[binding.Source]; !ok {
			errs = errors.Join(errs, fmt.Errorf("%s.source: unknown push button %q", prefix, binding.Source))
		}
		if _, ok := covers[binding.Target]; !ok {
			errs = errors.Join(errs, fmt.Errorf("%s.target: unknown cover %q", prefix, binding.Target))
		}
		// One direction per source and cover; each binding owns both edges
		key := binding.Source + "\x00" + binding.Target
		if _, ok := seen[key]; ok {
			errs = errors.Join(errs, fmt.Errorf("%s: duplicate or conflicting cover binding", prefix))
		}
		seen[key] = struct{}{}
	}
	return errs
}

func validateCoverBinding(prefix string, binding BindingConfig) error {
	var errs error
	if binding.Action != string(entity.ActionHoldOpen) && binding.Action != string(entity.ActionHoldClose) {
		errs = errors.Join(errs, fmt.Errorf("%s.action: expected hold_open or hold_close", prefix))
	}
	if !entity.IsID(binding.Source, entity.TypeButton) || strings.Split(binding.Source, ".")[0] != strings.Split(binding.Target, ".")[0] {
		errs = errors.Join(errs, fmt.Errorf("%s.source: cover buttons must be on the same unit", prefix))
	}
	if binding.ExecutionTransport != "" {
		errs = errors.Join(errs, fmt.Errorf("%s.execution_transport: cover bindings must be local", prefix))
	}
	return errs
}
