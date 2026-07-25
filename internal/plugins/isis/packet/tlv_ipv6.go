// Design: plan/learned/928-isis-2-wire.md -- TLV 232 (IPv6 Interface Address), TLV 236 (IPv6 Reachability)
// RFC: rfc/short/rfc5308.md -- TLV 232 (sec 3), TLV 236 entry layout (sec 2): metric, flags, prefixlen, packed prefix, sub-TLVs

package packet

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// IPv6AddrLen is the length of one IPv6 address (TLV 232 entry).
const IPv6AddrLen = 16

// MaxIPv6PrefixLen is the largest IPv6 prefix length.
const MaxIPv6PrefixLen = 128

// ---- TLV 232: IPv6 Interface Address (RFC 5308 sec 3) ----
//
// Value is a flat list of 16-octet IPv6 addresses. In Hellos these are the
// link-local addresses; in LSPs the non-link-local addresses (RFC 5308 sec 3).
// The codec does not enforce that policy (it is isis-5/isis-12's concern); it
// round-trips the 16-octet list.

// IPv6InterfaceAddrTLV is the decoded TLV 232.
type IPv6InterfaceAddrTLV struct {
	Addresses []netip.Addr // each a 16-octet IPv6 address
}

// DecodeIPv6InterfaceAddrTLV parses a TLV 232 value (one 16-octet address per
// entry). A value length that is not a multiple of 16 is rejected (ErrLength).
func DecodeIPv6InterfaceAddrTLV(value []byte) (IPv6InterfaceAddrTLV, error) {
	if len(value)%IPv6AddrLen != 0 {
		return IPv6InterfaceAddrTLV{}, ErrLength
	}
	n := len(value) / IPv6AddrLen
	out := IPv6InterfaceAddrTLV{Addresses: make([]netip.Addr, 0, n)}
	for off := 0; off < len(value); off += IPv6AddrLen {
		var a16 [IPv6AddrLen]byte
		copy(a16[:], value[off:off+IPv6AddrLen])
		out.Addresses = append(out.Addresses, netip.AddrFrom16(a16))
	}
	return out, nil
}

// valueLen returns the encoded TLV 232 value length.
func (t IPv6InterfaceAddrTLV) valueLen() int { return len(t.Addresses) * IPv6AddrLen }

// writeIPv6InterfaceAddrTLV emits TLV 232 into buf at off. Each address is
// written as its 16-octet form (the caller passes IPv6 addresses).
func writeIPv6InterfaceAddrTLV(buf []byte, off int, t IPv6InterfaceAddrTLV) int {
	vlen := t.valueLen()
	buf[off] = TLVIPv6InterfaceAddress
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	for _, a := range t.Addresses {
		a16 := a.As16()
		off += copy(buf[off:], a16[:])
	}
	return off
}

// ---- TLV 236: IPv6 Reachability (RFC 5308 sec 2) ----
//
// Each entry (the canonical layout in the umbrella "Shared Contracts -> TLV
// 135 / 236 entry layout"):
//
//	4 octets : metric (32-bit)
//	1 octet  : flags = U up/down (0x80) | X external (0x40)
//	           | S sub-TLV-present (0x20) | 5 reserved bits
//	1 octet  : prefix length (0..128)
//	ceil(len/8) octets : packed IPv6 prefix
//	(only if S set) 1 octet : sub-TLV length
//	(only if S set) N octets : sub-TLVs
//
// BIT-MASK SOURCE NOTE: RFC 5308 sec 2 draws the flags octet as "|U|X|S|Reserve|"
// MSB-first, so reading the high bits in order gives U=0x80, X=0x40, S=0x20. An
// interop peer (FRR) encodes per the RFC, so these constants follow the RFC bit
// order; TestISISTLVIPv6RoundTrip and TestISISTLVIPv6FlagBits pin them.

// TLV 236 flags-octet bit masks. RFC 5308 sec 2 lays them out MSB-first as
// U|X|S|Reserve(5).
const (
	// RFC 5308 sec 2: U (up/down) is the most-significant flag bit.
	ipv6ReachFlagUpDown = 0x80 // U: up/down bit (RFC 5308 sec 2, RFC 2966)
	// RFC 5308 sec 2: X (external original) is the second flag bit.
	ipv6ReachFlagExternal = 0x40 // X: external bit
	// RFC 5308 sec 2: S (sub-TLV present) is the third flag bit.
	ipv6ReachFlagSubTLV = 0x20 // S: sub-TLV-present bit
)

// IPv6ReachEntry is one decoded TLV 236 prefix entry.
type IPv6ReachEntry struct {
	Metric   types.PrefixMetric
	UpDown   bool // U flag (set = leaked down a level)
	External bool // X flag (set = redistributed from another protocol)
	Prefix   netip.Prefix
	SubTLVs  []SubTLV // present only when the S flag was set; retained verbatim
}

// IPv6ReachabilityTLV is the decoded TLV 236: a list of IPv6 prefix entries.
type IPv6ReachabilityTLV struct {
	Entries []IPv6ReachEntry
}

// DecodeIPv6ReachabilityTLV parses a TLV 236 value. Every length is
// bound-checked before slicing (security review, R-5): a prefix length > 128, a
// packed prefix or sub-TLV block that overruns the value, is rejected without
// reading out of bounds. The 32-bit metric is preserved as-is. The
// sub-TLV-length octet and sub-TLVs are read ONLY when the S flag is set.
func DecodeIPv6ReachabilityTLV(value []byte) (IPv6ReachabilityTLV, error) {
	var out IPv6ReachabilityTLV
	off := 0
	for off < len(value) {
		// metric (4) + flags (1) + prefix length (1) minimum.
		if off+types.PrefixMetricLen+2 > len(value) {
			return IPv6ReachabilityTLV{}, ErrTruncated
		}
		metric, err := types.PrefixMetricFromBytes(value[off : off+types.PrefixMetricLen])
		if err != nil {
			return IPv6ReachabilityTLV{}, err
		}
		off += types.PrefixMetricLen
		flags := value[off]
		off++
		plen := int(value[off])
		off++
		if plen > MaxIPv6PrefixLen {
			return IPv6ReachabilityTLV{}, ErrLength
		}
		poct := prefixOctets(plen)
		if off+poct > len(value) {
			return IPv6ReachabilityTLV{}, ErrTruncated
		}
		var a16 [IPv6AddrLen]byte
		copy(a16[:], value[off:off+poct])
		off += poct
		prefix := netip.PrefixFrom(netip.AddrFrom16(a16), plen)

		entry := IPv6ReachEntry{
			Metric:   metric,
			UpDown:   flags&ipv6ReachFlagUpDown != 0,
			External: flags&ipv6ReachFlagExternal != 0,
			Prefix:   prefix,
		}
		if flags&ipv6ReachFlagSubTLV != 0 {
			if off+1 > len(value) {
				return IPv6ReachabilityTLV{}, ErrTruncated
			}
			subLen := int(value[off])
			off++
			if off+subLen > len(value) {
				return IPv6ReachabilityTLV{}, ErrTruncated
			}
			subs, err := decodeSubTLVs(value[off : off+subLen])
			if err != nil {
				return IPv6ReachabilityTLV{}, err
			}
			entry.SubTLVs = subs
			off += subLen
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}

// entryLen returns the encoded length of one TLV 236 entry.
func (e IPv6ReachEntry) entryLen() int {
	n := types.PrefixMetricLen + 1 + 1 + prefixOctets(e.Prefix.Bits())
	if len(e.SubTLVs) > 0 {
		n += 1 + subTLVsEncodedLen(e.SubTLVs)
	}
	return n
}

// valueLen returns the encoded TLV 236 value length.
func (t IPv6ReachabilityTLV) valueLen() int {
	n := 0
	for _, e := range t.Entries {
		n += e.entryLen()
	}
	return n
}

// EncodedLen returns the full on-wire size of TLV 236 (type + length + value).
func (t IPv6ReachabilityTLV) EncodedLen() int { return tlvOverhead(t.valueLen()) }

// WriteTo emits TLV 236 into buf at off and returns the new offset. The S flag
// and the sub-TLV-length octet are written ONLY when the entry has sub-TLVs,
// mirroring the decode (RFC 5308 sec 2). The U and X bits go in the flags
// octet, the prefix length in its own octet. Buffer-first; the originator
// (isis-12) lives in a sibling package.
func (t IPv6ReachabilityTLV) WriteTo(buf []byte, off int) int {
	vlen := t.valueLen()
	buf[off] = TLVIPv6Reachability
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	for _, e := range t.Entries {
		off += e.Metric.WriteTo(buf, off)
		var flags byte
		if e.UpDown {
			flags |= ipv6ReachFlagUpDown
		}
		if e.External {
			flags |= ipv6ReachFlagExternal
		}
		hasSub := len(e.SubTLVs) > 0
		if hasSub {
			flags |= ipv6ReachFlagSubTLV
		}
		buf[off] = flags
		off++
		plen := e.Prefix.Bits()
		buf[off] = byte(plen)
		off++
		poct := prefixOctets(plen)
		a16 := e.Prefix.Addr().As16()
		off += copy(buf[off:off+poct], a16[:poct])
		if hasSub {
			subLen := subTLVsEncodedLen(e.SubTLVs)
			buf[off] = byte(subLen)
			off++
			off = writeSubTLVs(buf, off, e.SubTLVs)
		}
	}
	return off
}
