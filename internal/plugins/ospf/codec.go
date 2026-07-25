// Design: plan/learned/972-ospf-af-unify.md -- Phase 2: the Codec seam. The engine decodes
// the common OSPF header and verifies the packet checksum through this interface
// instead of calling ospf/packet directly, so a second (IPv6/OSPFv3) instance can
// later supply a v6 codec (ospfv3/packet) with its IPv6 upper-layer checksum. The v4
// adapter wraps ospf/packet and is behavior-identical to the direct calls, so OSPFv2
// is bit-for-bit unchanged.
//
// RFC: rfc/short/rfc2328.md (App A), rfc/short/rfc5340.md (App A)

package ospf

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// ErrNot* are returned by the body decoders when the payload decodes to a packet of the
// wrong type. The dispatcher routes by header type before calling the per-type handler, so
// these only fire on a malformed or type-mismatched datagram (dropped as a decode failure).
var (
	ErrNotHello    = errors.New("ospf: packet is not a Hello")
	ErrNotDBDesc   = errors.New("ospf: packet is not a Database Description")
	ErrNotLSReq    = errors.New("ospf: packet is not a Link State Request")
	ErrNotLSUpdate = errors.New("ospf: packet is not a Link State Update")
	ErrNotLSAck    = errors.New("ospf: packet is not a Link State Ack")
)

// PacketType is the address-family-neutral OSPF packet type. The 1..5 type codes are
// identical across OSPFv2 (RFC 2328 sec A.3.1) and OSPFv3 (RFC 5340 sec A.3.1).
type PacketType uint8

const (
	PacketTypeHello    PacketType = 1
	PacketTypeDBDesc   PacketType = 2
	PacketTypeLSReq    PacketType = 3
	PacketTypeLSUpdate PacketType = 4
	PacketTypeLSAck    PacketType = 5
)

// Header is the address-family-neutral OSPF common header the engine dispatches on.
// It carries only the fields the shared engine reads -- Type (dispatch), AreaID (area
// check), RouterID (neighbor identity), Length/Checksum (validation). Version-specific
// header details stay inside the codec: OSPFv2's AuType/Auth (RFC 2328 App D) are read
// from the raw payload by the auth path, and the Instance ID (OSPFv3 RFC 5340 sec 2.5,
// OSPFv2 RFC 6549 sec 2) is surfaced here as InstanceID for the engine's per-interface
// demux -- one header location per family, one shared demux rule.
type Header struct {
	Type       PacketType
	Length     uint16
	RouterID   types.RouterID
	AreaID     types.AreaID
	Checksum   uint16
	InstanceID uint8
}

// Codec is the engine's view of the version-specific wire codec: it decodes the common
// header and verifies the packet checksum. The OSPFv2 checksum covers the datagram
// itself, so the v4 adapter ignores src/dst; the OSPFv3 checksum is the IPv6
// upper-layer checksum bound to the datagram's source and destination (RFC 5340 sec
// A.3.1), so the v6 adapter (a later phase) consumes them.
type Codec interface {
	DecodeHeader(payload []byte) (Header, error)
	// DecodeHello decodes a Hello packet body onto the AF-neutral types.Hello superset.
	// OSPFv2 fills NetworkMask; OSPFv3 fills InterfaceID. The shared neighbor FSM applies
	// AF-aware validation to the result, so the engine never sees a version-specific body.
	DecodeHello(payload []byte) (types.Hello, error)
	// DecodeDBDesc / DecodeLSReq / DecodeLSAck decode the neighbor-FSM framing bodies onto the
	// shared types. The carried LSA headers (DBDesc, LSAck) and request entries (LSReq) are
	// AF-neutral; only the wire layout differs (v2 vs v6), so the engine's NSM consumes one
	// shape regardless of version.
	DecodeDBDesc(payload []byte) (types.DBDesc, error)
	DecodeLSReq(payload []byte) (types.LSReq, error)
	DecodeLSAck(payload []byte) (types.LSAck, error)
	// DecodeLSUpdate decodes a Link State Update onto packet.LSUpdate (a slice of packet.LSA).
	// packet.LSA is AF-neutral in structure (shared types.LSAHeader + Body + RawBytes); the
	// typed LSA body decode stays version-specific behind the AFPrefixStrategy. The v4 adapter
	// returns ospf/packet's own LSAs; the v6 adapter converts ospfv3 LSAs to the same shape.
	DecodeLSUpdate(payload []byte) (packet.LSUpdate, error)
	VerifyChecksum(payload []byte, src, dst netip.Addr) bool
	// IsV6 reports whether this codec is the OSPFv3 (IPv6) codec. The engine uses it to mark
	// each interface's address family so the neighbor FSM applies AF-aware checks (e.g. the
	// Network Mask match is OSPFv2-only; OSPFv3 carries an Interface ID instead).
	IsV6() bool
}

// v4Codec is the OSPFv2 Codec adapter over internal/plugins/ospf/packet.
type v4Codec struct{}

// DecodeHeader wraps packet.DecodeHeader and projects the OSPFv2 common header onto the
// neutral Header (the v2 AuType/Auth fields are left to the auth path, which reads them
// from the raw payload). InstanceID surfaces the RFC 6549 OSPFv2 Instance ID (header
// offset 14) so the shared dispatcher demux sees it the same way it sees the OSPFv3 one;
// it is 0 for a base (single-instance) OSPFv2 router.
func (v4Codec) DecodeHeader(payload []byte) (Header, error) {
	h, _, err := packet.DecodeHeader(payload)
	if err != nil {
		return Header{}, err
	}
	return Header{
		Type:       PacketType(h.Type),
		Length:     h.Length,
		RouterID:   h.RouterID,
		AreaID:     h.AreaID,
		Checksum:   h.Checksum,
		InstanceID: h.InstanceID,
	}, nil
}

// VerifyChecksum ignores src/dst: the OSPFv2 checksum (RFC 2328 sec A.3.1) covers the
// OSPF datagram, not an IP pseudo-header.
func (v4Codec) VerifyChecksum(payload []byte, _, _ netip.Addr) bool {
	return packet.VerifyPacketChecksum(payload)
}

// DecodeHello decodes the OSPFv2 Hello body. packet.Hello is an alias for types.Hello, so the
// v2 body is already the neutral superset (NetworkMask set, InterfaceID zero).
func (v4Codec) DecodeHello(payload []byte) (types.Hello, error) {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return types.Hello{}, err
	}
	if p.Hello == nil {
		return types.Hello{}, ErrNotHello
	}
	return *p.Hello, nil
}

// DecodeDBDesc decodes the OSPFv2 Database Description body (packet.DBDesc aliases types.DBDesc).
func (v4Codec) DecodeDBDesc(payload []byte) (types.DBDesc, error) {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return types.DBDesc{}, err
	}
	if p.DBDesc == nil {
		return types.DBDesc{}, ErrNotDBDesc
	}
	return *p.DBDesc, nil
}

// DecodeLSReq decodes the OSPFv2 Link State Request body.
func (v4Codec) DecodeLSReq(payload []byte) (types.LSReq, error) {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return types.LSReq{}, err
	}
	if p.LSReq == nil {
		return types.LSReq{}, ErrNotLSReq
	}
	return *p.LSReq, nil
}

// DecodeLSAck decodes the OSPFv2 Link State Ack body.
func (v4Codec) DecodeLSAck(payload []byte) (types.LSAck, error) {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return types.LSAck{}, err
	}
	if p.LSAck == nil {
		return types.LSAck{}, ErrNotLSAck
	}
	return *p.LSAck, nil
}

// DecodeLSUpdate decodes the OSPFv2 Link State Update body (packet.LSUpdate carries
// ospf/packet's own LSAs, whose typed body decode is the OSPFv2 prefix model).
func (v4Codec) DecodeLSUpdate(payload []byte) (packet.LSUpdate, error) {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return packet.LSUpdate{}, err
	}
	if p.LSUpdate == nil {
		return packet.LSUpdate{}, ErrNotLSUpdate
	}
	return *p.LSUpdate, nil
}

// IsV6 reports false: this is the OSPFv2 (IPv4) codec.
func (v4Codec) IsV6() bool { return false }

// The OSPFv2 packet codec satisfies the engine Codec interface unchanged.
var _ Codec = v4Codec{}
