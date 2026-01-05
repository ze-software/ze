package attribute

import (
	"testing"

	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
)

// TestOpaqueAttributeBasic verifies OpaqueAttribute creation and accessors.
//
// VALIDATES: OpaqueAttribute stores flags, code, and data correctly.
// PREVENTS: Data corruption or flag loss for unknown attributes.
func TestOpaqueAttributeBasic(t *testing.T) {
	flags := FlagOptional | FlagTransitive
	code := AttributeCode(99) // Unknown attribute code
	data := []byte{0x01, 0x02, 0x03, 0x04}

	attr := NewOpaqueAttribute(flags, code, data)

	if attr.Code() != code {
		t.Errorf("Code() = %d, want %d", attr.Code(), code)
	}
	if attr.Flags() != flags {
		t.Errorf("Flags() = %02x, want %02x", attr.Flags(), flags)
	}
	if attr.Len() != len(data) {
		t.Errorf("Len() = %d, want %d", attr.Len(), len(data))
	}
}

// TestOpaqueAttributePack verifies Pack returns original data unchanged.
//
// VALIDATES: Unknown attribute value is preserved exactly.
// PREVENTS: Modification or corruption of unknown attribute data.
func TestOpaqueAttributePack(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	attr := NewOpaqueAttribute(FlagOptional|FlagTransitive, AttributeCode(200), data)

	packed := attr.Pack()

	if len(packed) != len(data) {
		t.Fatalf("Pack() len = %d, want %d", len(packed), len(data))
	}
	for i := range data {
		if packed[i] != data[i] {
			t.Errorf("Pack()[%d] = %02x, want %02x", i, packed[i], data[i])
		}
	}
}

// TestOpaqueAttributePackWithContext verifies context-independent packing.
//
// VALIDATES: Unknown attributes pack the same regardless of context.
// PREVENTS: Incorrect re-encoding of unknown attribute structures.
func TestOpaqueAttributePackWithContext(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	attr := NewOpaqueAttribute(FlagOptional|FlagTransitive, AttributeCode(100), data)

	// Both nil contexts - should return data unchanged
	packed := attr.PackWithContext(nil, nil)

	if len(packed) != len(data) {
		t.Fatalf("PackWithContext() len = %d, want %d", len(packed), len(data))
	}
	for i := range data {
		if packed[i] != data[i] {
			t.Errorf("PackWithContext()[%d] = %02x, want %02x", i, packed[i], data[i])
		}
	}

	// With actual contexts - should still return data unchanged
	ctx := &bgpctx.EncodingContext{ASN4: true}
	packed2 := attr.PackWithContext(ctx, ctx)

	if len(packed2) != len(data) {
		t.Fatalf("PackWithContext(ctx,ctx) len = %d, want %d", len(packed2), len(data))
	}
	for i := range data {
		if packed2[i] != data[i] {
			t.Errorf("PackWithContext(ctx,ctx)[%d] = %02x, want %02x", i, packed2[i], data[i])
		}
	}
}

// TestOpaqueAttributeNoBorrowedDataModification verifies data is not copied.
//
// VALIDATES: OpaqueAttribute borrows data (zero-copy).
// PREVENTS: Unnecessary allocations for unknown attributes.
func TestOpaqueAttributeNoBorrowedDataModification(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	attr := NewOpaqueAttribute(FlagOptional|FlagTransitive, AttributeCode(100), data)

	packed := attr.Pack()

	// Verify it's the same underlying slice (zero-copy)
	if &packed[0] != &data[0] {
		t.Error("Pack() returned a copy, expected borrowed slice")
	}
}

// TestOpaqueAttributePreservesPartialFlag verifies Partial bit preservation.
//
// VALIDATES: Partial flag is preserved for forwarding.
// PREVENTS: RFC 4271 violation - Partial bit must be preserved for transitive unknown attributes.
func TestOpaqueAttributePreservesPartialFlag(t *testing.T) {
	// RFC 4271: Partial bit indicates incomplete propagation
	flags := FlagOptional | FlagTransitive | FlagPartial
	attr := NewOpaqueAttribute(flags, AttributeCode(150), []byte{0x01})

	if !attr.Flags().IsPartial() {
		t.Error("Partial flag not preserved")
	}
	if attr.Flags() != flags {
		t.Errorf("Flags() = %02x, want %02x", attr.Flags(), flags)
	}
}

// TestOpaqueAttributeEmptyData verifies handling of zero-length attributes.
//
// VALIDATES: Empty data is valid for opaque attributes.
// PREVENTS: Nil pointer issues with zero-length unknown attributes.
func TestOpaqueAttributeEmptyData(t *testing.T) {
	attr := NewOpaqueAttribute(FlagOptional|FlagTransitive, AttributeCode(50), nil)

	if attr.Len() != 0 {
		t.Errorf("Len() = %d, want 0", attr.Len())
	}

	packed := attr.Pack()
	if len(packed) != 0 {
		t.Errorf("Pack() = %v, want nil or empty", packed)
	}
}

// TestOpaqueAttributeImplementsInterface verifies Attribute interface compliance.
//
// VALIDATES: OpaqueAttribute satisfies Attribute interface.
// PREVENTS: Compile-time interface mismatch.
func TestOpaqueAttributeImplementsInterface(t *testing.T) {
	var _ Attribute = (*OpaqueAttribute)(nil)
}
