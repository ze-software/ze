// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE wire codec
// RFC: rfc/short/rfc2205.md
// RFC: rfc/short/rfc3209.md
// RFC: rfc/short/rfc4090.md
// Related: build.go -- composes these object encoders into whole messages
// Related: frr.go -- FAST_REROUTE/DETOUR/SESSION_ATTRIBUTE codecs (RFC 4090)
// Related: transport.go -- carries encoded messages over raw IP
//
// RSVP-TE message encoding/decoding per RFC 2205 (base RSVP) and
// RFC 3209 (TE extensions). RSVP runs directly on IP (protocol 46).
// All multi-byte fields are big-endian.
//
// Message format: Common Header (8 bytes) + Objects (variable).
// Each Object: Length (2) + Class-Num (1) + C-Type (1) + Body (variable).
package rsvpte

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net/netip"
)

// RFC 2205 Section 3.1: RSVP version and header sizes. The IP protocol number
// (46) lives with the raw socket in transport_linux.go.
const (
	rsvpVersion = 1
	rsvpHdrLen  = 8
	objHdrLen   = 4
)

// RFC 2205 Section 3.1.1: Message types.
const (
	MsgTypePath     uint8 = 1
	MsgTypeResv     uint8 = 2
	MsgTypePathErr  uint8 = 3
	MsgTypeResvErr  uint8 = 4
	MsgTypePathTear uint8 = 5
	MsgTypeResvTear uint8 = 6
	MsgTypeResvConf uint8 = 7
)

// RFC 2205 Section A.1 / RFC 3209: Object class numbers.
const (
	ClassSession        uint8 = 1
	ClassRSVPHop        uint8 = 3
	ClassTimeValues     uint8 = 5
	ClassErrorSpec      uint8 = 6
	ClassSenderTemplate uint8 = 11
	ClassSenderTSpec    uint8 = 12
	ClassFilterSpec     uint8 = 10
	ClassFlowSpec       uint8 = 9
	ClassStyle          uint8 = 8
	ClassExplicitRoute  uint8 = 20
	ClassRecordRoute    uint8 = 21
	ClassLabelRequest   uint8 = 19
	ClassLabel          uint8 = 16
	ClassSessionAttr    uint8 = 207
	ClassResvConfirm uint8 = 15 // RFC 2205 Section A.14: reservation-confirmation receiver.
	ClassAdspec      uint8 = 13 // RFC 2210 Section 3.3: IntServ path characterization.
	// RFC 4090 Section 4: Fast Reroute object classes.
	ClassFastReroute uint8 = 205
	ClassDetour      uint8 = 63
)

// Object classes ze knows and deliberately reads no body for. RFC 2205 Sections
// 3.1.3 and 3.1.4 make each one optional in the Path or Resv message that
// carries it, so a conformant peer can send it at any time. Their Class-Num
// high-order bit is zero, which is why they are named here: without this list
// classifyUnknownClass would reject them as unknown and refuse a legal message.
//
// NULL heads the list. RFC 2205 Section 3.1: "An RSVP implementation must
// recognize the following classes: NULL. A NULL object has a Class-Num of zero,
// and its C-Type is ignored. Its length must be at least 4, but can be any
// multiple of 4. A NULL object may appear anywhere in a sequence of objects,
// and its contents will be ignored by the receiver.".
const (
	ClassNull        uint8 = 0  // RFC 2205 Section 3.1: padding; its contents are ignored.
	ClassIntegrity   uint8 = 4  // RFC 2205 Section A.3: ze implements no RSVP authentication.
	ClassScope       uint8 = 7  // RFC 2205 Section A.6: WF style only; ze signals FF and SE LSPs.
	ClassPolicyData  uint8 = 14 // RFC 2205 Section A.13: ze runs no policy module.
)

// classNumIgnoreBit is the high-order bit of the Class-Num. RFC 2205 Section
// 3.10: an object of a class the node does not know is rejected when this bit is
// zero, and ignored when it is set.
const classNumIgnoreBit uint8 = 0x80

// C-Types for objects.
const (
	CTypeLSPTunnelIPv4 uint8 = 7
	CTypeLSPTunnelIPv6 uint8 = 8
	CTypeIPv4          uint8 = 1
	CTypeIPv6          uint8 = 2
	CTypeGeneric       uint8 = 1
	CTypeLabel         uint8 = 1
	CTypeStyle         uint8 = 1
	// RFC 4090 Section 4.1: FAST_REROUTE C-Type 1.
	CTypeFastReroute uint8 = 1
	// RFC 3209 Section 4.7.2: SESSION_ATTRIBUTE C-Type 7 (LSP_TUNNEL, no resource
	// affinities). RFC 4090 Section 4.2.1: DETOUR C-Type 7 (IPv4).
	CTypeSessionAttr uint8 = 7
	CTypeDetourIPv4  uint8 = 7
	// RFC 3209 Section 4.7.1: SESSION_ATTRIBUTE C-Type 1 (LSP_TUNNEL_RA), with a
	// 12-byte resource-affinity prefix before the priorities. ze emits C-Type 7 but
	// must decode C-Type 1 from interop peers.
	CTypeSessionAttrRA uint8 = 1
)

// RFC 4090 Section 4.1: FAST_REROUTE object Flags.
const (
	FRRFlagOneToOneBackup uint8 = 0x01
	FRRFlagFacilityBackup uint8 = 0x02
)

// SESSION_ATTRIBUTE Flags (RFC 3209 Section 4.7.1, extended by RFC 4090 Section
// 4.3). The head-end sets these to express the local protection it wants.
const (
	SessAttrLocalProtection     uint8 = 0x01 // RFC 3209: local protection desired
	SessAttrLabelRecording      uint8 = 0x02 // RFC 3209: label recording desired
	SessAttrSEStyle             uint8 = 0x04 // RFC 3209: SE style desired
	SessAttrBandwidthProtection uint8 = 0x08 // RFC 4090 Section 4.3: bandwidth protection desired
	SessAttrNodeProtection      uint8 = 0x10 // RFC 4090 Section 4.3: node protection desired
)

// RRO subobject Flags (RFC 3209 Section 4.4.1, extended by RFC 4090 Section 4.4).
// A PLR sets these in its RRO subobject as the RESV travels upstream, reporting
// protection state to the head-end.
const (
	RROFlagProtectionAvailable uint8 = 0x01 // RFC 3209: local protection available
	RROFlagProtectionInUse     uint8 = 0x02 // RFC 3209: local protection in use
	RROFlagBandwidthProtection uint8 = 0x04 // RFC 4090 Section 4.4: bandwidth protection
	RROFlagNodeProtection      uint8 = 0x08 // RFC 4090 Section 4.4: node protection
)

// RFC 3209 Section 4.1: ERO subobject types.
const (
	EROSubIPv4Prefix uint8 = 1
	EROSubIPv6Prefix uint8 = 2
)

// RFC 3209 Section 4.4: RRO subobject types.
const (
	RROSubIPv4  uint8 = 1
	RROSubIPv6  uint8 = 2
	RROSubLabel uint8 = 3
)

// RFC 3032 Section 2.1: MPLS label constraints.
const (
	MaxLabel      = 1048575
	MaxLabelStack = 16
)

// maxRecordRouteHops bounds the Record Route Object. RFC 3209 does not cap the
// RRO, but an unbounded RRO (a routing loop, or a malicious peer) would overflow
// the fixed maxRSVPMessage encode buffer. 32 hops is far beyond any real LSP and
// keeps the encoded RRO (<= 20 bytes/hop) well inside the message buffer.
const maxRecordRouteHops = 32

// maxExplicitRouteHops bounds the Explicit Route Object on decode, for the same
// reason as maxRecordRouteHops: a transit node re-encodes the (remaining) ERO into
// the fixed maxRSVPMessage buffer when it relays a PATH, so an unbounded ERO from a
// malicious peer would otherwise overflow the encode buffer. 64 hops is far beyond
// any real LSP and keeps the encoded ERO (<= 20 bytes/hop) inside the message.
const maxExplicitRouteHops = 64

// RFC 3209 Section 4.7.4: Style constants.
const (
	StyleWildcardFilter uint32 = 17
	StyleFixedFilter    uint32 = 10
	StyleSharedExplicit uint32 = 18
)

var (
	errShortHeader  = errors.New("rsvp: header too short")
	errBadVersion   = errors.New("rsvp: unsupported version")
	errShortObject  = errors.New("rsvp: object too short")
	errLabelRange   = errors.New("rsvp: label out of 20-bit range")
	errBadObjLen    = errors.New("rsvp: invalid object length")
	errShortERO     = errors.New("rsvp: ERO subobject too short")
	errShortRRO     = errors.New("rsvp: RRO subobject too short")
	errBadBandwidth = errors.New("rsvp: invalid token rate")
	errObjectAbsent = errors.New("rsvp: mandatory object absent")
	errBadChecksum = errors.New("rsvp: invalid checksum")
)

// Header is the RSVP common header (RFC 2205 Section 3.1).
type Header struct {
	Version  uint8
	Flags    uint8
	MsgType  uint8
	Checksum uint16
	TTL      uint8
	Length   uint16
}

// encodeHeader writes the fixed-size RSVP common header at the start of buf.
func encodeHeader(buf []byte, h Header) {
	buf[0] = (h.Version << 4) | (h.Flags & 0x0F)
	buf[1] = h.MsgType
	binary.BigEndian.PutUint16(buf[2:4], h.Checksum)
	buf[4] = h.TTL
	buf[5] = 0
	binary.BigEndian.PutUint16(buf[6:8], h.Length)
}

// DecodeHeader reads an RSVP common header.
func DecodeHeader(buf []byte) (Header, error) {
	if len(buf) < rsvpHdrLen {
		return Header{}, errShortHeader
	}
	h := Header{
		Version:  buf[0] >> 4,
		Flags:    buf[0] & 0x0F,
		MsgType:  buf[1],
		Checksum: binary.BigEndian.Uint16(buf[2:4]),
		TTL:      buf[4],
		Length:   binary.BigEndian.Uint16(buf[6:8]),
	}
	if h.Version != rsvpVersion {
		return Header{}, fmt.Errorf("%w: got %d", errBadVersion, h.Version)
	}
	return h, nil
}

// objectHeader is a generic RSVP object header (RFC 2205 Section 3.1.2).
type objectHeader struct {
	Length   uint16
	ClassNum uint8
	CType    uint8
}

// encodeObjectHeader writes the fixed-size object header at the start of buf.
func encodeObjectHeader(buf []byte, o objectHeader) {
	binary.BigEndian.PutUint16(buf[0:2], o.Length)
	buf[2] = o.ClassNum
	buf[3] = o.CType
}

// decodeObjectHeader reads an object header.
func decodeObjectHeader(buf []byte) (objectHeader, error) {
	if len(buf) < objHdrLen {
		return objectHeader{}, errShortObject
	}
	o := objectHeader{
		Length:   binary.BigEndian.Uint16(buf[0:2]),
		ClassNum: buf[2],
		CType:    buf[3],
	}
	if o.Length < objHdrLen {
		return objectHeader{}, fmt.Errorf("%w: %d", errBadObjLen, o.Length)
	}
	// RFC 2205 Section 3.1.2: "Must always be a multiple of 4, and at least 4."
	if o.Length%4 != 0 {
		return objectHeader{}, fmt.Errorf("%w: %d", errBadObjLen, o.Length)
	}
	return o, nil
}

// sessionIPv4 is the SESSION object for LSP tunnels (RFC 3209 Section 4.6.1).
type sessionIPv4 struct {
	TunnelEndpoint netip.Addr
	TunnelID       uint16
	ExtTunnelID    uint32
}

// encodeSessionIPv4 writes a SESSION object. Returns bytes written.
func encodeSessionIPv4(buf []byte, s sessionIPv4) int {
	objLen := uint16(objHdrLen + 12)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassSession, CType: CTypeLSPTunnelIPv4})
	addr := s.TunnelEndpoint.As4()
	copy(buf[4:8], addr[:])
	buf[8] = 0
	buf[9] = 0
	binary.BigEndian.PutUint16(buf[10:12], s.TunnelID)
	binary.BigEndian.PutUint32(buf[12:16], s.ExtTunnelID)
	return int(objLen)
}

// decodeSessionIPv4 reads a SESSION object body (after object header).
func decodeSessionIPv4(body []byte) (sessionIPv4, error) {
	if len(body) != 12 {
		return sessionIPv4{}, errShortObject
	}
	s := sessionIPv4{
		TunnelEndpoint: netip.AddrFrom4([4]byte(body[0:4])),
		TunnelID:       binary.BigEndian.Uint16(body[6:8]),
		ExtTunnelID:    binary.BigEndian.Uint32(body[8:12]),
	}
	// RFC 2205 Appendix A.1: "This field must be non-zero."
	if s.TunnelEndpoint.IsUnspecified() || s.TunnelEndpoint.IsMulticast() {
		return sessionIPv4{}, fmt.Errorf("rsvp: invalid unicast tunnel endpoint")
	}
	return s, nil
}

// senderTemplateIPv4 is the SENDER_TEMPLATE object (RFC 3209 Section 4.6.2).
type senderTemplateIPv4 struct {
	SenderAddr netip.Addr
	LSPID      uint16
}

// encodeSenderTemplate writes a SENDER_TEMPLATE object (Class-Num 11), the
// sender identity a Path, PathTear or PathErr carries. Returns bytes written.
func encodeSenderTemplate(buf []byte, st senderTemplateIPv4) int {
	return encodeSenderIdentity(buf, ClassSenderTemplate, st)
}

// encodeFilterSpec writes a FILTER_SPEC object (Class-Num 10), the sender
// identity a Resv carries. Returns bytes written.
//
// RFC 2205 Section 3.1.4 writes the FF flow descriptor as "<FLOWSPEC>
// <FILTER_SPEC>", and RFC 3209 Section 3 places the LABEL by it: "In Resv
// messages they MUST appear after the associated FILTER_SPEC and prior to any
// subsequent FILTER_SPEC." RFC 3209 Section 4.6.3.1 gives the object its
// C-Type, "Class = FILTER SPECIFICATION, LSP_TUNNEL_IPv4 C-Type = 7", and its
// body: "The format of the LSP_TUNNEL_IPv4 FILTER_SPEC object is identical to
// the LSP_TUNNEL_IPv4 SENDER_TEMPLATE object." So the two encoders share one
// body and differ in the Class-Num only.
func encodeFilterSpec(buf []byte, st senderTemplateIPv4) int {
	return encodeSenderIdentity(buf, ClassFilterSpec, st)
}

// encodeSenderIdentity writes the LSP_TUNNEL_IPv4 body RFC 3209 Section 4.6.2.1
// defines under the given Class-Num:
//
//	0                   1                   2                   3
//	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                   IPv4 tunnel sender address                  |  body 0-3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  MUST be zero                 |            LSP ID             |  body 4-7
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func encodeSenderIdentity(buf []byte, classNum uint8, st senderTemplateIPv4) int {
	objLen := uint16(objHdrLen + 8)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: classNum, CType: CTypeLSPTunnelIPv4})
	addr := st.SenderAddr.As4()
	copy(buf[4:8], addr[:])
	buf[8] = 0
	buf[9] = 0
	binary.BigEndian.PutUint16(buf[10:12], st.LSPID)
	return int(objLen)
}

// decodeSenderTemplate reads a SENDER_TEMPLATE or a FILTER_SPEC object body.
// RFC 3209 Section 4.6.3.1: "The format of the LSP_TUNNEL_IPv4 FILTER_SPEC
// object is identical to the LSP_TUNNEL_IPv4 SENDER_TEMPLATE object.".
func decodeSenderTemplate(body []byte) (senderTemplateIPv4, error) {
	if len(body) != 8 {
		return senderTemplateIPv4{}, errShortObject
	}
	if body[0] == 0 && body[1] == 0 && body[2] == 0 && body[3] == 0 {
		return senderTemplateIPv4{}, fmt.Errorf("rsvp: unspecified sender")
	}
	return senderTemplateIPv4{
		SenderAddr: netip.AddrFrom4([4]byte(body[0:4])),
		LSPID:      binary.BigEndian.Uint16(body[6:8]),
	}, nil
}

// rsvpHop is the RSVP_HOP object (RFC 2205 Section A.2).
type rsvpHop struct {
	NextHop netip.Addr
	LIH     uint32
}

// encodeRSVPHop writes an RSVP_HOP object. Returns bytes written.
func encodeRSVPHop(buf []byte, h rsvpHop) int {
	objLen := uint16(objHdrLen + 8)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassRSVPHop, CType: CTypeIPv4})
	addr := h.NextHop.As4()
	copy(buf[4:8], addr[:])
	binary.BigEndian.PutUint32(buf[8:12], h.LIH)
	return int(objLen)
}

// decodeRSVPHop reads an RSVP_HOP object body.
func decodeRSVPHop(body []byte) (rsvpHop, error) {
	if len(body) < 8 {
		return rsvpHop{}, errShortObject
	}
	return rsvpHop{
		NextHop: netip.AddrFrom4([4]byte(body[0:4])),
		LIH:     binary.BigEndian.Uint32(body[4:8]),
	}, nil
}

// timeValues is the TIME_VALUES object (RFC 2205 Section A.4).
type timeValues struct {
	RefreshPeriod uint32
}

// encodeTimeValues writes a TIME_VALUES object. Returns bytes written.
func encodeTimeValues(buf []byte, tv timeValues) int {
	objLen := uint16(objHdrLen + 4)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassTimeValues, CType: CTypeGeneric})
	binary.BigEndian.PutUint32(buf[4:8], tv.RefreshPeriod)
	return int(objLen)
}

// decodeTimeValues reads a TIME_VALUES object body.
func decodeTimeValues(body []byte) (timeValues, error) {
	if len(body) < 4 {
		return timeValues{}, errShortObject
	}
	return timeValues{
		RefreshPeriod: binary.BigEndian.Uint32(body[0:4]),
	}, nil
}

// labelRequest is the LABEL_REQUEST object (RFC 3209 Section 4.2).
type labelRequest struct {
	L3PID uint16
}

// encodeLabelRequest writes a LABEL_REQUEST object. Returns bytes written.
func encodeLabelRequest(buf []byte, lr labelRequest) int {
	objLen := uint16(objHdrLen + 4)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassLabelRequest, CType: CTypeGeneric})
	buf[4] = 0
	buf[5] = 0
	binary.BigEndian.PutUint16(buf[6:8], lr.L3PID)
	return int(objLen)
}

// decodeLabelRequest reads a LABEL_REQUEST object body.
func decodeLabelRequest(body []byte) (labelRequest, error) {
	if len(body) < 4 {
		return labelRequest{}, errShortObject
	}
	return labelRequest{
		L3PID: binary.BigEndian.Uint16(body[2:4]),
	}, nil
}

// labelObject is the LABEL object (RFC 3209 Section 4.1).
type labelObject struct {
	Label uint32
}

// encodeLabelObject writes a LABEL object. Returns bytes written.
func encodeLabelObject(buf []byte, l labelObject) int {
	objLen := uint16(objHdrLen + 4)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassLabel, CType: CTypeLabel})
	binary.BigEndian.PutUint32(buf[4:8], l.Label)
	return int(objLen)
}

// decodeLabelObject reads a LABEL object body.
func decodeLabelObject(body []byte) (labelObject, error) {
	if len(body) < 4 {
		return labelObject{}, errShortObject
	}
	label := binary.BigEndian.Uint32(body[0:4])
	if label > MaxLabel {
		return labelObject{}, fmt.Errorf("%w: %d", errLabelRange, label)
	}
	return labelObject{Label: label}, nil
}

// eroHop is a single hop in an Explicit Route Object.
type eroHop struct {
	Loose   bool
	Address netip.Prefix
}

// encodeERO writes an ERO object with the given hops. Returns bytes written.
func encodeERO(buf []byte, hops []eroHop) int {
	off := objHdrLen
	for _, h := range hops {
		// Never write past the fixed message buffer: each subobject is at most 20
		// bytes (IPv6). Stop early rather than overflow if a relayed ERO is longer
		// than the buffer can hold (defense in depth; decodeERO also caps the hop
		// count, matching encodeRRO's guard).
		if off+20 > len(buf) {
			break
		}
		if h.Address.Addr().Is4() {
			var flags uint8
			if h.Loose {
				flags = 0x80
			}
			buf[off] = EROSubIPv4Prefix | flags
			buf[off+1] = 8
			addr := h.Address.Addr().As4()
			copy(buf[off+2:off+6], addr[:])
			buf[off+6] = uint8(h.Address.Bits())
			buf[off+7] = 0
			off += 8
		} else {
			var flags uint8
			if h.Loose {
				flags = 0x80
			}
			buf[off] = EROSubIPv6Prefix | flags
			buf[off+1] = 20
			addr := h.Address.Addr().As16()
			copy(buf[off+2:off+18], addr[:])
			buf[off+18] = uint8(h.Address.Bits())
			buf[off+19] = 0
			off += 20
		}
	}
	objLen := uint16(off)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassExplicitRoute, CType: CTypeGeneric})
	return off
}

// decodeERO reads an ERO object body and returns the hops.
func decodeERO(body []byte) ([]eroHop, error) {
	var hops []eroHop
	off := 0
	for off < len(body) {
		// Cap the explicit route so a malicious or looping ERO cannot grow an
		// unbounded slice, and (after a transit relay) cannot be re-encoded past
		// the fixed message buffer. Mirrors decodeRRO's maxRecordRouteHops cap.
		if len(hops) >= maxExplicitRouteHops {
			break
		}
		if off+2 > len(body) {
			return hops, errShortERO
		}
		subType := body[off] & 0x7F
		loose := body[off]&0x80 != 0
		subLen := int(body[off+1])
		if subLen < 4 || off+subLen > len(body) {
			return hops, errShortERO
		}
		switch subType {
		case EROSubIPv4Prefix:
			if subLen < 8 {
				return hops, errShortERO
			}
			addr := netip.AddrFrom4([4]byte(body[off+2 : off+6]))
			bits := int(body[off+6])
			hops = append(hops, eroHop{Loose: loose, Address: netip.PrefixFrom(addr, bits)})
		case EROSubIPv6Prefix:
			if subLen < 20 {
				return hops, errShortERO
			}
			addr := netip.AddrFrom16([16]byte(body[off+2 : off+18]))
			bits := int(body[off+18])
			hops = append(hops, eroHop{Loose: loose, Address: netip.PrefixFrom(addr, bits)})
		}
		off += subLen
	}
	return hops, nil
}

// rroEntry is a single entry in a Record Route Object.
type rroEntry struct {
	Type    uint8
	Address netip.Addr
	Label   uint32
	Flags   uint8
}

// encodeRRO writes a whole RRO or returns zero when it does not fit. Callers MUST
// append it last, so a dropped object leaves no bytes inside the message length.
func encodeRRO(buf []byte, entries []rroEntry) int {
	// RFC 3209 Section 4.4.3: "the RRO object SHALL be dropped from the
	// message and message processing continues as normal."
	if len(entries) == 0 || len(entries) > maxRecordRouteHops || len(buf) < objHdrLen {
		return 0
	}
	off := objHdrLen
	for _, e := range entries {
		size := 8
		if e.Type == RROSubIPv6 {
			size = 20
		}
		if size > len(buf)-off {
			return 0
		}
		switch e.Type {
		case RROSubIPv4:
			buf[off] = RROSubIPv4
			buf[off+1] = 8
			addr := e.Address.As4()
			copy(buf[off+2:off+6], addr[:])
			buf[off+6] = 32
			buf[off+7] = e.Flags
			off += 8
		case RROSubIPv6:
			buf[off] = RROSubIPv6
			buf[off+1] = 20
			addr := e.Address.As16()
			copy(buf[off+2:off+18], addr[:])
			buf[off+18] = 128
			buf[off+19] = e.Flags
			off += 20
		case RROSubLabel:
			buf[off] = RROSubLabel
			buf[off+1] = 8
			buf[off+2] = e.Flags
			buf[off+3] = 0
			binary.BigEndian.PutUint32(buf[off+4:off+8], e.Label)
			off += 8
		}
	}
	objLen := uint16(off)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassRecordRoute, CType: CTypeGeneric})
	return off
}

// decodeRRO reads an RRO object body.
func decodeRRO(body []byte) ([]rroEntry, error) {
	var entries []rroEntry
	off := 0
	for off < len(body) {
		// Cap the recorded route so a malformed or looping RRO cannot grow an
		// unbounded slice (and cannot be re-encoded past the message buffer).
		if len(entries) >= maxRecordRouteHops {
			break
		}
		if off+2 > len(body) {
			return entries, errShortRRO
		}
		subType := body[off]
		subLen := int(body[off+1])
		if subLen < 4 || off+subLen > len(body) {
			return entries, errShortRRO
		}
		switch subType {
		case RROSubIPv4:
			if subLen < 8 {
				return entries, errShortRRO
			}
			entries = append(entries, rroEntry{
				Type:    RROSubIPv4,
				Address: netip.AddrFrom4([4]byte(body[off+2 : off+6])),
				Flags:   body[off+7],
			})
		case RROSubIPv6:
			if subLen < 20 {
				return entries, errShortRRO
			}
			entries = append(entries, rroEntry{
				Type:    RROSubIPv6,
				Address: netip.AddrFrom16([16]byte(body[off+2 : off+18])),
				Flags:   body[off+19],
			})
		case RROSubLabel:
			if subLen < 8 {
				return entries, errShortRRO
			}
			entries = append(entries, rroEntry{
				Type:  RROSubLabel,
				Flags: body[off+2],
				Label: binary.BigEndian.Uint32(body[off+4 : off+8]),
			})
		}
		off += subLen
	}
	return entries, nil
}

// FlowSpec holds RFC 2210 token-bucket or RFC 2997 Null Service parameters.
type FlowSpec struct {
	// Service is the IntServ service number. Zero selects the standard token
	// bucket: service 1 in SENDER_TSPEC and service 5 in FLOWSPEC.
	Service        uint8
	TokenRate      float32
	TokenBucket    float32
	PeakRate       float32
	MinPolicedUnit uint32
	MaxPacketSize  uint32
}

// encodeFlowSpec writes a FLOWSPEC or SENDER_TSPEC into the caller's buffer.
// It returns the byte count, or -1 for insufficient space or unsupported service.
func encodeFlowSpec(buf []byte, classNum uint8, fs FlowSpec) int {
	if fs.Service == serviceNull {
		if len(buf) < 20 {
			return -1
		}
		// RFC 2997 Sections 4.3 and 4.4: service 6, parameter 128, one M word.
		// Bytes 4-7: version/length; 8-11: service; 12-15: parameter; 16-19: M.
		clear(buf[:20])
		encodeObjectHeader(buf, objectHeader{Length: 20, ClassNum: classNum, CType: 2})
		binary.BigEndian.PutUint16(buf[6:8], 3)
		buf[8] = serviceNull
		binary.BigEndian.PutUint16(buf[10:12], 2)
		buf[12] = 128
		binary.BigEndian.PutUint16(buf[14:16], 1)
		binary.BigEndian.PutUint32(buf[16:20], fs.MaxPacketSize)
		return 20
	}
	if fs.Service != 0 && fs.Service != serviceGeneral && fs.Service != serviceControlledLoad {
		return -1
	}
	if len(buf) < objHdrLen+32 {
		return -1
	}
	objLen := uint16(objHdrLen + 32)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: classNum, CType: 2})
	off := objHdrLen

	buf[off] = 0
	buf[off+1] = 0
	binary.BigEndian.PutUint16(buf[off+2:off+4], 7)
	off += 4

	// RFC 2210 Section 3.1: "The required RSVP SENDER_TSPEC object
	// contains a global Token_Bucket_TSpec parameter (service_number 1,
	// parameter 127, as defined in [RFC 2215])."
	buf[off] = serviceControlledLoad
	if classNum == ClassSenderTSpec {
		buf[off] = serviceGeneral
	}
	buf[off+1] = 0
	binary.BigEndian.PutUint16(buf[off+2:off+4], 6)
	off += 4

	buf[off] = 127
	buf[off+1] = 0
	binary.BigEndian.PutUint16(buf[off+2:off+4], 5)
	off += 4

	binary.BigEndian.PutUint32(buf[off:off+4], math.Float32bits(fs.TokenRate))
	off += 4
	binary.BigEndian.PutUint32(buf[off:off+4], math.Float32bits(fs.TokenBucket))
	off += 4
	binary.BigEndian.PutUint32(buf[off:off+4], math.Float32bits(fs.PeakRate))
	off += 4
	binary.BigEndian.PutUint32(buf[off:off+4], fs.MinPolicedUnit)
	off += 4
	binary.BigEndian.PutUint32(buf[off:off+4], fs.MaxPacketSize)
	return int(objLen)
}

// decodeFlowSpec reads RFC 2210 token buckets and RFC 2997 Null Service TSpecs.
// Every nested length is checked before the first parameter value is read.
func decodeFlowSpec(body []byte) (FlowSpec, error) {
	var fs FlowSpec
	if err := intservHeader(body); err != nil {
		return fs, err
	}
	for off := 4; off < len(body); {
		n, err := intservBlock(body[off:])
		if err != nil {
			return fs, err
		}
		service := body[off]
		if body[off+1]&intservBreak != 0 {
			return fs, errIntserv
		}
		if fs.Service != 0 {
			// RFC 2997 Section 3.2: "If guaranteed or controlled load
			// services are also offered in the ADSPEC, then the new Tspec
			// is appended following the standard Intserv token-bucket Tspec."
			if fs.Service != serviceGeneral || service != serviceNull || off != 32 {
				return fs, errIntserv
			}
		}
		var current FlowSpec
		current.Service = service
		switch service {
		case serviceGeneral, serviceControlledLoad:
			if n != 28 || body[off+4] != 127 || body[off+5]&intservBreak != 0 ||
				binary.BigEndian.Uint16(body[off+6:off+8]) != 5 {
				return fs, errIntserv
			}
			current.TokenRate = math.Float32frombits(binary.BigEndian.Uint32(body[off+8:off+12]))
			current.TokenBucket = math.Float32frombits(binary.BigEndian.Uint32(body[off+12:off+16]))
			current.PeakRate = math.Float32frombits(binary.BigEndian.Uint32(body[off+16:off+20]))
			current.MinPolicedUnit = binary.BigEndian.Uint32(body[off+20:off+24])
			current.MaxPacketSize = binary.BigEndian.Uint32(body[off+24:off+28])
			if current.TokenRate < 0 || math.IsNaN(float64(current.TokenRate)) ||
				math.IsInf(float64(current.TokenRate), 0) {
				return fs, errBadBandwidth
			}
		case serviceNull:
			if n != 12 || body[off+4] != 128 || body[off+5]&intservBreak != 0 ||
				binary.BigEndian.Uint16(body[off+6:off+8]) != 1 {
				return fs, errIntserv
			}
			current.MaxPacketSize = binary.BigEndian.Uint32(body[off+8:off+12])
		default:
			return fs, errIntserv
		}
		if fs.Service == 0 {
			fs = current
		}
		off += n
	}
	if fs.Service == 0 {
		return fs, errIntserv
	}
	return fs, nil
}

// errorSpec is the ERROR_SPEC object (RFC 2205 Section A.5).
type errorSpec struct {
	ErrorNode  netip.Addr
	Flags      uint8
	ErrorCode  uint8
	ErrorValue uint16
}

// encodeErrorSpec writes an ERROR_SPEC object. Returns bytes written.
func encodeErrorSpec(buf []byte, e errorSpec) int {
	objLen := uint16(objHdrLen + 8)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassErrorSpec, CType: CTypeIPv4})
	addr := e.ErrorNode.As4()
	copy(buf[4:8], addr[:])
	buf[8] = e.Flags
	buf[9] = e.ErrorCode
	binary.BigEndian.PutUint16(buf[10:12], e.ErrorValue)
	return int(objLen)
}

// decodeErrorSpec reads an ERROR_SPEC object body.
func decodeErrorSpec(body []byte) (errorSpec, error) {
	if len(body) < 8 {
		return errorSpec{}, errShortObject
	}
	return errorSpec{
		ErrorNode:  netip.AddrFrom4([4]byte(body[0:4])),
		Flags:      body[4],
		ErrorCode:  body[5],
		ErrorValue: binary.BigEndian.Uint16(body[6:8]),
	}, nil
}

// encodeStyle writes a STYLE object. Returns bytes written.
func encodeStyle(buf []byte, style uint32) int {
	objLen := uint16(objHdrLen + 4)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassStyle, CType: CTypeStyle})
	binary.BigEndian.PutUint32(buf[4:8], style)
	return int(objLen)
}

// ParsedMessage is a decoded RSVP message with all its objects.
type ParsedMessage struct {
	Header         Header
	Session        sessionIPv4
	SenderTemplate senderTemplateIPv4
	Hop            rsvpHop
	TimeValues     timeValues
	LabelRequest   labelRequest
	ERO            []eroHop
	RRO            []rroEntry
	SenderTSpec    FlowSpec
	ErrorSpec      errorSpec
	Style          uint32
	ResvConfirm    netip.Addr
	FastReroute    fastReroute
	SessionAttr    sessionAttribute
	// SessionAttrRaw is the SESSION_ATTRIBUTE object as it arrived, header
	// included. It aliases the decode buffer, so a holder that outlives the
	// packet copies it (handlePathTransit). RFC 3209 Section 4.7.4 requires a
	// transit to forward the object unmodified.
	SessionAttrRaw []byte
	// The raw objects alias the decode buffer. State holders MUST keep owned
	// copies before releasing the packet. Builders preserve opaque contents.
	AdspecRaw      []byte
	SenderTSpecRaw []byte
	// ForwardObjects retains unknown 11bbbbbb classes and opaque POLICY_DATA.
	ForwardObjects [][]byte
	FlowDescriptors []flowDescriptor
	// PathMTU comes only from a usable ADSPEC, including its service override.
	// Zero means no complete advertisement. The value includes MPLS labels.
	PathMTU uint32

	HasSession        bool
	HasSenderTemplate bool
	HasHop            bool
	HasTimeValues     bool
	HasLabelRequest   bool
	HasERO            bool
	HasRRO            bool
	HasSenderTSpec    bool
	HasErrorSpec      bool
	HasStyle          bool
	HasFastReroute    bool
	HasSessionAttr    bool
	HasResvConfirm bool
	HasAdspec bool

	// UnknownObject is the header of the first object whose class ze does not
	// implement and whose Class-Num high-order bit is zero. RFC 2205 Section 3.10
	// makes the whole message unacceptable then, so the caller MUST check
	// HasUnknownObject before it acts on any other field. The decoder records the
	// object rather than failing, because the caller needs the SESSION and the
	// SENDER_TEMPLATE that follow it to address the error message.
	UnknownObject    objectHeader
	HasUnknownObject bool
	UnknownCType bool
}

// classifyUnknownClass reports whether an object of this class, for which
// DecodeMessage has no case, makes the whole message unacceptable.
//
// RFC 2205 Section 3.10 chooses by the two high-order bits of the Class-Num.
// 0bbbbbbb rejects the message with an "Unknown Object Class" error, 10bbbbbb is
// ignored and no error is sent, and 11bbbbbb is ignored but retained in
// ForwardObjects for unchanged forwarding in resulting messages.
//
// RFC 4090 Section 4.2 rests on the first form: an LSR that does not support the
// DETOUR object (Class-Num 63) MUST reject a Path carrying one and send a PathErr
// to notify the PLR. ze gains DETOUR support by adding a case to DecodeMessage,
// which takes the object out of this default arm at the same time.
func classifyUnknownClass(classNum uint8) bool {
	if classNum&classNumIgnoreBit != 0 {
		return false
	}
	return !classKnownUnprocessed(classNum)
}

// classKnownUnprocessed reports whether an object class is one ze knows and has
// decided not to process. The RFC 2205 Section 3.10 rule covers a class the node
// does not KNOW, so an object listed here is tolerated rather than rejected:
// refusing it would deny a Path or a Resv that RFC 2205 Sections 3.1.3 and 3.1.4
// permit a conformant peer to send.
func classKnownUnprocessed(classNum uint8) bool {
	switch classNum {
	case ClassNull, ClassIntegrity, ClassScope, ClassPolicyData:
		return true
	}
	return false
}

// DecodeMessage parses a complete RSVP message from wire bytes.
func DecodeMessage(data []byte) (*ParsedMessage, error) {
	hdr, err := DecodeHeader(data)
	if err != nil {
		return nil, err
	}
	if int(hdr.Length) > len(data) {
		return nil, fmt.Errorf("rsvp: message length %d exceeds buffer %d", hdr.Length, len(data))
	}
	if hdr.Length < rsvpHdrLen {
		return nil, fmt.Errorf("rsvp: message length %d is smaller than its header", hdr.Length)
	}
	if hdr.Length%4 != 0 {
		return nil, errBadObjLen
	}
	if hdr.Length > maxRSVPPacket {
		return nil, fmt.Errorf("rsvp: message length %d exceeds carrier limit", hdr.Length)
	}
	// RFC 2205 Section 3.1.1: "An all-zero value means that no checksum
	// was transmitted." Otherwise verify exactly the declared RSVP message.
	if hdr.Checksum != 0 {
		if internetChecksum(data[:hdr.Length]) != 0 {
			return nil, errBadChecksum
		}
	}

	msg := &ParsedMessage{Header: hdr}
	var seen [256]bool

	off := rsvpHdrLen
	end := int(hdr.Length)
	for off < end {
		objHdr, err := decodeObjectHeader(data[off:end])
		if err != nil {
			return msg, err
		}
		if int(objHdr.Length)+off > end {
			return msg, fmt.Errorf("rsvp: object overflows message at offset %d", off)
		}
		body := data[off+objHdrLen : off+int(objHdr.Length)]
		if err := checkObjectPlacement(msg, objHdr, off, &seen); err != nil {
			return msg, err
		}
		if !knownCType(objHdr) {
			if !msg.HasUnknownObject {
				msg.UnknownObject, msg.HasUnknownObject, msg.UnknownCType = objHdr, true, true
			}
			off += int(objHdr.Length)
			continue
		}

		switch objHdr.ClassNum {
		case ClassSession:
			s, err := decodeSessionIPv4(body)
			if err != nil {
				return msg, err
			}
			msg.Session = s
			msg.HasSession = true
		case ClassSenderTemplate:
			st, err := decodeSenderTemplate(body)
			if err != nil {
				return msg, err
			}
			msg.SenderTemplate = st
			msg.HasSenderTemplate = true
		case ClassFilterSpec:
			st, err := decodeSenderTemplate(body)
			if err != nil {
				return msg, err
			}
			if err := msg.appendFilter(st); err != nil {
				return msg, err
			}
		case ClassRSVPHop:
			h, err := decodeRSVPHop(body)
			if err != nil {
				return msg, err
			}
			msg.Hop = h
			msg.HasHop = true
		case ClassTimeValues:
			tv, err := decodeTimeValues(body)
			if err != nil {
				return msg, err
			}
			msg.TimeValues = tv
			msg.HasTimeValues = true
		case ClassLabelRequest:
			lr, err := decodeLabelRequest(body)
			if err != nil {
				return msg, err
			}
			msg.LabelRequest = lr
			msg.HasLabelRequest = true
		case ClassLabel:
			l, err := decodeLabelObject(body)
			if err != nil {
				return msg, err
			}
			filter := msg.lastFilter()
			if filter == nil || filter.HasLabel || filter.HasRRO {
				return msg, fmt.Errorf("rsvp: misplaced or duplicate LABEL")
			}
			filter.Label, filter.HasLabel = l, true
		case ClassExplicitRoute:
			hops, err := decodeERO(body)
			if err != nil {
				return msg, err
			}
			msg.ERO = hops
			msg.HasERO = true
		case ClassRecordRoute:
			entries, err := decodeRRO(body)
			if err != nil {
				return msg, err
			}
			if reservationMessage(hdr.MsgType) {
				filter := msg.lastFilter()
				if filter == nil || !filter.HasLabel || filter.HasRRO {
					return msg, fmt.Errorf("rsvp: misplaced or duplicate RECORD_ROUTE")
				}
				filter.RRO, filter.HasRRO = entries, true
			} else {
				msg.RRO, msg.HasRRO = entries, true
			}
		case ClassFlowSpec:
			if hdr.MsgType == MsgTypeResvTear {
				off += int(objHdr.Length)
				continue
			}
			if objHdr.CType != 2 {
				return msg, errIntserv
			}
			fs, err := decodeFlowSpec(body)
			if err != nil {
				return msg, err
			}
			if fs.Service != serviceControlledLoad && fs.Service != serviceNull {
				return msg, errIntserv
			}
			if err := msg.appendFlowSpec(fs, data[off:off+int(objHdr.Length)]); err != nil {
				return msg, err
			}
		case ClassSenderTSpec:
			if hdr.MsgType == MsgTypePathTear {
				off += int(objHdr.Length)
				continue
			}
			if objHdr.CType != 2 {
				return msg, errIntserv
			}
			ts, err := decodeFlowSpec(body)
			if err != nil {
				return msg, err
			}
			if ts.Service != serviceGeneral && ts.Service != serviceNull {
				return msg, errIntserv
			}
			msg.SenderTSpec = ts
			msg.HasSenderTSpec = true
			msg.SenderTSpecRaw = data[off : off+int(objHdr.Length)]
		case ClassAdspec:
			if hdr.MsgType == MsgTypePathTear {
				off += int(objHdr.Length)
				continue
			}
			if msg.HasAdspec {
				return msg, errIntserv
			}
			raw := data[off : off+int(objHdr.Length)]
			if err := validateAdspec(raw); err != nil {
				return msg, err
			}
			msg.AdspecRaw = raw
			msg.HasAdspec = true
		case ClassErrorSpec:
			es, err := decodeErrorSpec(body)
			if err != nil {
				return msg, err
			}
			msg.ErrorSpec = es
			msg.HasErrorSpec = true
		case ClassResvConfirm:
			if objHdr.CType != CTypeIPv4 || len(body) != 4 {
				return msg, fmt.Errorf("rsvp: invalid IPv4 RESV_CONFIRM object")
			}
			msg.ResvConfirm = netip.AddrFrom4([4]byte(body))
			msg.HasResvConfirm = true
		case ClassStyle:
			if len(body) != 4 {
				return msg, errBadObjLen
			}
			msg.Style = binary.BigEndian.Uint32(body[0:4]) & 0x00ffffff
			msg.HasStyle = true
		case ClassFastReroute:
			fr, err := decodeFastReroute(body)
			if err != nil {
				return msg, err
			}
			msg.FastReroute = fr
			msg.HasFastReroute = true
		case ClassSessionAttr:
			// RFC 3209 Section 4.7.4: "If a Path message contains multiple
			// SESSION_ATTRIBUTE objects, only the first SESSION_ATTRIBUTE
			// object is meaningful."
			if msg.HasSessionAttr {
				break
			}
			sa, err := decodeSessionAttr(body, objHdr.CType)
			if err != nil {
				return msg, err
			}
			msg.SessionAttr = sa
			msg.SessionAttrRaw = data[off : off+int(objHdr.Length)]
			msg.HasSessionAttr = true
		default:
			// RFC 2205 Section 3.10: the Class-Num of an object ze has no case for
			// says whether the message survives it. classifyUnknownClass carries the
			// rule. Only the first such object is kept: it is the one the error
			// message reports, and the message is already rejected by then.
			if !msg.HasUnknownObject && classifyUnknownClass(objHdr.ClassNum) {
				msg.UnknownObject = objHdr
				msg.HasUnknownObject = true
			}
			// RFC 2205 Section 3.1.3: "Any POLICY_DATA, SENDER_TSPEC, and
			// ADSPEC objects are also saved in the path state."
			if objHdr.ClassNum&0xc0 == 0xc0 || objHdr.ClassNum == ClassPolicyData {
				msg.ForwardObjects = append(msg.ForwardObjects, data[off:off+int(objHdr.Length)])
			}
		}

		off += int(objHdr.Length)
	}

	msg.PathMTU = adspecPathMTU(msg.AdspecRaw, msg.SenderTSpec.Service)
	if msg.HasUnknownObject {
		return msg, nil
	}
	if err := checkMandatoryObjects(msg); err != nil {
		return msg, err
	}
	if err := checkFlowDescriptors(msg); err != nil {
		return msg, err
	}
	return msg, nil
}

// ValidateLabel checks that a label is within the 20-bit range.
func ValidateLabel(label uint32) error {
	if label > MaxLabel {
		return fmt.Errorf("%w: %d", errLabelRange, label)
	}
	return nil
}
