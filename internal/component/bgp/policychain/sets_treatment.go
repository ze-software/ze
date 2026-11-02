// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: sets.go -- the Sets the treatments ride in
// Related: external.go -- applyExternalSets, which reaches them

package policychain

import (
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SetTreatment records that the member asks the forward path to apply t to
// this destination's UPDATE (policyview.Treatment).
func (s *Sets) SetTreatment(t policyview.Treatment) { s.treatments |= t.Bit() }

// Treatments answers the mask of the treatments the sets ask for, one
// policyview.Treatment.Bit each. A treatment changes no attribute, so Empty
// does not count it: a member that sets only a treatment asks no body change,
// and a later member's whole-body rewrite stays allowed.
func (s *Sets) Treatments() uint8 { return s.treatments }

// mergeTreatments folds a later member's treatments onto s: a treatment one
// member set stays set for the route.
func (s *Sets) mergeTreatments(later *Sets) { s.treatments |= later.treatments }

// treatmentsSettable answers, one bit per treatment, the treatments the
// attachment lets the external member reference set: only bgp export lets a
// filter set one (bgp/config exportSettable).
func treatmentsSettable(attachment, reference string) uint8 {
	var mask uint8
	for _, field := range policyview.TreatmentFields() {
		t, ok := policyview.TreatmentNamed(field.String())
		if !ok {
			continue
		}
		if policy.CheckDeclaredFilter(attachment, reference, nil, []string{field.String()}) == nil {
			mask |= t.Bit()
		}
	}
	return mask
}

// applyExternalTreatment applies one reply set of a treatment: the filter
// declared it, the attachment lets it set it, the action is set, and the
// value is JSON true, the one value a treatment takes.
func applyExternalTreatment(sets *Sets, decl *ExternalDeclaration, settable uint8, t policyview.Treatment, set *rpc.FilterSet) error {
	if !slices.Contains(decl.Sets, set.Field) {
		return fmt.Errorf("%w: %s", errUndeclaredSet, set.Field)
	}
	if settable&t.Bit() == 0 {
		return fmt.Errorf("sets %s, which a filter at this attachment may not set", set.Field)
	}
	if set.Action != rpc.FilterSetActionSet {
		return fmt.Errorf("%s %s: a treatment takes set", set.Field, set.Action)
	}
	if string(set.Value) != "true" {
		return fmt.Errorf("%s set %s: a treatment takes the value true", set.Field, set.Value)
	}
	sets.SetTreatment(t)
	return nil
}
