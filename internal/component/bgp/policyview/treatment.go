// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: register.go -- preserve-opaque-attributes, the treatment the BGP forward path applies
// Related: metadata.go -- the route metadata fields, which an import filter sets

package policyview

import (
	"github.com/ze-software/ze/internal/component/policy"
)

// treatmentMax bounds the registered forwarding treatments, so a run's
// treatments fit one uint8 mask (Treatment.Bit). Raise it with the mask's
// width when an owner registers one past it: MustRegisterTreatment refuses
// it at init.
const treatmentMax = 8

// Treatment is one registered forwarding treatment: a flag a filter at bgp
// export sets for one destination, which the forward path applies when it
// builds that destination's UPDATE. A treatment is no route attribute and no
// metadata: nothing reads it back, it carries no value beyond being set, and
// it is per destination, where the route's metadata map is shared by every
// destination (WithExportMetadata).
type Treatment struct {
	field policy.Field
	slot  uint8
}

// Field answers the vocabulary field.
func (t Treatment) Field() policy.Field { return t.field }

// Bit answers the treatment's bit in a run's treatment mask.
func (t Treatment) Bit() uint8 { return 1 << t.slot }

// treatmentTable holds the registered treatments. It is written only from
// init() or package variable initialisation (MustRegisterTreatment) and read
// afterwards, so it needs no lock.
var treatmentTable struct {
	byName map[string]Treatment
	fields []policy.Field
}

// MustRegisterTreatment registers field as a forwarding treatment a filter
// at bgp export may set (spec-filter-wire-6-stage-filters D-12). The forward
// path that applies the treatment registers it and keeps the answer, so it
// asks the run's mask for the treatment's Bit. A second registration, or one
// past treatmentMax, is a Ze defect and panics.
func MustRegisterTreatment(field policy.Field) Treatment {
	if treatmentTable.byName == nil {
		treatmentTable.byName = make(map[string]Treatment)
	}
	if _, dup := treatmentTable.byName[field.String()]; dup {
		panic("BUG: policyview: treatment " + field.String() + " registered twice")
	}
	if len(treatmentTable.fields) == treatmentMax {
		panic("BUG: policyview: more treatments than treatmentMax; raise it for " + field.String())
	}
	t := Treatment{field: field, slot: uint8(len(treatmentTable.fields))}
	treatmentTable.byName[field.String()] = t
	treatmentTable.fields = append(treatmentTable.fields, field)
	return t
}

// TreatmentFields answers every registered treatment field, in registration
// order. The caller MUST NOT modify it.
func TreatmentFields() []policy.Field { return treatmentTable.fields }

// TreatmentNamed answers the treatment named name, and false for a name no
// owner registered as a treatment.
func TreatmentNamed(name string) (Treatment, bool) {
	t, ok := treatmentTable.byName[name]
	return t, ok
}
