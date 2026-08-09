// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 AS-External / NSSA LSA body codec.
// RFC: rfc/short/rfc5340.md (§A.4.7 AS-External-LSA; §A.4.8 NSSA-LSA reuses the body)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// AS-External / NSSA flag bits (RFC 5340 §A.4.7). They occupy the low three bits
// of the first body octet, which shares the leading 32-bit word with the 24-bit
// Metric: byte 0 = "00000|E|F|T", bytes 1-3 = Metric.
const (
	ExternalFlagE = 0x04 // E-bit: Type 2 external metric (not comparable to OSPF cost)
	ExternalFlagF = 0x02 // F-bit: a Forwarding Address is present
	ExternalFlagT = 0x01 // T-bit: an External Route Tag is present
)

// AS-External-LSA body field offsets (RFC 5340 §A.4.7, body-relative). The first
// 32-bit word is the E/F/T flags octet (offset 0) followed by the 24-bit Metric
// (offset 1); PrefixLength and PrefixOptions follow; the Referenced LS Type is a
// 16-bit field at offset 6; the AddressPrefix starts at offset 8.
const (
	externalFlagsOff    = 0 // E/F/T flags (high octet of the Metric word)
	externalMetricOff   = 1 // 24-bit metric
	externalLenOff      = 4 // PrefixLength
	externalOptionsOff  = 5 // PrefixOptions
	externalRefTypeOff  = 6 // Referenced LS Type (16-bit)
	externalAddrOff     = 8 // AddressPrefix
	externalFwdAddrLen  = 16
	externalRouteTagLen = 4
	externalRefLSIDLen  = 4
)

// ExternalLSA is the OSPFv3 AS-External-LSA (Type 0x4005) or NSSA-LSA (Type
// 0x2007) body; the two share an identical layout (RFC 5340 §A.4.7-A.4.8) and
// differ only by LS Type and flooding scope (AS-External is AS scope with the
// U-bit set; NSSA is area scope). The optional trailing fields are gated, in RFC
// order, by the F-bit (Forwarding Address, 128 bits), the T-bit (External Route
// Tag, 32 bits), and a non-zero Referenced LS Type (Referenced Link State ID, 32
// bits). The 16-bit Referenced LS Type is carried in ReferencedLSType, not in the
// embedded Prefix's Field16 (which stays zero for this body).
type ExternalLSA struct {
	ExternalType2     bool // E-bit
	Metric            uint32
	Prefix            Prefix
	ReferencedLSType  types.LSType
	ForwardingAddr    [16]byte // present iff F-bit
	HasForwardingAddr bool
	ExternalRouteTag  uint32 // present iff T-bit
	HasRouteTag       bool
	ReferencedLSID    [4]byte // present iff ReferencedLSType != 0
}

// DecodeExternalLSA parses an AS-External / NSSA body. The optional trailing
// fields are consumed in RFC order (Forwarding Address, External Route Tag,
// Referenced Link State ID) and the body must end exactly after the last present
// field.
func DecodeExternalLSA(body []byte) (ExternalLSA, error) {
	if len(body) < externalAddrOff {
		return ExternalLSA{}, ErrTruncated
	}
	// RFC 5340 §A.4.7: the E/F/T flags are the low three bits of body[0]; the
	// 24-bit Metric fills the rest of the first word. The 16-bit field at offset 6
	// (decodeInlinePrefix's Field16) is the Referenced LS Type, not a per-prefix
	// field, so it is lifted out and the stored Prefix.Field16 is left zero.
	flags := body[externalFlagsOff]
	pfx, addrLen, err := decodeInlinePrefix(body, externalLenOff, externalOptionsOff, externalRefTypeOff, externalAddrOff)
	if err != nil {
		return ExternalLSA{}, err
	}
	refType := types.LSType(pfx.Field16)
	pfx.Field16 = 0
	out := ExternalLSA{
		ExternalType2:    flags&ExternalFlagE != 0,
		Metric:           readUint24(body, externalMetricOff),
		Prefix:           pfx,
		ReferencedLSType: refType,
	}
	off := externalAddrOff + addrLen
	if flags&ExternalFlagF != 0 {
		if off+externalFwdAddrLen > len(body) {
			return ExternalLSA{}, ErrTruncated
		}
		copy(out.ForwardingAddr[:], body[off:off+externalFwdAddrLen])
		out.HasForwardingAddr = true
		off += externalFwdAddrLen
	}
	if flags&ExternalFlagT != 0 {
		if off+externalRouteTagLen > len(body) {
			return ExternalLSA{}, ErrTruncated
		}
		out.ExternalRouteTag = readUint32(body, off)
		out.HasRouteTag = true
		off += externalRouteTagLen
	}
	if out.ReferencedLSType != 0 {
		if off+externalRefLSIDLen > len(body) {
			return ExternalLSA{}, ErrTruncated
		}
		copy(out.ReferencedLSID[:], body[off:off+externalRefLSIDLen])
		off += externalRefLSIDLen
	}
	if off != len(body) {
		return ExternalLSA{}, ErrLength
	}
	return out, nil
}

// EncodedLen returns the AS-External / NSSA body length.
func (l ExternalLSA) EncodedLen() int {
	n := externalAddrOff + l.Prefix.Length.ByteLen()
	if l.HasForwardingAddr {
		n += externalFwdAddrLen
	}
	if l.HasRouteTag {
		n += externalRouteTagLen
	}
	if l.ReferencedLSType != 0 {
		n += externalRefLSIDLen
	}
	return n
}

// WriteTo serializes the AS-External / NSSA body into buf at off. Byte 0 carries
// the E/F/T flags (E for a Type 2 metric, F when a Forwarding Address is present,
// T when an External Route Tag is present); the 24-bit Metric, PrefixLength,
// PrefixOptions, the 16-bit Referenced LS Type, and the AddressPrefix follow, then
// the present optional fields in RFC order.
func (l ExternalLSA) WriteTo(buf []byte, off int) int {
	start := off
	var flags byte
	if l.ExternalType2 {
		flags |= ExternalFlagE
	}
	if l.HasForwardingAddr {
		flags |= ExternalFlagF
	}
	if l.HasRouteTag {
		flags |= ExternalFlagT
	}
	buf[start+externalFlagsOff] = flags
	writeUint24(buf, start+externalMetricOff, l.Metric)
	buf[start+externalLenOff] = byte(l.Prefix.Length)
	buf[start+externalOptionsOff] = byte(l.Prefix.Options)
	writeUint16(buf, start+externalRefTypeOff, uint16(l.ReferencedLSType))
	off = writePrefixAddress(buf, start+externalAddrOff, l.Prefix.Length, l.Prefix.Address)
	if l.HasForwardingAddr {
		copy(buf[off:off+externalFwdAddrLen], l.ForwardingAddr[:])
		off += externalFwdAddrLen
	}
	if l.HasRouteTag {
		off += writeUint32(buf, off, l.ExternalRouteTag)
	}
	if l.ReferencedLSType != 0 {
		copy(buf[off:off+externalRefLSIDLen], l.ReferencedLSID[:])
		off += externalRefLSIDLen
	}
	return off
}
