// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: dataplane.go -- PortMatch, the value-and-mask pair this file decomposes into
// RFC: rfc/short/rfc4301.md -- the ICMP type and code selector (Section 4.4.1.1)

package dataplane

// IP protocol numbers of ICMP and ICMPv6 (IANA "Protocol Numbers" registry). For these
// two, an SPParams port pair carries the ICMP message type and code rather than ports:
// the Linux kernel reads the type into the selector's source port and the code into its
// destination port (include/net/xfrm.h, xfrm_flowi_sport and xfrm_flowi_dport).
const (
	UpperProtoICMP   uint8 = 1
	UpperProtoICMPv6 uint8 = 58
)

// IsICMPProto reports whether an upper-layer protocol number is ICMP or ICMPv6, the two
// protocols whose port pair carries a message type and code.
func IsICMPProto(proto uint8) bool {
	return proto == UpperProtoICMP || proto == UpperProtoICMPv6
}

// MaskedBlocks decomposes the inclusive range start..end into the fewest PortMatch
// blocks whose union is EXACTLY that range. Each block is a value whose low bits are
// zero and a mask that leaves those bits free, which is the only range shape a kernel
// selector (value plus mask) can express.
//
// RFC 4301 Section 4.4.1.1: "Given a policy entry with a range of Types (T-start to
// T-end) and a range of Codes (C-start to C-end), and an ICMP packet with Type t and
// Code c, an implementation MUST test for a match using". A single type with a range of
// codes is one selector to RFC 4301 and several to the kernel: one policy per block,
// all installed and removed together. The decomposition is exact, never a rounding: a
// value outside the range matches no block, and every value inside matches one.
//
// An empty range (start above end) yields no block. A caller that reaches it skipped
// the validation that refuses one.
func MaskedBlocks(start, end uint16) []PortMatch {
	var out []PortMatch
	for v := uint32(start); v <= uint32(end); {
		// The largest aligned block starting at v: bounded by v's lowest set bit (a
		// block of size 2^k must start on a multiple of 2^k), then halved until it
		// stays inside the range.
		size := uint32(1) << 16
		if v != 0 {
			size = v & -v
		}
		for v+size-1 > uint32(end) {
			size >>= 1
		}
		out = append(out, PortMatch{Port: uint16(v), Mask: uint16(^(size - 1))})
		v += size
	}
	return out
}
