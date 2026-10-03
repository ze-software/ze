// Design: docs/architecture/wire/nlri-bgpls.md -- native OSPF snapshot contracts.
package ospf

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	v3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	v3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// The observer copies during Emit, just as the encoded exporter must. No LSDB
// view or borrowed event storage escapes the synchronous subscription callback.
func bgplsTestSource(t *testing.T, v3 bool) (*engine, *fakeBus, <-chan linkstateevents.Snapshot) {
	t.Helper()
	codec := Codec(v4Codec{})
	af := afIPv4Unicast
	if v3 {
		codec = v6Codec{}
		af = afIPv6Unicast
	}
	e := newEngineWithCodecAF(nil, codec, af)
	e.setConfig(ospfConfig{present: true, RouterID: types.RouterID{1, 1, 1, 1},
		Areas:  []areaConfig{{AreaID: types.BackboneArea}},
		Timers: timerConfig{SPFDelayMS: 3600000, SPFHoldMS: 3600000, SPFMaxHoldMS: 3600000}})
	bus := newFakeBus()
	out := make(chan linkstateevents.Snapshot, 64)
	unsubscribe := bgplsSnapshots.Subscribe(bus, func(snapshot *linkstateevents.Snapshot) {
		raw, err := json.Marshal(snapshot)
		if err != nil {
			t.Errorf("copy snapshot: %v", err)
			return
		}
		var owned linkstateevents.Snapshot
		if err := json.Unmarshal(raw, &owned); err != nil {
			t.Errorf("decode snapshot: %v", err)
			return
		}
		out <- owned
	})
	t.Cleanup(func() { e.shutdown(); unsubscribe() })
	e.startBGPLS(bus)
	return e, bus, out
}

func bgplsAwait(t *testing.T, events <-chan linkstateevents.Snapshot, accept func(*linkstateevents.Snapshot) bool) linkstateevents.Snapshot {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case snapshot := <-events:
			if accept(&snapshot) {
				return snapshot
			}
		case <-timer.C:
			t.Fatal("native LSDB change did not publish expected replacement")
			return linkstateevents.Snapshot{}
		}
	}
}

// Replay is synchronous. Drain earlier notifications as well so a transition
// assertion cannot accidentally accept the preceding SPF result from the queue.
func bgplsReplaySnapshot(t *testing.T, bus *fakeBus, events <-chan linkstateevents.Snapshot) linkstateevents.Snapshot {
	const area uint32 = 0
	t.Helper()
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	var latest linkstateevents.Snapshot
	found := false
	for {
		select {
		case snapshot := <-events:
			if snapshot.Domain.Area == area {
				latest, found = snapshot, true
			}
		default:
			if !found {
				t.Fatalf("synchronous replay omitted area %d", area)
			}
			return latest
		}
	}
}

func bgplsTestRouter(metric types.Metric, sequence types.LSSequenceNumber) packet.LSA {
	return packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeRouter,
		LinkStateID: types.LinkStateID{2, 2, 2, 2}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: sequence},
		Router: &packet.RouterLSA{Flags: packet.RouterFlagB | packet.RouterFlagE, Links: []packet.RouterLink{
			{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{192, 0, 2, 0}, LinkData: [4]byte{255, 255, 255, 0}, Metric: metric},
			{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{3, 3, 3, 3}, LinkData: [4]byte{10, 0, 0, 2}, Metric: metric},
		}},
	}
}

func bgplsTestAttribute(attrs []linkstateevents.TLV, typ uint16) []byte {
	for _, attr := range attrs {
		if attr.Type == typ {
			return attr.Value
		}
	}
	return nil
}

// Mutations exercise the real LSDB callback, not a supplied topology or an SPF
// projection. MaxAge, physical removal, replay and stop must all replace state.
func TestBGPLSNativeV2MutationRemovalReplayStop(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	initial := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Nodes) == 0 })
	lsa := bgplsTestRouter(10, types.InitialSequenceNumber)
	if !e.lsdb.Install(types.BackboneArea, lsa) {
		t.Fatal("install router LSA")
	}
	first := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) == 1 })
	if first.Generation <= initial.Generation || first.Domain.Protocol != linkstateevents.OSPFv2 {
		t.Fatalf("domain/generation: %+v", first)
	}
	if !first.Nodes[0].ID.HasArea || first.Nodes[0].ID.Area != 0 {
		t.Fatal("backbone area descriptor disappeared")
	}
	if !bytes.Equal(bgplsTestAttribute(first.Nodes[0].Attributes, 1024), []byte{0x30}) {
		t.Fatal("ABR/ASBR flags mistranslated")
	}
	if len(first.Links) != 1 || !bytes.Equal(first.Links[0].Remote.RouterID, []byte{3, 3, 3, 3}) || !bytes.Equal(bgplsTestAttribute(first.Links[0].Attributes, 1095), []byte{0, 10}) {
		t.Fatalf("link descriptors/metric: %+v", first.Links)
	}
	if first.Prefixes[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") || first.Prefixes[0].RouteType != 1 {
		t.Fatalf("prefix: %+v", first.Prefixes)
	}
	lsa = bgplsTestRouter(90, types.InitialSequenceNumber+1)
	if !e.lsdb.Install(types.BackboneArea, lsa) {
		t.Fatal("replace router LSA")
	}
	changed := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return len(s.Prefixes) == 1 && bytes.Equal(bgplsTestAttribute(s.Prefixes[0].Attributes, 1155), []byte{0, 0, 0, 90})
	})
	lsa.Header.Age = types.LSAge(types.MaxAge)
	lsa.Header.Sequence++
	if !e.lsdb.Install(types.BackboneArea, lsa) {
		t.Fatal("install MaxAge")
	}
	withdrawn := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > changed.Generation && len(s.Nodes) == 0 && len(s.Links) == 0 && len(s.Prefixes) == 0
	})
	if !e.lsdb.Delete(types.BackboneArea, lsa.Header.Key()) {
		t.Fatal("remove MaxAge LSA")
	}
	removed := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > withdrawn.Generation && len(s.Nodes) == 0
	})
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	replayed := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return s.Generation > removed.Generation && len(s.Nodes) == 0 })
	e.stopBGPLS()
	var stopped linkstateevents.Snapshot
drain:
	for {
		select {
		case stopped = <-events:
		default:
			break drain
		}
	}
	if stopped.Generation <= replayed.Generation || len(stopped.Nodes) != 0 || len(stopped.Links) != 0 || len(stopped.Prefixes) != 0 {
		t.Fatalf("stop did not end in withdrawal: %+v", stopped)
	}
	if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(50, types.InitialSequenceNumber)) {
		t.Fatal("post-stop DB mutation")
	}
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	select {
	case snapshot := <-events:
		t.Fatalf("stopped source published stale state: %+v", snapshot)
	default:
	}
}

func bgplsInstallOpaque(t *testing.T, e *engine, opaque uint8, body []byte) {
	t.Helper()
	lsa := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeOpaqueArea,
		LinkStateID: types.LinkStateID{opaque, 0, 0, 1}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber}, Body: body}
	if !e.lsdb.Install(types.BackboneArea, lsa) {
		t.Fatal("install opaque LSA")
	}
}

// The same unknown value in RI, TE and Extended LSAs must keep its original
// permitted LSA class; TE must not leak into Opaque Link or Opaque Node.
func TestBGPLSNativeV2OpaqueProvenanceAndSRTopology(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(10, types.InitialSequenceNumber)) {
		t.Fatal("install base router")
	}
	unknown := packet.RITLV{Type: 65000, Value: []byte{0xaa, 0xbb, 0xcc}}
	bgplsInstallOpaque(t, e, packet.RIOpaqueType, packet.EncodeRITLVs([]packet.RITLV{
		unknown, {Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(sr.LabelRange{Base: 16000, Size: 8000})},
	}))
	bgplsInstallOpaque(t, e, packet.TEOpaqueType, packet.EncodeRITLVs([]packet.RITLV{{Type: 2, Value: packet.EncodeRITLVs([]packet.RITLV{
		{Type: 1, Value: []byte{1}}, {Type: 2, Value: []byte{3, 3, 3, 3}}, {Type: 3, Value: []byte{10, 0, 0, 2}}, unknown,
	})}}))
	bgplsInstallOpaque(t, e, packet.ExtLinkOpaqueType, packet.EncodeExtLinkLSA(packet.ExtLinkTLV{
		LinkType: 1, LinkID: [4]byte{3, 3, 3, 3}, LinkData: [4]byte{10, 0, 0, 2}, SubTLVs: []packet.ExtSubTLV{{Type: unknown.Type, Value: unknown.Value}},
	}))
	bgplsInstallOpaque(t, e, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AddressPrefix: [4]byte{192, 0, 2, 0},
		SubTLVs: []packet.ExtSubTLV{{Type: unknown.Type, Value: unknown.Value}, {Type: sr.V4TypePrefixSID, Value: sr.EncodePrefixSIDValue(sr.PrefixSID{MTID: 9, Algorithm: 0, Index: 77})}},
	}}}))
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		for _, p := range s.Prefixes {
			if p.Topology == 9 {
				return true
			}
		}
		return false
	})
	if len(snapshot.Nodes) != 1 || len(snapshot.Nodes[0].Opaque) != 1 || snapshot.Nodes[0].Opaque[0].Source != linkstateevents.OSPFRouterInformation {
		t.Fatalf("node provenance: %+v", snapshot.Nodes)
	}
	if !bytes.Equal(snapshot.Nodes[0].Opaque[0].Value, packet.EncodeRITLVs([]packet.RITLV{unknown})) {
		t.Fatal("native RI TLV bytes changed")
	}
	if !bytes.Equal(bgplsTestAttribute(snapshot.Nodes[0].Attributes, 1034), []byte{0, 0, 0, 31, 64, 4, 137, 0, 3, 0, 62, 128}) {
		t.Fatal("SRGB not translated to BGP-LS range encoding")
	}
	if len(snapshot.Links) != 1 || len(snapshot.Links[0].Opaque) != 1 || snapshot.Links[0].Opaque[0].Source != linkstateevents.OSPFv2ExtendedLink {
		t.Fatalf("link provenance: %+v", snapshot.Links)
	}
	var topologyFound, opaqueFound bool
	for _, p := range snapshot.Prefixes {
		if p.Topology == 9 {
			topologyFound = p.RouteType == 1 && bytes.Equal(bgplsTestAttribute(p.Attributes, 1158), []byte{0, 0, 0, 0, 0, 0, 0, 77})
		}
		for _, opaque := range p.Opaque {
			opaqueFound = opaque.Source == linkstateevents.OSPFv2ExtendedPrefix
		}
	}
	if !topologyFound || !opaqueFound {
		t.Fatalf("prefix SID topology/provenance: %+v", snapshot.Prefixes)
	}
}

func bgplsNativeV3(t *testing.T, router v3types.RouterID, typ v3types.LSType, id uint32, body []byte, seq types.LSSequenceNumber, purge bool) packet.LSA {
	t.Helper()
	var lsid v3types.LinkStateID
	binary.BigEndian.PutUint32(lsid[:], id)
	lsa := v3packet.LSA{Header: v3packet.LSAHeader{Type: typ, LinkStateID: lsid,
		AdvertisingRouter: router, Sequence: v3types.LSSequenceNumber(seq)}, Body: body}
	if purge {
		lsa.Header.Age = v3types.LSAge(types.MaxAge)
	}
	raw := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(raw, 0)
	decoded, err := v3packet.DecodeLSA(raw)
	if err != nil {
		t.Fatal(err)
	}
	return packet.LSA{Header: v6LSAHeaderToNeutral(decoded.Header), Body: decoded.Body, RawBytes: decoded.RawBytes}
}

// Every extended prefix LSA has a distinct provenance, including E-NSSA versus
// E-AS-External. AS-scope NLRI omits the area descriptor even in backbone domain.
func TestBGPLSNativeV3ExtendedPrefixProvenance(t *testing.T) {
	cases := []struct {
		typ    v3types.LSType
		tlv    uint16
		route  uint8
		source linkstateevents.Provenance
	}{
		{v3types.LSTypeEIntraAreaPrefix, 6, 1, linkstateevents.OSPFv3ExtendedIntraAreaPrefix},
		{v3types.LSTypeEInterAreaPrefix, 3, 2, linkstateevents.OSPFv3ExtendedInterAreaPrefix},
		{v3types.LSTypeEASExternal, 5, 4, linkstateevents.OSPFv3ExtendedASExternal},
		{v3types.LSTypeEType7, 5, 6, linkstateevents.OSPFv3ExtendedNSSA},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%04x", tc.typ), func(t *testing.T) {
			e, _, events := bgplsTestSource(t, true)
			fixed := []byte{0, 0, 0, 25, 64, 1, 0, 0, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0}
			if tc.route >= 3 {
				fixed[0] = 4
			}
			value := v3packet.AppendSubTLVs(fixed, []v3packet.ExtendedTLV{{Type: 65001, Value: []byte{1, 2, 3, 4}}})
			body := v3packet.EncodeExtendedLSABody(v3packet.ExtendedLSA{TLVs: []v3packet.ExtendedTLV{{Type: tc.tlv, Value: value}}})
			if tc.typ == v3types.LSTypeEIntraAreaPrefix {
				header := make([]byte, 12)
				binary.BigEndian.PutUint16(header[2:4], uint16(v3types.LSTypeERouter))
				copy(header[8:12], []byte{2, 2, 2, 2})
				body = append(header, body...)
			}
			lsa := bgplsNativeV3(t, v3types.RouterID{2, 2, 2, 2}, tc.typ, 1, body, types.InitialSequenceNumber, false)
			if !e.lsdb.Install(types.BackboneArea, lsa) {
				t.Fatal("install extended prefix")
			}
			snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) == 1 })
			p := snapshot.Prefixes[0]
			if p.Prefix != netip.MustParsePrefix("2001:db8::/64") || p.RouteType != tc.route || p.Topology != 0 {
				t.Fatalf("prefix: %+v", p)
			}
			if p.Node.HasArea != (tc.typ != v3types.LSTypeEASExternal) {
				t.Fatal("AS/area scope conflated")
			}
			if len(p.Opaque) != 1 || p.Opaque[0].Source != tc.source {
				t.Fatalf("provenance: %+v", p.Opaque)
			}
			if !bytes.Equal(bgplsTestAttribute(p.Attributes, 1152), []byte{0x40}) {
				t.Fatal("NU flag not translated")
			}
			if !e.lsdb.Delete(types.BackboneArea, lsa.Header.Key()) {
				t.Fatal("delete extended prefix")
			}
			bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
				return s.Generation > snapshot.Generation && len(s.Prefixes) == 0
			})
		})
	}
}

// Link-scope originations and release add/remove E-Link opaque data on the
// surviving area link. Its IPv6 link-local address must never become a descriptor.
func TestBGPLSNativeV3LinkScopeRelease(t *testing.T) {
	e, _, events := bgplsTestSource(t, true)
	router := v3packet.RouterLSA{Links: []v3packet.RouterLink{{Type: 1, Metric: 20, InterfaceID: 7, NeighborInterfaceID: 8, NeighborRouterID: v3types.RouterID{3, 3, 3, 3}}}}
	body := make([]byte, router.EncodedLen())
	router.WriteTo(body, 0)
	if !e.lsdb.Install(types.BackboneArea, bgplsNativeV3(t, v3types.RouterID{2, 2, 2, 2}, v3types.LSTypeRouter, 0, body, types.InitialSequenceNumber, false)) {
		t.Fatal("install v3 router")
	}
	address := netip.MustParseAddr("fe80::2").As16()
	nested := packet.RITLV{Type: 65004, Value: []byte{1, 3, 5, 7}}
	addressValue := append(address[:], packet.EncodeRITLVs([]packet.RITLV{nested})...)
	body = append([]byte{0, 0, 0, 0}, packet.EncodeRITLVs([]packet.RITLV{
		{Type: 7, Value: addressValue}, {Type: 65002, Value: []byte{9, 8, 7, 6}},
	})...)
	key := types.LSAKey{Type: types.LSType(v3types.LSTypeELink), LinkStateID: types.LinkStateID{0, 0, 0, 7}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}}
	if _, ok := e.lsdb.OriginateLinkSelf("test-link", types.BackboneArea, key, body, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return bgplsNativeV3(t, v3types.RouterID{2, 2, 2, 2}, v3types.LSTypeELink, 7, body, seq, purge)
	}); !ok {
		t.Fatal("originate link-scope LSA")
	}
	published := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Links) == 1 && len(s.Links[0].Opaque) == 2 })
	if len(published.Links[0].LocalAddresses) != 0 || !published.Links[0].HasLinkIDs || published.Links[0].LocalID != 7 || published.Links[0].RemoteID != 8 {
		t.Fatalf("native link identity: %+v", published.Links)
	}
	for _, opaque := range published.Links[0].Opaque {
		if opaque.Source != linkstateevents.OSPFv3ExtendedLink {
			t.Fatal("E-Link provenance lost")
		}
	}
	if !bytes.Equal(published.Links[0].Opaque[0].Value, packet.EncodeRITLVs([]packet.RITLV{nested})) {
		t.Fatal("nested E-Link extension lost")
	}
	e.lsdb.ReleaseLink("test-link")
	bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > published.Generation && len(s.Links) == 1 && len(s.Links[0].Opaque) == 0
	})
}

// AS-wide prefixes survive removal of the backbone attachment; retained area
// LSAs must not re-enter the AS bucket merely because it also uses Domain.Area=0.
func TestBGPLSNativeAreaRemovalKeepsOnlyASWide(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(10, types.InitialSequenceNumber)) {
		t.Fatal("install router")
	}
	external := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeASExternal,
		LinkStateID: types.LinkStateID{203, 0, 113, 0}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber},
		External: &packet.ExternalLSA{NetworkMask: [4]byte{255, 255, 255, 0}, ExternalType2: true, Metric: 100}}
	if !e.lsdb.Install(types.BackboneArea, external) {
		t.Fatal("install external")
	}
	before := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) == 2 })
	e.setConfig(ospfConfig{present: true, RouterID: types.RouterID{1, 1, 1, 1}})
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	after := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > before.Generation && len(s.Prefixes) == 1 && len(s.Links) == 0 && len(s.Nodes) == 0
	})
	if after.Prefixes[0].Prefix != netip.MustParsePrefix("203.0.113.0/24") || after.Prefixes[0].RouteType != 4 || after.Prefixes[0].Node.HasArea {
		t.Fatalf("AS replacement: %+v", after)
	}
}

// Network-LSA identity includes both the DR router ID and its LAN address, and
// its reverse links point at real attached routers, not the DR for every edge.
func TestBGPLSNativeV2PseudonodeAndPrefixClasses(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	router := bgplsTestRouter(10, types.InitialSequenceNumber)
	router.Router.Links = append(router.Router.Links, packet.RouterLink{Type: packet.RouterLinkTypeTransit,
		LinkID: types.LinkStateID{10, 1, 0, 1}, LinkData: [4]byte{10, 1, 0, 2}, Metric: 30})
	network := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeNetwork,
		LinkStateID: types.LinkStateID{10, 1, 0, 1}, AdvertisingRouter: types.RouterID{4, 4, 4, 4}, Sequence: types.InitialSequenceNumber},
		Network: &packet.NetworkLSA{NetworkMask: [4]byte{255, 255, 255, 0}, AttachedRouters: []types.RouterID{{2, 2, 2, 2}, {4, 4, 4, 4}}}}
	if !e.lsdb.Install(types.BackboneArea, router) || !e.lsdb.Install(types.BackboneArea, network) {
		t.Fatal("install LAN")
	}
	for i, typ := range []types.LSType{types.LSTypeSummaryNetwork, types.LSTypeASExternal} {
		lsa := packet.LSA{Header: packet.LSAHeader{Type: typ, LinkStateID: types.LinkStateID{198, 51, byte(i), 0},
			AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber}}
		if typ == types.LSTypeSummaryNetwork {
			lsa.Summary = &packet.SummaryLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: 40}
		} else {
			lsa.External = &packet.ExternalLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: 50}
		}
		if !e.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatal("install prefix class")
		}
	}
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) == 4 && len(s.Links) == 4 })
	pseudo := []byte{4, 4, 4, 4, 10, 1, 0, 1}
	var forward, reverse bool
	for index := range snapshot.Links {
		link := &snapshot.Links[index]
		if bytes.Equal(link.Remote.RouterID, pseudo) {
			forward = bytes.Equal(link.Local.RouterID, []byte{2, 2, 2, 2})
		}
		if bytes.Equal(link.Local.RouterID, pseudo) && bytes.Equal(link.Remote.RouterID, []byte{2, 2, 2, 2}) {
			reverse = true
		}
	}
	if !forward || !reverse {
		t.Fatalf("pseudonode directions: %+v", snapshot.Links)
	}
	routes := map[uint8]bool{}
	for _, prefix := range snapshot.Prefixes {
		routes[prefix.RouteType] = true
	}
	if !routes[1] || !routes[2] || !routes[3] {
		t.Fatalf("missing prefix classes: %+v", routes)
	}
}

// Multiple TE Router Address LSAs remain distinct auxiliary IDs and accompany
// both directions of a link, rather than disappearing after the node export.
func TestBGPLSNativeAuxiliaryRouterIDs(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(10, types.InitialSequenceNumber)) {
		t.Fatal("install router")
	}
	for i, router := range []types.RouterID{{2, 2, 2, 2}, {2, 2, 2, 2}, {3, 3, 3, 3}} {
		te := packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{192, 0, 2, byte(10 + i)}}
		lsa := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeOpaqueArea,
			LinkStateID: types.LinkStateID{packet.TEOpaqueType, 0, 0, byte(i + 1)}, AdvertisingRouter: router, Sequence: types.InitialSequenceNumber}, Body: te.Encode()}
		if !e.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatal("install router address")
		}
	}
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		if len(s.Links) != 1 {
			return false
		}
		var local, remote int
		for _, attr := range s.Links[0].Attributes {
			if attr.Type == 1028 {
				local++
			}
			if attr.Type == 1030 && bytes.Equal(attr.Value, []byte{192, 0, 2, 12}) {
				remote++
			}
		}
		return local == 2 && remote == 1
	})
}

// An AS-scoped Extended Prefix has to inspect the original NSSA area database
// to distinguish NSSA type 1 from type 2, not invent type 1 or omit route type.
func TestBGPLSNativeNSSAExtendedPrefixRouteType(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	nssa := types.AreaID{0, 0, 0, 1}
	e.setConfig(ospfConfig{present: true, RouterID: types.RouterID{1, 1, 1, 1}, Areas: []areaConfig{
		{AreaID: types.BackboneArea}, {AreaID: nssa, AreaType: types.AreaTypeNSSA},
	}})
	external := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeNSSA,
		LinkStateID: types.LinkStateID{203, 0, 113, 0}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber},
		External: &packet.ExternalLSA{NetworkMask: [4]byte{255, 255, 255, 0}, ExternalType2: true, Metric: 100,
			ForwardingAddr: [4]byte{10, 1, 0, 2}}}
	if !e.lsdb.Install(nssa, external) {
		t.Fatal("install NSSA LSA")
	}
	extended := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeOpaqueAS,
		LinkStateID: types.LinkStateID{packet.ExtPrefixOpaqueType, 0, 0, 1}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber},
		Body: packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
			RouteType: packet.ExtRouteTypeNSSAExternal, PrefixLength: 24, AddressPrefix: [4]byte{203, 0, 113, 0},
			SubTLVs: []packet.ExtSubTLV{{Type: 65003, Value: []byte{1, 2, 3, 4}}},
		}}})}
	if !e.lsdb.Install(types.BackboneArea, extended) {
		t.Fatal("install extended NSSA prefix")
	}
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Domain.Area == 0 && len(s.Prefixes) == 1 && len(s.Prefixes[0].Opaque) == 1
	})
	if snapshot.Prefixes[0].RouteType != 6 || snapshot.Prefixes[0].Node.HasArea {
		t.Fatalf("extended NSSA source identity/type: %+v", snapshot.Prefixes[0])
	}
}

// An IPv6-only remote ASBR advertisement cannot invent a four-octet OSPF
// identity. Another live native TE LSA can supply the same router's two IDs;
// withdrawing that evidence must withdraw the correlated link as well.
func TestBGPLSNativeInterASIPv6IdentityCorrelation(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	initial := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return s.Domain.Area == 0 })
	remote := netip.MustParseAddr("2001:db8::9").As16()
	link := packet.TELink{HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
		HasRemoteAS: true, RemoteAS: 65100, HasRemoteASBRv6: true, RemoteASBRv6: remote,
		LocalIPs: [][4]byte{{10, 1, 0, 2}}, RemoteIPs: [][4]byte{{10, 1, 0, 9}}}
	unknown := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeOpaqueArea,
		LinkStateID: types.LinkStateID{packet.InterAsTEOpaqueType, 0, 0, 1}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber},
		Body: packet.TELSA{IsLink: true, Link: link}.Encode()}
	if !e.lsdb.Install(types.BackboneArea, unknown) {
		t.Fatal("install IPv6-only ASBR")
	}
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	unidentified := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return s.Generation > initial.Generation })
	if len(unidentified.Links) != 0 {
		t.Fatalf("invented remote identity: %+v", unidentified.Links)
	}
	link.HasRemoteASBRv4 = true
	link.RemoteASBRv4 = [4]byte{9, 9, 9, 9}
	link.LocalIPs = [][4]byte{{10, 2, 0, 4}}
	witness := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeOpaqueArea,
		LinkStateID: types.LinkStateID{packet.InterAsTEOpaqueType, 0, 0, 2}, AdvertisingRouter: types.RouterID{4, 4, 4, 4}, Sequence: types.InitialSequenceNumber},
		Body: packet.TELSA{IsLink: true, Link: link}.Encode()}
	if !e.lsdb.Install(types.BackboneArea, witness) {
		t.Fatal("install ID correlation")
	}
	correlated := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Links) == 2 })
	for _, exported := range correlated.Links {
		if !bytes.Equal(exported.Remote.RouterID, []byte{9, 9, 9, 9}) || exported.Remote.ASN != 65100 ||
			!bytes.Equal(bgplsTestAttribute(exported.Attributes, 1031), remote[:]) {
			t.Fatalf("native ID correlation: %+v", exported)
		}
	}
	link.RemoteASBRv4 = [4]byte{8, 8, 8, 8}
	link.LocalIPs = [][4]byte{{10, 3, 0, 5}}
	conflict := witness
	conflict.Header.AdvertisingRouter = types.RouterID{5, 5, 5, 5}
	conflict.Body = packet.TELSA{IsLink: true, Link: link}.Encode()
	if !e.lsdb.Install(types.BackboneArea, conflict) {
		t.Fatal("install conflicting ID evidence")
	}
	ambiguous := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		if s.Generation <= correlated.Generation || len(s.Links) != 2 {
			return false
		}
		for _, exported := range s.Links {
			if bytes.Equal(exported.Local.RouterID, []byte{2, 2, 2, 2}) {
				return false
			}
		}
		return true
	})
	if !e.lsdb.Delete(types.BackboneArea, conflict.Header.Key()) {
		t.Fatal("remove conflicting ID evidence")
	}
	restored := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		if s.Generation <= ambiguous.Generation || len(s.Links) != 2 {
			return false
		}
		for _, exported := range s.Links {
			if bytes.Equal(exported.Local.RouterID, []byte{2, 2, 2, 2}) {
				return true
			}
		}
		return false
	})
	if !e.lsdb.Delete(types.BackboneArea, witness.Header.Key()) {
		t.Fatal("withdraw ID correlation")
	}
	bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return s.Generation > restored.Generation && len(s.Links) == 0 })
}

func bgplsReachabilityRouter(router types.RouterID, links ...packet.RouterLink) packet.LSA {
	const sequence = types.InitialSequenceNumber
	return packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router),
		AdvertisingRouter: router, Sequence: sequence}, Router: &packet.RouterLSA{Links: links}}
}

func bgplsOriginUnreachable(snapshot *linkstateevents.Snapshot, id linkstateevents.NodeID) bool {
	for _, unreachable := range snapshot.Unreachable {
		if unreachable.HasArea == id.HasArea && unreachable.Area == id.Area &&
			bytes.Equal(unreachable.RouterID, id.RouterID) {
			return true
		}
	}
	return false
}

func bgplsLocalLinkCount(snapshot *linkstateevents.Snapshot, router []byte) int {
	count := 0
	for i := range snapshot.Links {
		if bytes.Equal(snapshot.Links[i].Local.RouterID, router) {
			count++
		}
	}
	return count
}

// RFC 9552 Section 5.9: stale live LSAs remain in the native database across
// a partition, but the BGP withdrawal identities follow actual completed SPF.
// The transit router has no stub prefix; another area's copy of the same router
// is disconnected throughout. Neither prefix ownership nor global router-ID
// membership is therefore an adequate reachability substitute.
// RFC requirement: RFC9552-5.9-1 positive -- completed native SPF restores source eligibility for unchanged live Router/Network-LSA origins.
// this asserts source snapshots, not collector BGP wire advertisements.
// RFC requirement: RFC9552-5.9-1 negative -- pending-SPF invalidation and replay preserve prior source withdrawals until completed native recovery.
func TestBGPLSNativeSPFPartitionRestoresStaleOrigins(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	otherArea := types.AreaID{0, 0, 0, 1}
	e.setConfig(ospfConfig{present: true, RouterID: types.RouterID{1, 1, 1, 1},
		Areas:  []areaConfig{{AreaID: types.BackboneArea}, {AreaID: otherArea}},
		Timers: timerConfig{SPFDelayMS: 3600000, SPFHoldMS: 3600000, SPFMaxHoldMS: 3600000}})
	rootLink := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{2, 2, 2, 2},
		LinkData: [4]byte{10, 0, 0, 1}, Metric: 10}
	root := bgplsReachabilityRouter(types.RouterID{1, 1, 1, 1}, rootLink)
	transit := bgplsReachabilityRouter(types.RouterID{2, 2, 2, 2},
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{1, 1, 1, 1}, LinkData: [4]byte{10, 0, 0, 2}, Metric: 10},
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{3, 3, 3, 3}, LinkData: [4]byte{10, 0, 1, 2}, Metric: 10})
	remote := bgplsReachabilityRouter(types.RouterID{3, 3, 3, 3},
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{2, 2, 2, 2}, LinkData: [4]byte{10, 0, 1, 3}, Metric: 10},
		packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{192, 0, 2, 0}, LinkData: [4]byte{255, 255, 255, 0}, Metric: 20},
		packet.RouterLink{Type: packet.RouterLinkTypeTransit, LinkID: types.LinkStateID{10, 3, 0, 3}, LinkData: [4]byte{10, 3, 0, 3}, Metric: 5})
	network := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID{10, 3, 0, 3},
		AdvertisingRouter: types.RouterID{3, 3, 3, 3}, Sequence: types.InitialSequenceNumber},
		Network: &packet.NetworkLSA{NetworkMask: [4]byte{255, 255, 255, 0}, AttachedRouters: []types.RouterID{{3, 3, 3, 3}}}}
	for _, lsa := range []packet.LSA{root, transit, remote, network} {
		if !e.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatal("install native topology")
		}
	}
	if !e.lsdb.Install(otherArea, bgplsReachabilityRouter(types.RouterID{1, 1, 1, 1})) ||
		!e.lsdb.Install(otherArea, bgplsReachabilityRouter(types.RouterID{3, 3, 3, 3},
			packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{198, 51, 100, 0}, LinkData: [4]byte{255, 255, 255, 0}, Metric: 20})) {
		t.Fatal("install isolated area")
	}
	e.spf.Run()
	isolated := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Domain.Area == 1 && len(s.Unreachable) == 1
	})
	if !isolated.Unreachable[0].HasArea || isolated.Unreachable[0].Area != 1 ||
		!bytes.Equal(isolated.Unreachable[0].RouterID, []byte{3, 3, 3, 3}) {
		t.Fatalf("wrong area identity: %+v", isolated.Unreachable)
	}
	connected := bgplsReplaySnapshot(t, bus, events)
	if connected.Generation <= isolated.Generation || len(connected.Unreachable) != 0 || len(connected.Nodes) != 4 || len(connected.Links) != 6 || len(connected.Prefixes) != 2 {
		t.Fatalf("connected native SPF view: %+v", connected)
	}
	root.Router.Links = nil
	root.Header.Sequence++
	if !e.lsdb.Install(types.BackboneArea, root) {
		t.Fatal("disconnect root")
	}
	beforeSPF := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Domain.Area == 0 && s.Generation > connected.Generation && bgplsLocalLinkCount(s, []byte{1, 1, 1, 1}) == 0
	})
	e.spf.Run()
	disconnected := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Domain.Area == 0 && s.Generation > beforeSPF.Generation && len(s.Unreachable) == 3
	})
	if len(disconnected.Nodes) != 4 || len(disconnected.Links) != 5 || len(disconnected.Prefixes) != 2 {
		t.Fatalf("SPF erased complete LSDB objects: %+v", disconnected)
	}
	for _, router := range [][]byte{{2, 2, 2, 2}, {3, 3, 3, 3}, {3, 3, 3, 3, 10, 3, 0, 3}} {
		if !bgplsOriginUnreachable(&disconnected, linkstateevents.NodeID{RouterID: router, HasArea: true}) {
			t.Fatalf("missing unreachable origin %v: %+v", router, disconnected.Unreachable)
		}
	}
	for _, prefix := range disconnected.Prefixes {
		if !bgplsOriginUnreachable(&disconnected, prefix.Node) {
			t.Fatalf("prefix lost origin provenance: %+v", prefix)
		}
	}
	// Invalidate the native result with a real area configuration change, but
	// do not complete another SPF yet. Replay must not falsely restore nodes.
	e.setConfig(ospfConfig{present: true, RouterID: types.RouterID{1, 1, 1, 1},
		Areas:  []areaConfig{{AreaID: types.BackboneArea}, {AreaID: otherArea}, {AreaID: types.AreaID{0, 0, 0, 2}}},
		Timers: timerConfig{SPFDelayMS: 3600000, SPFHoldMS: 3600000, SPFMaxHoldMS: 3600000}})
	pending := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Domain.Area == 0 && s.Generation > disconnected.Generation
	})
	if len(pending.Unreachable) != 3 {
		t.Fatalf("pending SPF falsely restored origins: %+v", pending.Unreachable)
	}
	replayed := bgplsReplaySnapshot(t, bus, events)
	if replayed.Generation <= pending.Generation || len(replayed.Unreachable) != 3 {
		t.Fatalf("unready SPF replay restored origins: %+v", replayed.Unreachable)
	}
	root.Router.Links = []packet.RouterLink{rootLink}
	root.Header.Sequence++
	if !e.lsdb.Install(types.BackboneArea, root) {
		t.Fatal("reconnect root")
	}
	beforeRestore := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Domain.Area == 0 && s.Generation > replayed.Generation && bgplsLocalLinkCount(s, []byte{1, 1, 1, 1}) == 1
	})
	e.spf.Run()
	restored := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Domain.Area == 0 && s.Generation > beforeRestore.Generation && len(s.Unreachable) == 0
	})
	if len(restored.Links) != 6 || len(restored.Prefixes) != 2 {
		t.Fatalf("incomplete restoration: %+v", restored)
	}
	stale, ok := e.lsdb.LookupLSA(types.BackboneArea, remote.Header.Key())
	if !ok || stale.Header.Sequence != types.InitialSequenceNumber || stale.Header.Age.IsMaxAge() {
		t.Fatalf("restoration depended on changing the stale remote LSA: %+v", stale.Header)
	}
}

// AS-wide origins can be reachable solely through a native Type-4 ASBR path.
// There is deliberately no Router-LSA for 9.9.9.9 in the attached area.
// RFC requirement: RFC9552-5.9-1 positive -- native inter-area ASBR recovery restores unchanged AS-External origin eligibility without a local Router-LSA.
// This asserts source snapshots; collector wire behavior is tested separately.
func TestBGPLSNativeSPFInterAreaASBROrigin(t *testing.T) {
	e, _, events := bgplsTestSource(t, false)
	rootLink := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{2, 2, 2, 2}, LinkData: [4]byte{10, 0, 0, 1}, Metric: 10}
	root := bgplsReachabilityRouter(types.RouterID{1, 1, 1, 1}, rootLink)
	abr := bgplsReachabilityRouter(types.RouterID{2, 2, 2, 2},
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{1, 1, 1, 1}, LinkData: [4]byte{10, 0, 0, 2}, Metric: 10})
	abr.Router.Flags = packet.RouterFlagB
	summary := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeSummaryASBR, LinkStateID: types.LinkStateID{9, 9, 9, 9},
		AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber}, Summary: &packet.SummaryLSA{Metric: 30}}
	external := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID{203, 0, 113, 0},
		AdvertisingRouter: types.RouterID{9, 9, 9, 9}, Sequence: types.InitialSequenceNumber},
		External: &packet.ExternalLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: 20}}
	for _, lsa := range []packet.LSA{root, abr, summary, external} {
		if !e.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatal("install inter-area ASBR topology")
		}
	}
	// Observe the mutation before SPF so the next publication must come from
	// completion, not a delayed LSDB notification for the same change.
	before := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) == 1 })
	e.spf.Run()
	connected := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > before.Generation && len(s.Prefixes) == 1 && len(s.Unreachable) == 0
	})
	origin := connected.Prefixes[0].Node
	if origin.HasArea || !bytes.Equal(origin.RouterID, []byte{9, 9, 9, 9}) {
		t.Fatalf("ASBR origin: %+v", origin)
	}
	root.Router.Links = nil
	root.Header.Sequence++
	if !e.lsdb.Install(types.BackboneArea, root) {
		t.Fatal("disconnect ABR path")
	}
	beforeSPF := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > connected.Generation && bgplsLocalLinkCount(s, []byte{1, 1, 1, 1}) == 0
	})
	e.spf.Run()
	disconnected := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > beforeSPF.Generation && bgplsOriginUnreachable(s, origin)
	})
	if len(disconnected.Prefixes) != 1 || disconnected.Prefixes[0].Prefix != netip.MustParsePrefix("203.0.113.0/24") {
		t.Fatalf("external LSA provenance lost on ASBR withdrawal: %+v", disconnected.Prefixes)
	}
	root.Router.Links = []packet.RouterLink{rootLink}
	root.Header.Sequence++
	if !e.lsdb.Install(types.BackboneArea, root) {
		t.Fatal("restore ABR path")
	}
	beforeRestore := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > disconnected.Generation && bgplsLocalLinkCount(s, []byte{1, 1, 1, 1}) == 1
	})
	e.spf.Run()
	bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > beforeRestore.Generation && len(s.Prefixes) == 1 && !bgplsOriginUnreachable(s, origin)
	})
}

// The v3 SPF graph uses synthetic 32-bit network handles. Reachability must
// recover the real (DR Router-ID, DR Interface-ID), not export that handle or
// join different LANs solely by interface number.
// RFC requirement: RFC9552-5.9-1 positive -- completed OSPFv3 SPF restores the real DR/Interface-ID pseudonode and unchanged prefix origin eligibility.
// This asserts native snapshots, not synthetic handles or collector wire output.
func TestBGPLSNativeV3SPFPseudonodeRestoration(t *testing.T) {
	e, _, events := bgplsTestSource(t, true)
	local := v3types.RouterID{1, 1, 1, 1}
	dr := v3types.RouterID{2, 2, 2, 2}
	root := v3packet.RouterLSA{Options: v3types.OptR | v3types.OptV6}
	rootBody := make([]byte, root.EncodedLen())
	root.WriteTo(rootBody, 0)
	remote := v3packet.RouterLSA{Options: v3types.OptR | v3types.OptV6, Links: []v3packet.RouterLink{{
		Type: v3packet.RouterLinkTypeTransit, Metric: 10, InterfaceID: 99, NeighborInterfaceID: 99, NeighborRouterID: dr,
	}}}
	remoteBody := make([]byte, remote.EncodedLen())
	remote.WriteTo(remoteBody, 0)
	network := v3packet.NetworkLSA{Options: v3types.OptR | v3types.OptV6, AttachedRouters: []v3types.RouterID{local, dr}}
	networkBody := make([]byte, network.EncodedLen())
	network.WriteTo(networkBody, 0)
	prefix := v3packet.IntraAreaPrefixLSA{ReferencedLSType: v3types.LSTypeNetwork,
		ReferencedLinkStateID: v3types.LinkStateID{0, 0, 0, 99}, ReferencedAdvRouter: dr,
		Prefixes: []v3packet.Prefix{{Length: 64, Field16: 5, Address: []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0}}}}
	prefixBody := make([]byte, prefix.EncodedLen())
	prefix.WriteTo(prefixBody, 0)
	for _, lsa := range []packet.LSA{
		bgplsNativeV3(t, local, v3types.LSTypeRouter, 0, rootBody, types.InitialSequenceNumber, false),
		bgplsNativeV3(t, dr, v3types.LSTypeRouter, 0, remoteBody, types.InitialSequenceNumber, false),
		bgplsNativeV3(t, dr, v3types.LSTypeNetwork, 99, networkBody, types.InitialSequenceNumber, false),
		bgplsNativeV3(t, dr, v3types.LSTypeIntraAreaPrefix, 1, prefixBody, types.InitialSequenceNumber, false),
	} {
		if !e.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatal("install native v3 topology")
		}
	}
	before := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) == 1 })
	e.spf.Run()
	disconnected := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > before.Generation && len(s.Unreachable) == 2
	})
	pseudo := linkstateevents.NodeID{RouterID: []byte{2, 2, 2, 2, 0, 0, 0, 99}, HasArea: true}
	if !bgplsOriginUnreachable(&disconnected, pseudo) || !bytes.Equal(disconnected.Prefixes[0].Node.RouterID, pseudo.RouterID) {
		t.Fatalf("v3 network/prefix identity: %+v", disconnected)
	}
	root.Links = []v3packet.RouterLink{{Type: v3packet.RouterLinkTypeTransit, Metric: 10,
		InterfaceID: 7, NeighborInterfaceID: 99, NeighborRouterID: dr}}
	rootBody = make([]byte, root.EncodedLen())
	root.WriteTo(rootBody, 0)
	if !e.lsdb.Install(types.BackboneArea, bgplsNativeV3(t, local, v3types.LSTypeRouter, 0, rootBody, types.InitialSequenceNumber+1, false)) {
		t.Fatal("restore native v3 LAN attachment")
	}
	beforeRestore := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > disconnected.Generation && bgplsLocalLinkCount(s, local[:]) == 1
	})
	e.spf.Run()
	restored := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > beforeRestore.Generation && len(s.Unreachable) == 0
	})
	if len(restored.Nodes) != 3 || len(restored.Links) != 4 || len(restored.Prefixes) != 1 {
		t.Fatalf("v3 native objects not restored: %+v", restored)
	}
}

// LSAs received after the completed SPF are unknown until the native next run
// observes them. This also applies to an external origin with no Router-LSA.
// RFC requirement: RFC9552-5.9-1 negative -- new unknown origins are not declared unreachable and prior source withdrawals survive ready-but-unknown SPF results.
func TestBGPLSNativeSPFUnknownOriginsWaitForComputation(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	root := bgplsReachabilityRouter(types.RouterID{1, 1, 1, 1})
	if !e.lsdb.Install(types.BackboneArea, root) {
		t.Fatal("install local router")
	}
	before := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Nodes) == 1 })
	e.spf.Run()
	computed := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return s.Generation > before.Generation })
	remote := bgplsReachabilityRouter(types.RouterID{2, 2, 2, 2})
	external := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeASExternal,
		LinkStateID: types.LinkStateID{203, 0, 113, 0}, AdvertisingRouter: types.RouterID{9, 9, 9, 9}, Sequence: types.InitialSequenceNumber},
		External: &packet.ExternalLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: 20}}
	if !e.lsdb.Install(types.BackboneArea, remote) || !e.lsdb.Install(types.BackboneArea, external) {
		t.Fatal("install new native origins")
	}
	unknown := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > computed.Generation && len(s.Nodes) == 2 && len(s.Prefixes) == 1
	})
	if len(unknown.Unreachable) != 0 {
		t.Fatalf("new origins were invented unreachable: %+v", unknown.Unreachable)
	}
	e.spf.Run()
	classified := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > unknown.Generation && len(s.Unreachable) == 2
	})
	if !bgplsOriginUnreachable(&classified, linkstateevents.NodeID{RouterID: []byte{2, 2, 2, 2}, HasArea: true}) ||
		!bgplsOriginUnreachable(&classified, linkstateevents.NodeID{RouterID: []byte{9, 9, 9, 9}}) {
		t.Fatalf("native area/AS scope evidence lost: %+v", classified.Unreachable)
	}
	// A new pseudonode is not yet in the completed graph, but its DR already
	// has a definite native failure. That evidence applies immediately.
	network := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeNetwork,
		LinkStateID: types.LinkStateID{10, 2, 0, 2}, AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber},
		Network: &packet.NetworkLSA{NetworkMask: [4]byte{255, 255, 255, 0}, AttachedRouters: []types.RouterID{{2, 2, 2, 2}}}}
	if !e.lsdb.Install(types.BackboneArea, network) {
		t.Fatal("install new pseudonode of failed DR")
	}
	withPseudo := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > classified.Generation && len(s.Nodes) == 3 && len(s.Prefixes) == 2
	})
	if !bgplsOriginUnreachable(&withPseudo, linkstateevents.NodeID{RouterID: []byte{2, 2, 2, 2, 10, 2, 0, 2}, HasArea: true}) {
		t.Fatalf("known DR failure lost for new pseudonode: %+v", withPseudo.Unreachable)
	}
	// Keep the same failed router represented only by a native link-scope
	// RI LSA. It is outside the area SPF input after its Router/Network LSAs
	// disappear, so a completed, ready tree now regards the origin as unknown.
	riKey := types.LSAKey{Type: types.LSTypeOpaqueLink, LinkStateID: types.LinkStateID{packet.RIOpaqueType, 0, 0, 1},
		AdvertisingRouter: types.RouterID{2, 2, 2, 2}}
	riBody := packet.EncodeRITLVs([]packet.RITLV{{Type: 65005, Value: []byte{1, 2, 3, 4}}})
	if _, ok := e.lsdb.OriginateLinkSelf("retained-ri", types.BackboneArea, riKey, riBody, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		header := packet.LSAHeader{Type: riKey.Type, LinkStateID: riKey.LinkStateID, AdvertisingRouter: riKey.AdvertisingRouter, Sequence: seq}
		if purge {
			header.Age = types.LSAge(types.MaxAge)
		}
		return packet.LSA{Header: header, Body: riBody}
	}); !ok {
		t.Fatal("install retained link-scope RI")
	}
	if !e.lsdb.Delete(types.BackboneArea, remote.Header.Key()) || !e.lsdb.Delete(types.BackboneArea, network.Header.Key()) {
		t.Fatal("remove area evidence")
	}
	beforeUnknown := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		if s.Generation <= withPseudo.Generation || len(s.Nodes) != 2 || len(s.Prefixes) != 1 {
			return false
		}
		for _, node := range s.Nodes {
			if bytes.Equal(node.ID.RouterID, []byte{2, 2, 2, 2}) && len(node.Opaque) == 1 {
				return true
			}
		}
		return false
	})
	e.spf.Run()
	stillWithdrawn := bgplsReplaySnapshot(t, bus, events)
	if stillWithdrawn.Generation <= beforeUnknown.Generation || !bgplsOriginUnreachable(&stillWithdrawn, linkstateevents.NodeID{RouterID: []byte{2, 2, 2, 2}, HasArea: true}) {
		t.Fatalf("ready but unknown origin falsely recovered: %+v", stillWithdrawn.Unreachable)
	}
}

// After a DR change, the old Network-LSA can remain live even though native
// SPF uses another DR's LSA for the same LAN address. Both router vertices are
// reachable; only the old eight-octet pseudonode identity must be suppressed.
func TestBGPLSNativeSPFOldDRPseudonodeIdentity(t *testing.T) {
	e, _, events := bgplsTestSource(t, false)
	root := bgplsReachabilityRouter(types.RouterID{1, 1, 1, 1},
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{3, 3, 3, 3}, LinkData: [4]byte{10, 0, 3, 1}, Metric: 10},
		packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{4, 4, 4, 4}, LinkData: [4]byte{10, 0, 4, 1}, Metric: 10})
	if !e.lsdb.Install(types.BackboneArea, root) {
		t.Fatal("install root")
	}
	lan := types.LinkStateID{10, 3, 0, 3}
	for _, octet := range []byte{3, 4} {
		router := types.RouterID{octet, octet, octet, octet}
		lsa := bgplsReachabilityRouter(router,
			packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{1, 1, 1, 1}, LinkData: [4]byte{10, 0, octet, octet}, Metric: 10},
			packet.RouterLink{Type: packet.RouterLinkTypeTransit, LinkID: lan, LinkData: [4]byte(lan), Metric: 5})
		network := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeNetwork, LinkStateID: lan,
			AdvertisingRouter: router, Sequence: types.InitialSequenceNumber},
			Network: &packet.NetworkLSA{NetworkMask: [4]byte{255, 255, 255, 0}, AttachedRouters: []types.RouterID{router}}}
		if !e.lsdb.Install(types.BackboneArea, lsa) || !e.lsdb.Install(types.BackboneArea, network) {
			t.Fatal("install DR state")
		}
	}
	before := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Nodes) == 5 && len(s.Prefixes) == 2 })
	e.spf.Run()
	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > before.Generation && len(s.Unreachable) == 1
	})
	if !bytes.Equal(snapshot.Unreachable[0].RouterID, []byte{3, 3, 3, 3, 10, 3, 0, 3}) ||
		!snapshot.Unreachable[0].HasArea || snapshot.Unreachable[0].Area != 0 {
		t.Fatalf("native DR identity collapsed to a LAN address: %+v", snapshot.Unreachable)
	}
	if len(snapshot.Nodes) != 5 || len(snapshot.Prefixes) != 2 {
		t.Fatal("old DR provenance removed from full LSDB snapshot")
	}
}

func TestBGPLSPseudonodeNativeEvidence(t *testing.T) {
	for _, tc := range []struct {
		name                                                     string
		routerKnown, routerReached, networkKnown, networkReached bool
		known, reachable                                         bool
	}{
		{"unknown DR with reached network", false, false, true, true, false, false},
		{"failed DR before network observed", true, false, false, false, true, false},
		{"failed network with unknown DR", false, false, true, false, true, false},
		{"both native vertices reached", true, true, true, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			known, reached := bgplsPseudonodeReachability(tc.routerKnown, tc.routerReached, tc.networkKnown, tc.networkReached)
			if known != tc.known || reached != tc.reachable {
				t.Fatalf("combined evidence = known:%v reached:%v; want known:%v reached:%v", known, reached, tc.known, tc.reachable)
			}
		})
	}
}

// Consumer generation tombstones outlive an engine. Replacing the engine for
// the same native domain must advance beyond its old stop, not restart at one.
func TestBGPLSNativeRestartAdvancesGeneration(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(10, types.InitialSequenceNumber)) {
		t.Fatal("install old engine state")
	}
	old := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) == 1 })
	e.stopBGPLS()
	tombstone := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return s.Generation > old.Generation && len(s.Nodes) == 0 && len(s.Links) == 0 && len(s.Prefixes) == 0
	})
	next := newEngineWithCodecAF(nil, v4Codec{}, afIPv4Unicast)
	next.setConfig(ospfConfig{present: true, RouterID: types.RouterID{1, 1, 1, 1},
		Areas:  []areaConfig{{AreaID: types.BackboneArea}},
		Timers: timerConfig{SPFDelayMS: 3600000, SPFHoldMS: 3600000, SPFMaxHoldMS: 3600000}})
	t.Cleanup(next.shutdown)
	next.startBGPLS(bus)
	started := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Nodes) == 0 && len(s.Prefixes) == 0 })
	if started.Domain != tombstone.Domain || started.Generation <= tombstone.Generation {
		t.Fatalf("restarted domain did not advance past stop: old=%+v new=%+v", tombstone, started)
	}
	if !next.lsdb.Install(types.BackboneArea, bgplsTestRouter(42, types.InitialSequenceNumber)) {
		t.Fatal("install restarted state")
	}
	restored := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) == 1 })
	replayed := bgplsReplaySnapshot(t, bus, events)
	if restored.Generation <= started.Generation || replayed.Generation <= restored.Generation || replayed.Domain != old.Domain || len(replayed.Prefixes) != 1 ||
		!bytes.Equal(bgplsTestAttribute(replayed.Prefixes[0].Attributes, 1155), []byte{0, 0, 0, 42}) {
		t.Fatalf("new-engine replay lost native state/order: start=%d restored=%d replay=%+v", started.Generation, restored.Generation, replayed)
	}
}
