// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: view.go -- the view the readers fill
// Related: register.go -- the registrations

package policyview

import (
	"errors"
	"fmt"
	"math/bits"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// slotsMax bounds the registered readers, so a View's slot table is a fixed
// array that Reset invalidates without clearing. Raise it when a BGP field
// beyond it registers: MustRegisterReader refuses the one past the bound.
const slotsMax = 32

// ErrNoReader is answered by ReaderFor for a field no BGP reader registered.
var ErrNoReader = errors.New("no BGP policy view reader is registered for the field")

// readFunc decodes one field from the view's UPDATE.
type readFunc func(v *View) (Value, error)

// readerTable holds the registered readers. It is written only from init()
// (MustRegisterReader) and read afterwards, so it needs no lock.
var readerTable struct {
	slots map[policy.Field]int
	reads []readFunc
}

// MustRegisterReader registers how the BGP view reads field. It MUST be called
// from init(): a reader registered after a chain compiled would be invisible
// to it. A second reader for one field, or one past slotsMax, is a Ze defect
// and panics.
func MustRegisterReader(field policy.Field, read func(v *View) (Value, error)) {
	if readerTable.slots == nil {
		readerTable.slots = make(map[policy.Field]int)
	}
	if _, dup := readerTable.slots[field]; dup {
		panic("BUG: policyview: field " + field.String() + " has two readers")
	}
	if len(readerTable.reads) == slotsMax {
		panic("BUG: policyview: more readers than slotsMax; raise slotsMax for field " + field.String())
	}
	readerTable.slots[field] = len(readerTable.reads)
	readerTable.reads = append(readerTable.reads, read)
}

// ReaderFor answers the reader of field, resolved once when a chain compiles.
// A field no BGP reader registered answers ErrNoReader: a BGP attachment MUST
// NOT carry a field the view cannot read.
func ReaderFor(field policy.Field) (Reader, error) {
	slot, ok := readerTable.slots[field]
	if !ok {
		return Reader{}, fmt.Errorf("%w: %s", ErrNoReader, field)
	}
	return Reader{slot: slot + 1, field: field}, nil
}

// Decodes is what one run decoded: the NLRI sections, and each attribute whose
// value bytes a reader took. A field read twice in a run decodes once, so a
// count above the number of distinct attributes would be a Ze defect.
type Decodes struct {
	nlri       int
	attributes int
	codes      [4]uint64 // one bit per attribute code
}

// NLRI answers how many times the run decoded the NLRI sections.
func (d Decodes) NLRI() int { return d.nlri }

// Attributes answers how many attribute decodes the run made.
func (d Decodes) Attributes() int { return d.attributes }

// Decoded reports whether the run decoded the attribute code.
func (d Decodes) Decoded(code attribute.AttributeCode) bool {
	return d.codes[code>>6]&(1<<(code&63)) != 0
}

// Distinct answers how many distinct attributes the run decoded.
func (d Decodes) Distinct() int {
	return bits.OnesCount64(d.codes[0]) + bits.OnesCount64(d.codes[1]) +
		bits.OnesCount64(d.codes[2]) + bits.OnesCount64(d.codes[3])
}

func (d *Decodes) countAttribute(code attribute.AttributeCode) {
	d.attributes++
	d.codes[code>>6] |= 1 << (code & 63)
}

func (d *Decodes) countNLRI() { d.nlri++ }
