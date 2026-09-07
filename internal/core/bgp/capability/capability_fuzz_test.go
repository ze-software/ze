// Design: docs/architecture/wire/capabilities.md -- the OPEN capability decode surface
//
// The two targets here fuzz the OPEN message's capability decode, which no
// target reached before. Parse takes the Capabilities Optional Parameter value,
// and ParseFromOptionalParams takes the whole optional-parameter block.
//
// ParseFromOptionalParams carries a negotiated dimension the fuzzer varies:
// extended selects the RFC 9072 Section 2 framing, whose Parameter Length field
// is two octets rather than one. The same bytes read two ways is exactly the
// shape a single-polarity target cannot explore, so extended is a fuzz argument
// and both polarities are seeded.

package capability

import (
	"testing"
)

// wireCapMultiprotocol is a Multiprotocol capability for IPv4 unicast.
// RFC 4760 Section 8: code 1, length 4, AFI(2) + Reserved(1) + SAFI(1).
var wireCapMultiprotocol = []byte{0x01, 0x04, 0x00, 0x01, 0x00, 0x01}

// wireCapASN4 is a four-octet AS capability for AS 65536.
// RFC 6793 Section 3: code 65, length 4, the four-octet AS number.
var wireCapASN4 = []byte{0x41, 0x04, 0x00, 0x01, 0x00, 0x00}

// wireCapRouteRefresh is the zero-length ROUTE-REFRESH capability.
// RFC 2918 Section 2: code 2, length 0.
var wireCapRouteRefresh = []byte{0x02, 0x00}

// wireCapAddPath advertises send-and-receive ADD-PATH for IPv4 unicast.
// RFC 7911 Section 4: code 69, length 4, AFI(2) + SAFI(1) + Send/Receive(1).
var wireCapAddPath = []byte{0x45, 0x04, 0x00, 0x01, 0x01, 0x03}

// wireCapGracefulRestart advertises graceful restart with a 120 second timer.
// RFC 4724 Section 3: code 64, length 2, Restart Flags and Restart Time.
var wireCapGracefulRestart = []byte{0x40, 0x02, 0x00, 0x78}

// capabilitySeeds are the capability TLV blocks both targets seed from.
// Each one is a shape a peer sends in an OPEN, so a decode defect in any of
// them is reachable from the socket.
var capabilitySeeds = [][]byte{
	wireCapMultiprotocol,
	wireCapASN4,
	wireCapRouteRefresh,
	wireCapAddPath,
	wireCapGracefulRestart,
	// Several capabilities in one Capabilities Optional Parameter, which is
	// what a real OPEN carries.
	concatBytes(wireCapMultiprotocol, wireCapASN4, wireCapRouteRefresh, wireCapAddPath),
	// RFC 5492 Section 4: a speaker MUST accept two instances of one code.
	concatBytes(wireCapMultiprotocol, wireCapMultiprotocol),
	{},                             // Empty block.
	{0x01},                         // Code with no length octet.
	{0x01, 0x04},                   // Length 4 with no value.
	{0x01, 0xFF, 0x00},             // Length far beyond the data.
	{0x41, 0x02, 0x00, 0x01},       // ASN4 with a two-octet value.
	{0x45, 0x03, 0x00, 0x01, 0x01}, // ADD-PATH with a partial entry.
	{0x02, 0x01, 0x00},             // Zero-length capability given a value.
	{0xFF, 0x02, 0xAB, 0xCD},       // Unknown code, which is ignored.
	{0x01, 0x04, 0x00, 0x01, 0x00, 0x01, 0x01}, // Trailing octet after a valid TLV.
}

// concatBytes joins wire fragments into one block.
func concatBytes(parts ...[]byte) []byte {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	out := make([]byte, 0, total)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// checkCapabilityEncode writes every parsed capability back to the wire and
// fails when a capability's Len disagrees with what its WriteTo produces.
//
// Len sizes the OPEN buffer the encoder writes into, so a capability that
// reports fewer octets than it writes overruns that buffer, and one that reports
// more leaves the octets after it uninitialized. Decoding from fuzzed bytes is
// what puts field values no constructor would build into the encoder's hands,
// which is why the check belongs on this side rather than in an encode test.
func checkCapabilityEncode(t *testing.T, caps []Capability) {
	t.Helper()
	for _, capability := range caps {
		size := capability.Len()
		if size < 0 {
			t.Fatalf("capability code %d reported a negative length %d", capability.Code(), size)
		}
		buf := make([]byte, size)
		written := capability.WriteTo(buf, 0)
		if written != size {
			t.Fatalf("capability code %d: Len() = %d but WriteTo wrote %d",
				capability.Code(), size, written)
		}
	}
}

// FuzzParseCapabilities tests the Capabilities Optional Parameter decode.
//
// VALIDATES: Parse handles arbitrary bytes without crashing, and every
// capability it returns writes back exactly the octet count it reports.
// PREVENTS: Remote crash or buffer overrun via a malformed OPEN capability TLV.
// SECURITY: Critical - these bytes arrive in the first message of an unauthenticated
// session, before any state exists to reject the peer on.
func FuzzParseCapabilities(f *testing.F) {
	for _, seed := range capabilitySeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Parse MUST NOT panic on any input.
		caps, err := Parse(data)
		if err != nil {
			// A rejected block is the expected answer for malformed bytes.
			return
		}
		checkCapabilityEncode(t, caps)
	})
}

// FuzzParseFromOptionalParams tests the OPEN optional-parameter decode under
// both parameter framings.
//
// VALIDATES: ParseFromOptionalParams handles arbitrary bytes without crashing
// under the one-octet and the two-octet Parameter Length, and every capability
// it returns writes back exactly the octet count it reports.
// PREVENTS: Remote crash or buffer overrun via a malformed OPEN optional-parameter block.
// SECURITY: Critical - the block comes from an unauthenticated peer's OPEN.
//
// extended is a fuzz argument rather than a fixed value because the framing is a
// negotiated property of the OPEN, not of the bytes: RFC 9072 Section 2 widens
// the Parameter Length field to two octets, so one block decodes two ways and a
// target holding extended fixed explores half of the input domain.
func FuzzParseFromOptionalParams(f *testing.F) {
	for _, seed := range capabilitySeeds {
		// A Capabilities Optional Parameter (type 2) wrapping the TLV block, in
		// each framing, plus the bare block under both so the fuzzer starts from
		// misframed input as well.
		f.Add(optionalParam(seed, false), false)
		f.Add(optionalParam(seed, true), true)
		f.Add(seed, false)
		f.Add(seed, true)
	}
	// A parameter whose declared length exceeds the block.
	f.Add([]byte{0x02, 0xFF, 0x01, 0x04}, false)
	f.Add([]byte{0x02, 0xFF, 0xFF, 0x01, 0x04}, true)
	// A non-capability parameter type, which is skipped by its length.
	f.Add([]byte{0x01, 0x02, 0xAA, 0xBB}, false)
	// A two-octet header read as a three-octet one, and the reverse.
	f.Add([]byte{0x02, 0x00}, true)
	f.Add([]byte{0x02, 0x00, 0x00}, false)

	f.Fuzz(func(t *testing.T, data []byte, extended bool) {
		// ParseFromOptionalParams MUST NOT panic on any input under either framing.
		caps, err := ParseFromOptionalParams(data, extended)
		if err != nil {
			// A rejected block is the expected answer for malformed bytes.
			return
		}
		checkCapabilityEncode(t, caps)
	})
}

// optionalParam wraps a capability TLV block in a Capabilities Optional
// Parameter, in the framing extended selects.
//
// RFC 5492 Section 4 gives the Parameter Length one octet. RFC 9072 Section 2
// gives it two. A block longer than the field can state is truncated to what it
// can state, because a seed must be a parameter the decoder can frame.
func optionalParam(block []byte, extended bool) []byte {
	if extended {
		length := min(len(block), 0xFFFF)
		out := []byte{2, byte(length >> 8), byte(length)}
		return append(out, block[:length]...)
	}
	length := min(len(block), 0xFF)
	out := []byte{2, byte(length)}
	return append(out, block[:length]...)
}
