// VALIDATES: AttributesWire.PackFor sizes its buffer for the header WriteHeaderTo
//            writes, when a relayed unknown attribute carries FlagExtLength over a
//            value of 255 octets or fewer.
// PREVENTS: an out-of-range write on the relay path. packWithContext (wire.go) does
//           make([]byte, total) and then writes every attribute into it. One octet
//           short per attribute is a bounds-check panic in WriteHeaderTo, and the
//           flags come from the peer, so the peer chooses whether it happens.

package attribute

import "testing"

// TestPackForSizesTheExtLengthHeaderItWritesForARelayedUnknownAttribute drives the
// re-encode path with the shape a peer can send.
//
// RFC 4271 Section 4.3 lets a sender set the Extended Length bit over any value,
// so the Attribute Length field is 2 octets and the header is 4 although the value
// is 4 octets. parseSpan keeps those flags on an OpaqueAttribute, WriteHeaderTo
// emits the 4-octet header on the flag alone, and PackFor takes the re-encode
// branch whenever the destination context differs from the source.
func TestPackForSizesTheExtLengthHeaderItWritesForARelayedUnknownAttribute(t *testing.T) {
	t.Parallel()

	sourceID := setupTestContext(true)
	targetID := setupTestContext(false)
	if sourceID == targetID {
		t.Fatalf("the two contexts must differ, or PackFor returns the packed bytes unchanged")
	}

	value := []byte{0xde, 0xad, 0xbe, 0xef}
	unknown := packAttr(FlagOptional|FlagTransitive|FlagExtLength, AttributeCode(99), value)
	if len(unknown) != 8 {
		t.Fatalf("packed unknown attribute = %d octets, want 8 (4-octet header, 4-octet value)", len(unknown))
	}
	packed := packAttrs(packAttr(FlagTransitive, AttrOrigin, []byte{0x00}), unknown)

	out, err := NewAttributesWire(packed, sourceID).PackFor(targetID)
	if err != nil {
		t.Fatalf("PackFor: %v", err)
	}

	if len(out) != len(packed) {
		t.Fatalf("re-encoded block = %d octets, want %d: neither attribute is context-dependent, so the block keeps its size", len(out), len(packed))
	}

	// Read the result back the way a peer's parser does, which is what the buffer
	// size has to serve.
	relayed, err := NewAttributesWire(out, targetID).Get(AttributeCode(99))
	if err != nil {
		t.Fatalf("re-parsing the re-encoded block: %v", err)
	}
	if relayed == nil {
		t.Fatalf("the unknown attribute did not survive the re-encode")
	}
	if !relayed.Flags().IsExtLength() {
		t.Fatalf("relayed flags = %#x, want the Extended Length bit the sender set", byte(relayed.Flags()))
	}
	if relayed.Len() != len(value) {
		t.Fatalf("relayed value = %d octets, want %d", relayed.Len(), len(value))
	}
}

// TestPackForKeepsTheZeroCopyPathForOneContext pins the branch the test above must
// avoid: with one context there is no re-encode and no allocation to get wrong.
func TestPackForKeepsTheZeroCopyPathForOneContext(t *testing.T) {
	t.Parallel()

	ctxID := setupTestContext(true)
	packed := packAttr(FlagOptional|FlagTransitive|FlagExtLength, AttributeCode(99), []byte{0x01, 0x02})

	out, err := NewAttributesWire(packed, ctxID).PackFor(ctxID)
	if err != nil {
		t.Fatalf("PackFor: %v", err)
	}
	if &out[0] != &packed[0] {
		t.Fatalf("PackFor re-encoded for its own context; it must return the packed bytes")
	}
}
