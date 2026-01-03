package pool

import (
	"bytes"
	"testing"
)

// TestAttributesPoolExists verifies the global Attributes pool is initialized.
//
// VALIDATES: Global Attributes pool is ready to use.
// PREVENTS: Nil pointer panic on first use.
func TestAttributesPoolExists(t *testing.T) {
	if Attributes == nil {
		t.Fatal("Attributes pool should be initialized")
	}
}

// TestAttributesPoolBasicOps verifies the global pool works correctly.
//
// VALIDATES: Intern/Get work on global Attributes pool.
// PREVENTS: Misconfigured global pool.
func TestAttributesPoolBasicOps(t *testing.T) {
	// Simulate typical BGP attribute data
	attrData := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE8, // AS_PATH
	}

	h := Attributes.Intern(attrData)
	if h == InvalidHandle {
		t.Fatal("Intern returned InvalidHandle")
	}

	got := Attributes.Get(h)
	if !bytes.Equal(got, attrData) {
		t.Errorf("Get() = %x, want %x", got, attrData)
	}

	// Clean up
	Attributes.Release(h)
}

// TestAttributesPoolDedup verifies deduplication works on global pool.
//
// VALIDATES: Same attributes share storage.
// PREVENTS: Memory waste from duplicate route attributes.
func TestAttributesPoolDedup(t *testing.T) {
	attrData := []byte{0x40, 0x01, 0x01, 0x02} // ORIGIN INCOMPLETE

	h1 := Attributes.Intern(attrData)
	h2 := Attributes.Intern(attrData)

	if h1 != h2 {
		t.Errorf("Same data should return same handle: %#x vs %#x", h1, h2)
	}

	// Clean up both references
	Attributes.Release(h1)
	Attributes.Release(h2)
}
