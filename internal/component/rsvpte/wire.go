// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE wire codec
// Related: build.go -- composes these object encoders into whole messages
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
)

// C-Types for objects.
const (
	CTypeLSPTunnelIPv4 uint8 = 7
	CTypeLSPTunnelIPv6 uint8 = 8
	CTypeIPv4          uint8 = 1
	CTypeIPv6          uint8 = 2
	CTypeGeneric       uint8 = 1
	CTypeLabel         uint8 = 1
	CTypeStyle         uint8 = 1
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

// EncodeHeader writes an RSVP common header. Returns bytes written.
func EncodeHeader(buf []byte, h Header) int {
	buf[0] = (h.Version << 4) | (h.Flags & 0x0F)
	buf[1] = h.MsgType
	binary.BigEndian.PutUint16(buf[2:4], h.Checksum)
	buf[4] = h.TTL
	buf[5] = 0
	binary.BigEndian.PutUint16(buf[6:8], h.Length)
	return rsvpHdrLen
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

// ObjectHeader is a generic RSVP object header (RFC 2205 Section 3.1.2).
type ObjectHeader struct {
	Length   uint16
	ClassNum uint8
	CType    uint8
}

// EncodeObjectHeader writes an object header. Returns bytes written.
func EncodeObjectHeader(buf []byte, o ObjectHeader) int {
	binary.BigEndian.PutUint16(buf[0:2], o.Length)
	buf[2] = o.ClassNum
	buf[3] = o.CType
	return objHdrLen
}

// DecodeObjectHeader reads an object header.
func DecodeObjectHeader(buf []byte) (ObjectHeader, error) {
	if len(buf) < objHdrLen {
		return ObjectHeader{}, errShortObject
	}
	o := ObjectHeader{
		Length:   binary.BigEndian.Uint16(buf[0:2]),
		ClassNum: buf[2],
		CType:    buf[3],
	}
	if o.Length < objHdrLen {
		return ObjectHeader{}, fmt.Errorf("%w: %d", errBadObjLen, o.Length)
	}
	return o, nil
}

// SessionIPv4 is the SESSION object for LSP tunnels (RFC 3209 Section 4.6.1).
type SessionIPv4 struct {
	TunnelEndpoint netip.Addr
	TunnelID       uint16
	ExtTunnelID    uint32
}

// EncodeSessionIPv4 writes a SESSION object. Returns bytes written.
func EncodeSessionIPv4(buf []byte, s SessionIPv4) int {
	objLen := uint16(objHdrLen + 12)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassSession, CType: CTypeLSPTunnelIPv4})
	addr := s.TunnelEndpoint.As4()
	copy(buf[4:8], addr[:])
	buf[8] = 0
	buf[9] = 0
	binary.BigEndian.PutUint16(buf[10:12], s.TunnelID)
	binary.BigEndian.PutUint32(buf[12:16], s.ExtTunnelID)
	return int(objLen)
}

// DecodeSessionIPv4 reads a SESSION object body (after object header).
func DecodeSessionIPv4(body []byte) (SessionIPv4, error) {
	if len(body) < 12 {
		return SessionIPv4{}, errShortObject
	}
	s := SessionIPv4{
		TunnelEndpoint: netip.AddrFrom4([4]byte(body[0:4])),
		TunnelID:       binary.BigEndian.Uint16(body[6:8]),
		ExtTunnelID:    binary.BigEndian.Uint32(body[8:12]),
	}
	return s, nil
}

// SenderTemplateIPv4 is the SENDER_TEMPLATE object (RFC 3209 Section 4.6.2).
type SenderTemplateIPv4 struct {
	SenderAddr netip.Addr
	LSPID      uint16
}

// EncodeSenderTemplate writes a SENDER_TEMPLATE object. Returns bytes written.
func EncodeSenderTemplate(buf []byte, st SenderTemplateIPv4) int {
	objLen := uint16(objHdrLen + 8)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassSenderTemplate, CType: CTypeLSPTunnelIPv4})
	addr := st.SenderAddr.As4()
	copy(buf[4:8], addr[:])
	buf[8] = 0
	buf[9] = 0
	binary.BigEndian.PutUint16(buf[10:12], st.LSPID)
	return int(objLen)
}

// DecodeSenderTemplate reads a SENDER_TEMPLATE object body.
func DecodeSenderTemplate(body []byte) (SenderTemplateIPv4, error) {
	if len(body) < 8 {
		return SenderTemplateIPv4{}, errShortObject
	}
	return SenderTemplateIPv4{
		SenderAddr: netip.AddrFrom4([4]byte(body[0:4])),
		LSPID:      binary.BigEndian.Uint16(body[6:8]),
	}, nil
}

// RSVPHop is the RSVP_HOP object (RFC 2205 Section A.2).
type RSVPHop struct {
	NextHop netip.Addr
	LIH     uint32
}

// EncodeRSVPHop writes an RSVP_HOP object. Returns bytes written.
func EncodeRSVPHop(buf []byte, h RSVPHop) int {
	objLen := uint16(objHdrLen + 8)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassRSVPHop, CType: CTypeIPv4})
	addr := h.NextHop.As4()
	copy(buf[4:8], addr[:])
	binary.BigEndian.PutUint32(buf[8:12], h.LIH)
	return int(objLen)
}

// DecodeRSVPHop reads an RSVP_HOP object body.
func DecodeRSVPHop(body []byte) (RSVPHop, error) {
	if len(body) < 8 {
		return RSVPHop{}, errShortObject
	}
	return RSVPHop{
		NextHop: netip.AddrFrom4([4]byte(body[0:4])),
		LIH:     binary.BigEndian.Uint32(body[4:8]),
	}, nil
}

// TimeValues is the TIME_VALUES object (RFC 2205 Section A.4).
type TimeValues struct {
	RefreshPeriod uint32
}

// EncodeTimeValues writes a TIME_VALUES object. Returns bytes written.
func EncodeTimeValues(buf []byte, tv TimeValues) int {
	objLen := uint16(objHdrLen + 4)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassTimeValues, CType: CTypeGeneric})
	binary.BigEndian.PutUint32(buf[4:8], tv.RefreshPeriod)
	return int(objLen)
}

// DecodeTimeValues reads a TIME_VALUES object body.
func DecodeTimeValues(body []byte) (TimeValues, error) {
	if len(body) < 4 {
		return TimeValues{}, errShortObject
	}
	return TimeValues{
		RefreshPeriod: binary.BigEndian.Uint32(body[0:4]),
	}, nil
}

// LabelRequest is the LABEL_REQUEST object (RFC 3209 Section 4.2).
type LabelRequest struct {
	L3PID uint16
}

// EncodeLabelRequest writes a LABEL_REQUEST object. Returns bytes written.
func EncodeLabelRequest(buf []byte, lr LabelRequest) int {
	objLen := uint16(objHdrLen + 4)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassLabelRequest, CType: CTypeGeneric})
	buf[4] = 0
	buf[5] = 0
	binary.BigEndian.PutUint16(buf[6:8], lr.L3PID)
	return int(objLen)
}

// DecodeLabelRequest reads a LABEL_REQUEST object body.
func DecodeLabelRequest(body []byte) (LabelRequest, error) {
	if len(body) < 4 {
		return LabelRequest{}, errShortObject
	}
	return LabelRequest{
		L3PID: binary.BigEndian.Uint16(body[2:4]),
	}, nil
}

// LabelObject is the LABEL object (RFC 3209 Section 4.1).
type LabelObject struct {
	Label uint32
}

// EncodeLabelObject writes a LABEL object. Returns bytes written.
func EncodeLabelObject(buf []byte, l LabelObject) int {
	objLen := uint16(objHdrLen + 4)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassLabel, CType: CTypeLabel})
	binary.BigEndian.PutUint32(buf[4:8], l.Label)
	return int(objLen)
}

// DecodeLabelObject reads a LABEL object body.
func DecodeLabelObject(body []byte) (LabelObject, error) {
	if len(body) < 4 {
		return LabelObject{}, errShortObject
	}
	label := binary.BigEndian.Uint32(body[0:4])
	if label > MaxLabel {
		return LabelObject{}, fmt.Errorf("%w: %d", errLabelRange, label)
	}
	return LabelObject{Label: label}, nil
}

// EROHop is a single hop in an Explicit Route Object.
type EROHop struct {
	Loose   bool
	Address netip.Prefix
}

// EncodeERO writes an ERO object with the given hops. Returns bytes written.
func EncodeERO(buf []byte, hops []EROHop) int {
	off := objHdrLen
	for _, h := range hops {
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
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassExplicitRoute, CType: CTypeGeneric})
	return off
}

// DecodeERO reads an ERO object body and returns the hops.
func DecodeERO(body []byte) ([]EROHop, error) {
	var hops []EROHop
	off := 0
	for off < len(body) {
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
			hops = append(hops, EROHop{Loose: loose, Address: netip.PrefixFrom(addr, bits)})
		case EROSubIPv6Prefix:
			if subLen < 20 {
				return hops, errShortERO
			}
			addr := netip.AddrFrom16([16]byte(body[off+2 : off+18]))
			bits := int(body[off+18])
			hops = append(hops, EROHop{Loose: loose, Address: netip.PrefixFrom(addr, bits)})
		}
		off += subLen
	}
	return hops, nil
}

// RROEntry is a single entry in a Record Route Object.
type RROEntry struct {
	Type    uint8
	Address netip.Addr
	Label   uint32
	Flags   uint8
}

// EncodeRRO writes an RRO object. Returns bytes written.
func EncodeRRO(buf []byte, entries []RROEntry) int {
	off := objHdrLen
	for _, e := range entries {
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
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassRecordRoute, CType: CTypeGeneric})
	return off
}

// DecodeRRO reads an RRO object body.
func DecodeRRO(body []byte) ([]RROEntry, error) {
	var entries []RROEntry
	off := 0
	for off < len(body) {
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
			entries = append(entries, RROEntry{
				Type:    RROSubIPv4,
				Address: netip.AddrFrom4([4]byte(body[off+2 : off+6])),
				Flags:   body[off+7],
			})
		case RROSubIPv6:
			if subLen < 20 {
				return entries, errShortRRO
			}
			entries = append(entries, RROEntry{
				Type:    RROSubIPv6,
				Address: netip.AddrFrom16([16]byte(body[off+2 : off+18])),
				Flags:   body[off+19],
			})
		case RROSubLabel:
			if subLen < 8 {
				return entries, errShortRRO
			}
			entries = append(entries, RROEntry{
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

// EncodeFlowSpec writes a FLOWSPEC (or SENDER_TSPEC) object. Returns bytes written.
func EncodeFlowSpec(buf []byte, classNum uint8, fs FlowSpec) int {
	objLen := uint16(objHdrLen + 32)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: classNum, CType: 2})
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

// DecodeFlowSpec reads a FLOWSPEC or SENDER_TSPEC object body.
func DecodeFlowSpec(body []byte) (FlowSpec, error) {
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

// ErrorSpec is the ERROR_SPEC object (RFC 2205 Section A.5).
type ErrorSpec struct {
	ErrorNode  netip.Addr
	Flags      uint8
	ErrorCode  uint8
	ErrorValue uint16
}

// EncodeErrorSpec writes an ERROR_SPEC object. Returns bytes written.
func EncodeErrorSpec(buf []byte, e ErrorSpec) int {
	objLen := uint16(objHdrLen + 8)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassErrorSpec, CType: CTypeIPv4})
	addr := e.ErrorNode.As4()
	copy(buf[4:8], addr[:])
	buf[8] = e.Flags
	buf[9] = e.ErrorCode
	binary.BigEndian.PutUint16(buf[10:12], e.ErrorValue)
	return int(objLen)
}

// DecodeErrorSpec reads an ERROR_SPEC object body.
func DecodeErrorSpec(body []byte) (ErrorSpec, error) {
	if len(body) < 8 {
		return ErrorSpec{}, errShortObject
	}
	return ErrorSpec{
		ErrorNode:  netip.AddrFrom4([4]byte(body[0:4])),
		Flags:      body[4],
		ErrorCode:  body[5],
		ErrorValue: binary.BigEndian.Uint16(body[6:8]),
	}, nil
}

// EncodeStyle writes a STYLE object. Returns bytes written.
func EncodeStyle(buf []byte, style uint32) int {
	objLen := uint16(objHdrLen + 4)
	EncodeObjectHeader(buf, ObjectHeader{Length: objLen, ClassNum: ClassStyle, CType: CTypeStyle})
	binary.BigEndian.PutUint32(buf[4:8], style)
	return int(objLen)
}

// ParsedMessage is a decoded RSVP message with all its objects.
type ParsedMessage struct {
	Header         Header
	Session        SessionIPv4
	SenderTemplate SenderTemplateIPv4
	Hop            RSVPHop
	TimeValues     TimeValues
	LabelRequest   LabelRequest
	Label          LabelObject
	ERO            []EROHop
	RRO            []RROEntry
	FlowSpec       FlowSpec
	SenderTSpec    FlowSpec
	ErrorSpec      ErrorSpec
	Style          uint32

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
		objHdr, err := DecodeObjectHeader(data[off:])
		if err != nil {
			return msg, err
		}
		if int(objHdr.Length)+off > end {
			return msg, fmt.Errorf("rsvp: object overflows message at offset %d", off)
		}
		body := data[off+objHdrLen : off+int(objHdr.Length)]

		switch objHdr.ClassNum {
		case ClassSession:
			s, err := DecodeSessionIPv4(body)
			if err != nil {
				return msg, err
			}
			msg.Session = s
			msg.HasSession = true
		case ClassSenderTemplate, ClassFilterSpec:
			st, err := DecodeSenderTemplate(body)
			if err != nil {
				return msg, err
			}
			msg.SenderTemplate = st
			msg.HasSenderTemplate = true
		case ClassRSVPHop:
			h, err := DecodeRSVPHop(body)
			if err != nil {
				return msg, err
			}
			msg.Hop = h
			msg.HasHop = true
		case ClassTimeValues:
			tv, err := DecodeTimeValues(body)
			if err != nil {
				return msg, err
			}
			msg.TimeValues = tv
			msg.HasTimeValues = true
		case ClassLabelRequest:
			lr, err := DecodeLabelRequest(body)
			if err != nil {
				return msg, err
			}
			msg.LabelRequest = lr
			msg.HasLabelRequest = true
		case ClassLabel:
			l, err := DecodeLabelObject(body)
			if err != nil {
				return msg, err
			}
			msg.Label = l
			msg.HasLabel = true
		case ClassExplicitRoute:
			hops, err := DecodeERO(body)
			if err != nil {
				return msg, err
			}
			msg.ERO = hops
			msg.HasERO = true
		case ClassRecordRoute:
			entries, err := DecodeRRO(body)
			if err != nil {
				return msg, err
			}
			msg.RRO = entries
			msg.HasRRO = true
		case ClassFlowSpec:
			fs, err := DecodeFlowSpec(body)
			if err != nil {
				return msg, err
			}
			msg.FlowSpec = fs
			msg.HasFlowSpec = true
		case ClassSenderTSpec:
			ts, err := DecodeFlowSpec(body)
			if err != nil {
				return msg, err
			}
			msg.SenderTSpec = ts
			msg.HasSenderTSpec = true
		case ClassErrorSpec:
			es, err := DecodeErrorSpec(body)
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
