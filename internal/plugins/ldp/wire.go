// Design: docs/architecture/ldp/mpls-ldp.md -- LDP wire codec
// Related: session.go -- decodes these messages in the read loop
// Related: register.go -- discovery encodes/decodes Hello via this codec
//
// LDP message encoding/decoding per RFC 5036. LDP uses TLV-based messages
// over TCP (sessions) and UDP (discovery). All multi-byte fields are
// big-endian.
//
// Message format: LDP Header (10 bytes) + Message (variable).
// LDP Header: Version (2) + PDU Length (2) + LSR ID (4) + Label Space (2).
// Message: Type (2) + Length (2) + Message ID (4) + [Mandatory TLVs] + [Optional TLVs].
package ldp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
)

// RFC 5036 Section 3.5: LDP PDU header.
const (
	ldpVersion     = 1
	ldpHeaderLen   = 10
	ldpMsgHdrLen   = 8
	ldpTLVHdrLen   = 4
	ldpHelloPort   = 646
	ldpSessionPort = 646
)

// RFC 5036 Section 3.5.1: Message types.
const (
	MsgTypeNotification    uint16 = 0x0001
	MsgTypeHello           uint16 = 0x0100
	MsgTypeInitialize      uint16 = 0x0200
	MsgTypeKeepAlive       uint16 = 0x0201
	MsgTypeAddress         uint16 = 0x0300
	MsgTypeAddressWithdraw uint16 = 0x0301
	MsgTypeLabelMapping    uint16 = 0x0400
	MsgTypeLabelRequest    uint16 = 0x0401
	MsgTypeLabelWithdraw   uint16 = 0x0402
	MsgTypeLabelRelease    uint16 = 0x0403
	MsgTypeLabelAbortReq   uint16 = 0x0404
)

// RFC 5036 Section 3.4.1: TLV types.
const (
	TLVTypeFEC             uint16 = 0x0100
	TLVTypeAddressList     uint16 = 0x0101
	TLVTypeHopCount        uint16 = 0x0103
	TLVTypePathVector      uint16 = 0x0104
	TLVTypeGenericLabel    uint16 = 0x0200
	TLVTypeATMLabel        uint16 = 0x0201
	TLVTypeFRLabel         uint16 = 0x0202
	TLVTypeStatus          uint16 = 0x0300
	TLVTypeExtStatus       uint16 = 0x0301
	TLVTypeReturnedPDU     uint16 = 0x0302
	TLVTypeReturnedMsg     uint16 = 0x0303
	TLVTypeCommonHello     uint16 = 0x0400
	TLVTypeIPv4Transport   uint16 = 0x0401
	TLVTypeConfigSeqNo     uint16 = 0x0402
	TLVTypeIPv6Transport   uint16 = 0x0403
	TLVTypeCommonSession   uint16 = 0x0500
	TLVTypeATMSessionParam uint16 = 0x0501
	TLVTypeFRSessionParam  uint16 = 0x0502
)

// RFC 5036 Section 3.4.1: FEC element types.
const (
	FECWildcard uint8 = 0x01
	FECPrefix   uint8 = 0x02
)

// RFC 5036 Section 3.4.1: Address family numbers.
const (
	AFIIPv4 uint16 = 1
	AFIIPv6 uint16 = 2
)

// RFC 3032 Section 2.1: 20-bit label field.
const (
	MaxLabel      = 1048575
	MaxLabelStack = 16

	ImplicitNull uint32 = 3
	ExplicitNull uint32 = 0
)

var (
	errShortPDU     = errors.New("ldp: PDU too short")
	errBadVersion   = errors.New("ldp: unsupported version")
	errShortMessage = errors.New("ldp: message too short")
	errShortTLV     = errors.New("ldp: TLV too short")
	errLabelRange   = errors.New("ldp: label out of 20-bit range")
)

// PDUHeader is the LDP PDU header (RFC 5036 Section 3.5).
type PDUHeader struct {
	Version    uint16
	PDULength  uint16
	LSRID      [4]byte
	LabelSpace uint16
}

// encodePDUHeader writes a PDU header to buf. Returns bytes written.
func encodePDUHeader(buf []byte, h PDUHeader) int {
	binary.BigEndian.PutUint16(buf[0:2], h.Version)
	binary.BigEndian.PutUint16(buf[2:4], h.PDULength)
	copy(buf[4:8], h.LSRID[:])
	binary.BigEndian.PutUint16(buf[8:10], h.LabelSpace)
	return ldpHeaderLen
}

// decodePDUHeader reads a PDU header from buf.
func decodePDUHeader(buf []byte) (PDUHeader, error) {
	if len(buf) < ldpHeaderLen {
		return PDUHeader{}, errShortPDU
	}
	h := PDUHeader{
		Version:    binary.BigEndian.Uint16(buf[0:2]),
		PDULength:  binary.BigEndian.Uint16(buf[2:4]),
		LabelSpace: binary.BigEndian.Uint16(buf[8:10]),
	}
	copy(h.LSRID[:], buf[4:8])
	if h.Version != ldpVersion {
		return PDUHeader{}, fmt.Errorf("%w: got %d", errBadVersion, h.Version)
	}
	return h, nil
}

// MessageHeader is the common message header (RFC 5036 Section 3.5.1).
type MessageHeader struct {
	Type      uint16
	Length    uint16
	MessageID uint32
}

// encodeMessageHeader writes a message header to buf. Returns bytes written.
func encodeMessageHeader(buf []byte, h MessageHeader) int {
	binary.BigEndian.PutUint16(buf[0:2], h.Type)
	binary.BigEndian.PutUint16(buf[2:4], h.Length)
	binary.BigEndian.PutUint32(buf[4:8], h.MessageID)
	return ldpMsgHdrLen
}

// decodeMessageHeader reads a message header from buf.
func decodeMessageHeader(buf []byte) (MessageHeader, error) {
	if len(buf) < ldpMsgHdrLen {
		return MessageHeader{}, errShortMessage
	}
	return MessageHeader{
		Type:      binary.BigEndian.Uint16(buf[0:2]),
		Length:    binary.BigEndian.Uint16(buf[2:4]),
		MessageID: binary.BigEndian.Uint32(buf[4:8]),
	}, nil
}

// TLV is a generic Type-Length-Value container.
type TLV struct {
	Type   uint16
	Length uint16
	Value  []byte
}

// EncodeTLV writes a TLV to buf. Returns bytes written.
func EncodeTLV(buf []byte, t TLV) int {
	binary.BigEndian.PutUint16(buf[0:2], t.Type)
	binary.BigEndian.PutUint16(buf[2:4], t.Length)
	copy(buf[4:4+t.Length], t.Value)
	return ldpTLVHdrLen + int(t.Length)
}

// DecodeTLV reads a TLV from buf.
func DecodeTLV(buf []byte) (TLV, int, error) {
	if len(buf) < ldpTLVHdrLen {
		return TLV{}, 0, errShortTLV
	}
	t := TLV{
		Type:   binary.BigEndian.Uint16(buf[0:2]),
		Length: binary.BigEndian.Uint16(buf[2:4]),
	}
	total := ldpTLVHdrLen + int(t.Length)
	if len(buf) < total {
		return TLV{}, 0, fmt.Errorf("%w: need %d, have %d", errShortTLV, total, len(buf))
	}
	t.Value = buf[ldpTLVHdrLen:total]
	return t, total, nil
}

// HelloMessage represents an LDP Hello (RFC 5036 Section 3.5.2).
type HelloMessage struct {
	MessageID     uint32
	HoldTime      uint16
	Targeted      bool
	RequestTarget bool
	TransportAddr netip.Addr
}

// EncodeHello writes a Hello message body (after PDU header) to buf.
// Returns total bytes written.
func EncodeHello(buf []byte, h HelloMessage) int {
	off := encodeMessageHeader(buf, MessageHeader{
		Type:      MsgTypeHello,
		MessageID: h.MessageID,
	})

	// Common Hello Parameters TLV: RFC 5036 Section 3.5.2.
	var flags uint16
	if h.Targeted {
		flags |= 0x8000
	}
	if h.RequestTarget {
		flags |= 0x4000
	}
	binary.BigEndian.PutUint16(buf[off+4:off+6], h.HoldTime)
	binary.BigEndian.PutUint16(buf[off+6:off+8], flags)
	n := EncodeTLV(buf[off:], TLV{
		Type:   TLVTypeCommonHello,
		Length: 4,
		Value:  buf[off+4 : off+8],
	})
	off += n

	// Optional IPv4/IPv6 transport address TLV.
	if h.TransportAddr.IsValid() {
		if h.TransportAddr.Is4() {
			addr4 := h.TransportAddr.As4()
			off += EncodeTLV(buf[off:], TLV{
				Type:   TLVTypeIPv4Transport,
				Length: 4,
				Value:  addr4[:],
			})
		} else {
			addr16 := h.TransportAddr.As16()
			off += EncodeTLV(buf[off:], TLV{
				Type:   TLVTypeIPv6Transport,
				Length: 16,
				Value:  addr16[:],
			})
		}
	}

	// Patch message length: total minus message header type+length fields.
	binary.BigEndian.PutUint16(buf[2:4], uint16(off-ldpTLVHdrLen))

	return off
}

// DecodeHello reads a Hello message body from buf (after message header).
func DecodeHello(msgID uint32, body []byte) (HelloMessage, error) {
	h := HelloMessage{MessageID: msgID}
	off := 0
	for off < len(body) {
		tlv, n, err := DecodeTLV(body[off:])
		if err != nil {
			return h, err
		}
		switch tlv.Type {
		case TLVTypeCommonHello:
			if len(tlv.Value) >= 4 {
				h.HoldTime = binary.BigEndian.Uint16(tlv.Value[0:2])
				flags := binary.BigEndian.Uint16(tlv.Value[2:4])
				h.Targeted = flags&0x8000 != 0
				h.RequestTarget = flags&0x4000 != 0
			}
		case TLVTypeIPv4Transport:
			if len(tlv.Value) >= 4 {
				h.TransportAddr = netip.AddrFrom4([4]byte(tlv.Value[:4]))
			}
		case TLVTypeIPv6Transport:
			if len(tlv.Value) >= 16 {
				h.TransportAddr = netip.AddrFrom16([16]byte(tlv.Value[:16]))
			}
		}
		off += n
	}
	return h, nil
}

// initMessage represents an LDP Initialization message (RFC 5036 Section 3.5.3).
type initMessage struct {
	MessageID          uint32
	ProtocolVersion    uint16
	KeepaliveTime      uint16
	MaxPDULength       uint16
	ReceiverLSRID      [4]byte
	ReceiverLabelSpace uint16
	LoopDetection      bool
	PathVectorLimit    uint8
}

// EncodeInit writes an Initialization message body to buf.
func EncodeInit(buf []byte, m initMessage) int {
	off := encodeMessageHeader(buf, MessageHeader{
		Type:      MsgTypeInitialize,
		MessageID: m.MessageID,
	})

	// Common Session Parameters TLV: RFC 5036 Section 3.5.3.
	var sessionBuf [14]byte
	binary.BigEndian.PutUint16(sessionBuf[0:2], m.ProtocolVersion)
	binary.BigEndian.PutUint16(sessionBuf[2:4], m.KeepaliveTime)
	var flags uint8
	if m.LoopDetection {
		flags |= 0x04
	}
	sessionBuf[4] = flags
	sessionBuf[5] = m.PathVectorLimit
	binary.BigEndian.PutUint16(sessionBuf[6:8], m.MaxPDULength)
	copy(sessionBuf[8:12], m.ReceiverLSRID[:])
	binary.BigEndian.PutUint16(sessionBuf[12:14], m.ReceiverLabelSpace)

	off += EncodeTLV(buf[off:], TLV{
		Type:   TLVTypeCommonSession,
		Length: 14,
		Value:  sessionBuf[:],
	})

	binary.BigEndian.PutUint16(buf[2:4], uint16(off-ldpTLVHdrLen))
	return off
}

// DecodeInit reads an Initialization message body from buf (after message header).
func DecodeInit(msgID uint32, body []byte) (initMessage, error) {
	m := initMessage{MessageID: msgID}
	off := 0
	for off < len(body) {
		tlv, n, err := DecodeTLV(body[off:])
		if err != nil {
			return m, err
		}
		if tlv.Type == TLVTypeCommonSession && len(tlv.Value) >= 14 {
			m.ProtocolVersion = binary.BigEndian.Uint16(tlv.Value[0:2])
			m.KeepaliveTime = binary.BigEndian.Uint16(tlv.Value[2:4])
			m.LoopDetection = tlv.Value[4]&0x04 != 0
			m.PathVectorLimit = tlv.Value[5]
			m.MaxPDULength = binary.BigEndian.Uint16(tlv.Value[6:8])
			copy(m.ReceiverLSRID[:], tlv.Value[8:12])
			m.ReceiverLabelSpace = binary.BigEndian.Uint16(tlv.Value[12:14])
		}
		off += n
	}
	return m, nil
}

// keepaliveMessage represents an LDP KeepAlive message.
type keepaliveMessage struct {
	MessageID uint32
}

// encodeKeepalive writes a KeepAlive message to buf.
func encodeKeepalive(buf []byte, m keepaliveMessage) int {
	off := encodeMessageHeader(buf, MessageHeader{
		Type:      MsgTypeKeepAlive,
		Length:    4,
		MessageID: m.MessageID,
	})
	return off
}

// FECElement is a single FEC element in a FEC TLV.
type FECElement struct {
	Type    uint8
	Family  uint16
	PrefLen uint8
	Prefix  netip.Prefix
}

// labelMappingMessage represents an LDP Label Mapping message (RFC 5036 Section 3.5.7).
type labelMappingMessage struct {
	MessageID uint32
	FEC       FECElement
	Label     uint32
}

// encodeFECTLV writes a FEC TLV for the given prefix into buf at offset off.
// Returns the number of bytes written.
func encodeFECTLV(buf []byte, off int, prefix netip.Prefix) int {
	var fecBuf [24]byte
	fecOff := 0
	fecBuf[fecOff] = FECPrefix
	fecOff++
	if prefix.Addr().Is4() {
		binary.BigEndian.PutUint16(fecBuf[fecOff:fecOff+2], AFIIPv4)
		fecOff += 2
		fecBuf[fecOff] = uint8(prefix.Bits())
		fecOff++
		prefixBytes := (prefix.Bits() + 7) / 8
		addr4 := prefix.Addr().As4()
		copy(fecBuf[fecOff:fecOff+prefixBytes], addr4[:prefixBytes])
		fecOff += prefixBytes
	} else {
		binary.BigEndian.PutUint16(fecBuf[fecOff:fecOff+2], AFIIPv6)
		fecOff += 2
		fecBuf[fecOff] = uint8(prefix.Bits())
		fecOff++
		prefixBytes := (prefix.Bits() + 7) / 8
		addr16 := prefix.Addr().As16()
		copy(fecBuf[fecOff:fecOff+prefixBytes], addr16[:prefixBytes])
		fecOff += prefixBytes
	}
	return EncodeTLV(buf[off:], TLV{
		Type:   TLVTypeFEC,
		Length: uint16(fecOff),
		Value:  fecBuf[:fecOff],
	})
}

// encodeLabelTLV writes a Generic Label TLV into buf at offset off.
func encodeLabelTLV(buf []byte, off int, label uint32) int {
	var labelBuf [4]byte
	binary.BigEndian.PutUint32(labelBuf[:], label)
	return EncodeTLV(buf[off:], TLV{
		Type:   TLVTypeGenericLabel,
		Length: 4,
		Value:  labelBuf[:],
	})
}

// encodeLabelMapping writes a Label Mapping message body to buf.
func encodeLabelMapping(buf []byte, m labelMappingMessage) int {
	off := encodeMessageHeader(buf, MessageHeader{
		Type:      MsgTypeLabelMapping,
		MessageID: m.MessageID,
	})
	off += encodeFECTLV(buf, off, m.FEC.Prefix)
	off += encodeLabelTLV(buf, off, m.Label)
	binary.BigEndian.PutUint16(buf[2:4], uint16(off-ldpTLVHdrLen))
	return off
}

// decodeLabelMapping reads a Label Mapping message body from buf.
func decodeLabelMapping(msgID uint32, body []byte) (labelMappingMessage, error) {
	m := labelMappingMessage{MessageID: msgID}
	off := 0
	for off < len(body) {
		tlv, n, err := DecodeTLV(body[off:])
		if err != nil {
			return m, err
		}
		switch tlv.Type {
		case TLVTypeFEC:
			fec, err := decodeFECElement(tlv.Value)
			if err != nil {
				return m, err
			}
			m.FEC = fec
		case TLVTypeGenericLabel:
			if len(tlv.Value) >= 4 {
				m.Label = binary.BigEndian.Uint32(tlv.Value[:4])
			}
		}
		off += n
	}
	return m, nil
}

func decodeFECElement(buf []byte) (FECElement, error) {
	if len(buf) < 1 {
		return FECElement{}, errors.New("ldp: empty FEC element")
	}
	fe := FECElement{Type: buf[0]}
	if fe.Type == FECWildcard {
		return fe, nil
	}
	if fe.Type != FECPrefix || len(buf) < 4 {
		return fe, fmt.Errorf("ldp: unsupported FEC type %d or too short", fe.Type)
	}
	fe.Family = binary.BigEndian.Uint16(buf[1:3])
	fe.PrefLen = buf[3]
	prefixBytes := int((fe.PrefLen + 7) / 8)
	if len(buf) < 4+prefixBytes {
		return fe, errors.New("ldp: FEC prefix truncated")
	}
	switch fe.Family {
	case AFIIPv4:
		var addr [4]byte
		copy(addr[:], buf[4:4+prefixBytes])
		fe.Prefix = netip.PrefixFrom(netip.AddrFrom4(addr), int(fe.PrefLen))
	case AFIIPv6:
		var addr [16]byte
		copy(addr[:], buf[4:4+prefixBytes])
		fe.Prefix = netip.PrefixFrom(netip.AddrFrom16(addr), int(fe.PrefLen))
	default:
		return fe, fmt.Errorf("ldp: unsupported address family %d", fe.Family)
	}
	return fe, nil
}

// labelWithdrawMessage represents an LDP Label Withdraw message (RFC 5036 Section 3.5.10).
type labelWithdrawMessage struct {
	MessageID uint32
	FEC       FECElement
	Label     uint32
	HasLabel  bool
}

// EncodeLabelWithdraw writes a Label Withdraw message body to buf.
func EncodeLabelWithdraw(buf []byte, m labelWithdrawMessage) int {
	off := encodeMessageHeader(buf, MessageHeader{
		Type:      MsgTypeLabelWithdraw,
		MessageID: m.MessageID,
	})
	off += encodeFECTLV(buf, off, m.FEC.Prefix)
	if m.HasLabel {
		off += encodeLabelTLV(buf, off, m.Label)
	}
	binary.BigEndian.PutUint16(buf[2:4], uint16(off-ldpTLVHdrLen))
	return off
}

// addressListMessage represents an LDP Address or Address Withdraw message
// (RFC 5036 Section 3.5.5/3.5.6): the set of interface addresses the peer uses as
// next hops. A downstream LSR uses these to resolve a label binding's advertising
// peer to the IP next hop on the shared link.
type addressListMessage struct {
	MessageID uint32
	Family    uint16
	Addresses []netip.Addr
}

// decodeAddressList reads an Address / Address Withdraw message body (after the
// message header). It tolerates and skips TLVs other than the Address List.
func decodeAddressList(msgID uint32, body []byte) (addressListMessage, error) {
	m := addressListMessage{MessageID: msgID}
	off := 0
	for off < len(body) {
		tlv, n, err := DecodeTLV(body[off:])
		if err != nil {
			return m, err
		}
		if tlv.Type == TLVTypeAddressList {
			if len(tlv.Value) < 2 {
				return m, errShortTLV
			}
			m.Family = binary.BigEndian.Uint16(tlv.Value[0:2])
			rest := tlv.Value[2:]
			switch m.Family {
			case AFIIPv4:
				for len(rest) >= 4 {
					m.Addresses = append(m.Addresses, netip.AddrFrom4([4]byte(rest[:4])))
					rest = rest[4:]
				}
			case AFIIPv6:
				for len(rest) >= 16 {
					m.Addresses = append(m.Addresses, netip.AddrFrom16([16]byte(rest[:16])))
					rest = rest[16:]
				}
			}
		}
		off += n
	}
	return m, nil
}

// ValidateLabel checks that a label is within the 20-bit range.
func ValidateLabel(label uint32) error {
	if label > MaxLabel {
		return fmt.Errorf("%w: %d", errLabelRange, label)
	}
	return nil
}
