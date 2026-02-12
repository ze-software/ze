package attribute

import (
	"testing"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
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

// TestOpaqueAttributeWriteTo verifies WriteTo writes original data unchanged.
//
// VALIDATES: Unknown attribute value is preserved exactly.
// PREVENTS: Modification or corruption of unknown attribute data.
func TestOpaqueAttributeWriteTo(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	attr := NewOpaqueAttribute(FlagOptional|FlagTransitive, AttributeCode(200), data)

	buf := make([]byte, 64)
	n := attr.WriteTo(buf, 0)

	if n != len(data) {
		t.Fatalf("WriteTo() = %d, want %d", n, len(data))
	}
	for i := range data {
		if buf[i] != data[i] {
			t.Errorf("WriteTo()[%d] = %02x, want %02x", i, buf[i], data[i])
		}
	}
}

// TestOpaqueAttributeWriteToWithContext verifies context-independent writing.
//
// VALIDATES: Unknown attributes write the same regardless of context.
// PREVENTS: Incorrect re-encoding of unknown attribute structures.
func TestOpaqueAttributeWriteToWithContext(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	attr := NewOpaqueAttribute(FlagOptional|FlagTransitive, AttributeCode(100), data)

	// Both nil contexts - should write data unchanged
	buf := make([]byte, 64)
	n := attr.WriteToWithContext(buf, 0, nil, nil)

	if n != len(data) {
		t.Fatalf("WriteToWithContext() = %d, want %d", n, len(data))
	}
	for i := range data {
		if buf[i] != data[i] {
			t.Errorf("WriteToWithContext()[%d] = %02x, want %02x", i, buf[i], data[i])
		}
	}

	// With actual contexts - should still write data unchanged
	ctx := bgpctx.EncodingContextForASN4(true)
	buf2 := make([]byte, 64)
	n2 := attr.WriteToWithContext(buf2, 0, ctx, ctx)

	if n2 != len(data) {
		t.Fatalf("WriteToWithContext(ctx,ctx) = %d, want %d", n2, len(data))
	}
	for i := range data {
		if buf2[i] != data[i] {
			t.Errorf("WriteToWithContext(ctx,ctx)[%d] = %02x, want %02x", i, buf2[i], data[i])
		}
	}
}

// TestOpaqueAttributeWriteToContent verifies WriteTo writes correct bytes.
//
// VALIDATES: OpaqueAttribute WriteTo produces expected content.
// PREVENTS: Data corruption in zero-copy forwarding path.
func TestOpaqueAttributeWriteToContent(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	attr := NewOpaqueAttribute(FlagOptional|FlagTransitive, AttributeCode(100), data)

	buf := make([]byte, 64)
	n := attr.WriteTo(buf, 0)

	// Verify correct bytes written
	if n != 3 {
		t.Fatalf("WriteTo() = %d, want 3", n)
	}
	for i, b := range data {
		if buf[i] != b {
			t.Errorf("buf[%d] = %02x, want %02x", i, buf[i], b)
		}
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

	buf := make([]byte, 64)
	n := attr.WriteTo(buf, 0)
	if n != 0 {
		t.Errorf("WriteTo() = %d, want 0", n)
	}
}

// TestOpaqueAttributeImplementsInterface verifies Attribute interface compliance.
//
// VALIDATES: OpaqueAttribute satisfies Attribute interface.
// PREVENTS: Compile-time interface mismatch.
func TestOpaqueAttributeImplementsInterface(t *testing.T) {
	var _ Attribute = (*OpaqueAttribute)(nil)
}
