// Design: plan/learned/972-ospf-af-unify.md -- the OSPFv3 Codec adapter over ospfv3/packet. It
// satisfies the SAME engine Codec interface as the v4 adapter, proving the wire codec is
// pluggable per address family: the engine decodes the common header and verifies the
// checksum through Codec regardless of version. Dependency direction is engine ->
// ospfv3 modules (never reverse); ospfv3/packet does not import the engine.
//
// RFC: rfc/short/rfc5340.md (App A)

package ospf

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
	ospfv3packet "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/types"
)

// v6Codec is the OSPFv3 Codec adapter over internal/plugins/ospf/v3/packet.
type v6Codec struct{}

// DecodeHeader parses the OSPFv3 16-byte common header (RFC 5340 sec A.3.1) and projects
// it onto the neutral Header. Router ID and Area ID remain 32-bit in OSPFv3; the Instance
// ID is the per-interface demux field surfaced to the engine.
func (v6Codec) DecodeHeader(payload []byte) (Header, error) {
	h, _, err := ospfv3packet.DecodeHeader(payload)
	if err != nil {
		return Header{}, err
	}
	return Header{
		Type:       PacketType(h.Type),
		Length:     h.Length,
		RouterID:   types.RouterID(h.RouterID),
		AreaID:     types.AreaID(h.AreaID),
		Checksum:   h.Checksum,
		InstanceID: uint8(h.InstanceID),
	}, nil
}

// VerifyChecksum validates the OSPFv3 IPv6 upper-layer checksum, which is bound to the
// datagram's source and destination (RFC 5340 sec A.3.1) -- hence src/dst are required,
// unlike the OSPFv2 codec, which checksums the datagram alone.
func (v6Codec) VerifyChecksum(payload []byte, src, dst netip.Addr) bool {
	return ospfv3packet.VerifyPacketChecksum(src, dst, payload)
}

// DecodeHello decodes the OSPFv3 Hello body and projects it onto the neutral types.Hello
// superset: the Interface ID replaces the (absent) OSPFv2 Network Mask, and the 24-bit Options
// map to the bits the shared neighbor FSM checks. DR/BDR are Router IDs in OSPFv3 (RFC 5340 sec
// A.3.2), carried in the 4-byte DR/BDR fields. Unmapped v6 option bits (V6/R/DC/AF) are not
// consulted by the AF-neutral FSM and are intentionally dropped.
func (v6Codec) DecodeHello(payload []byte) (types.Hello, error) {
	p, err := ospfv3packet.DecodePacket(payload)
	if err != nil {
		return types.Hello{}, err
	}
	if p.Hello == nil {
		return types.Hello{}, ErrNotHello
	}
	h := p.Hello
	hello := types.Hello{
		InterfaceID:   uint32(h.InterfaceID),
		HelloInterval: h.HelloInterval,
		Options:       v6OptionsToNeutral(h.Options),
		Priority:      h.Priority,
		DeadInterval:  uint32(h.RouterDeadInterval),
		DR:            [4]byte(h.DR),
		BDR:           [4]byte(h.BDR),
	}
	if len(h.Neighbors) > 0 {
		hello.Neighbors = make([]types.RouterID, len(h.Neighbors))
		for i, n := range h.Neighbors {
			hello.Neighbors[i] = types.RouterID(n)
		}
	}
	return hello, nil
}

// v6OptionsToNeutral maps the OSPFv3 24-bit Options to the neutral Options bits the engine's
// neighbor FSM validates: the E-bit (external capability) and N-bit (NSSA). The OSPFv3 E/N bit
// positions coincide with OSPFv2's, but the mapping is explicit so a future divergence is
// caught here rather than silently aliased.
func v6OptionsToNeutral(o ospfv3types.Options) types.Options {
	var n types.Options
	if o.External() {
		n = n.Set(types.OptionE)
	}
	if o.NSSA() {
		n = n.Set(types.OptionNP)
	}
	return n
}

// DecodeDBDesc decodes the OSPFv3 Database Description body onto the neutral types.DBDesc,
// converting the carried LSA headers (RFC 5340 sec A.3.3).
func (v6Codec) DecodeDBDesc(payload []byte) (types.DBDesc, error) {
	p, err := ospfv3packet.DecodePacket(payload)
	if err != nil {
		return types.DBDesc{}, err
	}
	if p.DBDesc == nil {
		return types.DBDesc{}, ErrNotDBDesc
	}
	d := p.DBDesc
	out := types.DBDesc{
		InterfaceMTU: d.InterfaceMTU,
		Options:      v6OptionsToNeutral(d.Options),
		Flags:        d.Flags,
		DDSequence:   d.DDSequence,
	}
	if len(d.Headers) > 0 {
		out.Headers = make([]types.LSAHeader, len(d.Headers))
		for i := range d.Headers {
			out.Headers[i] = v6LSAHeaderToNeutral(d.Headers[i])
		}
	}
	return out, nil
}

// DecodeLSReq decodes the OSPFv3 Link State Request body (RFC 5340 sec A.3.4); each 12-octet
// entry carries the scope-typed 16-bit LS Type plus the LSA identity.
func (v6Codec) DecodeLSReq(payload []byte) (types.LSReq, error) {
	p, err := ospfv3packet.DecodePacket(payload)
	if err != nil {
		return types.LSReq{}, err
	}
	if p.LSReq == nil {
		return types.LSReq{}, ErrNotLSReq
	}
	r := p.LSReq
	out := types.LSReq{}
	if len(r.Requests) > 0 {
		out.Requests = make([]types.LSRequestEntry, len(r.Requests))
		for i, e := range r.Requests {
			out.Requests[i] = types.LSRequestEntry{
				Type:              types.LSType(e.Type),
				LinkStateID:       types.LinkStateID(e.LinkStateID),
				AdvertisingRouter: types.RouterID(e.AdvertisingRouter),
			}
		}
	}
	return out, nil
}

// DecodeLSAck decodes the OSPFv3 Link State Ack body, a list of acknowledged LSA headers
// (RFC 5340 sec A.3.6).
func (v6Codec) DecodeLSAck(payload []byte) (types.LSAck, error) {
	p, err := ospfv3packet.DecodePacket(payload)
	if err != nil {
		return types.LSAck{}, err
	}
	if p.LSAck == nil {
		return types.LSAck{}, ErrNotLSAck
	}
	a := p.LSAck
	out := types.LSAck{}
	if len(a.Headers) > 0 {
		out.Headers = make([]types.LSAHeader, len(a.Headers))
		for i := range a.Headers {
			out.Headers[i] = v6LSAHeaderToNeutral(a.Headers[i])
		}
	}
	return out, nil
}

// DecodeLSUpdate decodes an OSPFv3 Link State Update (RFC 5340 sec A.3.5) onto the AF-neutral
// packet.LSUpdate. Each OSPFv3 LSA becomes a packet.LSA carrying the neutral LSA header plus the
// original Body and RawBytes spans; the typed LSA body (Router/Network/Intra-Area-Prefix/Link)
// is left undecoded -- that decode is the AFPrefixStrategy boundary. RawBytes is the complete
// OSPFv3 LSA, so the engine's LSA Fletcher checksum (byte-identical to OSPFv2 per RFC 5340 sec
// A.4.2.1; both verify lsa[2:length]) accepts a v6 LSA unchanged.
func (v6Codec) DecodeLSUpdate(payload []byte) (packet.LSUpdate, error) {
	p, err := ospfv3packet.DecodePacket(payload)
	if err != nil {
		return packet.LSUpdate{}, err
	}
	if p.LSUpdate == nil {
		return packet.LSUpdate{}, ErrNotLSUpdate
	}
	u := p.LSUpdate
	out := packet.LSUpdate{}
	if len(u.LSAs) > 0 {
		out.LSAs = make([]packet.LSA, len(u.LSAs))
		for i := range u.LSAs {
			out.LSAs[i] = packet.LSA{
				Header:   v6LSAHeaderToNeutral(u.LSAs[i].Header),
				Body:     u.LSAs[i].Body,
				RawBytes: u.LSAs[i].RawBytes,
			}
		}
	}
	return out, nil
}

// v6LSAHeaderToNeutral converts an OSPFv3 LSA header (RFC 5340 sec A.4.2: 20 octets, scope-typed
// 16-bit LS Type, no Options field) to the shared types.LSAHeader. The OSPFv3 header has no
// Options (Options moved into the LSA bodies), so it is left zero; the sequence number is a
// signed 32-bit value reinterpreted into the shared unsigned field (bit pattern preserved).
func v6LSAHeaderToNeutral(h ospfv3packet.LSAHeader) types.LSAHeader {
	return types.LSAHeader{
		Age:               types.LSAge(h.Age),
		Type:              types.LSType(h.Type),
		LinkStateID:       types.LinkStateID(h.LinkStateID),
		AdvertisingRouter: types.RouterID(h.AdvertisingRouter),
		Sequence:          types.LSSequenceNumber(uint32(h.Sequence)),
		Checksum:          h.Checksum,
		Length:            h.Length,
	}
}

// IsV6 reports true: this is the OSPFv3 (IPv6) codec.
func (v6Codec) IsV6() bool { return true }

// The OSPFv3 packet codec satisfies the engine Codec interface.
var _ Codec = v6Codec{}
