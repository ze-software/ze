// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: sets.go -- the Sets the attribute number sets ride in
// Related: external.go -- applyExternalSets, which reaches them

package policychain

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// attributeNumberSet is one owner-registered attribute a member sets.
type attributeNumberSet struct {
	attr  policyview.AttributeNumber
	value uint32
}

// SetAttributeNumber records a set of an owner-registered attribute number.
// A later set of the same attribute replaces the earlier one.
func (s *Sets) SetAttributeNumber(attr policyview.AttributeNumber, value uint32) {
	for i := range s.numbersN {
		if s.numbers[i].attr.Code() == attr.Code() {
			s.numbers[i].value = value
			return
		}
	}
	// One slot per registered attribute: a set of a new one always fits.
	s.numbers[s.numbersN] = attributeNumberSet{attr: attr, value: value}
	s.numbersN++
}

// mergeAttributeNumbers folds a later member's attribute number sets onto s:
// the later value of an attribute replaces the earlier one.
func (s *Sets) mergeAttributeNumbers(later *Sets) {
	for i := range later.numbersN {
		s.SetAttributeNumber(later.numbers[i].attr, later.numbers[i].value)
	}
}

// applyAttributeNumbersTo writes each attribute number set as one AttrModSet
// operation on its code, through the handler the owner registered for the
// attribute (filterapi.RegisterAttrModHandler).
func (s *Sets) applyAttributeNumbersTo(acc *filterapi.ModAccumulator) {
	var value [4]byte
	for i := range s.numbersN {
		set := &s.numbers[i]
		binary.BigEndian.PutUint32(value[:], set.value)
		acc.OpCopy(uint8(set.attr.Code()), filterapi.AttrModSet, value[:])
	}
}

// overlayAttributeNumbers overlays the view with the member's attribute
// number sets, so a later member reads the set value (read-after-set).
func overlayAttributeNumbers(view *policyview.View, member *Sets) error {
	for i := range member.numbersN {
		set := &member.numbers[i]
		if err := view.SetNumber(set.attr.Reader(), uint64(set.value)); err != nil {
			return fmt.Errorf("%s: %w", set.attr.Field(), err)
		}
	}
	return nil
}

// attributeNumbersSettable answers, one bit per attribute number slot, the
// owner-registered attributes the external member plugin:reference may set:
// the attachment lets a filter set the field, and plugin is the attribute's
// owner, the one plugin whose procedure the attribute serves.
func attributeNumbersSettable(attachment, reference, plugin string) uint8 {
	var mask uint8
	for _, field := range policyview.AttributeNumberFields() {
		attr, ok := policyview.AttributeNumberNamed(field.String())
		if !ok {
			continue
		}
		if attr.Owner() != plugin {
			continue
		}
		if policy.CheckDeclaredFilter(attachment, reference, nil, []string{field.String()}) == nil {
			mask |= 1 << attr.Slot()
		}
	}
	return mask
}

// applyExternalAttributeNumber applies one reply set of an owner-registered
// attribute number: the filter declared it, its plugin owns the attribute
// and the attachment lets it set it, the action is set, and the value is a
// four-octet unsigned number.
func applyExternalAttributeNumber(sets *Sets, decl *ExternalDeclaration, settable uint8, attr policyview.AttributeNumber, set *rpc.FilterSet) error {
	if !slices.Contains(decl.Sets, set.Field) {
		return fmt.Errorf("%w: %s", errUndeclaredSet, set.Field)
	}
	if settable&(1<<attr.Slot()) == 0 {
		return fmt.Errorf("sets %s, which only a filter of %s may set at this attachment", set.Field, attr.Owner())
	}
	if set.Action != rpc.FilterSetActionSet {
		return fmt.Errorf("%s %s: the field takes set", set.Field, set.Action)
	}
	value, err := strconv.ParseUint(string(set.Value), 10, 32)
	if err != nil {
		return fmt.Errorf("%s set: the value is not an integer from 0 to 4294967295", set.Field)
	}
	sets.SetAttributeNumber(attr, uint32(value))
	return nil
}
