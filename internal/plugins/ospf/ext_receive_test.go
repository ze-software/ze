// Design: docs/architecture/ospf/ospf-ext-4-extended-link-prefix.md.
// RFC 7684 Section 5 -- see rfc/short/rfc7684.md.
// VALIDATES: checksum-valid malformed extended LSAs stop at router ingress before
// LSDB storage, acknowledgement or flooding, while valid and unknown applications pass.
package ospf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"sync"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

type extIngressSend struct {
	iface string
	kind  packet.PacketType
	key   types.LSAKey
}

type extIngressRecorder struct {
	mu    sync.Mutex
	sends []extIngressSend
}

func (r *extIngressRecorder) send(iface string, _ netip.Addr, payload []byte) error {
	p, err := packet.DecodePacket(payload)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.LSUpdate != nil {
		for _, lsa := range p.LSUpdate.LSAs {
			if lsa.Header.Type.IsOpaque() {
				r.sends = append(r.sends, extIngressSend{iface: iface, kind: packet.PacketTypeLSUpdate, key: lsa.Header.Key()})
			}
		}
	}
	if p.LSAck != nil {
		for _, h := range p.LSAck.Headers {
			if h.Type.IsOpaque() {
				r.sends = append(r.sends, extIngressSend{iface: iface, kind: packet.PacketTypeLSAck, key: h.Key()})
			}
		}
	}
	return nil
}

// extIngressEngine negotiates an Exchange neighbor through the router dispatcher.
// Origination flags remain disabled: receive validation cannot depend on them.
func extIngressEngine(t *testing.T) (*engine, *extIngressRecorder, func(...packet.LSA)) {
	t.Helper()
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{}
	eng := newEngine(transport.New(backend))
	eng.setConfig(cfg)
	t.Cleanup(eng.shutdown)
	if err := eng.openInterfaces(); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	handle := backend.handles["eth0"]
	backend.mu.Unlock()
	if handle == nil {
		t.Fatal("eth0 transport handle missing")
	}
	peer := types.RouterID{2, 2, 2, 2}
	src := netip.MustParseAddr("10.0.0.2")
	dispatch := func(p packet.Packet) {
		eng.dispatch.dispatch(transport.RawPacket{
			IfIndex: handle.ifindex, Src: src,
			Payload: encodePacketPayloadForTest(t, p),
		})
	}
	header := packet.Header{RouterID: peer, AreaID: types.BackboneArea}
	dispatch(packet.Packet{Header: header, Hello: &packet.Hello{
		HelloInterval: DefaultHelloInterval, DeadInterval: uint32(DefaultDeadInterval),
		Options: types.OptionE, Priority: 1, Neighbors: []types.RouterID{cfg.RouterID},
	}})
	dispatch(packet.Packet{Header: header, DBDesc: &packet.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE | types.OptionO,
		Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7,
	}})
	if reason := eng.neighbors.AcceptsFlooding("eth0", peer); reason != "" {
		t.Fatalf("neighbor did not reach Exchange: %s", reason)
	}
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{
			{Name: "eth0", AreaID: types.BackboneArea, AreaType: types.AreaTypeNormal,
				NetworkType: types.NetworkBroadcast, State: ospflsdb.InterfaceStateBackup,
				RouterID: cfg.RouterID, DR: peer, BDR: cfg.RouterID, TransmitDelay: 1,
				Neighbors: []ospflsdb.NeighborInfo{{RouterID: peer, Address: src, State: ospflsdb.NeighborStateFull, OpaqueCapable: true}}},
			{Name: "eth1", AreaID: types.BackboneArea, AreaType: types.AreaTypeNormal,
				NetworkType: types.NetworkPointToPoint, RouterID: cfg.RouterID, TransmitDelay: 1,
				Neighbors: []ospflsdb.NeighborInfo{{RouterID: types.RouterID{3, 3, 3, 3}, Address: netip.MustParseAddr("10.0.1.3"), State: ospflsdb.NeighborStateFull, OpaqueCapable: true}}},
			{Name: "eth2", AreaID: types.AreaID{0, 0, 0, 1}, AreaType: types.AreaTypeNormal,
				NetworkType: types.NetworkPointToPoint, RouterID: cfg.RouterID, TransmitDelay: 1,
				Neighbors: []ospflsdb.NeighborInfo{{RouterID: types.RouterID{4, 4, 4, 4}, Address: netip.MustParseAddr("10.0.2.4"), State: ospflsdb.NeighborStateFull, OpaqueCapable: true}}},
		}
	})
	recorder := &extIngressRecorder{}
	eng.lsdb.SetTx(recorder.send)
	return eng, recorder, func(lsas ...packet.LSA) {
		dispatch(packet.Packet{Header: header, LSUpdate: &packet.LSUpdate{LSAs: lsas}})
		eng.lsdb.FlushDelayedAcks("eth0")
		eng.lsdb.FlushDelayedAcks("eth1")
		eng.lsdb.FlushDelayedAcks("eth2")
	}
}

func extIngressBody(opaqueType uint8) []byte {
	if opaqueType == packet.ExtLinkOpaqueType {
		return packet.EncodeExtLinkLSA(packet.ExtLinkTLV{
			LinkType: packet.RouterLinkTypeP2P, LinkID: [4]byte{2, 2, 2, 2}, LinkData: [4]byte{10, 0, 0, 1},
		})
	}
	return extPrefixBody(packet.ExtRouteTypeIntraArea, 24, 0, [4]byte{192, 0, 2, 0})
}

// TestExtIngressMalformedDiscard checks each framing boundary through the dispatcher,
// including self-originated and MaxAge cases that have their own acknowledgement paths.
// MUTATION: omit ReceiveUpdate's receiveValidator call, or skip a nested TLV scan.
// RFC requirement: RFC7684-5-3 negative -- checksum-valid extended prefix/link LSAs with TLV or sub-TLV overruns, short trailing headers, or malformed duplicate containers are neither stored nor acknowledged nor flooded through router ingress in link, area and AS scopes, including self-originated and MaxAge input.
func TestExtIngressMalformedDiscard(t *testing.T) {
	for _, opaqueType := range []uint8{packet.ExtPrefixOpaqueType, packet.ExtLinkOpaqueType} {
		for _, scope := range []types.LSType{types.LSTypeOpaqueLink, types.LSTypeOpaqueArea, types.LSTypeOpaqueAS} {
			t.Run(packet.OpaqueLinkStateID(opaqueType, uint32(scope)).String(), func(t *testing.T) {
				eng, recorder, receive := extIngressEngine(t)
				valid := extIngressBody(opaqueType)
				overrun := bytes.Clone(valid)
				binary.BigEndian.PutUint16(overrun[2:4], uint16(len(valid)))
				subOverrun := append(bytes.Clone(valid), 0, 99, 0, 4)
				binary.BigEndian.PutUint16(subOverrun[2:4], uint16(len(subOverrun)-4))
				shortSub := append(bytes.Clone(valid), 0, 0, 0, 0)
				binary.BigEndian.PutUint16(shortSub[2:4], uint16(len(valid)-3))
				cases := []struct {
					name string
					body []byte
					self bool
					age  types.LSAge
				}{
					{name: "tlv-overrun", body: overrun},
					{name: "trailing-header", body: append(bytes.Clone(valid), 0)},
					{name: "subtlv-overrun", body: subOverrun},
					{name: "subtlv-trailing-header", body: shortSub},
					{name: "duplicate-malformed-container", body: append(bytes.Clone(valid), subOverrun...)},
					{name: "max-age", body: subOverrun, age: types.LSAge(types.MaxAge)},
					{name: "self-originated", body: subOverrun, self: true},
				}
				for i, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						adv := types.RouterID{2, 2, 2, 2}
						if tc.self {
							adv = eng.cfg.RouterID
						}
						lsa := opaqueLSAForTest(t, scope, opaqueType, uint32(i+1), adv, types.InitialSequenceNumber, tc.body)
						if tc.age != 0 {
							if _, ok := packet.RefreshLSAInPlace(lsa.RawBytes, tc.age, lsa.Header.Sequence); !ok {
								t.Fatal("failed to stamp MaxAge")
							}
							lsa.Header.Age = tc.age
						}
						receive(lsa)
						if _, ok := eng.lsdb.Lookup(types.BackboneArea, lsa.Header.Key()); ok {
							t.Fatal("malformed LSA entered area or AS storage")
						}
						if _, ok := eng.lsdb.LookupLink("eth0", lsa.Header.Key()); ok {
							t.Fatal("malformed LSA entered link storage")
						}
					})
				}
				eng.lsdb.RetransmitTick(time.Now().Add(time.Hour))
				recorder.mu.Lock()
				defer recorder.mu.Unlock()
				if len(recorder.sends) != 0 {
					t.Fatalf("malformed LSAs produced acknowledgements or floods: %+v", recorder.sends)
				}
			})
		}
	}
}

// TestExtIngressValidCarriage checks that the reject gate leaves valid framing and
// unrecognised opaque applications eligible for storage, acknowledgement and scoped flooding.
// MUTATION: make validateExtLSA reject every extended LSA or validate unknown application bodies.
// RFC requirement: RFC7684-5-3 positive -- valid extended prefix/link LSAs, including unknown TLVs and padded sub-TLVs, are stored byte-for-byte, acknowledged and flooded within their area or AS scope even after a malformed LSA in the same update; unknown opaque application bytes retain the same carrier behavior.
func TestExtIngressValidCarriage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		opaqueType uint8
		scope      types.LSType
	}{
		{name: "prefix-area", opaqueType: packet.ExtPrefixOpaqueType, scope: types.LSTypeOpaqueArea},
		{name: "prefix-as", opaqueType: packet.ExtPrefixOpaqueType, scope: types.LSTypeOpaqueAS},
		{name: "link-area", opaqueType: packet.ExtLinkOpaqueType, scope: types.LSTypeOpaqueArea},
		{name: "unknown-area", opaqueType: 42, scope: types.LSTypeOpaqueArea},
		{name: "unknown-as", opaqueType: 42, scope: types.LSTypeOpaqueAS},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eng, recorder, receive := extIngressEngine(t)
			body := []byte{0xff}
			if tc.opaqueType != 42 {
				body = append(extIngressBody(tc.opaqueType), 0, 99, 0, 1, 0xff, 0, 0, 0)
				binary.BigEndian.PutUint16(body[2:4], uint16(len(body)-4))
				body = append(body, 0, 99, 0, 1, 0xff, 0, 0, 0)
			}
			lsa := opaqueLSAForTest(t, tc.scope, tc.opaqueType, 1, types.RouterID{2, 2, 2, 2}, types.InitialSequenceNumber, body)
			malformed := opaqueLSAForTest(t, tc.scope, packet.ExtPrefixOpaqueType, 2, types.RouterID{2, 2, 2, 2}, types.InitialSequenceNumber, []byte{0xff})
			receive(malformed, lsa)
			if _, ok := eng.lsdb.Lookup(types.BackboneArea, malformed.Header.Key()); ok {
				t.Fatal("malformed companion LSA was stored")
			}
			stored, ok := eng.lsdb.LookupLSA(types.BackboneArea, lsa.Header.Key())
			if !ok {
				t.Fatal("valid LSA was not stored")
			}
			if !bytes.Equal(stored.Body, body) {
				t.Fatalf("stored body = %x, want %x", stored.Body, body)
			}
			recorder.mu.Lock()
			defer recorder.mu.Unlock()
			observed := map[extIngressSend]bool{}
			for _, send := range recorder.sends {
				observed[send] = true
			}
			want := map[extIngressSend]bool{
				{iface: "eth0", kind: packet.PacketTypeLSAck, key: lsa.Header.Key()}:    true,
				{iface: "eth1", kind: packet.PacketTypeLSUpdate, key: lsa.Header.Key()}: true,
			}
			if tc.scope == types.LSTypeOpaqueAS {
				want[extIngressSend{iface: "eth2", kind: packet.PacketTypeLSUpdate, key: lsa.Header.Key()}] = true
			}
			if len(observed) != len(want) {
				t.Fatalf("acknowledgement/flood destinations = %+v, want %+v", observed, want)
			}
			for send := range want {
				if !observed[send] {
					t.Errorf("missing acknowledgement/flood: %+v; sent %+v", send, observed)
				}
			}
		})
	}
}
