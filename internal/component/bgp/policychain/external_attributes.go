// Design: docs/plugin-development/protocol.md -- The Sets of a filter-update answer
// Related: external.go -- applyExternalSet, which routes each field here
// Related: sets_attributes.go -- the Sets methods these call

package policychain

import (
	"errors"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// applyNumericSet writes a set, increment or decrement of a numeric field:
// local-preference, med and aigp, each a JSON number of four octets.
func applyNumericSet(sets *Sets, kind setKind, set *rpc.FilterSet) error {
	action, ok := numericActionOf(set.Action)
	if !ok {
		return errors.New("the field takes set, increment or decrement")
	}
	value, err := strconv.ParseUint(string(set.Value), 10, 32)
	if err != nil {
		return errors.New("the value is not a number from 0 to 4294967295")
	}
	switch kind {
	case setKindMED:
		return sets.MED(action, uint32(value))
	case setKindAIGP:
		return sets.AIGP(action, uint32(value))
	case setKindLocalPreference:
		return sets.LocalPreference(action, uint32(value))
	case setKindUnspecified, setKindCommunity, setKindASPath, setKindPrefix, setKindFamily, setKindOrigin,
		setKindNextHop, setKindLargeCommunity, setKindExtendedCommunity, setKindCount:
	}
	return errors.New("BUG: policychain: applyNumericSet called for a field that is not numeric")
}

// removeMED writes the configured MULTI_EXIT_DISC removal, which carries no
// value and is honored on an import chain only.
//
// RFC 4271 Section 5.1.4: "If a BGP speaker is configured to remove the
// MULTI_EXIT_DISC attribute from a route, then this removal MUST be done prior
// to determining the degree of preference of the route and prior to performing
// route selection (Decision Process phases 1 and 2)."
func removeMED(sets *Sets, target intentTarget) error {
	if !target.importSide {
		return errMEDRemoveOnExport
	}
	sets.RemoveMED()
	return nil
}

// applyAttributeSet writes an origin or next-hop replacement, or a large or
// extended community add or remove. Each value is a JSON string in the form
// the UPDATE event's attr map writes.
func applyAttributeSet(sets *Sets, kind setKind, set *rpc.FilterSet) error {
	text, ok := plainString(set.Value)
	if !ok {
		return errors.New("the value is not a JSON string")
	}
	switch kind {
	case setKindOrigin:
		return setOrigin(sets, set.Action, text)
	case setKindNextHop:
		return setNextHop(sets, set.Action, text)
	case setKindLargeCommunity:
		community, err := attribute.ParseLargeCommunity(text)
		if err != nil {
			return err
		}
		return listAction(set.Action, community, sets.AddLargeCommunity, sets.RemoveLargeCommunity)
	case setKindExtendedCommunity:
		community, err := attribute.ParseSingleExtCommunity(text)
		if err != nil {
			return err
		}
		return listAction(set.Action, community, sets.AddExtendedCommunity, sets.RemoveExtendedCommunity)
	case setKindUnspecified, setKindLocalPreference, setKindMED, setKindCommunity, setKindASPath, setKindPrefix,
		setKindFamily, setKindAIGP, setKindCount:
	}
	return errors.New("BUG: policychain: applyAttributeSet called for a field it does not write")
}

// setOrigin writes "igp", "egp" or "incomplete" (RFC 4271 Section 5.1.1).
func setOrigin(sets *Sets, action rpc.FilterSetAction, text string) error {
	if action != rpc.FilterSetActionSet {
		return errors.New("the field takes set")
	}
	origin, ok := attribute.OriginFromText(text)
	if !ok {
		return errors.New("the value is not igp, egp or incomplete")
	}
	return sets.Origin(origin)
}

// setNextHop writes an IPv4 NEXT_HOP (RFC 4271 Section 5.1.3).
func setNextHop(sets *Sets, action rpc.FilterSetAction, text string) error {
	if action != rpc.FilterSetActionSet {
		return errors.New("the field takes set")
	}
	addr, err := netip.ParseAddr(text)
	if err != nil {
		return errors.New("the value is not an IPv4 address")
	}
	return sets.NextHop(addr)
}

// listAction writes one add or remove of a list attribute's value.
func listAction[T any](action rpc.FilterSetAction, value T, add, remove func(T) error) error {
	switch action {
	case rpc.FilterSetActionAdd:
		return add(value)
	case rpc.FilterSetActionRemove:
		return remove(value)
	case rpc.FilterSetActionSet, rpc.FilterSetActionIncrement, rpc.FilterSetActionDecrement,
		rpc.FilterSetActionPrepend:
	}
	return errors.New("the field takes add or remove")
}
