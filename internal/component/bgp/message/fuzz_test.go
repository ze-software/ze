package message

import (
	"testing"
)

// FuzzParseHeader tests BGP message header parsing robustness.
//
// VALIDATES: ParseHeader handles arbitrary bytes without crashing.
// PREVENTS: Panic on malformed marker, truncated input, or invalid length.
// SECURITY: Header is the first thing parsed from untrusted TCP data.
func FuzzParseHeader(f *testing.F) {
	// Valid header: 16x 0xFF marker + length(29) + type(OPEN=1)
	valid := make([]byte, 19)
	for i := range 16 {
		valid[i] = 0xFF
	}
	valid[16] = 0x00
	valid[17] = 0x1D // length = 29
	valid[18] = 0x01 // OPEN

	f.Add(valid)
	f.Add([]byte{})                             // Empty
	f.Add([]byte{0xFF})                         // Truncated marker
	f.Add(make([]byte, 18))                     // One byte short
	f.Add(make([]byte, 19))                     // All zeros (bad marker)
	f.Add(append(valid[:16], 0, 18, 1))         // Length < 19
	f.Add(append(valid[:16], 0xFF, 0xFF, 0x02)) // Max length UPDATE
	f.Add(append(valid[:16], 0x10, 0x00, 0x06)) // Length 4096, unknown type 6

	f.Fuzz(func(t *testing.T, data []byte) {
		h, err := ParseHeader(data)
		if err != nil {
			return
		}
		// If parsing succeeded, exercise validation paths — MUST NOT panic
		_ = h.ValidateLength()
		_ = h.ValidateLengthWithMax(false)
		_ = h.ValidateLengthWithMax(true)
		_ = h.Type.String()
	})
}

// FuzzUnpackOpen tests OPEN message parsing robustness.
//
// VALIDATES: UnpackOpen handles arbitrary bytes without crashing.
// PREVENTS: Panic on malformed capabilities, truncated optional params, RFC 9072 extended format.
// SECURITY: OPEN is parsed from untrusted peer data during session setup.
func FuzzUnpackOpen(f *testing.F) {
	// Valid minimal OPEN body: version(4) + AS(65001) + holdtime(90) + BGPID(1.1.1.1) + optlen(0)
	valid := []byte{
		0x04,       // Version 4
		0xFD, 0xE9, // AS 65001
		0x00, 0x5A, // Hold time 90
		0x01, 0x01, 0x01, 0x01, // BGP ID 1.1.1.1
		0x00, // Opt param len 0
	}
	f.Add(valid)
	f.Add([]byte{})     // Empty
	f.Add([]byte{0x04}) // Truncated after version
	f.Add(valid[:9])    // Missing opt param length
	f.Add(valid[:5])    // Truncated after hold time

	// OPEN with optional parameters
	withOpts := []byte{
		0x04, 0xFD, 0xE9, 0x00, 0x5A, 0x01, 0x01, 0x01, 0x01,
		0x04,                   // Opt param len = 4
		0x02, 0x02, 0x06, 0x41, // Capability TLV
	}
	f.Add(withOpts)

	// Opt param length claims more data than available
	truncOpts := []byte{
		0x04, 0xFD, 0xE9, 0x00, 0x5A, 0x01, 0x01, 0x01, 0x01,
		0x20, // Opt param len = 32, but no data follows
	}
	f.Add(truncOpts)

	// RFC 9072 extended format trigger
	extended := []byte{
		0x04, 0xFD, 0xE9, 0x00, 0x5A, 0x01, 0x01, 0x01, 0x01,
		0xFF,       // Non-ext marker
		0xFF,       // Non-ext type marker
		0x00, 0x04, // Extended opt len = 4
		0x02, 0x02, 0x06, 0x41,
	}
	f.Add(extended)

	// Extended format with truncated extended length
	extTrunc := []byte{
		0x04, 0xFD, 0xE9, 0x00, 0x5A, 0x01, 0x01, 0x01, 0x01,
		0xFF, 0xFF, // Markers present but no length follows
	}
	f.Add(extTrunc)

	f.Fuzz(func(t *testing.T, data []byte) {
		o, err := UnpackOpen(data)
		if err != nil {
			return
		}
		// Exercise all methods — MUST NOT panic
		_ = o.Type()
		_ = o.Len(nil)
		_ = o.RouterID()
		_ = o.ValidateHoldTime()
		_ = o.String()
	})
}

// FuzzUnpackUpdate tests UPDATE message parsing robustness.
//
// VALIDATES: UnpackUpdate handles arbitrary bytes without crashing.
// PREVENTS: Panic on malformed withdrawn/attribute lengths, truncated NLRI.
// SECURITY: UPDATE is the most complex BGP message, parsed from untrusted peers.
func FuzzUnpackUpdate(f *testing.F) {
	// Minimal valid UPDATE body: withdrawn_len(0) + attr_len(0)
	minimal := []byte{0x00, 0x00, 0x00, 0x00}
	f.Add(minimal)
	f.Add([]byte{})     // Empty
	f.Add([]byte{0x00}) // Truncated

	// UPDATE with withdrawn routes
	withWithdrawn := []byte{
		0x00, 0x03, // Withdrawn len = 3
		24, 10, 0, // 10.0.0.0/24 (withdrawn)
		0x00, 0x00, // Attr len = 0
	}
	f.Add(withWithdrawn)

	// Withdrawn length exceeds data
	badWithdrawn := []byte{0x00, 0xFF, 0x00, 0x00}
	f.Add(badWithdrawn)

	// UPDATE with path attributes and NLRI
	withNLRI := []byte{
		0x00, 0x00, // Withdrawn len = 0
		0x00, 0x07, // Attr len = 7
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x00, // Empty AS_PATH
		24, 10, 0, 0, // NLRI: 10.0.0.0/24
	}
	f.Add(withNLRI)

	f.Fuzz(func(t *testing.T, data []byte) {
		u, err := UnpackUpdate(data)
		if err != nil {
			return
		}
		// Exercise all methods — MUST NOT panic
		_ = u.Type()
		_ = u.Len(nil)
		_ = u.IsEndOfRIB()
		_ = u.RawData()
	})
}

// FuzzUnpackNotification tests NOTIFICATION message parsing robustness.
//
// VALIDATES: UnpackNotification handles arbitrary bytes without crashing.
// PREVENTS: Panic on malformed error data, invalid UTF-8 shutdown messages.
// SECURITY: NOTIFICATION is parsed from untrusted peer data.
func FuzzUnpackNotification(f *testing.F) {
	// Valid: Cease / Admin Shutdown with message
	ceaseMsg := []byte{
		0x06, 0x02, // Cease / Admin Shutdown
		0x05,                    // Message length = 5
		'h', 'e', 'l', 'l', 'o', // UTF-8 message
	}
	f.Add(ceaseMsg)
	f.Add([]byte{})                                // Empty
	f.Add([]byte{0x01})                            // Only error code
	f.Add([]byte{0x01, 0x01})                      // Minimal: code + subcode
	f.Add([]byte{0x06, 0x02})                      // Cease/shutdown, no data
	f.Add([]byte{0x06, 0x02, 0x00})                // Cease/shutdown, zero-length message
	f.Add([]byte{0x06, 0x02, 0xFF})                // Cease/shutdown, length exceeds data
	f.Add([]byte{0x03, 0x01, 0xDE, 0xAD})          // UPDATE error with data
	f.Add([]byte{0x06, 0x02, 0x02, 0xFE, 0xFF})    // Invalid UTF-8 in shutdown
	f.Add([]byte{0x06, 0x04, 0x03, 'b', 'y', 'e'}) // Admin Reset

	f.Fuzz(func(t *testing.T, data []byte) {
		n, err := UnpackNotification(data)
		if err != nil {
			return
		}
		// Exercise all methods — MUST NOT panic
		_ = n.Type()
		_ = n.Len(nil)
		_ = n.String()
		_ = n.Error()
		//nolint:errcheck // fuzz: testing panic safety, not error values
		n.ShutdownMessage()
	})
}

// FuzzUnpackRouteRefresh tests ROUTE-REFRESH message parsing robustness.
//
// VALIDATES: UnpackRouteRefresh handles arbitrary bytes without crashing.
// PREVENTS: Panic on invalid AFI/SAFI combinations.
// SECURITY: ROUTE-REFRESH is parsed from untrusted peer data.
func FuzzUnpackRouteRefresh(f *testing.F) {
	// Valid: IPv4 unicast normal refresh
	valid := []byte{0x00, 0x01, 0x00, 0x01} // AFI=1 SAFI=1
	f.Add(valid)
	f.Add([]byte{})                             // Empty
	f.Add([]byte{0x00})                         // Truncated
	f.Add([]byte{0x00, 0x01, 0x00})             // 3 bytes (short)
	f.Add([]byte{0x00, 0x02, 0x00, 0x01})       // IPv6 unicast
	f.Add([]byte{0x00, 0x01, 0x01, 0x01})       // BoRR
	f.Add([]byte{0x00, 0x01, 0x02, 0x01})       // EoRR
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})       // Max values
	f.Add([]byte{0x00, 0x01, 0x00, 0x01, 0xDE}) // Extra trailing bytes

	f.Fuzz(func(t *testing.T, data []byte) {
		r, err := UnpackRouteRefresh(data)
		if err != nil {
			return
		}
		// Exercise all methods — MUST NOT panic
		_ = r.Type()
		_ = r.Len(nil)
	})
}

// FuzzChunkNLRI tests NLRI chunking robustness.
//
// VALIDATES: ChunkNLRI handles arbitrary NLRI bytes without crashing.
// PREVENTS: Panic on malformed prefix lengths, zero maxSize.
// SECURITY: NLRI data originates from untrusted UPDATE messages.
func FuzzChunkNLRI(f *testing.F) {
	f.Add([]byte{24, 10, 0, 0, 16, 172, 16}, 100) // Two valid prefixes
	f.Add([]byte{24, 10, 0, 0}, 2)                // maxSize smaller than prefix
	f.Add([]byte{}, 100)                          // Empty
	f.Add([]byte{32, 10, 0, 0, 1}, 5)             // Exact fit
	f.Add([]byte{0xFF}, 10)                       // Invalid prefix length 255

	f.Fuzz(func(t *testing.T, nlri []byte, maxSize int) {
		if maxSize < 1 {
			maxSize = 1 // Avoid zero/negative
		}
		_ = ChunkNLRI(nlri, maxSize) // MUST NOT panic
	})
}
