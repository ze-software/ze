// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: sets.go -- the Sets the metadata sets ride in
// Related: external.go -- applyExternalSets, which reaches them

package policychain

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// metadataSetsMax bounds the metadata sets one member answers for one route:
// one per registered metadata field fits it.
const metadataSetsMax = 8

// errTooManyMetadataSets is answered past metadataSetsMax.
var errTooManyMetadataSets = errors.New("a filter may set at most 8 metadata fields per route")

// metadataSet is one route metadata key a member sets, the value already of
// the key's registered kind (a string, a uint8 or a bool).
type metadataSet struct {
	field policyview.Metadata
	value any
}

// SetMetadata records a set of a route metadata field. value MUST be of the
// field's kind. A later set of the same field replaces the earlier one.
func (s *Sets) SetMetadata(field policyview.Metadata, value any) error {
	for i := range s.metaN {
		if s.meta[i].field.Field() == field.Field() {
			s.meta[i].value = value
			return nil
		}
	}
	if int(s.metaN) == metadataSetsMax {
		return errTooManyMetadataSets
	}
	s.meta[s.metaN] = metadataSet{field: field, value: value}
	s.metaN++
	return nil
}

// metadataWritable refuses a member's metadata sets on a view whose route
// has no metadata map (an export rail, a dry run), before any of the
// member's sets fold, so a refusal leaves none of them applied.
func metadataWritable(view *policyview.View, member *Sets) error {
	if member.metaN == 0 || view.MetadataWritable() {
		return nil
	}
	return fmt.Errorf("%s: %w", member.meta[0].field.Field(), policyview.ErrNoMetadataMap)
}

// writeMetadata writes the member's metadata sets into the route's metadata
// map, so a later member of the pass reads them and the received event
// carries them (spec-filter-wire-6-stage-filters D-8). metadataWritable
// MUST have passed for the member first.
func writeMetadata(view *policyview.View, member *Sets) {
	for i := range member.metaN {
		set := &member.meta[i]
		//nolint:errcheck // metadataWritable proved the map exists, SetMetadata's only refusal
		view.SetMetadata(set.field, set.value)
	}
}

// metadataSettable answers, one bit per metadata field slot, the metadata
// fields the attachment lets the external member reference set.
func metadataSettable(attachment, reference string) uint8 {
	var mask uint8
	for _, field := range policyview.MetadataFields() {
		m, ok := policyview.MetadataNamed(field.String())
		if !ok {
			continue
		}
		if policy.CheckDeclaredFilter(attachment, reference, nil, []string{field.String()}) == nil {
			mask |= 1 << m.Slot()
		}
	}
	return mask
}

// applyExternalMetadata applies one reply set of a metadata field: the
// filter declared it, the attachment lets it set it, the action is set, and
// the value is of the field's kind.
func applyExternalMetadata(sets *Sets, decl *ExternalDeclaration, settable uint8, m policyview.Metadata, set *rpc.FilterSet) error {
	if !slices.Contains(decl.Sets, set.Field) {
		return fmt.Errorf("%w: %s", errUndeclaredSet, set.Field)
	}
	if settable&(1<<m.Slot()) == 0 {
		return fmt.Errorf("sets %s, which a filter at this attachment may not set", set.Field)
	}
	if set.Action != rpc.FilterSetActionSet {
		return fmt.Errorf("%s %s: a metadata field takes set", set.Field, set.Action)
	}
	value, err := metadataValue(m.Kind(), set.Value)
	if err != nil {
		return fmt.Errorf("%s set: %w", set.Field, err)
	}
	return sets.SetMetadata(m, value)
}

// metadataValue reads a set's JSON value as kind: a plain string, an integer
// from 0 to 255, or JSON true or false.
func metadataValue(kind policyview.MetadataKind, value []byte) (any, error) {
	switch kind {
	case policyview.MetadataText:
		if text, ok := plainString(value); ok {
			return text, nil
		}
		return nil, errors.New("the value is not a plain JSON string")
	case policyview.MetadataOctet:
		octet, err := strconv.ParseUint(string(value), 10, 8)
		if err != nil {
			return nil, errors.New("the value is not an integer from 0 to 255")
		}
		return uint8(octet), nil
	case policyview.MetadataFlag:
		switch string(value) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return nil, errors.New("the value is not true or false")
	case policyview.MetadataUnspecified:
	}
	return nil, fmt.Errorf("BUG: policychain: metadata kind %d has no value form", kind)
}
