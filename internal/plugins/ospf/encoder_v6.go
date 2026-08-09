// Design: docs/architecture/ospf/ospf-af-unify.md -- Phase 5: the OSPFv3 send (encode) path.
//
// The Codec seam decodes incoming packets; the interface's Hello SEND goes through
// iface.Encoder. v6Encoder is the OSPFv3 Hello encoder the engine injects into a v6
// interface (iface.SetEncoder). It maps the address-family-neutral packet.Hello
// (InterfaceID, neutral Options, DR/BDR carried as Router IDs) to the OSPFv3 wire
// form and serializes via ospfv3/packet. The OSPFv3 upper-layer checksum is bound
// to the datagram src/dst and is finalized by the v6 transport at send time, so the
// packet encoded here carries a zero checksum (mirrors the ospfv3 transport path).
//
// RFC: rfc/short/rfc5340.md (App A.2 Options, A.3.2 Hello), rfc/short/rfc5838.md (§2.4 AF-bit)

package ospf

import (
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6Encoder is the OSPFv3 (IPv6) Hello encoder for a v6 interface. instanceID is the
// interface's OSPFv3 Instance ID (RFC 5340 sec 2.5), surfaced in the common header. emitAF
// sets the RFC 5838 §2.4 AF-bit in the Hello and DD Options for a multi-AF instance.
type v6Encoder struct {
	instanceID uint8
	emitAF     bool
}

// packetOptions applies the RFC 5838 §2.4 AF-bit to the Hello/DD Options for a multi-AF
// instance. The AF-bit is a Hello/DD signal only, so it is applied here and NOT in the
// LSA-origination Options paths.
func (e v6Encoder) packetOptions(o types.Options) ospfv3types.Options {
	v6 := neutralToV6Options(o)
	if e.emitAF {
		v6 = v6.SetAF()
	}
	return v6
}

// EncodeHello serializes the neutral Hello as an OSPFv3 Hello (RFC 5340 sec A.3.2).
func (e v6Encoder) EncodeHello(routerID types.RouterID, areaID types.AreaID, h packet.Hello) []byte {
	hello := ospfv3packet.Hello{
		InterfaceID:        ospfv3types.InterfaceID(h.InterfaceID),
		Priority:           h.Priority,
		Options:            e.packetOptions(h.Options),
		HelloInterval:      h.HelloInterval,
		RouterDeadInterval: uint16(h.DeadInterval),
		DR:                 ospfv3types.RouterID(h.DR),
		BDR:                ospfv3types.RouterID(h.BDR),
	}
	if len(h.Neighbors) > 0 {
		hello.Neighbors = make([]ospfv3types.RouterID, len(h.Neighbors))
		for i, n := range h.Neighbors {
			hello.Neighbors[i] = ospfv3types.RouterID(n)
		}
	}
	p := ospfv3packet.Packet{
		Header: ospfv3packet.Header{
			Type:       ospfv3packet.PacketTypeHello,
			RouterID:   ospfv3types.RouterID(routerID),
			AreaID:     ospfv3types.AreaID(areaID),
			InstanceID: ospfv3types.InstanceID(e.instanceID),
		},
		Hello: &hello,
	}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	return buf
}

// neutralToV6Options maps the AF-neutral Options to OSPFv3 Options (RFC 5340 sec
// A.2): an active IPv6 router always sets V6 and R; E/N follow the area type. It is
// the inverse of v6OptionsToNeutral.
func neutralToV6Options(o types.Options) ospfv3types.Options {
	v6 := ospfv3types.OptV6 | ospfv3types.OptR
	if o.Has(types.OptionE) {
		v6 |= ospfv3types.OptE
	}
	if o.Has(types.OptionNP) {
		v6 |= ospfv3types.OptN
	}
	return v6
}

// EncodeDBDesc serializes the neutral Database Description as an OSPFv3 DD packet
// (RFC 5340 sec A.3.3): the carried LSA headers use the 20-octet OSPFv3 header.
func (e v6Encoder) EncodeDBDesc(routerID types.RouterID, areaID types.AreaID, dd packet.DBDesc) []byte {
	out := ospfv3packet.DBDesc{
		Options:      e.packetOptions(dd.Options),
		InterfaceMTU: dd.InterfaceMTU,
		Flags:        dd.Flags,
		DDSequence:   dd.DDSequence,
	}
	if len(dd.Headers) > 0 {
		out.Headers = make([]ospfv3packet.LSAHeader, len(dd.Headers))
		for i := range dd.Headers {
			out.Headers[i] = neutralToV6LSAHeader(dd.Headers[i])
		}
	}
	return e.encode(routerID, areaID, ospfv3packet.Packet{DBDesc: &out, Header: e.header(routerID, areaID, ospfv3packet.PacketTypeDBDesc)})
}

// EncodeLSReq serializes the neutral Link State Request as an OSPFv3 LSReq packet
// (RFC 5340 sec A.3.4); each entry carries the scope-typed 16-bit LS Type.
func (e v6Encoder) EncodeLSReq(routerID types.RouterID, areaID types.AreaID, r packet.LSReq) []byte {
	out := ospfv3packet.LSReq{}
	if len(r.Requests) > 0 {
		out.Requests = make([]ospfv3packet.LSRequestEntry, len(r.Requests))
		for i, e := range r.Requests {
			out.Requests[i] = ospfv3packet.LSRequestEntry{
				Type:              v6WireLSType(e.Type),
				LinkStateID:       ospfv3types.LinkStateID(e.LinkStateID),
				AdvertisingRouter: ospfv3types.RouterID(e.AdvertisingRouter),
			}
		}
	}
	return e.encode(routerID, areaID, ospfv3packet.Packet{LSReq: &out, Header: e.header(routerID, areaID, ospfv3packet.PacketTypeLSReq)})
}

// EncodeLSUpdate serializes the neutral Link State Update as an OSPFv3 LSUpdate
// (RFC 5340 sec A.3.5). Each LSA carries the already-OSPFv3-encoded RawBytes
// (decoded by the v6 codec or originated by the v6 strategy), re-emitted verbatim.
func (e v6Encoder) EncodeLSUpdate(routerID types.RouterID, areaID types.AreaID, u packet.LSUpdate) []byte {
	out := ospfv3packet.LSUpdate{}
	if len(u.LSAs) > 0 {
		out.LSAs = make([]ospfv3packet.LSA, len(u.LSAs))
		for i := range u.LSAs {
			out.LSAs[i] = ospfv3packet.LSA{
				Header:   neutralToV6LSAHeader(u.LSAs[i].Header),
				Body:     u.LSAs[i].Body,
				RawBytes: u.LSAs[i].RawBytes,
			}
		}
	}
	return e.encode(routerID, areaID, ospfv3packet.Packet{LSUpdate: &out, Header: e.header(routerID, areaID, ospfv3packet.PacketTypeLSUpdate)})
}

// EncodeLSAck serializes the neutral Link State Acknowledgment as an OSPFv3 LSAck (RFC 5340
// sec A.3.6): the LSDB's flooded acks for the IPv6 family. Each acked header is mapped back
// to the 20-octet OSPFv3 LSA header.
func (e v6Encoder) EncodeLSAck(routerID types.RouterID, areaID types.AreaID, a packet.LSAck) []byte {
	out := ospfv3packet.LSAck{}
	if len(a.Headers) > 0 {
		out.Headers = make([]ospfv3packet.LSAHeader, len(a.Headers))
		for i := range a.Headers {
			out.Headers[i] = neutralToV6LSAHeader(a.Headers[i])
		}
	}
	return e.encode(routerID, areaID, ospfv3packet.Packet{LSAck: &out, Header: e.header(routerID, areaID, ospfv3packet.PacketTypeLSAck)})
}

func (e v6Encoder) header(routerID types.RouterID, areaID types.AreaID, t ospfv3packet.PacketType) ospfv3packet.Header {
	return ospfv3packet.Header{
		Type:       t,
		RouterID:   ospfv3types.RouterID(routerID),
		AreaID:     ospfv3types.AreaID(areaID),
		InstanceID: ospfv3types.InstanceID(e.instanceID),
	}
}

func (v6Encoder) encode(_ types.RouterID, _ types.AreaID, p ospfv3packet.Packet) []byte {
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	return buf
}

// neutralToV6LSAHeader converts a shared types.LSAHeader back to the OSPFv3 20-octet
// header (RFC 5340 sec A.4.2): the OSPFv3 header has no Options, and the sequence
// number is the OSPFv2 signed 32-bit space reinterpreted (bit pattern preserved). It
// is the inverse of v6LSAHeaderToNeutral.
func neutralToV6LSAHeader(h types.LSAHeader) ospfv3packet.LSAHeader {
	return ospfv3packet.LSAHeader{
		Age:               ospfv3types.LSAge(h.Age),
		Type:              v6WireLSType(h.Type),
		LinkStateID:       ospfv3types.LinkStateID(h.LinkStateID),
		AdvertisingRouter: ospfv3types.RouterID(h.AdvertisingRouter),
		Sequence:          ospfv3types.LSSequenceNumber(int32(uint32(h.Sequence))),
		Checksum:          h.Checksum,
		Length:            h.Length,
	}
}

// The OSPFv3 encoder satisfies both the interface Hello-encode seam and the neighbor
// DD/LSReq/LSUpdate encode seam.
var (
	_ ospfiface.Encoder    = v6Encoder{}
	_ ospfneighbor.Encoder = v6Encoder{}
)
