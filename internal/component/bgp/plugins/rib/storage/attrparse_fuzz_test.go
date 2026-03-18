package storage

import (
	"testing"
)

// FuzzParseAttributes tests attribute parsing robustness.
//
// VALIDATES: Parser handles arbitrary bytes without crashing.
// PREVENTS: Remote crash via malformed UPDATE, buffer overflow, panics.
// SECURITY: Critical - attributes come from untrusted BGP peers.
func FuzzParseAttributes(f *testing.F) {
	// Seed with valid attribute formats.
	f.Add(wireOriginIGP)                           // Valid ORIGIN.
	f.Add(wireASPath65001)                         // Valid AS_PATH.
	f.Add(wireNextHop)                             // Valid NEXT_HOP.
	f.Add(wireMED100)                              // Valid MED.
	f.Add(wireLocalPref100)                        // Valid LOCAL_PREF.
	f.Add(wireCommunity)                           // Valid COMMUNITIES.
	f.Add(wireUnknown)                             // Unknown attribute.
	f.Add(concat(wireOriginIGP, wireLocalPref100)) // Multiple attrs.
	f.Add([]byte{})                                // Empty.
	f.Add([]byte{0x40, 0x01})                      // Truncated header.
	f.Add([]byte{0x40, 0x01, 0xFF})                // Length exceeds data.
	f.Add([]byte{0x50, 0x01, 0x00, 0x01, 0x00})    // Extended length.
	f.Add([]byte{0x40, 0x01, 0x01, 0x00, 0x40})    // Truncated second attr.

	f.Fuzz(func(t *testing.T, data []byte) {
		// ParseAttributes MUST NOT panic on any input.
		entry, err := ParseAttributes(data)
		if err != nil {
			// Errors are acceptable.
			return
		}
		// If successful, entry must be valid and releasable.
		if entry == nil {
			t.Fatal("ParseAttributes returned nil entry without error")
			return
		}
		entry.Release()
	})
}
