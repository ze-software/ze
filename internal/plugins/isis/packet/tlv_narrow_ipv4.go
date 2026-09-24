// Design: docs/architecture/wire/isis.md -- narrow IPv4 reachability, TLVs 128 and 130.
// RFC 1195 sections 5.3.4/5.3.5, as extended by RFC 2966 section 2.

package packet

import (
	"encoding/binary"
	"math/bits"
	"net/netip"
)

// Each entry has the following offsets relative to the start of its TLV value:
//
//	0     | U | I/E | default metric (6 bits) |
//	1     | S |  0  | delay metric   (6 bits) |
//	2     | S |  0  | expense metric (6 bits) |
//	3     | S |  0  | error metric   (6 bits) |
//	4..7  | IPv4 address                    |
//	8..11 | IPv4 subnet mask                |
//
// RFC 2966 section 2 reassigns default-metric bit8 to U (up/down).
// Default-octet I/E selects the type for all four metrics and is valid only in
// TLV 130. Bit7 of each optional metric octet remains reserved.
const (
	narrowIPReachEntryLen = 12
	narrowIPMetricDown    = 0x80
	narrowIPOptionalMask  = 0xbf
)

// NarrowIPReachEntry is one TLV 128 or 130 route. The optional metric octets
// retain their unsupported bit (0x80) and six-bit value, but not reserved bit7.
// ExternalMetric is metric type, not route origin: TLV 130 can use internal metrics.
type NarrowIPReachEntry struct {
	DefaultMetricValue uint8
	ExternalMetric     bool
	UpDown             bool
	DelayMetric        uint8
	ExpenseMetric      uint8
	ErrorMetric        uint8
	Prefix             netip.Prefix
}

// NarrowIPReachTLV carries internal (128) or external (130) IPv4 reachability.
// External selects the TLV type, independently of each entry's metric type.
type NarrowIPReachTLV struct {
	External bool
	Entries  []NarrowIPReachEntry
}

// DecodeNarrowIPReachTLV decodes a TLV 128/130 value. Non-contiguous masks cannot
// be represented as a netip.Prefix and are reported as ErrLength, not rounded to
// a different route. The value bounds the loop to at most 21 entries.
func DecodeNarrowIPReachTLV(value []byte, external bool) (NarrowIPReachTLV, error) {
	if len(value) > MaxTLVValueLen {
		return NarrowIPReachTLV{}, ErrLength
	}
	if len(value)%narrowIPReachEntryLen != 0 {
		return NarrowIPReachTLV{}, ErrLength
	}
	out := NarrowIPReachTLV{External: external}
	if len(value) == 0 {
		return out, nil
	}
	out.Entries = make([]NarrowIPReachEntry, len(value)/narrowIPReachEntryLen)
	for i := range out.Entries {
		off := i * narrowIPReachEntryLen
		mask := binary.BigEndian.Uint32(value[off+8 : off+12])
		prefixLen := bits.LeadingZeros32(^mask)
		if mask != ^uint32(0)<<(32-prefixLen) {
			return NarrowIPReachTLV{}, ErrLength
		}
		address := netip.AddrFrom4([4]byte(value[off+4 : off+8]))
		entry := &out.Entries[i]
		entry.Prefix = netip.PrefixFrom(address, prefixLen).Masked()
		entry.DefaultMetricValue = value[off] & narrowMetricValueMask
		// RFC 2966 section 3.3: "Upon receipt of an IP prefix with this
		// combination, routers must ignore this prefix." Preserve an invalid
		// TLV 128 external metric so the SPF consumer can reject that prefix.
		entry.ExternalMetric = value[off]&narrowMetricExternalIE != 0
		entry.UpDown = value[off]&narrowIPMetricDown != 0
		// RFC 1195 section 5.3.4: "Bit 7 of this field is reserved, and
		// must be set to zero on transmission and ignored on reception."
		// Section 5.3.5 repeats this for all three TLV 130 optional metrics.
		entry.DelayMetric = value[off+1] & narrowIPOptionalMask
		entry.ExpenseMetric = value[off+2] & narrowIPOptionalMask
		entry.ErrorMetric = value[off+3] & narrowIPOptionalMask
	}
	return out, nil
}

// EncodedLen returns the full TLV size, including the two-octet header.
func (t NarrowIPReachTLV) EncodedLen() int {
	return TLVHeaderLen + narrowIPReachEntryLen*len(t.Entries)
}

// WriteTo writes a narrow reachability TLV and returns the new offset. The
// caller MUST provide room, at most 21 entries, valid IPv4 prefixes, and metrics
// in the six-bit range. Reserved bits are cleared even in supplied raw metrics.
func (t NarrowIPReachTLV) WriteTo(buf []byte, off int) int {
	buf[off] = TLVIPInternalReachability
	if t.External {
		buf[off] = TLVIPExternalReachability
	}
	buf[off+1] = byte(narrowIPReachEntryLen * len(t.Entries))
	off += TLVHeaderLen
	for _, entry := range t.Entries {
		buf[off] = entry.DefaultMetricValue & narrowMetricValueMask
		if t.External {
			if entry.ExternalMetric {
				buf[off] |= narrowMetricExternalIE
			}
		}
		if entry.UpDown {
			buf[off] |= narrowIPMetricDown
		}
		// RFC 1195 section 5.3.4: "Bit 7 of this field is reserved, and
		// must be set to zero on transmission and ignored on reception."
		buf[off+1] = entry.DelayMetric & narrowIPOptionalMask
		buf[off+2] = entry.ExpenseMetric & narrowIPOptionalMask
		buf[off+3] = entry.ErrorMetric & narrowIPOptionalMask
		address := entry.Prefix.Masked().Addr().As4()
		copy(buf[off+4:off+8], address[:])
		mask := ^uint32(0) << (32 - entry.Prefix.Bits())
		binary.BigEndian.PutUint32(buf[off+8:off+12], mask)
		off += narrowIPReachEntryLen
	}
	return off
}
