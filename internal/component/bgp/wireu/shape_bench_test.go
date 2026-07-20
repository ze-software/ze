package wireu

import "testing"

// BenchmarkMixesNLRIFields measures the per-destination cost of the RFC 7606 Section 5.1
// shape check in the forward loop: one received UPDATE, checked once per destination peer.
func BenchmarkMixesNLRIFields(b *testing.B) {
	// A realistic relayed announce: ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, COMMUNITY.
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x0a, 0x02, 0x02, 0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0xfd, 0xe9,
		0x40, 0x03, 0x04, 192, 0, 2, 1,
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x00,
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
		0xc0, 0x08, 0x08, 0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0xfd, 0xe9,
	}
	payload := buildTestUpdatePayload(nil, attrs, []byte{0x18, 0xC0, 0x00, 0x02})

	b.Run("PerDestination", func(b *testing.B) {
		wu := NewWireUpdate(payload, 0)
		wu.MixesNLRIFields() // first destination pays the walk; the rest are the measured ones
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if wu.MixesNLRIFields() {
				b.Fatal("fixture must be compliant")
			}
		}
	})
}
