// Design: docs/architecture/ospf/ospf-2-wire.md -- offline decode JSON rendering (cold CLI path).

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: AC-1 - Packet.ToJSON renders a decoded Hello with the packet type token, the
// dotted-quad Router ID / DR / BDR, the network mask, a valid checksum flag, and the neighbor
// list, exercising the ipv4String / dotted-quad helpers.
// PREVENTS: the `ze` decode CLI showing an empty or wrong-typed Hello, or a mask/address that
// drops octets.
func TestPacketToJSONHello(t *testing.T) {
	hello := sampleHello(t)
	buf := encodePacket(t, Packet{Header: sampleHeader(t, PacketTypeHello), Hello: &hello})
	p, err := DecodePacket(buf)
	if err != nil {
		t.Fatalf("DecodePacket: %v", err)
	}
	v := p.ToJSON()
	if v.Type != "hello" {
		t.Errorf("Type = %q, want hello", v.Type)
	}
	if v.RouterID != "10.0.0.1" || v.AreaID != "0.0.0.0" {
		t.Errorf("RouterID/AreaID = %q/%q, want 10.0.0.1/0.0.0.0", v.RouterID, v.AreaID)
	}
	if !v.ChecksumValid {
		t.Errorf("ChecksumValid = false, want true")
	}
	if v.Hello == nil {
		t.Fatalf("Hello view nil")
	}
	if v.Hello.NetworkMask != "255.255.255.0" {
		t.Errorf("NetworkMask = %q, want 255.255.255.0", v.Hello.NetworkMask)
	}
	if v.Hello.DR != "10.0.0.1" || v.Hello.BDR != "10.0.0.2" {
		t.Errorf("DR/BDR = %q/%q, want 10.0.0.1/10.0.0.2", v.Hello.DR, v.Hello.BDR)
	}
	if len(v.Hello.Neighbors) != 2 || v.Hello.Neighbors[0] != "10.0.0.2" {
		t.Errorf("Neighbors = %v, want [10.0.0.2 10.0.0.3]", v.Hello.Neighbors)
	}
	if v.Hello.HelloInterval != 10 || v.Hello.DeadInterval != 40 {
		t.Errorf("intervals = %d/%d, want 10/40", v.Hello.HelloInterval, v.Hello.DeadInterval)
	}
}

// VALIDATES: AC-1 - ToJSON renders each LSA body type in an LS Update: Router (with per-link
// data + metric), Network (mask + attached routers), Summary (24-bit metric), and External
// (E-bit + route tag), each with a valid per-LSA checksum flag.
// PREVENTS: the decode CLI omitting a body variant or rendering a wrong metric/tag/mask.
func TestPacketToJSONLSUpdateBodies(t *testing.T) {
	upd := &LSUpdate{LSAs: []LSA{
		sampleRouterLSA(t),
		sampleNetworkLSA(t),
		sampleSummaryLSA(t, types.LSTypeSummaryNetwork),
		sampleExternalLSA(t, types.LSTypeASExternal),
	}}
	buf := encodePacket(t, Packet{Header: sampleHeader(t, PacketTypeLSUpdate), LSUpdate: upd})
	p, err := DecodePacket(buf)
	if err != nil {
		t.Fatalf("DecodePacket: %v", err)
	}
	v := p.ToJSON()
	if v.Type != "ls-update" || v.LSUpdate == nil || len(v.LSUpdate.LSAs) != 4 {
		t.Fatalf("ls-update view wrong: %+v", v.LSUpdate)
	}
	lsas := v.LSUpdate.LSAs

	router := lsas[0]
	if router.Router == nil || len(router.Router.Links) != 4 {
		t.Fatalf("router LSA view wrong: %+v", router.Router)
	}
	if !router.ChecksumValid {
		t.Errorf("router LSA ChecksumValid = false, want true")
	}
	// The transit link (index 1) carries metric 65535 and its LinkData is the local address.
	if router.Router.Links[1].Metric != 65535 || router.Router.Links[1].LinkData != "10.0.0.1" {
		t.Errorf("router link 1 = %+v, want metric 65535 link-data 10.0.0.1", router.Router.Links[1])
	}

	network := lsas[1]
	if network.Network == nil || network.Network.NetworkMask != "255.255.255.0" || len(network.Network.AttachedRouters) != 2 {
		t.Fatalf("network LSA view wrong: %+v", network.Network)
	}

	summary := lsas[2]
	if summary.Summary == nil || summary.Summary.NetworkMask != "255.255.255.0" {
		t.Fatalf("summary LSA view wrong: %+v", summary.Summary)
	}

	external := lsas[3]
	if external.External == nil || !external.External.ExternalType2 || external.External.ExternalRouteTag != 0xfeedcafe {
		t.Fatalf("external LSA view wrong: %+v", external.External)
	}
}

// VALIDATES: spec-ospf-ext-4 -- ToJSON decodes an RFC 7684 Extended Prefix Opaque LSA body
// inline: the opaque view carries the raw hex plus a structured extended-prefix entry whose
// sub-TLVs render as type/length/hex rows (exercising opaqueBodyToJSON + extSubTLVsToJSON).
// PREVENTS: the decode CLI showing an Extended Prefix LSA as opaque hex only, hiding its fields.
func TestPacketToJSONOpaqueExtendedPrefix(t *testing.T) {
	extBody := EncodeExtPrefixLSA(ExtPrefixLSA{Prefixes: []ExtPrefixTLV{{
		RouteType:     ExtRouteTypeIntraArea,
		PrefixLength:  24,
		AF:            ExtPrefixAFIPv4Unicast,
		Flags:         ExtPrefixFlagN,
		AddressPrefix: [4]byte{10, 1, 2, 0},
		SubTLVs:       []ExtSubTLV{{Type: 9, Value: []byte{0x0a, 0x0b, 0x0c}}},
	}}})
	h := sampleLSAHeader(t, types.LSTypeOpaqueArea, "0.0.0.0")
	h.LinkStateID = OpaqueLinkStateID(ExtPrefixOpaqueType, 1)
	lsa := LSA{Header: h, Opaque: &OpaqueLSA{Type: types.LSTypeOpaqueArea, Data: extBody}}

	buf := encodePacket(t, Packet{Header: sampleHeader(t, PacketTypeLSUpdate), LSUpdate: &LSUpdate{LSAs: []LSA{lsa}}})
	p, err := DecodePacket(buf)
	if err != nil {
		t.Fatalf("DecodePacket: %v", err)
	}
	v := p.ToJSON()
	if v.LSUpdate == nil || len(v.LSUpdate.LSAs) != 1 {
		t.Fatalf("ls-update view wrong: %+v", v.LSUpdate)
	}
	op := v.LSUpdate.LSAs[0].Opaque
	if op == nil {
		t.Fatalf("opaque view nil, want extended-prefix decode")
	}
	if op.Data == "" {
		t.Errorf("opaque Data hex is empty, want the raw body hex")
	}
	if len(op.ExtendedPrefix) != 1 {
		t.Fatalf("extended-prefix entries = %d, want 1", len(op.ExtendedPrefix))
	}
	ep := op.ExtendedPrefix[0]
	if ep.PrefixLength != 24 || ep.AddressPrefix != "10.1.2.0" {
		t.Errorf("extended prefix = %+v, want /24 10.1.2.0", ep)
	}
	if len(ep.SubTLVs) != 1 || ep.SubTLVs[0].Type != 9 || ep.SubTLVs[0].Length != 3 || ep.SubTLVs[0].Data != "0a0b0c" {
		t.Errorf("extended-prefix sub-TLVs = %+v, want type 9 len 3 data 0a0b0c", ep.SubTLVs)
	}
}

// VALIDATES: AC-1 - ToJSON renders LS Request, LS Ack, and DB Description bodies with their
// header/entry fields (LS type token, options string, LSA header identity).
// PREVENTS: the decode CLI dropping the request/ack/dd body or its LSA header fields.
func TestPacketToJSONReqAckDD(t *testing.T) {
	// LS Request.
	req := &LSReq{Requests: []types.LSRequestEntry{{
		Type:              types.LSTypeNetwork,
		LinkStateID:       mustLSID(t, "10.0.0.254"),
		AdvertisingRouter: mustRouterID(t, "10.0.0.1"),
	}}}
	reqBuf := encodePacket(t, Packet{Header: sampleHeader(t, PacketTypeLSReq), LSReq: req})
	reqPkt, err := DecodePacket(reqBuf)
	if err != nil {
		t.Fatalf("DecodePacket LSReq: %v", err)
	}
	rv := reqPkt.ToJSON()
	if rv.Type != "ls-request" || rv.LSReq == nil || len(rv.LSReq.Requests) != 1 {
		t.Fatalf("ls-request view wrong: %+v", rv.LSReq)
	}
	if rv.LSReq.Requests[0].Type != "network" || rv.LSReq.Requests[0].LinkStateID != "10.0.0.254" {
		t.Errorf("request entry = %+v, want type network lsid 10.0.0.254", rv.LSReq.Requests[0])
	}

	// LS Ack.
	ackHeader := sampleLSAHeader(t, types.LSTypeRouter, "10.0.0.1")
	ackHeader.Length = types.LSAHeaderLen
	ackBuf := encodePacket(t, Packet{Header: sampleHeader(t, PacketTypeLSAck), LSAck: &LSAck{Headers: []LSAHeader{ackHeader}}})
	ackPkt, err := DecodePacket(ackBuf)
	if err != nil {
		t.Fatalf("DecodePacket LSAck: %v", err)
	}
	av := ackPkt.ToJSON()
	if av.Type != "ls-ack" || av.LSAck == nil || len(av.LSAck.Headers) != 1 {
		t.Fatalf("ls-ack view wrong: %+v", av.LSAck)
	}
	if av.LSAck.Headers[0].Type != "router" || av.LSAck.Headers[0].AdvertisingRouter != "10.0.0.1" {
		t.Errorf("ack header = %+v, want type router adv 10.0.0.1", av.LSAck.Headers[0])
	}

	// DB Description.
	ddHeader := sampleLSAHeader(t, types.LSTypeRouter, "10.0.0.1")
	ddHeader.Length = types.LSAHeaderLen
	ddBuf := encodePacket(t, Packet{Header: sampleHeader(t, PacketTypeDBDesc), DBDesc: &DBDesc{
		InterfaceMTU: 1500,
		Options:      types.OptionE,
		Flags:        DDFlagInit | DDFlagMore | DDFlagMaster,
		DDSequence:   0x01020304,
		Headers:      []LSAHeader{ddHeader},
	}})
	ddPkt, err := DecodePacket(ddBuf)
	if err != nil {
		t.Fatalf("DecodePacket DBDesc: %v", err)
	}
	dv := ddPkt.ToJSON()
	if dv.Type != "dbdesc" || dv.DBDesc == nil {
		t.Fatalf("dbdesc view wrong: %+v", dv.DBDesc)
	}
	if dv.DBDesc.InterfaceMTU != 1500 || dv.DBDesc.DDSequence != 0x01020304 || len(dv.DBDesc.Headers) != 1 {
		t.Errorf("dbdesc fields = %+v, want mtu 1500 seq 0x01020304 one header", dv.DBDesc)
	}
}
