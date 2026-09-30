// Design: docs/architecture/wire/attributes.md — LARGE_COMMUNITY (Code 32)
// Overview: session_validation.go — publishBase, the ingest step that calls this
// RFC: rfc/short/rfc8092.md — BGP Large Communities

package reactor

import (
	"bytes"
	"encoding/binary"
	"hash/maphash"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// largeCommunityOctets is the size of one BGP Large Community value.
//
// RFC 8092 Section 3: "Each BGP Large Community value is encoded as a 12-octet
// quantity".
const largeCommunityOctets = 12

// largeCommunityPairScanMax is the value count up to which the redundancy scan
// compares every pair. 16 values cost at most 120 comparisons, less than
// clearing the filter the larger case uses.
const largeCommunityPairScanMax = 16

// largeCommunitySeed keys the filter hash. A per-process random seed keeps a
// peer from choosing values that all collide, which would turn the filter back
// into the quadratic scan it exists to avoid.
var largeCommunitySeed = maphash.MakeSeed()

// removeRedundantLargeCommunities returns wu unchanged when its LARGE_COMMUNITY
// attribute holds no value twice, and otherwise a new WireUpdate whose attribute
// keeps the first occurrence of each value, in the received order.
//
// It is an ingest step of publishBase: the bytes it returns are the bytes the RIB
// retains, a route server relays zero-copy and every rebuild copies from, so the
// removal reaches every consumer. The common case, no repeat, only reads the
// attribute and allocates nothing. The rebuild allocates only when a peer
// actually sent a repeat, the same trade the RFC 4271 Section 5 strip makes.
//
// Only the first LARGE_COMMUNITY attribute is examined: the RFC 7606 Section 3.g
// keep-first strip has already removed any later copy of the attribute itself.
// A value that is not a multiple of 12 octets never reaches here, because the
// validator treats it as withdrawn (RFC 8092 Section 6), and publishBase does not
// run on that path.
//
// LARGE_COMMUNITY attribute, as the peer sent it (RFC 4271 Section 4.3,
// RFC 8092 Section 3):
//
//	 0                   1                   2                   3
//	+---------------+---------------+---------------+---------------+
//	|  Attr. Flags  |Attr. Type (32)| Length (1 octet, or 2 when    |
//	+---------------+---------------+  Extended Length is set)      |
//	| value[0]: Global Administrator (4) Local Data 1 (4)           |
//	|           Local Data 2 (4)                         octets 0-11|
//	| value[1] ...                                      octets 12-23|
//	+---------------------------------------------------------------+
func removeRedundantLargeCommunities(wu *wireu.WireUpdate) *wireu.WireUpdate {
	payload := wu.Payload()
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		// publishBase reports an unparseable payload through wu.Attrs(); nothing to
		// remove from a section that cannot be located.
		return wu
	}
	attrs := sections.Attrs(payload)
	hdrStart, flags, value, found := attribute.AttrFind(attrs, attribute.AttrLargeCommunity)
	if !found {
		return wu
	}
	if len(value)%largeCommunityOctets != 0 {
		return wu
	}
	if !largeCommunityRedundant(value) {
		return wu
	}

	// RFC 8092 Section 3: "A receiving speaker MUST silently remove redundant BGP
	// Large Community values from a BGP Large Community attribute."
	rebuilt := wireu.NewWireUpdate(
		message.RebuildUpdateBody(payload, largeCommunityAttrsDeduped(attrs, hdrStart, flags, value)),
		wu.SourceCtxID())
	rebuilt.SetSourceID(wu.SourceID())
	sessionLogger().Debug("RFC 8092 Section 3: removed redundant large community values",
		"values-received", len(value)/largeCommunityOctets)
	return rebuilt
}

// largeCommunityRedundant reports whether value, a whole number of 12-octet
// Large Community values, holds any value twice.
//
// The loop count is bounded by the attribute length, itself bounded by the
// message length: 65535 octets hold at most 5461 values.
func largeCommunityRedundant(value []byte) bool {
	count := len(value) / largeCommunityOctets
	if count <= largeCommunityPairScanMax {
		for i := 1; i < count; i++ {
			current := value[i*largeCommunityOctets : (i+1)*largeCommunityOctets]
			for j := range i {
				if bytes.Equal(current, value[j*largeCommunityOctets:(j+1)*largeCommunityOctets]) {
					return true
				}
			}
		}
		return false
	}

	// A 65536-bit filter on the stack. A clear bit proves the value is new; a set
	// bit is confirmed against the earlier values, so the answer is exact.
	var seen [1024]uint64
	for i := range count {
		current := value[i*largeCommunityOctets : (i+1)*largeCommunityOctets]
		bit := uint16(maphash.Bytes(largeCommunitySeed, current))
		word, mask := bit>>6, uint64(1)<<(bit&63)
		if seen[word]&mask != 0 {
			for j := range i {
				if bytes.Equal(current, value[j*largeCommunityOctets:(j+1)*largeCommunityOctets]) {
					return true
				}
			}
		}
		seen[word] |= mask
	}
	return false
}

// largeCommunityAttrsDeduped returns a copy of the attribute section attrs in
// which the LARGE_COMMUNITY attribute at hdrStart keeps the first occurrence of
// each value in value. The attribute keeps the peer's flags and header width;
// only its length changes.
//
// Called only after largeCommunityRedundant found a repeat, so the map and the
// copy are paid by the UPDATE that carries one.
func largeCommunityAttrsDeduped(attrs []byte, hdrStart int, flags attribute.AttributeFlags, value []byte) []byte {
	headerOctets := 3
	if flags.IsExtLength() {
		headerOctets = 4
	}
	valueStart := hdrStart + headerOctets
	valueEnd := valueStart + len(value)

	count := len(value) / largeCommunityOctets
	kept := make([]byte, 0, len(value))
	seen := make(map[[largeCommunityOctets]byte]struct{}, count)
	for i := range count {
		key := [largeCommunityOctets]byte(value[i*largeCommunityOctets : (i+1)*largeCommunityOctets])
		if _, repeat := seen[key]; repeat {
			continue
		}
		seen[key] = struct{}{}
		kept = append(kept, key[:]...)
	}

	out := make([]byte, len(attrs)-(len(value)-len(kept)))
	pos := copy(out, attrs[:valueStart])
	copy(out[pos:], kept)
	copy(out[pos+len(kept):], attrs[valueEnd:])
	if flags.IsExtLength() {
		//nolint:gosec // len(kept) is at most the received length, which fit in 16 bits.
		binary.BigEndian.PutUint16(out[hdrStart+2:], uint16(len(kept)))
	} else {
		//nolint:gosec // len(kept) is at most the received length, which fit in 8 bits.
		out[hdrStart+2] = byte(len(kept))
	}
	return out
}
