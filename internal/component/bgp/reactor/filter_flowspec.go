// Design: docs/architecture/core-design.md -- prefix policy on FlowSpec NLRIs
// RFC: rfc/short/rfc8955.md -- Sections 4 and 4.2.1
// RFC: rfc/short/rfc8956.md -- Section 3.1
package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

func isFlowSpecFamily(fam family.Family) bool {
	return (fam.AFI == family.AFIIPv4 || fam.AFI == family.AFIIPv6) &&
		(fam.SAFI == family.SAFIFlowSpec || fam.SAFI == family.SAFIFlowSpecVPN)
}

// A destination-only projection lets the existing prefix-list policy apply its
// prefix/ge/le rules without discarding the rest of an accepted FlowSpec NLRI.
// An absent or nonzero-offset destination has no CIDR equivalent and is denied.
func appendFlowSpecFilterBlock(buf []byte, fam family.Family, op string, raw []byte, addPath, empty bool) []byte {
	buf = appendMPBlock(buf, fam, op, nil, empty)
	_, err := nlrisplit.SplitFlowSpec(raw, addPath, func(one []byte) {
		if addPath {
			one = one[4:]
		}
		destination := flowSpecFilterDestination(fam, one)
		buf = append(buf, ' ')
		if !destination.IsValid() {
			buf = append(buf, "invalid"...)
			return
		}
		buf = destination.AppendTo(buf)
	})
	if err != nil {
		buf = append(buf, " invalid"...)
	}
	return buf
}

// flowSpecFilterDestination reads only the first, destination-prefix component.
// Receive validation already checks component ordering and the remaining rule;
// filter text is a projection, not a second FlowSpec parser or a unicast rewrite.
// one MUST be a complete native NLRI returned by SplitFlowSpec.
func flowSpecFilterDestination(fam family.Family, one []byte) netip.Prefix {
	off := 1
	if one[0] >= 240 {
		off = 2
	}
	if fam.SAFI == family.SAFIFlowSpecVPN {
		off += 8
	}
	if off+2 > len(one) {
		return netip.Prefix{}
	}
	if one[off] != 1 {
		return netip.Prefix{}
	}
	bits := int(one[off+1])
	off += 2
	width := 4
	if fam.AFI == family.AFIIPv6 {
		width = 16
		if off == len(one) {
			return netip.Prefix{}
		}
		if one[off] != 0 {
			return netip.Prefix{}
		}
		off++
	}
	if bits > width*8 {
		return netip.Prefix{}
	}
	n := (bits + 7) / 8
	if off+n > len(one) {
		return netip.Prefix{}
	}
	var addr [16]byte
	copy(addr[:], one[off:off+n])
	ip := netip.AddrFrom16(addr)
	if width == 4 {
		ip = netip.AddrFrom4([4]byte(addr[:4]))
	}
	return netip.PrefixFrom(ip, bits).Masked()
}
