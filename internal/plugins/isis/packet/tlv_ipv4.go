// Design: docs/architecture/wire/isis.md -- TLV 132 (IP Interface Address), TLV 135 (Extended IP Reachability)
// RFC: rfc/short/rfc5305.md -- TLV 135 entry layout (sec 4): metric, control octet, packed prefix, sub-TLVs
// RFC: rfc/short/rfc1195.md -- TLV 132 (IP Interface Address), list of 4-octet IPv4 addresses

package packet

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// IPv4AddrLen is the length of one IPv4 address (TLV 132 entry).
const IPv4AddrLen = 4

// ---- TLV 132: IP Interface Address (RFC 1195) ----
//
// Value is a flat list of 4-octet IPv4 addresses assigned to the interface
// (IIH) or the IS (LSP). It is the source of the adjacency next-hop for SPF.

// IPv4InterfaceAddrTLV is the decoded TLV 132.
type IPv4InterfaceAddrTLV struct {
	Addresses []netip.Addr // each a 4-octet IPv4 address
}

// DecodeIPv4InterfaceAddrTLV parses a TLV 132 value (one 4-octet address per
// entry). A value length that is not a multiple of 4 is rejected (ErrLength).
func DecodeIPv4InterfaceAddrTLV(value []byte) (IPv4InterfaceAddrTLV, error) {
	if len(value)%IPv4AddrLen != 0 {
		return IPv4InterfaceAddrTLV{}, ErrLength
	}
	n := len(value) / IPv4AddrLen
	out := IPv4InterfaceAddrTLV{Addresses: make([]netip.Addr, 0, n)}
	for off := 0; off < len(value); off += IPv4AddrLen {
		var a4 [IPv4AddrLen]byte
		copy(a4[:], value[off:off+IPv4AddrLen])
		out.Addresses = append(out.Addresses, netip.AddrFrom4(a4))
	}
	return out, nil
}

// valueLen returns the encoded TLV 132 value length.
func (t IPv4InterfaceAddrTLV) valueLen() int { return len(t.Addresses) * IPv4AddrLen }

// writeIPv4InterfaceAddrTLV emits TLV 132 into buf at off. Non-IPv4 addresses
// in the slice are skipped (the caller is responsible for passing IPv4 only);
// each address is written as its 4-octet form.
func writeIPv4InterfaceAddrTLV(buf []byte, off int, t IPv4InterfaceAddrTLV) int {
	vlen := t.valueLen()
	buf[off] = TLVIPInterfaceAddress
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	for _, a := range t.Addresses {
		a4 := a.As4()
		off += copy(buf[off:], a4[:])
	}
	return off
}

// ---- TLV 135: Extended IP Reachability (RFC 5305 sec 4) ----
//
// Each entry (the canonical layout in the umbrella "Shared Contracts -> TLV
// 135 / 236 entry layout"):
//
//	4 octets : metric (32-bit)
//	1 octet  : control = up/down bit (0x80) | sub-TLV-present S bit (0x40)
//	           | 6-bit prefix length (0..32)
//	ceil(len/8) octets : packed IPv4 prefix
//	(only if S set) 1 octet : sub-TLV length
//	(only if S set) N octets : sub-TLVs
//
// The up/down bit lives in the CONTROL octet, NOT the high bit of the metric
// (RFC 5305 sec 4.1, RFC 2966). The prefix metric is 4 octets, distinct from
// the 3-octet 24-bit TLV 22 IS metric.

// TLV 135 control-octet bit masks (RFC 5305 sec 4).
const (
	extIPCtrlUpDown    = 0x80 // up/down bit (RFC 5305 sec 4.1 / RFC 2966)
	extIPCtrlSubTLV    = 0x40 // sub-TLV-present (S) bit
	extIPCtrlPrefixLen = 0x3f // low 6 bits: prefix length 0..32
)

// MaxIPv4PrefixLen is the largest IPv4 prefix length.
const MaxIPv4PrefixLen = 32

// ExtIPReachEntry is one decoded TLV 135 prefix entry.
type ExtIPReachEntry struct {
	Metric  types.PrefixMetric
	UpDown  bool // up/down bit from the control octet (set = leaked down a level)
	Prefix  netip.Prefix
	SubTLVs []SubTLV // present only when the S bit was set; retained verbatim
}

// ExtendedIPReachTLV is the decoded TLV 135: a list of prefix entries.
type ExtendedIPReachTLV struct {
	Entries []ExtIPReachEntry
}

// prefixOctets returns the number of packed prefix octets for a prefix length
// (ceil(len/8)); RFC 5305 sec 4 prefix-length-to-octets mapping.
func prefixOctets(bits int) int { return (bits + 7) / 8 }

// DecodeExtendedIPReachTLV parses a TLV 135 value. Every length is
// bound-checked before slicing (security review, R-5): a prefix length > 32, a
// packed prefix or sub-TLV block that overruns the value, is rejected without
// reading out of bounds. The 32-bit metric is preserved as-is (never capped at
// 24-bit). The sub-TLV-length octet and sub-TLVs are read ONLY when the S bit
// is set.
func DecodeExtendedIPReachTLV(value []byte) (ExtendedIPReachTLV, error) {
	var out ExtendedIPReachTLV
	off := 0
	for off < len(value) {
		// metric (4) + control (1) minimum.
		if off+types.PrefixMetricLen+1 > len(value) {
			return ExtendedIPReachTLV{}, ErrTruncated
		}
		metric, err := types.PrefixMetricFromBytes(value[off : off+types.PrefixMetricLen])
		if err != nil {
			return ExtendedIPReachTLV{}, err
		}
		off += types.PrefixMetricLen
		ctrl := value[off]
		off++
		plen := int(ctrl & extIPCtrlPrefixLen)
		if plen > MaxIPv4PrefixLen {
			return ExtendedIPReachTLV{}, ErrLength
		}
		poct := prefixOctets(plen)
		if off+poct > len(value) {
			return ExtendedIPReachTLV{}, ErrTruncated
		}
		// Reassemble the packed prefix into a 4-octet IPv4 address (trailing
		// octets zero, per RFC 5305 sec 4: remaining bits transmitted as zero).
		var a4 [IPv4AddrLen]byte
		copy(a4[:], value[off:off+poct])
		off += poct
		prefix := netip.PrefixFrom(netip.AddrFrom4(a4), plen)

		entry := ExtIPReachEntry{
			Metric: metric,
			UpDown: ctrl&extIPCtrlUpDown != 0,
			Prefix: prefix,
		}
		if ctrl&extIPCtrlSubTLV != 0 {
			if off+1 > len(value) {
				return ExtendedIPReachTLV{}, ErrTruncated
			}
			subLen := int(value[off])
			off++
			if off+subLen > len(value) {
				return ExtendedIPReachTLV{}, ErrTruncated
			}
			subs, err := decodeSubTLVs(value[off : off+subLen])
			if err != nil {
				return ExtendedIPReachTLV{}, err
			}
			entry.SubTLVs = subs
			off += subLen
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}

// entryLen returns the encoded length of one TLV 135 entry.
func (e ExtIPReachEntry) entryLen() int {
	n := types.PrefixMetricLen + 1 + prefixOctets(e.Prefix.Bits())
	if len(e.SubTLVs) > 0 {
		n += 1 + subTLVsEncodedLen(e.SubTLVs)
	}
	return n
}

// valueLen returns the encoded TLV 135 value length.
func (t ExtendedIPReachTLV) valueLen() int {
	n := 0
	for _, e := range t.Entries {
		n += e.entryLen()
	}
	return n
}

// EncodedLen returns the full on-wire size of TLV 135 (type + length + value).
func (t ExtendedIPReachTLV) EncodedLen() int { return tlvOverhead(t.valueLen()) }

// WriteTo emits TLV 135 into buf at off and returns the new offset. The S
// (sub-TLV-present) bit and the sub-TLV-length octet are written ONLY when the
// entry has sub-TLVs, exactly mirroring the decode (RFC 5305 sec 4.2). The
// up/down bit is placed in the control octet. Buffer-first; the originator
// (isis-6/9/11) lives in a sibling package.
func (t ExtendedIPReachTLV) WriteTo(buf []byte, off int) int {
	vlen := t.valueLen()
	buf[off] = TLVExtendedIPReach
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	for _, e := range t.Entries {
		off += e.Metric.WriteTo(buf, off)
		plen := e.Prefix.Bits()
		ctrl := byte(plen) & extIPCtrlPrefixLen
		if e.UpDown {
			ctrl |= extIPCtrlUpDown
		}
		hasSub := len(e.SubTLVs) > 0
		if hasSub {
			ctrl |= extIPCtrlSubTLV
		}
		buf[off] = ctrl
		off++
		poct := prefixOctets(plen)
		a4 := e.Prefix.Addr().As4()
		off += copy(buf[off:off+poct], a4[:poct])
		if hasSub {
			subLen := subTLVsEncodedLen(e.SubTLVs)
			buf[off] = byte(subLen)
			off++
			off = writeSubTLVs(buf, off, e.SubTLVs)
		}
	}
	return off
}
