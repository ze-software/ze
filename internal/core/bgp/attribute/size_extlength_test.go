// VALIDATES: the attribute sizers agree with WriteHeaderTo about the header size
//            class, for an attribute whose flags already carry FlagExtLength over
//            a value of 255 octets or fewer.
// PREVENTS: an under-allocated attribute buffer. packAttributesWithContext
//           (internal/component/bgp/rib/route.go) sizes with
//           AttributesSizeWithContext. It then does make([]byte, totalSize) and
//           writes into that buffer. One byte short is an out-of-range panic,
//           on any route relayed from a peer that sets the bit.

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// extLengthShortValue is the shape a length-only test misses. RFC 4271
// Section 4.3 lets a sender set the Extended Length bit over any value. The
// Attribute Length field is then 2 octets and the header is 4. A test on the
// value length alone concludes 3.
//
// OpaqueAttribute keeps a peer's flags unchanged (opaque.go, Flags). This
// shape therefore arrives off the wire.
func extLengthShortValue() Attribute {
	return NewOpaqueAttribute(FlagOptional|FlagTransitive|FlagExtLength, AttributeCode(99), []byte{0xde, 0xad, 0xbe, 0xef})
}

func TestAttrWireLenCountsTheExtLengthFlagNotOnlyTheValueLength(t *testing.T) {
	attr := extLengthShortValue()

	// What the writer actually emits, measured rather than assumed.
	buf := make([]byte, 64)
	written := WriteAttrTo(attr, buf, 0)

	assert.Equal(t, 8, written,
		"4-octet extended header plus a 4-octet value: WriteHeaderTo returns 4 whenever the flags carry FlagExtLength")
	assert.Equal(t, written, AttrWireLen(attr),
		"AttrWireLen must agree with what WriteAttrTo writes")
}

func TestAttributesSizeMatchesWhatIsWrittenForAnExtLengthShortValue(t *testing.T) {
	attrs := []Attribute{extLengthShortValue()}

	size := AttributesSize(attrs)

	buf := make([]byte, 64)
	off := 0
	for _, a := range attrs {
		off += WriteAttrTo(a, buf, off)
	}

	assert.Equal(t, off, size,
		"AttributesSize is the allocation the caller makes. Short by one byte is an out-of-range panic, not a truncation")
}

func TestAttributesSizeWithContextMatchesWhatIsWrittenForAnExtLengthShortValue(t *testing.T) {
	attrs := []Attribute{extLengthShortValue()}

	// A nil destination context is the no-translation case. A relayed unknown
	// attribute takes it: ValueLenWithContext falls through to the attribute's
	// own length.
	size := AttributesSizeWithContext(attrs, nil)

	buf := make([]byte, 64)
	off := 0
	for _, a := range attrs {
		off += WriteAttrToWithContext(a, buf, off, nil, nil)
	}

	assert.Equal(t, off, size,
		"AttributesSizeWithContext sizes the buffer packAttributesWithContext allocates in internal/component/bgp/rib/route.go")
}
