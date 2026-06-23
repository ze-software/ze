// Design: plan/spec-ospf-2-wire.md -- common OSPFv2 packet header and dispatch
// RFC 2328 Appendix A.3.1: OSPF packet header.

package packet

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// PacketType is the 1-octet OSPF packet type field.
type PacketType uint8

const (
	PacketTypeHello    PacketType = 1
	PacketTypeDBDesc   PacketType = 2
	PacketTypeLSReq    PacketType = 3
	PacketTypeLSUpdate PacketType = 4
	PacketTypeLSAck    PacketType = 5
)

// AuType is the 2-octet authentication type field from the common header.
type AuType uint16

const (
	AuTypeNull             AuType = 0
	AuTypeSimple           AuType = 1
	AuTypeCryptographic    AuType = 2
	AuTypeCryptographicESN AuType = 3
)

const (
	Version         = 2
	CommonHeaderLen = 24
	AuthFieldLen    = 8
	MaxPacketLen    = 65535
)

const (
	offVersion  = 0
	offType     = 1
	offLength   = 2
	offRouterID = 4
	offAreaID   = 8
	offChecksum = 12
	offAuType   = 14
	offAuth     = 16
)

var (
	ErrShortBuffer    = errors.New("ospf packet: buffer too short")
	ErrBadVersion     = errors.New("ospf packet: unsupported version")
	ErrUnknownType    = errors.New("ospf packet: unknown packet type")
	ErrLength         = errors.New("ospf packet: invalid length")
	ErrTruncated      = errors.New("ospf packet: truncated packet")
	ErrUnknownLSAType = errors.New("ospf packet: unknown LSA type")
)

// AuthField is the raw 8-octet authentication field carried in the common header.
type AuthField [AuthFieldLen]byte

// AuthFieldFromBytes copies the 8-octet authentication field from b.
func AuthFieldFromBytes(b []byte) (AuthField, error) {
	if len(b) != AuthFieldLen {
		return AuthField{}, ErrLength
	}
	var a AuthField
	copy(a[:], b)
	return a, nil
}

// WriteTo writes the raw authentication field into buf at off.
func (a AuthField) WriteTo(buf []byte, off int) int {
	copy(buf[off:off+AuthFieldLen], a[:])
	return AuthFieldLen
}

// Header is the parsed 24-octet OSPFv2 common header.
type Header struct {
	Type     PacketType
	Length   uint16
	RouterID types.RouterID
	AreaID   types.AreaID
	Checksum uint16
	AuType   AuType
	Auth     AuthField
}

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

// DecodeHeader parses and validates the 24-octet common OSPFv2 header. It
// validates Version == 2, packet type 1..5, and Packet Length against the caller
// slice before any body parser runs.
func DecodeHeader(buf []byte) (Header, int, error) {
	if len(buf) < CommonHeaderLen {
		return Header{}, 0, ErrShortBuffer
	}
	if buf[offVersion] != Version {
		return Header{}, 0, ErrBadVersion
	}
	pt := PacketType(buf[offType])
	if !pt.known() {
		return Header{}, 0, ErrUnknownType
	}
	plen := readUint16(buf, offLength)
	if plen < CommonHeaderLen {
		return Header{}, 0, ErrLength
	}
	if int(plen) > len(buf) {
		return Header{}, 0, ErrTruncated
	}
	router, err := types.RouterIDFromBytes(buf[offRouterID : offRouterID+types.RouterIDLen])
	if err != nil {
		return Header{}, 0, err
	}
	area, err := types.AreaIDFromBytes(buf[offAreaID : offAreaID+types.AreaIDLen])
	if err != nil {
		return Header{}, 0, err
	}
	auth, err := AuthFieldFromBytes(buf[offAuth : offAuth+AuthFieldLen])
	if err != nil {
		return Header{}, 0, err
	}
	return Header{
		Type:     pt,
		Length:   plen,
		RouterID: router,
		AreaID:   area,
		Checksum: readUint16(buf, offChecksum),
		AuType:   AuType(readUint16(buf, offAuType)),
		Auth:     auth,
	}, CommonHeaderLen, nil
}

// WriteTo writes the common header as-is into buf at off and returns off+24.
// Packet.WriteTo usually backfills Length and Checksum after writing the body.
func (h Header) WriteTo(buf []byte, off int) int {
	buf[off+offVersion] = Version
	buf[off+offType] = byte(h.Type)
	writeUint16(buf, off+offLength, h.Length)
	h.RouterID.WriteTo(buf, off+offRouterID)
	h.AreaID.WriteTo(buf, off+offAreaID)
	writeUint16(buf, off+offChecksum, h.Checksum)
	writeUint16(buf, off+offAuType, uint16(h.AuType))
	h.Auth.WriteTo(buf, off+offAuth)
	return off + CommonHeaderLen
}

// Packet is a decoded or encodable OSPFv2 packet. Exactly one body pointer is
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

// DecodePacket parses a complete OSPF payload beginning at the common header.
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

// EncodedLen returns the total packet length in octets.
func (p Packet) EncodedLen() int {
	return CommonHeaderLen + p.bodyLen()
}

func (p Packet) bodyLen() int {
	switch {
	case p.Hello != nil:
		return helloEncodedLen(*p.Hello)
	case p.DBDesc != nil:
		return dbDescEncodedLen(*p.DBDesc)
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
// Packet Length and Checksum fields are backfilled after body serialization. For
// AuType 2 and 3 the Checksum field is left zero for the authentication layer.
func (p *Packet) WriteTo(buf []byte, off int) int {
	start := off
	h := p.Header
	h.Type = p.packetType()
	h.Length = 0
	h.Checksum = 0
	off = h.WriteTo(buf, off)
	switch {
	case p.Hello != nil:
		off = writeHello(*p.Hello, buf, off)
	case p.DBDesc != nil:
		off = writeDBDesc(*p.DBDesc, buf, off)
	case p.LSReq != nil:
		off = writeLSReq(*p.LSReq, buf, off)
	case p.LSUpdate != nil:
		off = p.LSUpdate.WriteTo(buf, off)
	case p.LSAck != nil:
		off = writeLSAck(*p.LSAck, buf, off)
	}
	total := off - start
	writeUint16(buf, start+offLength, uint16(total))
	if h.AuType == AuTypeCryptographic || h.AuType == AuTypeCryptographicESN {
		p.Header.Length = uint16(total)
		p.Header.Checksum = 0
		return off
	}
	checksum := PacketChecksum(buf[start:off])
	writeUint16(buf, start+offChecksum, checksum)
	p.Header.Length = uint16(total)
	p.Header.Checksum = checksum
	return off
}

// VerifyChecksum verifies the packet checksum on decoded RawBytes. For AuType 2
// and 3 the OSPF packet checksum is not used; a zero checksum is accepted here
// and cryptographic verification is owned by ospf-12.
func (p Packet) VerifyChecksum() bool {
	if len(p.RawBytes) == 0 {
		return false
	}
	return VerifyPacketChecksum(p.RawBytes)
}
