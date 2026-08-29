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
	// RFC 4090 Section 4: Fast Reroute object classes.
	ClassFastReroute uint8 = 205
	ClassDetour      uint8 = 63
)

// Object classes ze knows and deliberately reads no body for. RFC 2205 Sections
// 3.1.3 and 3.1.4 make each one optional in the Path or Resv message that
// carries it, so a conformant peer can send it at any time. Their Class-Num
// high-order bit is zero, which is why they are named here: without this list
// classifyUnknownClass would reject them as unknown and refuse a legal message.
const (
	ClassIntegrity   uint8 = 4  // RFC 2205 Section A.3: ze implements no RSVP authentication.
	ClassScope       uint8 = 7  // RFC 2205 Section A.6: WF style only; ze signals FF and SE LSPs.
	ClassAdspec      uint8 = 13 // RFC 2205 Section A.12: Int-Serv advertisement; label signaling does not read it.
	ClassPolicyData  uint8 = 14 // RFC 2205 Section A.13: ze runs no policy module.
	ClassResvConfirm uint8 = 15 // RFC 2205 Section A.14: ze requests no reservation confirmation.
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
	errBadBandwidth = errors.New("rsvp: negative bandwidth")
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
	if len(body) < 12 {
		return sessionIPv4{}, errShortObject
	}
	s := sessionIPv4{
		TunnelEndpoint: netip.AddrFrom4([4]byte(body[0:4])),
		TunnelID:       binary.BigEndian.Uint16(body[6:8]),
		ExtTunnelID:    binary.BigEndian.Uint32(body[8:12]),
	}
	return s, nil
}

// senderTemplateIPv4 is the SENDER_TEMPLATE object (RFC 3209 Section 4.6.2).
type senderTemplateIPv4 struct {
	SenderAddr netip.Addr
	LSPID      uint16
}

// encodeSenderTemplate writes a SENDER_TEMPLATE object. Returns bytes written.
func encodeSenderTemplate(buf []byte, st senderTemplateIPv4) int {
	objLen := uint16(objHdrLen + 8)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: ClassSenderTemplate, CType: CTypeLSPTunnelIPv4})
	addr := st.SenderAddr.As4()
	copy(buf[4:8], addr[:])
	buf[8] = 0
	buf[9] = 0
	binary.BigEndian.PutUint16(buf[10:12], st.LSPID)
	return int(objLen)
}

// decodeSenderTemplate reads a SENDER_TEMPLATE object body.
func decodeSenderTemplate(body []byte) (senderTemplateIPv4, error) {
	if len(body) < 8 {
		return senderTemplateIPv4{}, errShortObject
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

// encodeRRO writes an RRO object. Returns bytes written.
func encodeRRO(buf []byte, entries []rroEntry) int {
	off := objHdrLen
	for _, e := range entries {
		// Never write past the fixed message buffer: each subobject is at most
		// 20 bytes (IPv6). Stop early rather than overflow if a caller passes an
		// over-long RRO (defense in depth; callers also cap via prependRRO).
		if off+20 > len(buf) {
			break
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

// FlowSpec encodes token-bucket parameters (RFC 2210 / RFC 2215).
type FlowSpec struct {
	TokenRate      float32
	TokenBucket    float32
	PeakRate       float32
	MinPolicedUnit uint32
	MaxPacketSize  uint32
}

// encodeFlowSpec writes a FLOWSPEC (or SENDER_TSPEC) object. Returns bytes written.
func encodeFlowSpec(buf []byte, classNum uint8, fs FlowSpec) int {
	objLen := uint16(objHdrLen + 32)
	encodeObjectHeader(buf, objectHeader{Length: objLen, ClassNum: classNum, CType: 2})
	off := objHdrLen

	buf[off] = 0
	buf[off+1] = 0
	binary.BigEndian.PutUint16(buf[off+2:off+4], 7)
	off += 4

	buf[off] = 5
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

// decodeFlowSpec reads a FLOWSPEC or SENDER_TSPEC object body.
func decodeFlowSpec(body []byte) (FlowSpec, error) {
	var fs FlowSpec
	if len(body) < 32 {
		return fs, errShortObject
	}
	off := 12
	fs.TokenRate = math.Float32frombits(binary.BigEndian.Uint32(body[off : off+4]))
	off += 4
	fs.TokenBucket = math.Float32frombits(binary.BigEndian.Uint32(body[off : off+4]))
	off += 4
	fs.PeakRate = math.Float32frombits(binary.BigEndian.Uint32(body[off : off+4]))
	off += 4
	fs.MinPolicedUnit = binary.BigEndian.Uint32(body[off : off+4])
	off += 4
	fs.MaxPacketSize = binary.BigEndian.Uint32(body[off : off+4])
	if fs.TokenRate < 0 {
		return fs, errBadBandwidth
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
	Label          labelObject
	ERO            []eroHop
	RRO            []rroEntry
	FlowSpec       FlowSpec
	SenderTSpec    FlowSpec
	ErrorSpec      errorSpec
	Style          uint32
	FastReroute    fastReroute
	SessionAttr    sessionAttribute

	HasSession        bool
	HasSenderTemplate bool
	HasHop            bool
	HasTimeValues     bool
	HasLabelRequest   bool
	HasLabel          bool
	HasERO            bool
	HasRRO            bool
	HasFlowSpec       bool
	HasSenderTSpec    bool
	HasErrorSpec      bool
	HasStyle          bool
	HasFastReroute    bool
	HasSessionAttr    bool

	// UnknownObject is the header of the first object whose class ze does not
	// implement and whose Class-Num high-order bit is zero. RFC 2205 Section 3.10
	// makes the whole message unacceptable then, so the caller MUST check
	// HasUnknownObject before it acts on any other field. The decoder records the
	// object rather than failing, because the caller needs the SESSION and the
	// SENDER_TEMPLATE that follow it to address the error message.
	UnknownObject    objectHeader
	HasUnknownObject bool
}

// classifyUnknownClass reports whether an object of this class, for which
// DecodeMessage has no case, makes the whole message unacceptable.
//
// RFC 2205 Section 3.10 chooses by the two high-order bits of the Class-Num.
// 0bbbbbbb rejects the message with an "Unknown Object Class" error, 10bbbbbb is
// ignored and no error is sent, and 11bbbbbb is ignored but forwarded unexamined
// in every message that results from this one. ze forwards no object it did not
// decode, so the last two forms are both a plain ignore here.
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
	case ClassIntegrity, ClassScope, ClassAdspec, ClassPolicyData, ClassResvConfirm:
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

	msg := &ParsedMessage{Header: hdr}

	off := rsvpHdrLen
	end := int(hdr.Length)
	for off < end {
		objHdr, err := decodeObjectHeader(data[off:])
		if err != nil {
			return msg, err
		}
		if int(objHdr.Length)+off > end {
			return msg, fmt.Errorf("rsvp: object overflows message at offset %d", off)
		}
		body := data[off+objHdrLen : off+int(objHdr.Length)]

		switch objHdr.ClassNum {
		case ClassSession:
			s, err := decodeSessionIPv4(body)
			if err != nil {
				return msg, err
			}
			msg.Session = s
			msg.HasSession = true
		case ClassSenderTemplate, ClassFilterSpec:
			st, err := decodeSenderTemplate(body)
			if err != nil {
				return msg, err
			}
			msg.SenderTemplate = st
			msg.HasSenderTemplate = true
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
			msg.Label = l
			msg.HasLabel = true
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
			msg.RRO = entries
			msg.HasRRO = true
		case ClassFlowSpec:
			fs, err := decodeFlowSpec(body)
			if err != nil {
				return msg, err
			}
			msg.FlowSpec = fs
			msg.HasFlowSpec = true
		case ClassSenderTSpec:
			ts, err := decodeFlowSpec(body)
			if err != nil {
				return msg, err
			}
			msg.SenderTSpec = ts
			msg.HasSenderTSpec = true
		case ClassErrorSpec:
			es, err := decodeErrorSpec(body)
			if err != nil {
				return msg, err
			}
			msg.ErrorSpec = es
			msg.HasErrorSpec = true
		case ClassStyle:
			if len(body) >= 4 {
				msg.Style = binary.BigEndian.Uint32(body[0:4])
				msg.HasStyle = true
			}
		case ClassFastReroute:
			fr, err := decodeFastReroute(body)
			if err != nil {
				return msg, err
			}
			msg.FastReroute = fr
			msg.HasFastReroute = true
		case ClassSessionAttr:
			sa, err := decodeSessionAttr(body, objHdr.CType)
			if err != nil {
				return msg, err
			}
			msg.SessionAttr = sa
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
		}

		off += int(objHdr.Length)
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
