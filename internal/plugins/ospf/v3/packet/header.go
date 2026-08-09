// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 common header and packet dispatch.
// RFC: rfc/short/rfc5340.md (§A.3.1 OSPFv3 packet header)

package packet

import (
	"errors"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// PacketType is the 1-octet OSPFv3 packet type field (RFC 5340 §A.3.1).
type PacketType uint8

// OSPFv3 packet types (RFC 5340 §A.3.1). Identical numbering to OSPFv2.
const (
	PacketTypeHello    PacketType = 1
	PacketTypeDBDesc   PacketType = 2
	PacketTypeLSReq    PacketType = 3
	PacketTypeLSUpdate PacketType = 4
	PacketTypeLSAck    PacketType = 5
)

const (
	// Version is the OSPFv3 version number (RFC 5340 §A.3.1).
	Version = 3
	// CommonHeaderLen is the fixed OSPFv3 common header width in octets. RFC 5340
	// §A.3.1 removed the OSPFv2 8-octet AuType + Authentication field; the two
	// reclaimed octets are Instance ID + Reserved.
	CommonHeaderLen = 16
	// MaxPacketLen bounds Packet Length to the 16-bit field width.
	MaxPacketLen = 65535
)

// Common header field offsets (RFC 5340 §A.3.1).
const (
	offVersion    = 0
	offType       = 1
	offLength     = 2
	offRouterID   = 4
	offAreaID     = 8
	offChecksum   = 12
	offInstanceID = 14
	offReserved   = 15
)

// Sentinel decode errors. They mirror the OSPFv2 codec's taxonomy so callers can
// branch on shape without coupling to OSPFv2.
var (
	// ErrShortBuffer reports a buffer shorter than a required fixed field.
	ErrShortBuffer = errors.New("ospfv3 packet: buffer too short")
	// ErrBadVersion reports a Version field that is not 3.
	ErrBadVersion = errors.New("ospfv3 packet: unsupported version")
	// ErrUnknownType reports a packet type outside 1..5.
	ErrUnknownType = errors.New("ospfv3 packet: unknown packet type")
	// ErrLength reports an internally inconsistent length (record misalignment,
	// declared length below the header minimum, or a count past the body).
	ErrLength = errors.New("ospfv3 packet: invalid length")
	// ErrTruncated reports a declared length that runs past the supplied buffer.
	ErrTruncated = errors.New("ospfv3 packet: truncated packet")
	// ErrUnknownLSAType reports an LS Type a typed body decoder does not handle.
	ErrUnknownLSAType = errors.New("ospfv3 packet: unknown LSA type")
)

// known reports whether the type is one of the five OSPFv3 packet types.
func (t PacketType) known() bool {
	switch t {
	case PacketTypeHello, PacketTypeDBDesc, PacketTypeLSReq, PacketTypeLSUpdate, PacketTypeLSAck:
		return true
	default:
		return false
	}
}

// String renders the packet type as a stable lowercase token for diagnostics.
func (t PacketType) String() string {
	switch t {
	case PacketTypeHello:
		return "hello"
	case PacketTypeDBDesc:
		return "dbdesc"
	case PacketTypeLSReq:
		return "ls-request"
	case PacketTypeLSUpdate:
		return "ls-update"
	case PacketTypeLSAck:
		return "ls-ack"
	default:
		return "unknown"
	}
}

// Header is the parsed 16-octet OSPFv3 common header. There is no AuType or
// Authentication field; Instance ID and Reserved occupy the reclaimed octets.
type Header struct {
	Type       PacketType
	Length     uint16
	RouterID   types.RouterID
	AreaID     types.AreaID
	Checksum   uint16
	InstanceID types.InstanceID
}

// DecodeHeader parses and validates the 16-octet OSPFv3 common header. It
// validates Version == 3, packet type 1..5, and Packet Length against the caller
// slice before any body parser runs.
func DecodeHeader(buf []byte) (Header, int, error) {
	if len(buf) < CommonHeaderLen {
		return Header{}, 0, ErrShortBuffer
	}
	// RFC 5340 §A.3.1: "Version # -- The OSPF version number. This specification
	// documents version 3 of the OSPF protocol."
	if buf[offVersion] != Version {
		return Header{}, 0, ErrBadVersion
	}
	pt := PacketType(buf[offType])
	if !pt.known() {
		return Header{}, 0, ErrUnknownType
	}
	plen := readUint16(buf, offLength)
	// RFC 5340 §A.3.1: "Packet Length -- The length in bytes of the OSPF protocol
	// packet." A packet must at least cover the common header.
	if plen < CommonHeaderLen {
		return Header{}, 0, ErrLength
	}
	if int(plen) > len(buf) {
		return Header{}, 0, ErrTruncated
	}
	router, err := types.RouterIDFromBytes(buf[offRouterID : offRouterID+types.IDLen])
	if err != nil {
		return Header{}, 0, err
	}
	area, err := types.AreaIDFromBytes(buf[offAreaID : offAreaID+types.IDLen])
	if err != nil {
		return Header{}, 0, err
	}
	return Header{
		Type:       pt,
		Length:     plen,
		RouterID:   router,
		AreaID:     area,
		Checksum:   readUint16(buf, offChecksum),
		InstanceID: types.InstanceID(buf[offInstanceID]),
	}, CommonHeaderLen, nil
}

// PeekInstanceID reads the OSPFv3 Instance ID (common-header byte 14) without
// decoding the rest of the packet. It returns false if pkt is shorter than the
// 16-octet common header. The transport uses this for its per-interface Instance
// ID demux (RFC 5340 §4.2.1: discard a packet whose Instance ID does not match
// the receiving interface); it is the only header field the transport reads --
// full header validation stays in DecodeHeader and the ospfv3-4 dispatcher.
func PeekInstanceID(pkt []byte) (types.InstanceID, bool) {
	if len(pkt) < CommonHeaderLen {
		return 0, false
	}
	return types.InstanceID(pkt[offInstanceID]), true
}

// WriteTo writes the common header as-is into buf at off and returns off+16.
// Packet.WriteTo usually backfills Length and Checksum after writing the body.
// The Reserved octet is always written zero (RFC 5340 §A.3.1).
func (h Header) WriteTo(buf []byte, off int) int {
	buf[off+offVersion] = Version
	buf[off+offType] = byte(h.Type)
	writeUint16(buf, off+offLength, h.Length)
	h.RouterID.WriteTo(buf, off+offRouterID)
	h.AreaID.WriteTo(buf, off+offAreaID)
	writeUint16(buf, off+offChecksum, h.Checksum)
	h.InstanceID.WriteTo(buf, off+offInstanceID)
	buf[off+offReserved] = 0
	return off + CommonHeaderLen
}

// Packet is a decoded or encodable OSPFv3 packet. Exactly one body pointer is
// non-nil for packets produced by DecodePacket or by the runtime before encode.
type Packet struct {
	Header   Header
	Hello    *Hello
	DBDesc   *DBDesc
	LSReq    *LSReq
	LSUpdate *LSUpdate
	LSAck    *LSAck
	RawBytes []byte
}

// DecodePacket parses a complete OSPFv3 payload beginning at the common header.
func DecodePacket(buf []byte) (Packet, error) {
	h, bodyOff, err := DecodeHeader(buf)
	if err != nil {
		return Packet{}, err
	}
	raw := buf[:h.Length]
	body := raw[bodyOff:]
	p := Packet{Header: h, RawBytes: raw}
	switch h.Type {
	case PacketTypeHello:
		v, err := DecodeHello(body)
		if err != nil {
			return Packet{}, err
		}
		p.Hello = &v
	case PacketTypeDBDesc:
		v, err := DecodeDBDesc(body)
		if err != nil {
			return Packet{}, err
		}
		p.DBDesc = &v
	case PacketTypeLSReq:
		v, err := DecodeLSReq(body)
		if err != nil {
			return Packet{}, err
		}
		p.LSReq = &v
	case PacketTypeLSUpdate:
		v, err := DecodeLSUpdate(body)
		if err != nil {
			return Packet{}, err
		}
		p.LSUpdate = &v
	case PacketTypeLSAck:
		v, err := DecodeLSAck(body)
		if err != nil {
			return Packet{}, err
		}
		p.LSAck = &v
	}
	return p, nil
}

// EncodedLen returns the total packet length in octets (header + body).
func (p Packet) EncodedLen() int {
	return CommonHeaderLen + p.bodyLen()
}

func (p Packet) bodyLen() int {
	switch {
	case p.Hello != nil:
		return p.Hello.EncodedLen()
	case p.DBDesc != nil:
		return p.DBDesc.EncodedLen()
	case p.LSReq != nil:
		return p.LSReq.EncodedLen()
	case p.LSUpdate != nil:
		return p.LSUpdate.EncodedLen()
	case p.LSAck != nil:
		return p.LSAck.EncodedLen()
	default:
		return 0
	}
}

func (p Packet) packetType() PacketType {
	if p.Header.Type != 0 {
		return p.Header.Type
	}
	switch {
	case p.Hello != nil:
		return PacketTypeHello
	case p.DBDesc != nil:
		return PacketTypeDBDesc
	case p.LSReq != nil:
		return PacketTypeLSReq
	case p.LSUpdate != nil:
		return PacketTypeLSUpdate
	case p.LSAck != nil:
		return PacketTypeLSAck
	default:
		return 0
	}
}

// WriteTo serializes the packet into buf at off and returns the new offset. The
// Packet Length field is backfilled after the body is written. The Checksum
// field is left zero here: RFC 5340 §A.3.1 binds the packet checksum to the IPv6
// pseudo-header (source and destination supplied by transport), so a caller that
// wants the on-wire checksum calls FinalizePacketChecksum with the addresses
// after WriteTo. When an RFC 7166 Authentication Trailer is appended the checksum
// computation is omitted (RFC 7166 §2.2), so leaving it zero is also correct
// there.
func (p *Packet) WriteTo(buf []byte, off int) int {
	start := off
	h := p.Header
	h.Type = p.packetType()
	h.Length = 0
	h.Checksum = 0
	off = h.WriteTo(buf, off)
	switch {
	case p.Hello != nil:
		off = p.Hello.WriteTo(buf, off)
	case p.DBDesc != nil:
		off = p.DBDesc.WriteTo(buf, off)
	case p.LSReq != nil:
		off = p.LSReq.WriteTo(buf, off)
	case p.LSUpdate != nil:
		off = p.LSUpdate.WriteTo(buf, off)
	case p.LSAck != nil:
		off = p.LSAck.WriteTo(buf, off)
	}
	total := off - start
	// The OSPFv3 packet length is a 16-bit field (RFC 5340 sec A.3.1). A real packet is bounded
	// by the link MTU well below this; clamp rather than silently wrap a hypothetical over-size
	// packet to a small value (which a receiver would misparse).
	total = min(total, 0xFFFF)
	writeUint16(buf, start+offLength, uint16(total))
	p.Header.Length = uint16(total)
	p.Header.Checksum = 0
	return off
}
