// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: metadata.go -- the other field kind an owner registers from its own init
// Related: readers.go -- fixed4, the reader every attribute number shares

package policyview

import (
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// AttributeNumbersMax bounds the owner-registered attribute numbers, so a
// Sets holds one set of each in a fixed array. Raise it when an owner
// registers one past it: MustRegisterAttributeNumber refuses that one.
const AttributeNumbersMax = 4

// AttributeNumber is a path attribute whose value is one four-octet number,
// registered as a field of the BGP route view by the plugin that owns the
// attribute (otc by bgp-role, spec-filter-wire-6-stage-filters D-8), never by
// the engine. Only a filter of that plugin may set it.
type AttributeNumber struct {
	field  policy.Field
	code   attribute.AttributeCode
	owner  string
	reader Reader
	slot   uint8
}

// Field answers the attribute's field.
func (a AttributeNumber) Field() policy.Field { return a.field }

// Code answers the attribute's type code.
func (a AttributeNumber) Code() attribute.AttributeCode { return a.code }

// Owner answers the plugin that registered the attribute: the one plugin
// whose filters may set it.
func (a AttributeNumber) Owner() string { return a.owner }

// Reader answers the field's reader, for a read-after-set overlay.
func (a AttributeNumber) Reader() Reader { return a.reader }

// Slot answers the attribute's index, below AttributeNumbersMax.
func (a AttributeNumber) Slot() uint8 { return a.slot }

// attributeNumberTable holds the registered attribute numbers. It is written
// only from init() (MustRegisterAttributeNumber) and read afterwards, so it
// needs no lock.
var attributeNumberTable struct {
	numbers []AttributeNumber
	fields  []policy.Field
}

// MustRegisterAttributeNumber registers code, a path attribute whose value is
// one four-octet number, as field, owned by the plugin named owner. It
// registers the field's reader and its JSON writer (a JSON number). It MUST be
// called from init(). A second registration of a field or a code, an empty
// owner, or one past AttributeNumbersMax is a Ze defect and panics.
func MustRegisterAttributeNumber(field policy.Field, code attribute.AttributeCode, owner string) {
	if owner == "" {
		panic("BUG: policyview: attribute number " + field.String() + " has no owner")
	}
	for _, a := range attributeNumberTable.numbers {
		if a.code == code {
			panic("BUG: policyview: attribute code of " + field.String() + " registered twice")
		}
	}
	if len(attributeNumberTable.numbers) == AttributeNumbersMax {
		panic("BUG: policyview: more attribute numbers than AttributeNumbersMax; raise it for " + field.String())
	}
	// The fixed4 reader judges the length: a value that is not four octets
	// is malformed, and the member reading it fails.
	MustRegisterReader(field, func(v *View) (Value, error) { return v.fixed4(code) })
	mustRegisterWriter(field, writeNumber, 0, "")
	reader, err := ReaderFor(field)
	if err != nil {
		panic("BUG: policyview: " + err.Error())
	}
	attributeNumberTable.numbers = append(attributeNumberTable.numbers, AttributeNumber{
		field: field, code: code, owner: owner, reader: reader,
		slot: uint8(len(attributeNumberTable.numbers)), //nolint:gosec // G115: bounded by AttributeNumbersMax
	})
	attributeNumberTable.fields = append(attributeNumberTable.fields, field)
}

// AttributeNumberFields answers every registered attribute number's field, in
// registration order: the BGP attachments let a filter set them, and the
// chain lets only the owner's filters do so. The caller MUST NOT modify it.
func AttributeNumberFields() []policy.Field { return attributeNumberTable.fields }

// AttributeNumberNamed answers the attribute number whose field is named name.
func AttributeNumberNamed(name string) (AttributeNumber, bool) {
	for _, a := range attributeNumberTable.numbers {
		if a.field.String() == name {
			return a, true
		}
	}
	return AttributeNumber{}, false
}
